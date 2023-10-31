package provider

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rdsdata"
	"github.com/dcarbone/terraform-plugin-framework-utils/v3/conv"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
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
	_ resource.Resource                = &MysqlGrantResource{}
	_ resource.ResourceWithImportState = &MysqlGrantResource{}
)

func NewMysqlGrantResource() resource.Resource {
	return &MysqlGrantResource{}
}

// MysqlGrantResource defines the resource implementation.
type MysqlGrantResource struct {
	client *rdsdata.Client
}

// MysqlGrantResourceModel describes the resource data model.
type MysqlGrantResourceModel struct {
	User                types.String `tfsdk:"user"`
	Host                types.String `tfsdk:"host"`
	Database            types.String `tfsdk:"database"`
	Privileges          types.List   `tfsdk:"privileges"`
	DatabaseResourceArn types.String `tfsdk:"database_resource_arn"`
	DatabaseSecretArn   types.String `tfsdk:"database_secret_arn"`
}

func (r *MysqlGrantResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_mysql_grant"
}

func (r *MysqlGrantResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "AWS RDS Data MySQL user privileges",

		Attributes: map[string]schema.Attribute{
			"user": schema.StringAttribute{
				MarkdownDescription: "The MySQL user name to grant privileges",
				Required:            true,
				Validators: []validator.String{
					// user value cannot be empty
					stringvalidator.LengthAtLeast(1),
					// protect against destroying system accounts
					stringvalidator.NoneOf([]string{"sys"}...),
				},
			},
			"host": schema.StringAttribute{
				MarkdownDescription: "The host field associated with the MySQL user",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^(([a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9\-]*[a-zA-Z0-9])\.)*([A-Za-z0-9]|[A-Za-z0-9][A-Za-z0-9\-]*[A-Za-z0-9]|%)$`),
						"must contain a valid hostname value",
					),
				},
			},
			"database": schema.StringAttribute{
				MarkdownDescription: "The MySQL database to grant privileges for",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
					// protect against destroying system databases
					stringvalidator.NoneOf([]string{"rdsadmin", "mysql.sys"}...),
				},
			},
			"privileges": schema.ListAttribute{
				MarkdownDescription: "The MySQL user privileges to grant",
				Required:            true,
				ElementType:         types.StringType,
				Validators: []validator.List{
					// at least one privilege must be defined
					listvalidator.SizeAtLeast(1),
					// privilege definitions cannot be empty
					listvalidator.ValueStringsAre(stringvalidator.LengthAtLeast(1)),
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
				},
			},
		},
	}
}

func (r *MysqlGrantResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *MysqlGrantResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan MysqlGrantResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// ======================= Resource CREATE Logic =======================

	grantUserPrivilegesSqlQuery := fmt.Sprintf(
		"GRANT %s ON %s.* TO '%s'@'%s'",
		strings.Join(conv.StringListToStrings(plan.Privileges), ","),
		plan.Database.ValueString(),
		plan.User.ValueString(),
		plan.Host.ValueString(),
	)

	grantUserPrivilegesStatementOpts := rdsdata.ExecuteStatementInput{
		ResourceArn: aws.String(plan.DatabaseResourceArn.ValueString()),
		SecretArn:   aws.String(plan.DatabaseSecretArn.ValueString()),
		Sql:         &grantUserPrivilegesSqlQuery,
	}

	_, grantSqlQueryErr := r.client.ExecuteStatement(ctx, &grantUserPrivilegesStatementOpts)

	if grantSqlQueryErr != nil {
		resp.Diagnostics.AddError("Resource CREATE operation error", grantSqlQueryErr.Error())
		return
	}

	tflog.Trace(ctx, "created a MySQL user grant resource")

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *MysqlGrantResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state MysqlGrantResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// ======================= Resource READ Logic =======================

	userGrantsSqlQuery := fmt.Sprintf(
		"SHOW GRANTS FOR '%s'@'%s'",
		state.User.ValueString(),
		state.Host.ValueString(),
	)

	userGrantsQueryStatementOpts := rdsdata.ExecuteStatementInput{
		ResourceArn: aws.String(state.DatabaseResourceArn.ValueString()),
		SecretArn:   aws.String(state.DatabaseSecretArn.ValueString()),
		Sql:         &userGrantsSqlQuery,
	}

	userGrantsSqlQueryResult, userGrantsSqlQueryErr := r.client.ExecuteStatement(ctx, &userGrantsQueryStatementOpts)

	userGrantsNotDefinedErrMsg := fmt.Sprintf(
		"There is no such grant defined for user '%s' on host '%s'",
		state.User.ValueString(),
		state.Host.ValueString(),
	)
	if userGrantsSqlQueryErr != nil && !strings.Contains(userGrantsSqlQueryErr.Error(), userGrantsNotDefinedErrMsg) {
		resp.Diagnostics.AddError("Resource READ operation error", userGrantsSqlQueryErr.Error())
		return
	}

	// The `GRANT USAGE` privilege is always present so at least one record is returned
	if userGrantsSqlQueryResult.Records != nil && len(userGrantsSqlQueryResult.Records) == 1 {
		tflog.Trace(ctx, "MySQL server returned no user grant records")
		// Force resource recreation if GRANTS deleted outside terraform
		state.Privileges = types.ListNull(types.StringType)
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *MysqlGrantResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan MysqlGrantResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// ======================= Resource UPDATE Logic =======================

	// Revoke all privileges first
	revokeUserPrivilegesSqlQuery := fmt.Sprintf(
		// Use IF NOT EXISTS condition (only supported by MySQL version >= 8.0)
		"REVOKE ALL PRIVILEGES ON %s.* FROM '%s'@'%s'",
		plan.Database.ValueString(),
		plan.User.ValueString(),
		plan.Host.ValueString(),
	)

	revokeUserPrivilegesStatementOpts := rdsdata.ExecuteStatementInput{
		ResourceArn: aws.String(plan.DatabaseResourceArn.ValueString()),
		SecretArn:   aws.String(plan.DatabaseSecretArn.ValueString()),
		Sql:         &revokeUserPrivilegesSqlQuery,
	}

	_, revokeSqlQueryErr := r.client.ExecuteStatement(ctx, &revokeUserPrivilegesStatementOpts)

	userGrantsNotDefinedErrMsg := fmt.Sprintf(
		"There is no such grant defined for user '%s' on host '%s'",
		plan.User.ValueString(),
		plan.Host.ValueString(),
	)
	if revokeSqlQueryErr != nil && !strings.Contains(revokeSqlQueryErr.Error(), userGrantsNotDefinedErrMsg) {
		resp.Diagnostics.AddError("Resource UPDATE operation error", revokeSqlQueryErr.Error())
		return
	}

	// Grant new privileges
	grantUserPrivilegesSqlQuery := fmt.Sprintf(
		"GRANT %s ON %s.* TO '%s'@'%s'",
		strings.Join(conv.StringListToStrings(plan.Privileges), ","),
		plan.Database.ValueString(),
		plan.User.ValueString(),
		plan.Host.ValueString(),
	)

	grantUserPrivilegesStatementOpts := rdsdata.ExecuteStatementInput{
		ResourceArn: aws.String(plan.DatabaseResourceArn.ValueString()),
		SecretArn:   aws.String(plan.DatabaseSecretArn.ValueString()),
		Sql:         &grantUserPrivilegesSqlQuery,
	}

	_, grantSqlQueryErr := r.client.ExecuteStatement(ctx, &grantUserPrivilegesStatementOpts)

	if grantSqlQueryErr != nil {
		resp.Diagnostics.AddError("Resource UPDATE operation error", grantSqlQueryErr.Error())
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *MysqlGrantResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state MysqlGrantResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// ======================= Resource DELETE Logic =======================

	revokeUserPrivilegesSqlQuery := fmt.Sprintf(
		"REVOKE %s ON %s.* FROM '%s'@'%s'",
		strings.Join(conv.StringListToStrings(state.Privileges), ","),
		state.Database.ValueString(),
		state.User.ValueString(),
		state.Host.ValueString(),
	)

	deleteUserStatementOpts := rdsdata.ExecuteStatementInput{
		ResourceArn: aws.String(state.DatabaseResourceArn.ValueString()),
		SecretArn:   aws.String(state.DatabaseSecretArn.ValueString()),
		Sql:         &revokeUserPrivilegesSqlQuery,
	}

	_, revokeUserPrivilegesSqlQueryErr := r.client.ExecuteStatement(ctx, &deleteUserStatementOpts)

	userGrantsNotDefinedErrMsg := fmt.Sprintf(
		"There is no such grant defined for user '%s' on host '%s'",
		state.User.ValueString(),
		state.Host.ValueString(),
	)
	if revokeUserPrivilegesSqlQueryErr != nil && !strings.Contains(revokeUserPrivilegesSqlQueryErr.Error(), userGrantsNotDefinedErrMsg) {
		resp.Diagnostics.AddError("Resource DELETE operation error", revokeUserPrivilegesSqlQueryErr.Error())
		return
	}
}

func (r *MysqlGrantResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// TO DO
	//resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
