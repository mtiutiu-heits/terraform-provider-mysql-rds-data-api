// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/rdsdata"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure MysqlRdsDataApiProvider satisfies various provider interfaces.
var _ provider.Provider = &MysqlRdsDataApiProvider{}

// MysqlRdsDataApiProvider defines the provider implementation.
type MysqlRdsDataApiProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

// MysqlRdsDataApiProviderModel describes the provider data model.
type MysqlRdsDataApiProviderModel struct {
	Arn types.String `tfsdk:"arn"`
}

func (p *MysqlRdsDataApiProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "mysql_rds_data"
	resp.Version = p.version
}

func (p *MysqlRdsDataApiProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"endpoint": schema.StringAttribute{
				MarkdownDescription: "Example provider attribute",
				Optional:            true,
			},
		},
	}
}

func (p *MysqlRdsDataApiProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data MysqlRdsDataApiProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Amazon RDS Data service configuration for data sources and resources
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		resp.Diagnostics.AddError(
			"AWS Client Config Error",
			err.Error(),
		)
		return
	}
	// Create an Amazon RDS Data service client
	client := rdsdata.NewFromConfig(cfg)

	resp.DataSourceData = client
	resp.ResourceData = client
}

func (p *MysqlRdsDataApiProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewMysqlUserResource,
	}
}

func (p *MysqlRdsDataApiProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return nil
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &MysqlRdsDataApiProvider{
			version: version,
		}
	}
}
