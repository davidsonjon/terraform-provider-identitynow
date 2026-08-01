package identity_v1

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework-validators/datasourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	sailpoint "github.com/sailpoint-oss/golang-sdk/v3"
	"github.com/sailpoint-oss/golang-sdk/v3/identities"

	"terraform-provider-identitynow/internal/provider/identity_v1/datasource_identity"
	"terraform-provider-identitynow/internal/provider/util"
)

const identityLookupPageSize int32 = 250

var (
	_ datasource.DataSource                     = (*identityDataSource)(nil)
	_ datasource.DataSourceWithConfigure        = (*identityDataSource)(nil)
	_ datasource.DataSourceWithConfigValidators = (*identityDataSource)(nil)
)

type clientProvider interface {
	GetClient() *sailpoint.APIClient
}

type identityDataSourceModel struct {
	Alias           types.String                            `tfsdk:"alias"`
	Attributes      jsontypes.Normalized                    `tfsdk:"attributes"`
	Created         types.String                            `tfsdk:"created"`
	EmailAddress    types.String                            `tfsdk:"email_address"`
	Id              types.String                            `tfsdk:"id"`
	IdentityStatus  types.String                            `tfsdk:"identity_status"`
	IsManager       types.Bool                              `tfsdk:"is_manager"`
	LastRefresh     types.String                            `tfsdk:"last_refresh"`
	LifecycleState  datasource_identity.LifecycleStateValue `tfsdk:"lifecycle_state"`
	ManagerRef      datasource_identity.ManagerRefValue     `tfsdk:"manager_ref"`
	Modified        types.String                            `tfsdk:"modified"`
	Name            types.String                            `tfsdk:"name"`
	ProcessingState types.String                            `tfsdk:"processing_state"`
}

func NewIdentityDataSource() datasource.DataSource {
	return &identityDataSource{}
}

type identityDataSource struct {
	client *sailpoint.APIClient
}

type identityLookupSpec struct {
	LookupID      string
	FilterPattern string
	LookupValue   string
	LookupAttr    string
	LookupLabel   string
}

func (d *identityDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_identity_v1"
}

func (d *identityDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = identityDataSourceSchema(ctx)
	resp.Schema.Description = "Reads an Identity from IdentityNow/ISC by id, exact alias, or exact email address."
	resp.Schema.MarkdownDescription = "Reads an Identity from IdentityNow/ISC by `id`, exact `alias`, or exact `email_address`. Exactly one of those arguments must be set. Returns the generated identity fields plus the hand-added `attributes` JSON blob."
}

func identityDataSourceSchema(ctx context.Context) datasourceschema.Schema {
	s := datasource_identity.IdentityDataSourceSchema(ctx)

	idAttr := s.Attributes["id"].(datasourceschema.StringAttribute)
	idAttr.Required = false
	idAttr.Optional = true
	idAttr.Computed = true
	idAttr.Description = "The identity ID to retrieve. Exactly one of `id`, `alias`, or `email_address` must be set."
	idAttr.MarkdownDescription = idAttr.Description
	s.Attributes["id"] = idAttr

	aliasAttr := s.Attributes["alias"].(datasourceschema.StringAttribute)
	aliasAttr.Optional = true
	aliasAttr.Computed = true
	aliasAttr.Description = "The identity alias to retrieve by exact match when `id` is not set. Exactly one of `id`, `alias`, or `email_address` must be set."
	aliasAttr.MarkdownDescription = aliasAttr.Description
	s.Attributes["alias"] = aliasAttr

	emailAttr := s.Attributes["email_address"].(datasourceschema.StringAttribute)
	emailAttr.Optional = true
	emailAttr.Computed = true
	emailAttr.Description = "The identity email address to retrieve by exact match when `id` and `alias` are not set. Exactly one of `id`, `alias`, or `email_address` must be set."
	emailAttr.MarkdownDescription = emailAttr.Description
	s.Attributes["email_address"] = emailAttr

	desc := "Raw identity attributes, represented as a normalized JSON object because the shape is connector-specific and truly dynamic."
	s.Attributes["attributes"] = datasourceschema.StringAttribute{
		CustomType:          jsontypes.NormalizedType{},
		Computed:            true,
		Description:         desc,
		MarkdownDescription: desc,
	}

	// Deliberately not implementing use_caller_identity/caller_identity_used in this pilot; no get-current-caller-identity endpoint was found in the v1 identities spec during scoping.
	return s
}

