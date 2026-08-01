// Package application_access_association_v1 implements a fully hand-written
// Terraform resource for managing a subset of access profile assignments on an
// existing IdentityNow/ISC Application.
//
// This target intentionally does not use the repo's OpenAPI codegen pipeline:
// SailPoint does not expose a dedicated "application access association"
// object/endpoint to CRUD. Instead, the only write surface is the existing
// Application PATCH endpoint's `/accessProfiles` field on
// `PATCH /source-apps/v1/{id}` - the same underlying API surface already used
// by `identitynow_application_v1`.
//
// Unlike `identitynow_application_v1`'s `access_profile_ids` attribute, this
// resource is deliberately additive/subset-scoped so multiple Terraform
// resources/configurations can each own part of an application's access profile
// membership without clobbering one another:
//   - Create unions this resource's desired ids with the application's current
//     full live access profile list, then PATCHes the full union back.
//   - Read self-heals by intersecting this resource's tracked ids with the
//     current live list, shrinking state only for ids that truly disappeared.
//   - Update removes only this resource's previously tracked ids from the live
//     list, then adds the new desired ids back.
//   - Delete removes only this resource's tracked ids, leaving everything else
//     on the application untouched.
package application_access_association_v1

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	sailpoint "github.com/sailpoint-oss/golang-sdk/v3"
	"github.com/sailpoint-oss/golang-sdk/v3/apps"

	"terraform-provider-identitynow/internal/provider/util"
)

const (
	applicationAccessAssociationListMaxLimit = 250
	importIDPartCount                        = 2
)

var (
	_ resource.Resource                = (*applicationAccessAssociationResource)(nil)
	_ resource.ResourceWithConfigure   = (*applicationAccessAssociationResource)(nil)
	_ resource.ResourceWithImportState = (*applicationAccessAssociationResource)(nil)
)

// clientProvider is satisfied by internal/provider.identitynowProvider without
// this package needing to import it (which would create an import cycle).
type clientProvider interface {
	GetClient() *sailpoint.APIClient
}

type applicationAccessAssociationResourceModel struct {
	Id               types.String `tfsdk:"id"`
	ApplicationID    types.String `tfsdk:"application_id"`
	AccessProfileIDs types.Set    `tfsdk:"access_profile_ids"`
}

type parsedImportState struct {
	ApplicationID    string
	AccessProfileIDs []string
}

func NewApplicationAccessAssociationResource() resource.Resource {
	return &applicationAccessAssociationResource{}
}

type applicationAccessAssociationResource struct {
	client *sailpoint.APIClient
}

func (r *applicationAccessAssociationResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_application_access_association_v1"
}

func (r *applicationAccessAssociationResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceschema.Schema{
		Description: "Manages an additive subset of Access Profile assignments on an existing Application in IdentityNow/ISC.",
		MarkdownDescription: "Manages an additive subset of [Access Profile](https://documentation.sailpoint.com/saas/help/access/access-profiles.html) " +
			"assignments on an existing [Application](https://documentation.sailpoint.com/) in IdentityNow/ISC. " +
			"This is a fully hand-written `_v1` resource with no OpenAPI codegen: it reuses the same application PATCH `/accessProfiles` " +
			"API surface as `identitynow_application_v1`, but with merge/remove-only-own-ids semantics so multiple Terraform resources can " +
			"independently manage different slices of the same application's access profile associations.",
		Attributes: map[string]resourceschema.Attribute{
			"id": resourceschema.StringAttribute{
				Computed:            true,
				Description:         "Same value as application_id.",
				MarkdownDescription: "Same value as `application_id`.",
			},
			"application_id": resourceschema.StringAttribute{
				Required:            true,
				Description:         "ID of the Application whose Access Profile associations are managed.",
				MarkdownDescription: "ID of the Application whose Access Profile associations are managed. Changing this forces replacement.",
			},
			"access_profile_ids": resourceschema.SetAttribute{
				Required:            true,
				ElementType:         types.StringType,
				Description:         "Subset of Access Profile IDs that this resource instance is responsible for contributing to the Application.",
				MarkdownDescription: "Subset of Access Profile IDs that this resource instance is responsible for contributing to the Application. This is **not** necessarily the application's full live access profile set.",
			},
		},
	}

	applyApplicationAccessAssociationPlanModifiers(&resp.Schema)
}

