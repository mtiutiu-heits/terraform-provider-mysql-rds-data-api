// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"regexp"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rdsdata"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource                = &MysqlUserResource{}
	_ resource.ResourceWithImportState = &MysqlUserResource{}
)

func NewMysqlUserResource() resource.Resource {
	return &MysqlUserResource{}
}

// MysqlUserResource defines the resource implementation.
type MysqlUserResource struct {
	client *rdsdata.Client
}

// MysqlUserResourceModel describes the resource data model.
type MysqlUserResourceModel struct {
	User                types.String `tfsdk:"user"`
	Password            types.String `tfsdk:"password"`
	Host                types.String `tfsdk:"host"`
	DatabaseResourceArn types.String `tfsdk:"database_resource_arn"`
	DatabaseSecretArn   types.String `tfsdk:"database_secret_arn"`
}

func (r *MysqlUserResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_mysql_user"
}

func (r *MysqlUserResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "AWS RDS Data MySQL user resource",

		Attributes: map[string]schema.Attribute{
			"user": schema.StringAttribute{
				MarkdownDescription: "The MySQL user name to create",
				Required:            true,
				Validators: []validator.String{
					// user value cannot be empty
					stringvalidator.LengthAtLeast(1),
					// protect against destroying system accounts
					stringvalidator.NoneOf([]string{"rdsadmin", "mysql.sys"}...),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"password": schema.StringAttribute{
				MarkdownDescription: "The MySQL password to set for the user (must be at least 16 characters long)",
				Required:            true,
				Sensitive:           true,
				Validators: []validator.String{
					// password must be at least 16 characters long
					stringvalidator.LengthAtLeast(16),
				},
			},
			"host": schema.StringAttribute{
				MarkdownDescription: "The MySQL user host value",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^(([a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9\-]*[a-zA-Z0-9])\.)*([A-Za-z0-9]|[A-Za-z0-9][A-Za-z0-9\-]*[A-Za-z0-9]|%)$`),
						"must contain a valid hostname value",
					),
				},
			},
			"database_resource_arn": schema.StringAttribute{
				MarkdownDescription: "The RDS database resource ARN to run SQL queries against",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^arn:aws:rds:.*\w-.*\w-.*\d:.*\d:cluster:.*\w|[-,_]$`),
						"must contain a valid ARN resource value",
					),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"database_secret_arn": schema.StringAttribute{
				MarkdownDescription: "The RDS database secret ARN to use for authentication",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^arn:aws:secretsmanager:.*\w-.*\w-.*\d:.*\d:secret:.*\w|[-,_]$`),
						"must contain a valid ARN resource value",
					),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *MysqlUserResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*rdsdata.Client)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *rdsdata.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = client
}

func (r *MysqlUserResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan MysqlUserResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// ======================= Resource CREATE Logic =======================

	createUserSqlQuery := fmt.Sprintf(
		"CREATE USER IF NOT EXISTS '%s'@'%s' IDENTIFIED BY '%s'",
		plan.User.ValueString(),
		plan.Host.ValueString(),
		plan.Password.ValueString(),
	)

	createUserStatementOpts := rdsdata.ExecuteStatementInput{
		ResourceArn: aws.String(plan.DatabaseResourceArn.ValueString()),
		SecretArn:   aws.String(plan.DatabaseSecretArn.ValueString()),
		Sql:         &createUserSqlQuery,
	}

	_, createUserSqlQueryErr := r.client.ExecuteStatement(ctx, &createUserStatementOpts)

	if createUserSqlQueryErr != nil {
		resp.Diagnostics.AddError("RDS data service client error", createUserSqlQueryErr.Error())
		return
	}

	tflog.Trace(ctx, "created a MySQL user resource")

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *MysqlUserResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state MysqlUserResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// ======================= Resource READ Logic =======================

	userSqlQuery := fmt.Sprintf(
		"SELECT user,host FROM mysql.user WHERE user='%s' AND host='%s'",
		state.User.ValueString(),
		state.Host.ValueString(),
	)
	userQueryStatementOpts := rdsdata.ExecuteStatementInput{
		ResourceArn: aws.String(state.DatabaseResourceArn.ValueString()),
		SecretArn:   aws.String(state.DatabaseSecretArn.ValueString()),
		Sql:         &userSqlQuery,
	}

	userSqlQueryResult, userSqlQueryErr := r.client.ExecuteStatement(ctx, &userQueryStatementOpts)

	if userSqlQueryErr != nil {
		resp.Diagnostics.AddError("RDS data service client error", userSqlQueryErr.Error())
		return
	}

	if len(userSqlQueryResult.Records) == 0 {
		tflog.Trace(ctx, "MySQL returned no user records")
		// Force resource recreation if data was deleted outside terraform
		state.User = types.StringValue("")
		state.Host = types.StringValue("")
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *MysqlUserResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan MysqlUserResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// ======================= Resource UPDATE Logic =======================

	updateUserSqlQuery := fmt.Sprintf(
		"ALTER USER '%s'@'%s' IDENTIFIED BY '%s'",
		plan.User.ValueString(),
		plan.Host.ValueString(),
		plan.Password.ValueString(),
	)

	updateUserStatementOpts := rdsdata.ExecuteStatementInput{
		ResourceArn: aws.String(plan.DatabaseResourceArn.ValueString()),
		SecretArn:   aws.String(plan.DatabaseSecretArn.ValueString()),
		Sql:         &updateUserSqlQuery,
	}

	_, updateUserSqlQueryErr := r.client.ExecuteStatement(ctx, &updateUserStatementOpts)

	if updateUserSqlQueryErr != nil {
		resp.Diagnostics.AddError("RDS data service client error", updateUserSqlQueryErr.Error())
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *MysqlUserResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state MysqlUserResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// ======================= Resource DELETE Logic =======================

	deleteUserSqlQuery := fmt.Sprintf(
		"DROP USER IF EXISTS '%s'@'%s'",
		state.User.ValueString(),
		state.Host.ValueString(),
	)

	deleteUserStatementOpts := rdsdata.ExecuteStatementInput{
		ResourceArn: aws.String(state.DatabaseResourceArn.ValueString()),
		SecretArn:   aws.String(state.DatabaseSecretArn.ValueString()),
		Sql:         &deleteUserSqlQuery,
	}

	_, deleteUserSqlQueryErr := r.client.ExecuteStatement(ctx, &deleteUserStatementOpts)

	if deleteUserSqlQueryErr != nil {
		resp.Diagnostics.AddError("RDS data service client error", deleteUserSqlQueryErr.Error())
		return
	}
}

func (r *MysqlUserResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// TO DO
	//resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