func (d *identityDataSource) ConfigValidators(context.Context) []datasource.ConfigValidator {
	return []datasource.ConfigValidator{
		datasourcevalidator.ExactlyOneOf(
			path.MatchRoot("id"),
			path.MatchRoot("alias"),
			path.MatchRoot("email_address"),
		),
	}
}

func (d *identityDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *identityDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config identityDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	dto, diags := d.lookupIdentity(ctx, config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	state, diags := identityDataSourceDTOToModel(ctx, dto, config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (d *identityDataSource) lookupIdentity(ctx context.Context, config identityDataSourceModel) (*identities.Identity, diag.Diagnostics) {
	var diags diag.Diagnostics
	spec := identityLookupSpecFromConfig(config)

	if spec.LookupID != "" || (!config.Id.IsNull() && !config.Id.IsUnknown()) {
		tflog.Debug(ctx, "Reading Identity data source by id", map[string]interface{}{"id": spec.LookupID})
		dto, httpResp, err := d.client.IdentitiesAPI.GetIdentityV1(ctx, spec.LookupID).Execute()
		if err != nil {
			tflog.Error(ctx, "Error reading Identity data source by id", map[string]interface{}{"id": spec.LookupID, "error": err.Error()})
			diags.AddError("Error reading Identity", errDetail(err, httpResp))
			return nil, diags
		}
		return dto, diags
	}

	return d.lookupIdentityByFilter(ctx, spec.FilterPattern, spec.LookupValue, spec.LookupAttr, spec.LookupLabel)
}

func (d *identityDataSource) lookupIdentityByFilter(ctx context.Context, filterPattern, lookupValue, lookupAttr, lookupLabel string) (*identities.Identity, diag.Diagnostics) {
	var diags diag.Diagnostics

	tflog.Debug(ctx, "Reading Identity data source by alternate lookup", map[string]interface{}{lookupAttr: lookupValue})

	matches := make([]identities.Identity, 0, 2)
	var offset int32
	for {
		items, httpResp, err := d.client.IdentitiesAPI.
			ListIdentitiesV1(ctx).
			Filters(fmt.Sprintf(filterPattern, lookupValue)).
			Limit(identityLookupPageSize).
			Offset(offset).
			Execute()
		if err != nil {
			tflog.Error(ctx, "Error listing Identities for alternate lookup", map[string]interface{}{lookupAttr: lookupValue, "error": err.Error()})
			diags.AddError(fmt.Sprintf("Error reading Identity by %s", lookupAttr), errDetail(err, httpResp))
			return nil, diags
		}

		matches = append(matches, items...)
		if len(items) < int(identityLookupPageSize) {
			break
		}
		offset += identityLookupPageSize
	}

	return identityFromMatches(matches, lookupValue, lookupAttr, lookupLabel)
}

func identityLookupSpecFromConfig(config identityDataSourceModel) identityLookupSpec {
	if !config.Id.IsNull() && !config.Id.IsUnknown() {
		return identityLookupSpec{
			LookupID: config.Id.ValueString(),
		}
	}

	if !config.Alias.IsNull() && !config.Alias.IsUnknown() {
		return identityLookupSpec{
			FilterPattern: "alias eq \"%s\"",
			LookupValue:   strings.TrimSpace(config.Alias.ValueString()),
			LookupAttr:    "alias",
			LookupLabel:   "alias",
		}
	}

	return identityLookupSpec{
		FilterPattern: "email eq \"%s\"",
		LookupValue:   strings.TrimSpace(config.EmailAddress.ValueString()),
		LookupAttr:    "email_address",
		LookupLabel:   "email address",
	}
}

func identityFromMatches(matches []identities.Identity, lookupValue, lookupAttr, lookupLabel string) (*identities.Identity, diag.Diagnostics) {
	var diags diag.Diagnostics

	switch len(matches) {
	case 0:
		diags.AddError(
			fmt.Sprintf("Identity not found by %s", lookupAttr),
			fmt.Sprintf("No identity with exact %s %q was found. Set `id` instead if the identity %s is not unique or has changed.", lookupLabel, lookupValue, lookupLabel),
		)
		return nil, diags
	case 1:
		return &matches[0], diags
	default:
		ids := make([]string, 0, len(matches))
		for i := range matches {
			if matches[i].Id != nil && *matches[i].Id != "" {
				ids = append(ids, *matches[i].Id)
			}
		}
		diags.AddError(
			fmt.Sprintf("Identity %s is not unique", lookupAttr),
			fmt.Sprintf("Found %d identities with exact %s %q. Use `id` instead. Matching identity ids: %s.", len(matches), lookupLabel, lookupValue, strings.Join(ids, ", ")),
		)
		return nil, diags
	}
}

func identityDataSourceDTOToModel(ctx context.Context, dto *identities.Identity, fallback identityDataSourceModel) (identityDataSourceModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	model := fallback
	model.Attributes = jsontypes.NewNormalizedNull()
	model.Created = types.StringNull()
	model.IdentityStatus = types.StringNull()
	model.IsManager = types.BoolNull()
	model.LastRefresh = types.StringNull()
	model.LifecycleState = datasource_identity.NewLifecycleStateValueNull()
	model.ManagerRef = datasource_identity.NewManagerRefValueNull()
	model.Modified = types.StringNull()
	model.Name = types.StringNull()
	model.ProcessingState = types.StringNull()

	if dto == nil {
		return model, diags
	}
	if dto.Id != nil {
		model.Id = types.StringValue(*dto.Id)
	}
	if dto.Alias != nil {
		model.Alias = types.StringValue(*dto.Alias)
	}
	model.EmailAddress = nullableStringValue(dto.GetEmailAddressOk())
	if dto.Name != "" {
		model.Name = types.StringValue(dto.Name)
	}
	if dto.IdentityStatus != nil {
		model.IdentityStatus = types.StringValue(*dto.IdentityStatus)
	}
	if dto.IsManager != nil {
		model.IsManager = types.BoolValue(*dto.IsManager)
	}
	model.ProcessingState = nullableStringValue(dto.GetProcessingStateOk())
	model.Created = timeToStringValue(dto.Created)
	model.Modified = timeToStringValue(dto.Modified)
	model.LastRefresh = timeToStringValue(dto.LastRefresh)

	attributes, d := normalizedJSONFromMap(dto.Attributes)
	diags.Append(d...)
	model.Attributes = attributes

	lifecycleState, d := identityLifecycleStateValueFromAPI(ctx, dto.LifecycleState)
	diags.Append(d...)
	model.LifecycleState = lifecycleState

	managerRef, d := identityManagerRefValueFromAPI(ctx, nullableIdentityManagerRef(dto.GetManagerRefOk()))
	diags.Append(d...)
	model.ManagerRef = managerRef

	return model, diags
}

func nullableStringValue(v *string, ok bool) types.String {
	if !ok || v == nil {
		return types.StringNull()
	}
	return types.StringValue(*v)
}

func nullableIdentityManagerRef(v *identities.IdentityManagerRef, ok bool) *identities.IdentityManagerRef {
	if !ok || v == nil {
		return nil
	}
	return v
}

func identityLifecycleStateValueFromAPI(ctx context.Context, dto *identities.IdentityLifecycleState) (datasource_identity.LifecycleStateValue, diag.Diagnostics) {
	var diags diag.Diagnostics
	if dto == nil {
		return datasource_identity.NewLifecycleStateValueNull(), diags
	}
	value, d := datasource_identity.NewLifecycleStateValue(
		datasource_identity.LifecycleStateValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"manually_updated": types.BoolValue(dto.ManuallyUpdated),
			"state_name":       types.StringValue(dto.StateName),
		},
	)
	diags.Append(d...)
	return value, diags
}

func identityManagerRefValueFromAPI(ctx context.Context, dto *identities.IdentityManagerRef) (datasource_identity.ManagerRefValue, diag.Diagnostics) {
	var diags diag.Diagnostics
	if dto == nil {
		return datasource_identity.NewManagerRefValueNull(), diags
	}
	value, d := datasource_identity.NewManagerRefValue(
		datasource_identity.ManagerRefValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"id":   types.StringPointerValue(dto.Id),
			"name": types.StringPointerValue(dto.Name),
			"type": types.StringPointerValue(dto.Type),
		},
	)
	diags.Append(d...)
	return value, diags
}

func normalizedJSONFromMap(v map[string]interface{}) (jsontypes.Normalized, diag.Diagnostics) {
	var diags diag.Diagnostics
	if v == nil {
		return jsontypes.NewNormalizedNull(), diags
	}
	b, err := json.Marshal(v)
	if err != nil {
		diags.AddError("Error encoding JSON attribute from API response", err.Error())
		return jsontypes.NewNormalizedNull(), diags
	}
	return jsontypes.NewNormalizedValue(string(b)), diags
}

func errDetail(err error, httpResp *http.Response) string {
	return util.SailpointErrorDetail(err, httpResp)
}

func timeToStringValue(t *identities.SailPointTime) types.String {
	if t == nil {
		return types.StringNull()
	}
	return types.StringValue(t.Format(time.RFC3339))
}
