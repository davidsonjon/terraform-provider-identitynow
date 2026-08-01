// Package segment_access_v1 implements a fully hand-written Terraform
// resource for managing Role and Access Profile assignments on an existing
// Segment from the Segment's perspective.
//
// This target intentionally does not use the repo's OpenAPI codegen pipeline:
// SailPoint does not currently publish a per-service v1 OpenAPI spec for this
// functionality, and there is no dedicated segment-side CRUD endpoint for the
// association. Instead:
//   - Create/Update/Delete reconcile each assignment individually by reading
//     the current Role or Access Profile, then PATCHing its `/segments` field
//     to add or remove this resource's `segment_id` while preserving any other
//     segment ids already present.
//   - Read reconstructs the real current state by listing Roles and Access
//     Profiles filtered to the Segment id, since there is no GET
//     /segment-access-style endpoint.
//   - Delete unassigns every currently tracked reference from the Segment, but
//     does not delete the Segment itself.
//   - Import adopts by plain Segment id, then lets the next Read reconcile the
//     current assignments.
//
// Only ROLE and ACCESS_PROFILE references are supported. ENTITLEMENT is
// deliberately excluded even though the reference davidsonjon/identitynow
// provider's Read path lists them: the only live write mechanism is PATCHing
// `/segments` on Roles and Access Profiles, so including ENTITLEMENTs here
// would create uncorrectable drift.
package segment_access_v1

import (
	"context"
	"fmt"
	"net/http"
	"sort"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	sailpoint "github.com/sailpoint-oss/golang-sdk/v3"
	"github.com/sailpoint-oss/golang-sdk/v3/access_profiles"
	"github.com/sailpoint-oss/golang-sdk/v3/roles"

	"terraform-provider-identitynow/internal/provider/util"
)

const (
	segmentAccessTypeAccessProfile = "ACCESS_PROFILE"
	segmentAccessTypeRole          = "ROLE"
)

var (
	_ resource.Resource                = (*segmentAccessResource)(nil)
	_ resource.ResourceWithConfigure   = (*segmentAccessResource)(nil)
	_ resource.ResourceWithImportState = (*segmentAccessResource)(nil)
)

// clientProvider is satisfied by internal/provider.identitynowProvider without
// this package needing to import it (which would create an import cycle).
type clientProvider interface {
	GetClient() *sailpoint.APIClient
}

type segmentAccessResourceModel struct {
	Id          types.String `tfsdk:"id"`
	SegmentID   types.String `tfsdk:"segment_id"`
	Assignments types.Set    `tfsdk:"assignments"`
}

type segmentAccessAssignmentModel struct {
	Type types.String `tfsdk:"type"`
	Id   types.String `tfsdk:"id"`
}

type segmentAccessAssignment struct {
	Type string
	ID   string
}

func NewSegmentAccessResource() resource.Resource {
	return &segmentAccessResource{}
}

type segmentAccessResource struct {
	client *sailpoint.APIClient
}

func (r *segmentAccessResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_segment_access_v1"
}