func (r *applicationAccessAssociationResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *applicationAccessAssociationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan applicationAccessAssociationResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	plan.Id = plan.ApplicationID

	desiredIDs, diags := stringSetToStrings(ctx, plan.AccessProfileIDs)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	currentIDs, httpResp, err := listApplicationAccessProfileIDs(ctx, r.client, plan.ApplicationID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error listing Application access profiles", applicationAccessAssociationErrDetail(err, httpResp))
		return
	}

	mergedIDs := mergeAccessProfileIDsForCreate(currentIDs, desiredIDs)

	tflog.Debug(ctx, "Creating Application access association", map[string]interface{}{
		"application_id":       plan.ApplicationID.ValueString(),
		"tracked_ids":          len(desiredIDs),
		"patched_total_ids":    len(mergedIDs),
		"existing_total_ids":   len(currentIDs),
		"operation_semantics":  "additive-union",
		"json_patch_operation": "add",
	})

	if err := r.patchApplicationAccessProfiles(ctx, plan.ApplicationID.ValueString(), mergedIDs); err != nil {
		resp.Diagnostics.AddError("Error creating Application access association", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *applicationAccessAssociationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state applicationAccessAssociationResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	state.Id = state.ApplicationID

	currentIDs, httpResp, err := listApplicationAccessProfileIDs(ctx, r.client, state.ApplicationID.ValueString())
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == http.StatusNotFound {
			tflog.Warn(ctx, "Application not found during Application access association read; removing from state", map[string]interface{}{
				"application_id": state.ApplicationID.ValueString(),
			})
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading Application access association", applicationAccessAssociationErrDetail(err, httpResp))
		return
	}

	trackedIDs, diags := stringSetToStrings(ctx, state.AccessProfileIDs)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	retainedIDs := retainManagedAccessProfileIDs(currentIDs, trackedIDs)
	accessProfileSet, diags := types.SetValueFrom(ctx, types.StringType, retainedIDs)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.AccessProfileIDs = accessProfileSet

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *applicationAccessAssociationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan applicationAccessAssociationResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state applicationAccessAssociationResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	plan.Id = plan.ApplicationID

	currentIDs, httpResp, err := listApplicationAccessProfileIDs(ctx, r.client, plan.ApplicationID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error listing Application access profiles", applicationAccessAssociationErrDetail(err, httpResp))
		return
	}

	oldTrackedIDs, diags := stringSetToStrings(ctx, state.AccessProfileIDs)
	resp.Diagnostics.Append(diags...)
	newTrackedIDs, diags := stringSetToStrings(ctx, plan.AccessProfileIDs)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	mergedIDs := mergeAccessProfileIDsForUpdate(currentIDs, oldTrackedIDs, newTrackedIDs)

	tflog.Debug(ctx, "Updating Application access association", map[string]interface{}{
		"application_id":      plan.ApplicationID.ValueString(),
		"old_tracked_ids":     len(oldTrackedIDs),
		"new_tracked_ids":     len(newTrackedIDs),
		"existing_total_ids":  len(currentIDs),
		"patched_total_ids":   len(mergedIDs),
		"operation_semantics": "strip-old-then-add-new",
	})

	if err := r.patchApplicationAccessProfiles(ctx, plan.ApplicationID.ValueString(), mergedIDs); err != nil {
		resp.Diagnostics.AddError("Error updating Application access association", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *applicationAccessAssociationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state applicationAccessAssociationResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	currentIDs, httpResp, err := listApplicationAccessProfileIDs(ctx, r.client, state.ApplicationID.ValueString())
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == http.StatusNotFound {
			return
		}
		resp.Diagnostics.AddError("Error listing Application access profiles", applicationAccessAssociationErrDetail(err, httpResp))
		return
	}

	trackedIDs, diags := stringSetToStrings(ctx, state.AccessProfileIDs)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	remainingIDs := removeAccessProfileIDs(currentIDs, trackedIDs)

	tflog.Debug(ctx, "Deleting Application access association", map[string]interface{}{
		"application_id":      state.ApplicationID.ValueString(),
		"tracked_ids":         len(trackedIDs),
		"existing_total_ids":  len(currentIDs),
		"remaining_total_ids": len(remainingIDs),
		"operation_semantics": "remove-only-own-ids",
	})

	if err := r.patchApplicationAccessProfiles(ctx, state.ApplicationID.ValueString(), remainingIDs); err != nil {
		resp.Diagnostics.AddError("Error deleting Application access association", err.Error())
	}
}

func (r *applicationAccessAssociationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parsed, err := parseImportStateID(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import identifier", err.Error())
		return
	}

	accessProfileSet, diags := types.SetValueFrom(ctx, types.StringType, parsed.AccessProfileIDs)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	state := applicationAccessAssociationResourceModel{
		Id:               types.StringValue(parsed.ApplicationID),
		ApplicationID:    types.StringValue(parsed.ApplicationID),
		AccessProfileIDs: accessProfileSet,
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *applicationAccessAssociationResource) patchApplicationAccessProfiles(ctx context.Context, applicationID string, ids []string) error {
	arr := make([]apps.ArrayInner, 0, len(ids))
	for i := range ids {
		id := ids[i]
		arr = append(arr, apps.ArrayInner{String: &id})
	}

	patch := []apps.JsonPatchOperation{
		applicationAccessAssociationJSONPatchAdd(
			"/accessProfiles",
			apps.ArrayOfArrayInnerAsJsonPatchOperationValue(&arr),
		),
	}

	_, httpResp, err := r.client.AppsAPI.
		PatchSourceAppV1(ctx, applicationID).
		JsonPatchOperation(patch).
		Execute()
	if err != nil {
		return errors.New(applicationAccessAssociationErrDetail(err, httpResp))
	}

	return nil
}

func listApplicationAccessProfileIDs(ctx context.Context, client *sailpoint.APIClient, applicationID string) ([]string, *http.Response, error) {
	ids := make([]string, 0)

	var offset int32
	for {
		page, httpResp, err := client.AppsAPI.
			ListAccessProfilesForSourceAppV1(ctx, applicationID).
			Limit(applicationAccessAssociationListMaxLimit).
			Offset(offset).
			Execute()
		if err != nil {
			return nil, httpResp, err
		}

		for i := range page {
			if page[i].Id != nil && *page[i].Id != "" {
				ids = append(ids, *page[i].Id)
			}
		}

		if len(page) < applicationAccessAssociationListMaxLimit {
			return ids, nil, nil
		}
		offset += applicationAccessAssociationListMaxLimit
	}
}

func stringSetToStrings(ctx context.Context, s types.Set) ([]string, diag.Diagnostics) {
	if s.IsNull() || s.IsUnknown() {
		return nil, nil
	}

	var ids []string
	diags := s.ElementsAs(ctx, &ids, false)
	return ids, diags
}

func mergeAccessProfileIDsForCreate(current, desired []string) []string {
	return unionAccessProfileIDs(current, desired)
}

func mergeAccessProfileIDsForUpdate(current, oldTracked, newTracked []string) []string {
	return unionAccessProfileIDs(removeAccessProfileIDs(current, oldTracked), newTracked)
}

func retainManagedAccessProfileIDs(current, tracked []string) []string {
	currentSet := make(map[string]struct{}, len(current))
	for _, id := range current {
		if id != "" {
			currentSet[id] = struct{}{}
		}
	}

	retained := make([]string, 0, len(tracked))
	seen := make(map[string]struct{}, len(tracked))
	for _, id := range tracked {
		if id == "" {
			continue
		}
		if _, duplicate := seen[id]; duplicate {
			continue
		}
		if _, ok := currentSet[id]; ok {
			retained = append(retained, id)
			seen[id] = struct{}{}
		}
	}

	return retained
}

func removeAccessProfileIDs(current, toRemove []string) []string {
	removeSet := make(map[string]struct{}, len(toRemove))
	for _, id := range toRemove {
		if id != "" {
			removeSet[id] = struct{}{}
		}
	}

	remaining := make([]string, 0, len(current))
	seen := make(map[string]struct{}, len(current))
	for _, id := range current {
		if id == "" {
			continue
		}
		if _, remove := removeSet[id]; remove {
			continue
		}
		if _, duplicate := seen[id]; duplicate {
			continue
		}
		remaining = append(remaining, id)
		seen[id] = struct{}{}
	}

	return remaining
}

func unionAccessProfileIDs(parts ...[]string) []string {
	total := 0
	for _, part := range parts {
		total += len(part)
	}

	union := make([]string, 0, total)
	seen := make(map[string]struct{}, total)
	for _, part := range parts {
		for _, id := range part {
			if id == "" {
				continue
			}
			if _, duplicate := seen[id]; duplicate {
				continue
			}
			union = append(union, id)
			seen[id] = struct{}{}
		}
	}

	return union
}

func parseImportStateID(id string) (parsedImportState, error) {
	parts := strings.Split(id, ",")
	if len(parts) != importIDPartCount {
		return parsedImportState{}, fmt.Errorf(
			"expected import id in the format <application_id>,<access_profile_id>/<access_profile_id>/...; got %q",
			id,
		)
	}

	applicationID := strings.TrimSpace(parts[0])
	if applicationID == "" {
		return parsedImportState{}, fmt.Errorf("application_id component must not be empty")
	}

	rawAccessProfileIDs := strings.TrimSpace(parts[1])
	if rawAccessProfileIDs == "" {
		return parsedImportState{}, fmt.Errorf("access_profile_ids component must not be empty")
	}

	accessProfileIDs := make([]string, 0)
	seen := make(map[string]struct{})
	for _, rawID := range strings.Split(rawAccessProfileIDs, "/") {
		accessProfileID := strings.TrimSpace(rawID)
		if accessProfileID == "" {
			return parsedImportState{}, fmt.Errorf("access_profile_ids component contains an empty access profile id")
		}
		if _, duplicate := seen[accessProfileID]; duplicate {
			continue
		}
		accessProfileIDs = append(accessProfileIDs, accessProfileID)
		seen[accessProfileID] = struct{}{}
	}

	return parsedImportState{
		ApplicationID:    applicationID,
		AccessProfileIDs: accessProfileIDs,
	}, nil
}

func applicationAccessAssociationErrDetail(err error, httpResp *http.Response) string {
	return util.SailpointErrorDetail(err, httpResp)
}

func applicationAccessAssociationJSONPatchAdd(path string, value apps.JsonPatchOperationValue) apps.JsonPatchOperation {
	return apps.JsonPatchOperation{
		Op:    "add",
		Path:  path,
		Value: &value,
	}
}
