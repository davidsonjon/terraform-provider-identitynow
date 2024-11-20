package identity

import (
	"context"
	"fmt"

	sailpoint "github.com/davidsonjon/golang-sdk/v2"
	"github.com/davidsonjon/terraform-provider-identitynow/identitynow/config"
	"github.com/hashicorp/terraform-plugin-framework-validators/datasourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var _ datasource.DataSource = &IdentityDataSource{}

func NewIdentityDataSource() datasource.DataSource {
	return &IdentityDataSource{}
}

type IdentityDataSource struct {
	client *sailpoint.APIClient
}

func (d *IdentityDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_identity"
}

func (d *IdentityDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Identity data source",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "System-generated unique ID of the Object",
			},
			"name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Name of the Object",
			},
			"created": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Creation date of the Object",
			},
			"modified": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Last modification date of the Object",
			},
			"alias": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Alternate unique identifier for the identity",
			},
			"email_address": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The email address of the identity",
			},
			"processing_state": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The processing state of the identity",
			},
			"identity_status": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The identity's status in the system",
			},
			"use_caller_identity": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "**beware** user with caution. Use the caller's identity if no user is found, to support lifecycle outside of terraform",
			},
			"caller_identity_used": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Helper flag to indicate if the caller's identity is being used",
			},
		},
	}
}

func (d *IdentityDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {

	if req.ProviderData == nil {
		return
	}

	config, ok := req.ProviderData.(config.ProviderConfig)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected config.ProviderConfig, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	d.client = config.APIClient
}

func (d *IdentityDataSource) ConfigValidators(ctx context.Context) []datasource.ConfigValidator {
	return []datasource.ConfigValidator{
		datasourcevalidator.Conflicting(
			path.MatchRoot("id"),
			path.MatchRoot("alias"),
			path.MatchRoot("email_address"),
		),
	}
}

func (d *IdentityDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data Identity

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	data.CallerIdentityUsed = types.BoolValue(false)

	if !data.Alias.IsNull() || !data.EmailAddress.IsNull() {

		var filter string

		if !data.Alias.IsNull() {
			filter = fmt.Sprintf(`alias eq "%v"`, data.Alias.ValueString())
		}

		if !data.EmailAddress.IsNull() {
			filter = fmt.Sprintf(`email eq "%v"`, data.EmailAddress.ValueString())
		}

		users, httpResp, err := d.client.Beta.IdentitiesAPI.ListIdentities(context.Background()).Filters(filter).Execute()
		if err != nil {
			tflog.Info(ctx, fmt.Sprintf("Full HTTP response: %v\n", httpResp))

			resp.Diagnostics.AddError(
				"Error when calling ListIdentities",
				fmt.Sprintf("Error: %v, see debug info for more information", err),
			)

			return
		}
		switch len(users) {
		case 0:
			if !data.UseCallerIdentity.ValueBool() {
				resp.Diagnostics.AddError(
					"No identities found",
					fmt.Sprint("Error: No users found for value"),
				)
				return
			}
			data.Id = types.StringValue("2c000000000000000000000000000000")
		case 1:
			data.Id = types.StringValue(*users[0].Id)
		default:
			resp.Diagnostics.AddError(
				"More than one identity found",
				fmt.Sprintf("Error: %v users found with query, only results with 1 will return data", len(users)),
			)
			return
		}
	}

	user, httpResp, err := d.client.Beta.IdentitiesAPI.GetIdentity(ctx, data.Id.ValueString()).Execute()
	if err != nil && !data.UseCallerIdentity.ValueBool() {
		tflog.Info(ctx, fmt.Sprintf("Full HTTP response: %v\n", httpResp))

		resp.Diagnostics.AddError(
			"Error when calling GetIdentity",
			fmt.Sprintf("Error: %v, see debug info for more information", err),
		)

		return
	}

	if data.UseCallerIdentity.ValueBool() && httpResp.StatusCode == 404 {
		userTokens, httpResp, err := d.client.Beta.PersonalAccessTokensAPI.ListPersonalAccessTokens(context.Background()).OwnerId("me").Execute()
		if err != nil {
			resp.Diagnostics.AddError(
				fmt.Sprintf("Error when calling `PersonalAccessTokensAPI.ListPersonalAccessTokens``:%v", err),
				fmt.Sprintf("Full HTTP response:%v", httpResp),
			)
			return
		}

		data.Id = types.StringPointerValue(userTokens[0].Owner.Id)

		user, httpResp, err = d.client.Beta.IdentitiesAPI.GetIdentity(ctx, data.Id.ValueString()).Execute()
		if err != nil {
			tflog.Info(ctx, fmt.Sprintf("Full HTTP response: %v\n", httpResp))

			resp.Diagnostics.AddError(
				"Error when calling GetIdentity for Caller",
				fmt.Sprintf("Error: %v, see debug info for more information", err),
			)

			return
		}
		data.CallerIdentityUsed = types.BoolValue(true)
	}

	data.Id = types.StringPointerValue(user.Id)
	data.Name = types.StringValue(user.Name)
	data.Created = types.StringPointerValue(user.Created)
	data.Modified = types.StringPointerValue(user.Modified)
	data.Alias = types.StringPointerValue(user.Alias)
	data.EmailAddress = types.StringPointerValue(user.EmailAddress.Get())
	data.ProcessingState = types.StringPointerValue(user.ProcessingState.Get())
	data.IdentityStatus = types.StringPointerValue(user.IdentityStatus)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