func (r *segmentAccessResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceschema.Schema{
		Description: "Manages which Roles and Access Profiles are assigned to an existing Segment in IdentityNow/ISC.",
		MarkdownDescription: "Manages which [Roles](https://documentation.sailpoint.com/saas/help/access/roles.html) and " +
			"[Access Profiles](https://documentation.sailpoint.com/saas/help/access/access-profiles.html) are assigned to an existing " +
			"[Segment](https://documentation.sailpoint.com/saas/help/common/segments.html) in IdentityNow/ISC from the Segment's " +
			"perspective. This is a fully hand-written `_v1` resource with no OpenAPI codegen involved: SailPoint does not currently " +
			"publish a per-service v1 spec or segment-side association endpoint for this functionality, so Create/Update/Delete reconcile " +
			"membership by GETting each referenced Role or Access Profile and PATCHing its `/segments` field to add or remove this " +
			"resource's `segment_id` while preserving any other segment ids already present. `Read` reconstructs live state by listing " +
			"Roles and Access Profiles filtered to the target Segment.",
		Attributes: map[string]resourceschema.Attribute{
			"id": resourceschema.StringAttribute{
				Computed:            true,
				Description:         "Same value as segment_id.",
				MarkdownDescription: "Same value as `segment_id`.",
			},
			"segment_id": resourceschema.StringAttribute{
				Required:            true,
				Description:         "ID of the Segment whose Role and Access Profile assignments are managed.",
				MarkdownDescription: "ID of the Segment whose Role and Access Profile assignments are managed. Changing this forces replacement.",
			},
			"assignments": resourceschema.SetNestedAttribute{
				Required:            true,
				Description:         "Complete set of Role and Access Profile assignments that should be present on the Segment.",
				MarkdownDescription: "Complete set of Role and Access Profile assignments that should be present on the Segment. `type` must be either `ROLE` or `ACCESS_PROFILE`; ENTITLEMENT is intentionally unsupported because this resource's only live write mechanism is PATCHing `/segments` on Roles and Access Profiles.",
				NestedObject: resourceschema.NestedAttributeObject{
					Attributes: map[string]resourceschema.Attribute{
						"type": resourceschema.StringAttribute{
							Required:            true,
							Description:         "Assignment type. Must be ROLE or ACCESS_PROFILE.",
							MarkdownDescription: "Assignment type. Must be `ROLE` or `ACCESS_PROFILE`.",
							Validators: []validator.String{
								stringvalidator.OneOf(segmentAccessTypeRole, segmentAccessTypeAccessProfile),
							},
						},
						"id": resourceschema.StringAttribute{
							Required:            true,
							Description:         "ID of the Role or Access Profile assigned to the Segment.",
							MarkdownDescription: "ID of the Role or Access Profile assigned to the Segment.",
						},
					},
				},
			},
		},
	}

	applySegmentAccessPlanModifiers(&resp.Schema)
}

func (r *segmentAccessResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	cp, ok := req.ProviderData.(clientProvider)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected a provider client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}
	if cp.GetClient() == nil {
		resp.Diagnostics.AddError("Missing API client", "Provider configured without an API client.")
		return
	}

	r.client = cp.GetClient()
}

func (r *segmentAccessResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("segment_id"), req.ID)...)
}

