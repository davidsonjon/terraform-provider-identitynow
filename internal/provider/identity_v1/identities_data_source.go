package identity_v1

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	sailpoint "github.com/sailpoint-oss/golang-sdk/v3"
	"github.com/sailpoint-oss/golang-sdk/v3/identities"

	"terraform-provider-identitynow/internal/provider/identity_v1/datasource_identities"
)

var (
	_ datasource.DataSource              = (*identitiesDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*identitiesDataSource)(nil)
)

func NewIdentitiesDataSource() datasource.DataSource {
	return &identitiesDataSource{}
}

type identitiesDataSource struct {
	client *sailpoint.APIClient
}

func (d *identitiesDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_identities_v1"
}

func (d *identitiesDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = identitiesDataSourceSchema(ctx)
	resp.Schema.Description = "Lists Identities from IdentityNow/ISC, optionally filtered, sorted, and paginated."
	resp.Schema.MarkdownDescription = "Lists Identities from IdentityNow/ISC via `GET /identities`, optionally filtered, sorted, and paginated. Returns the same generated identity fields per entry as the singular `identitynow_identity_v1` data source, except the hand-written `attributes` JSON blob is intentionally singular-only in this pilot."
}

func identitiesDataSourceSchema(ctx context.Context) datasourceschema.Schema {
	return datasource_identities.IdentitiesDataSourceSchema(ctx)
}

func (d *identitiesDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	cp, ok := req.ProviderData.(clientProvider)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected a provider client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}
	d.client = cp.GetClient()
}

func (d *identitiesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config datasource_identities.IdentitiesModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Reading Identities data source", map[string]interface{}{"filters": config.Filters.ValueString()})

	apiReq := d.client.IdentitiesAPI.ListIdentitiesV1(ctx)
	if !config.Filters.IsNull() && !config.Filters.IsUnknown() {
		apiReq = apiReq.Filters(config.Filters.ValueString())
	}
	if !config.Sorters.IsNull() && !config.Sorters.IsUnknown() {
		apiReq = apiReq.Sorters(config.Sorters.ValueString())
	}
	if !config.DefaultFilter.IsNull() && !config.DefaultFilter.IsUnknown() {
		apiReq = apiReq.DefaultFilter(config.DefaultFilter.ValueString())
	}
	if !config.Limit.IsNull() && !config.Limit.IsUnknown() {
		apiReq = apiReq.Limit(int32(config.Limit.ValueInt64()))
	}
	if !config.Offset.IsNull() && !config.Offset.IsUnknown() {
		apiReq = apiReq.Offset(int32(config.Offset.ValueInt64()))
	}

	dtos, httpResp, err := apiReq.Execute()
	if err != nil {
		tflog.Error(ctx, "Error reading Identities data source", map[string]interface{}{"error": err.Error()})
		resp.Diagnostics.AddError("Error listing Identities", errDetail(err, httpResp))
		return
	}

	fullType := identitiesDataSourceSchema(ctx).Type().(basetypes.ObjectType)
	elemType := fullType.AttrTypes["identities"].(basetypes.SetType).ElemType

	items := make([]datasource_identities.IdentitiesValue, 0, len(dtos))
	for i := range dtos {
		item, diags := identitiesListItemFromDTO(ctx, &dtos[i])
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		items = append(items, item)
	}

	identitiesSet, diags := types.SetValueFrom(ctx, elemType, items)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	config.Identities = identitiesSet

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}

func identitiesListItemFromDTO(ctx context.Context, dto *identities.Identity) (datasource_identities.IdentitiesValue, diag.Diagnostics) {
	var diags diag.Diagnostics

	lifecycleState, d := identityLifecycleStateValueFromAPI(ctx, nil)
	diags.Append(d...)
	lifecycleStateObj, d := lifecycleState.ToObjectValue(ctx)
	diags.Append(d...)

	managerRef, d := identityManagerRefValueFromAPI(ctx, nil)
	diags.Append(d...)
	managerRefObj, d := managerRef.ToObjectValue(ctx)
	diags.Append(d...)

	attrs := map[string]attr.Value{
		"alias":            types.StringNull(),
		"created":          types.StringNull(),
		"email_address":    types.StringNull(),
		"id":               types.StringNull(),
		"identity_status":  types.StringNull(),
		"is_manager":       types.BoolNull(),
		"last_refresh":     types.StringNull(),
		"lifecycle_state":  lifecycleStateObj,
		"manager_ref":      managerRefObj,
		"modified":         types.StringNull(),
		"name":             types.StringNull(),
		"processing_state": types.StringNull(),
	}
	if diags.HasError() {
		return datasource_identities.NewIdentitiesValueUnknown(), diags
	}
	if dto == nil {
		return datasource_identities.NewIdentitiesValue(
			datasource_identities.IdentitiesValue{}.AttributeTypes(ctx),
			attrs,
		)
	}

	if dto.Id != nil {
		attrs["id"] = types.StringValue(*dto.Id)
	}
	if dto.Alias != nil {
		attrs["alias"] = types.StringValue(*dto.Alias)
	}
	attrs["email_address"] = nullableStringValue(dto.GetEmailAddressOk())
	if dto.Name != "" {
		attrs["name"] = types.StringValue(dto.Name)
	}
	if dto.IdentityStatus != nil {
		attrs["identity_status"] = types.StringValue(*dto.IdentityStatus)
	}
	if dto.IsManager != nil {
		attrs["is_manager"] = types.BoolValue(*dto.IsManager)
	}
	attrs["processing_state"] = nullableStringValue(dto.GetProcessingStateOk())
	attrs["created"] = timeToStringValue(dto.Created)
	attrs["modified"] = timeToStringValue(dto.Modified)
	attrs["last_refresh"] = timeToStringValue(dto.LastRefresh)

	lifecycleState, d = identityLifecycleStateValueFromAPI(ctx, dto.LifecycleState)
	diags.Append(d...)
	attrs["lifecycle_state"], d = lifecycleState.ToObjectValue(ctx)
	diags.Append(d...)

	managerRef, d = identityManagerRefValueFromAPI(ctx, nullableIdentityManagerRef(dto.GetManagerRefOk()))
	diags.Append(d...)
	attrs["manager_ref"], d = managerRef.ToObjectValue(ctx)
	diags.Append(d...)
	if diags.HasError() {
		return datasource_identities.NewIdentitiesValueUnknown(), diags
	}

	item, d := datasource_identities.NewIdentitiesValue(
		datasource_identities.IdentitiesValue{}.AttributeTypes(ctx),
		attrs,
	)
	diags.Append(d...)
	return item, diags
}
