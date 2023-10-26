// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"regexp"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/rdsdata"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure RdsDataProvider satisfies various provider interfaces.
var _ provider.Provider = &RdsDataProvider{}

// RdsDataProvider defines the provider implementation.
type RdsDataProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

// RdsDataProviderModel describes the provider data model.
type RdsDataProviderModel struct {
	Region types.String `tfsdk:"region"`
}

func (p *RdsDataProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "awsrdsdata"
	resp.Version = p.version
}

func (p *RdsDataProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"region": schema.StringAttribute{
				MarkdownDescription: "The RDS data service AWS region",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^.*\w-.*\w-.*\d$`),
						"must contain a valid region value",
					),
				},
			},
		},
	}
}

func (p *RdsDataProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var provider_config RdsDataProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &provider_config)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// ======================= Custom Provider Logic =======================

	// Check if provider required attributes are valid first
	if provider_config.Region.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("region"),
			"Unknown AWS region.",
			"The provider cannot create the AWS client because the region attribute value is not known. "+
				"Either apply the source of the value first, or set the region attribute value statically in the configuration",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	// Next, configure the AWS client
	aws_client_cfg, err := config.LoadDefaultConfig(
		context.TODO(),
		config.WithRegion(provider_config.Region.ValueString()),
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"AWS Client Config Error",
			err.Error(),
		)
		return
	}
	// Finally, create the Amazon RDS Data service client to be used by resources
	aws_rds_data_client := rdsdata.NewFromConfig(aws_client_cfg)

	resp.DataSourceData = aws_rds_data_client
	resp.ResourceData = aws_rds_data_client
}

func (p *RdsDataProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewMysqlUserResource,
		NewMysqlGrantResource,
	}
}

func (p *RdsDataProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return nil
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &RdsDataProvider{
			version: version,
		}
	}
}