func (r *segmentAccessResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan segmentAccessResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	segmentID := plan.SegmentID.ValueString()
	assignments, diags := segmentAccessAssignmentsFromSet(ctx, plan.Assignments)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Creating Segment access", map[string]interface{}{"segment_id": segmentID, "assignments": len(assignments)})

	httpResp, err := r.applyAssignmentsChange(ctx, segmentID, assignments, nil)
	if err != nil {
		tflog.Error(ctx, "Error creating Segment access", map[string]interface{}{"segment_id": segmentID, "error": err.Error()})
		resp.Diagnostics.AddError("Error creating Segment access", segmentAccessErrDetail(err, httpResp))
		return
	}

	state := plan
	state.Id = types.StringValue(segmentID)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *segmentAccessResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state segmentAccessResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	segmentID := state.SegmentID.ValueString()
	tflog.Debug(ctx, "Reading Segment access", map[string]interface{}{"segment_id": segmentID})

	liveAssignments, diags := r.readSegmentAssignments(ctx, segmentID)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	assignmentSet, diags := segmentAccessAssignmentsToSet(liveAssignments)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	state.Id = types.StringValue(segmentID)
	state.Assignments = assignmentSet
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *segmentAccessResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan segmentAccessResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state segmentAccessResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	desired, diags := segmentAccessAssignmentsFromSet(ctx, plan.Assignments)
	resp.Diagnostics.Append(diags...)
	current, diags := segmentAccessAssignmentsFromSet(ctx, state.Assignments)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	toAssign, toRemove := diffSegmentAccessAssignments(current, desired)
	segmentID := plan.SegmentID.ValueString()

	tflog.Debug(ctx, "Updating Segment access", map[string]interface{}{
		"segment_id":    segmentID,
		"to_assign":     len(toAssign),
		"to_remove":     len(toRemove),
		"planned_total": len(desired),
	})

	newState := plan
	newState.Id = types.StringValue(segmentID)
	if len(toAssign) == 0 && len(toRemove) == 0 {
		resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
		return
	}

	httpResp, err := r.applyAssignmentsChange(ctx, segmentID, toAssign, toRemove)
	if err != nil {
		tflog.Error(ctx, "Error updating Segment access", map[string]interface{}{"segment_id": segmentID, "error": err.Error()})
		resp.Diagnostics.AddError("Error updating Segment access", segmentAccessErrDetail(err, httpResp))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *segmentAccessResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state segmentAccessResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	segmentID := state.SegmentID.ValueString()
	current, diags := segmentAccessAssignmentsFromSet(ctx, state.Assignments)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Deleting Segment access", map[string]interface{}{"segment_id": segmentID, "assignments": len(current)})

	httpResp, err := r.applyAssignmentsChange(ctx, segmentID, nil, current)
	if err != nil {
		tflog.Error(ctx, "Error deleting Segment access", map[string]interface{}{"segment_id": segmentID, "error": err.Error()})
		resp.Diagnostics.AddError("Error deleting Segment access", segmentAccessErrDetail(err, httpResp))
		return
	}
}

func (r *segmentAccessResource) applyAssignmentsChange(ctx context.Context, segmentID string, assignments, removals []segmentAccessAssignment) (*http.Response, error) {
	var lastHTTPResp *http.Response

	for _, assignment := range removals {
		httpResp, err := r.reconcileAssignmentSegment(ctx, assignment, segmentID, false)
		if httpResp != nil {
			lastHTTPResp = httpResp
		}
		if err != nil {
			return httpResp, err
		}
	}

	for _, assignment := range assignments {
		httpResp, err := r.reconcileAssignmentSegment(ctx, assignment, segmentID, true)
		if httpResp != nil {
			lastHTTPResp = httpResp
		}
		if err != nil {
			return httpResp, err
		}
	}

	return lastHTTPResp, nil
}

func (r *segmentAccessResource) reconcileAssignmentSegment(ctx context.Context, assignment segmentAccessAssignment, segmentID string, ensurePresent bool) (*http.Response, error) {
	switch assignment.Type {
	case segmentAccessTypeRole:
		return r.reconcileRoleSegment(ctx, assignment.ID, segmentID, ensurePresent)
	case segmentAccessTypeAccessProfile:
		return r.reconcileAccessProfileSegment(ctx, assignment.ID, segmentID, ensurePresent)
	default:
		return nil, fmt.Errorf("unsupported assignment type %q", assignment.Type)
	}
}

func (r *segmentAccessResource) reconcileRoleSegment(ctx context.Context, roleID, segmentID string, ensurePresent bool) (*http.Response, error) {
	role, httpResp, err := r.client.RolesAPI.GetRoleV1(ctx, roleID).Execute()
	if err != nil {
		if !ensurePresent && httpResp != nil && httpResp.StatusCode == http.StatusNotFound {
			tflog.Warn(ctx, "Role already absent while removing Segment access assignment", map[string]interface{}{"role_id": roleID, "segment_id": segmentID})
			return httpResp, nil
		}
		return httpResp, err
	}

	desiredSegments, changed := segmentAccessDesiredSegments(role.Segments, segmentID, ensurePresent)
	if !changed {
		tflog.Debug(ctx, "Role segments already in desired state for Segment access", map[string]interface{}{"role_id": roleID, "segment_id": segmentID, "ensure_present": ensurePresent})
		return httpResp, nil
	}

	_, httpResp, err = r.client.RolesAPI.
		PatchRoleV1(ctx, roleID).
		JsonPatchOperation(segmentAccessRoleSegmentsPatch(desiredSegments)).
		Execute()
	if err != nil {
		if !ensurePresent && httpResp != nil && httpResp.StatusCode == http.StatusNotFound {
			tflog.Warn(ctx, "Role disappeared before Segment access removal patch completed", map[string]interface{}{"role_id": roleID, "segment_id": segmentID})
			return httpResp, nil
		}
		return httpResp, err
	}

	tflog.Info(ctx, "Reconciled Role segment membership for Segment access", map[string]interface{}{"role_id": roleID, "segment_id": segmentID, "ensure_present": ensurePresent})
	return httpResp, nil
}

func (r *segmentAccessResource) reconcileAccessProfileSegment(ctx context.Context, accessProfileID, segmentID string, ensurePresent bool) (*http.Response, error) {
	accessProfile, httpResp, err := r.client.AccessProfilesAPI.GetAccessProfileV1(ctx, accessProfileID).Execute()
	if err != nil {
		if !ensurePresent && httpResp != nil && httpResp.StatusCode == http.StatusNotFound {
			tflog.Warn(ctx, "Access Profile already absent while removing Segment access assignment", map[string]interface{}{"access_profile_id": accessProfileID, "segment_id": segmentID})
			return httpResp, nil
		}
		return httpResp, err
	}

	desiredSegments, changed := segmentAccessDesiredSegments(accessProfile.Segments, segmentID, ensurePresent)
	if !changed {
		tflog.Debug(ctx, "Access Profile segments already in desired state for Segment access", map[string]interface{}{"access_profile_id": accessProfileID, "segment_id": segmentID, "ensure_present": ensurePresent})
		return httpResp, nil
	}

	_, httpResp, err = r.client.AccessProfilesAPI.
		PatchAccessProfileV1(ctx, accessProfileID).
		JsonPatchOperation(segmentAccessAccessProfileSegmentsPatch(desiredSegments)).
		Execute()
	if err != nil {
		if !ensurePresent && httpResp != nil && httpResp.StatusCode == http.StatusNotFound {
			tflog.Warn(ctx, "Access Profile disappeared before Segment access removal patch completed", map[string]interface{}{"access_profile_id": accessProfileID, "segment_id": segmentID})
			return httpResp, nil
		}
		return httpResp, err
	}

	tflog.Info(ctx, "Reconciled Access Profile segment membership for Segment access", map[string]interface{}{"access_profile_id": accessProfileID, "segment_id": segmentID, "ensure_present": ensurePresent})
	return httpResp, nil
}

func (r *segmentAccessResource) readSegmentAssignments(ctx context.Context, segmentID string) ([]segmentAccessAssignment, diag.Diagnostics) {
	var diags diag.Diagnostics

	accessProfiles, httpResp, err := r.client.AccessProfilesAPI.
		ListAccessProfilesV1(ctx).
		ForSegmentIds(segmentID).
		IncludeUnsegmented(false).
		Execute()
	if err != nil {
		tflog.Error(ctx, "Error listing Access Profiles for Segment access read", map[string]interface{}{"segment_id": segmentID, "error": err.Error()})
		diags.AddError("Error reading Segment access", segmentAccessErrDetail(err, httpResp))
		return nil, diags
	}

	roles, httpResp, err := r.client.RolesAPI.
		ListRolesV1(ctx).
		ForSegmentIds(segmentID).
		IncludeUnsegmented(false).
		Execute()
	if err != nil {
		tflog.Error(ctx, "Error listing Roles for Segment access read", map[string]interface{}{"segment_id": segmentID, "error": err.Error()})
		diags.AddError("Error reading Segment access", segmentAccessErrDetail(err, httpResp))
		return nil, diags
	}

	items := make([]segmentAccessAssignment, 0, len(accessProfiles)+len(roles))
	for i := range accessProfiles {
		if accessProfiles[i].HasId() {
			items = append(items, segmentAccessAssignment{Type: segmentAccessTypeAccessProfile, ID: accessProfiles[i].GetId()})
		}
	}
	// Deliberately exclude ENTITLEMENTs here even though the reference provider
	// lists them: the only real write path for this resource is PATCHing
	// `/segments` on Roles and Access Profiles, so including ENTITLEMENT in Read
	// would create permanent uncorrectable drift.
	for i := range roles {
		if roles[i].HasId() {
			items = append(items, segmentAccessAssignment{Type: segmentAccessTypeRole, ID: roles[i].GetId()})
		}
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Type == items[j].Type {
			return items[i].ID < items[j].ID
		}
		return items[i].Type < items[j].Type
	})

	return items, diags
}

func segmentAccessAssignmentsFromSet(ctx context.Context, set types.Set) ([]segmentAccessAssignment, diag.Diagnostics) {
	var diags diag.Diagnostics
	if set.IsNull() || set.IsUnknown() {
		return nil, diags
	}

	var models []segmentAccessAssignmentModel
	diags.Append(set.ElementsAs(ctx, &models, false)...)
	if diags.HasError() {
		return nil, diags
	}

	items := make([]segmentAccessAssignment, 0, len(models))
	for _, model := range models {
		items = append(items, segmentAccessAssignment{
			Type: model.Type.ValueString(),
			ID:   model.Id.ValueString(),
		})
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Type == items[j].Type {
			return items[i].ID < items[j].ID
		}
		return items[i].Type < items[j].Type
	})

	return items, diags
}

func segmentAccessAssignmentsToSet(items []segmentAccessAssignment) (types.Set, diag.Diagnostics) {
	var diags diag.Diagnostics
	objectType := types.ObjectType{AttrTypes: segmentAccessAssignmentAttrTypes()}
	elements := make([]attr.Value, 0, len(items))

	for _, item := range items {
		obj, d := types.ObjectValue(segmentAccessAssignmentAttrTypes(), map[string]attr.Value{
			"type": types.StringValue(item.Type),
			"id":   types.StringValue(item.ID),
		})
		diags.Append(d...)
		if diags.HasError() {
			return types.Set{}, diags
		}
		elements = append(elements, obj)
	}

	set, d := types.SetValue(objectType, elements)
	diags.Append(d...)
	return set, diags
}

func segmentAccessAssignmentAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"type": types.StringType,
		"id":   types.StringType,
	}
}

func diffSegmentAccessAssignments(current, desired []segmentAccessAssignment) (toAssign, toRemove []segmentAccessAssignment) {
	currentSet := make(map[string]struct{}, len(current))
	for _, item := range current {
		currentSet[item.key()] = struct{}{}
	}

	desiredSet := make(map[string]struct{}, len(desired))
	for _, item := range desired {
		desiredSet[item.key()] = struct{}{}
		if _, ok := currentSet[item.key()]; !ok {
			toAssign = append(toAssign, item)
		}
	}

	for _, item := range current {
		if _, ok := desiredSet[item.key()]; !ok {
			toRemove = append(toRemove, item)
		}
	}

	return toAssign, toRemove
}

func segmentAccessDesiredSegments(current []string, segmentID string, ensurePresent bool) ([]string, bool) {
	if ensurePresent {
		return segmentAccessAddSegment(current, segmentID)
	}
	return segmentAccessRemoveSegment(current, segmentID)
}

func segmentAccessAddSegment(current []string, segmentID string) ([]string, bool) {
	for _, existing := range current {
		if existing == segmentID {
			return append([]string(nil), current...), false
		}
	}
	updated := append(append([]string(nil), current...), segmentID)
	return updated, true
}

func segmentAccessRemoveSegment(current []string, segmentID string) ([]string, bool) {
	updated := make([]string, 0, len(current))
	changed := false
	for _, existing := range current {
		if existing == segmentID {
			changed = true
			continue
		}
		updated = append(updated, existing)
	}
	if !changed {
		return append([]string(nil), current...), false
	}
	return updated, true
}

func segmentAccessRoleSegmentsPatch(segments []string) []roles.JsonPatchOperation {
	arr := make([]roles.ArrayInner, 0, len(segments))
	for _, segmentID := range segments {
		segment := segmentID
		arr = append(arr, roles.ArrayInner{String: &segment})
	}
	value := roles.ArrayOfArrayInnerAsJsonPatchOperationValue(&arr)
	return []roles.JsonPatchOperation{
		{
			Op:    "replace",
			Path:  "/segments",
			Value: &value,
		},
	}
}

func segmentAccessAccessProfileSegmentsPatch(segments []string) []access_profiles.JsonPatchOperation {
	arr := make([]access_profiles.ArrayInner, 0, len(segments))
	for _, segmentID := range segments {
		segment := segmentID
		arr = append(arr, access_profiles.ArrayInner{String: &segment})
	}
	value := access_profiles.ArrayOfArrayInnerAsJsonPatchOperationValue(&arr)
	return []access_profiles.JsonPatchOperation{
		{
			Op:    "replace",
			Path:  "/segments",
			Value: &value,
		},
	}
}

func (a segmentAccessAssignment) key() string {
	return a.Type + "\x00" + a.ID
}

func segmentAccessErrDetail(err error, httpResp *http.Response) string {
	return util.SailpointErrorDetail(err, httpResp)
}
