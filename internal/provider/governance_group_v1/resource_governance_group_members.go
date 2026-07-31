// This file implements identitynow_governance_group_members_v1, a follow-up
// to the "Deferred" members sub-resource note in resource_governance_group.go's
// package doc.
//
// Unlike the parent identitynow_governance_group_v1 resource (which has
// standard single-item create/read/update/delete endpoints and therefore
// goes through the normal OpenAPI-codegen pipeline), the members API
// (/workgroups/v1/{workgroupId}/members[/bulk-add|/bulk-delete]) only
// exposes list + bulk-add + bulk-delete operations - there is no
// single-member get/patch/delete endpoint. This shape does not fit the
// codegen pipeline's create/read/update/delete-by-id assumptions at all, so
// this resource (schema, model, and CRUD) is entirely hand-written - there is
// no generator_config_governance_group_members_v1.yml and no *_gen.go
// package for it.
//
// Design: this is a "join" resource that owns the *complete* desired
// membership set for one governance group (one identitynow_governance_group_members_v1
// per governance group is the intended usage, analogous to
// aws_iam_group_membership). Create/Update reconcile the API's actual
// membership against the configured member_ids by diffing and calling
// bulk-add/bulk-delete only for the identities that actually need to change;
// Read always re-lists the API's current membership (so drift - e.g. a
// member removed out-of-band - is detected); Delete bulk-removes every
// member currently tracked in state.
//
// The top-level POST /workgroups/v1/bulk-delete (bulk *governance group*
// deletion, not member deletion) is intentionally not modeled anywhere in
// this provider - it is redundant for Terraform's purposes since the
// generated DELETE /workgroups/v1/{id} (already wired into
// governanceGroupResource.Delete) covers the same single-object-delete
// semantics Terraform actually needs.
//
// The read-only GET /workgroups/v1/{workgroupId}/connections endpoint is
// modeled separately as the identitynow_governance_group_connections_v1 data
// source (see datasource_governance_group_connections.go) - connections have
// no write endpoint at all (they are established from the referencing
// object's side, e.g. a role/access profile/SOD policy/source), so a
// data source is the only sensible shape for that endpoint.
package governance_group_v1

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	sailpoint "github.com/sailpoint-oss/golang-sdk/v2"
	"github.com/sailpoint-oss/golang-sdk/v2/api_beta"
)

var (
	_ resource.Resource                = (*governanceGroupMembersResource)(nil)
	_ resource.ResourceWithConfigure   = (*governanceGroupMembersResource)(nil)
	_ resource.ResourceWithImportState = (*governanceGroupMembersResource)(nil)
)

func NewGovernanceGroupMembersResource() resource.Resource {
	return &governanceGroupMembersResource{}
}

type governanceGroupMembersResource struct {
	client *sailpoint.APIClient
}

// GovernanceGroupMembersModel is entirely hand-written - see the package doc
// for why this resource has no generated schema/model counterpart.
type GovernanceGroupMembersModel struct {
	Id                types.String `tfsdk:"id"`
	GovernanceGroupId types.String `tfsdk:"governance_group_id"`
	MemberIds         types.Set    `tfsdk:"member_ids"`
}

func (r *governanceGroupMembersResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_governance_group_members_v1"
}

func (r *governanceGroupMembersResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages the complete set of identity members of a Governance Group in IdentityNow/ISC.",
		MarkdownDescription: "Manages the complete set of identity members of a [Governance Group]" +
			"(https://documentation.sailpoint.com/saas/help/common/governance_groups.html) in IdentityNow/ISC, via " +
			"`POST /workgroups/v1/{workgroupId}/members/bulk-add` and `POST /workgroups/v1/{workgroupId}/members/bulk-delete`.\n\n" +
			"~> This is a `_v1` pilot resource. Exactly one `identitynow_governance_group_members_v1` resource should be " +
			"created per governance group - it manages the *entire* membership list, reconciling it to match `member_ids` " +
			"exactly (including removing any members not listed) on every apply. Only identity members are supported " +
			"(the underlying API's `type` field only accepts `IDENTITY`).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				Description:         "Same value as governance_group_id.",
				MarkdownDescription: "Same value as `governance_group_id`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"governance_group_id": schema.StringAttribute{
				Required:            true,
				Description:         "ID of the Governance Group whose membership is managed.",
				MarkdownDescription: "ID of the Governance Group whose membership is managed. Changing this forces replacement.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"member_ids": schema.SetAttribute{
				Required:    true,
				ElementType: types.StringType,
				Description: "Complete set of identity IDs that should be members of the Governance Group.",
				MarkdownDescription: "Complete set of identity IDs that should be members of the Governance Group. " +
					"This resource reconciles actual membership to exactly match this set on every apply.",
			},
		},
	}
}

func (r *governanceGroupMembersResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
	r.client = cp.GetClient()
}

func (r *governanceGroupMembersResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("governance_group_id"), req.ID)...)
}

func (r *governanceGroupMembersResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan GovernanceGroupMembersModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	workgroupID := plan.GovernanceGroupId.ValueString()
	desired, diags := setToStrings(ctx, plan.MemberIds)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Creating Governance Group members", map[string]interface{}{"governance_group_id": workgroupID, "count": len(desired)})

	if len(desired) > 0 {
		if _, err := r.addMembers(ctx, workgroupID, desired); err != nil {
			resp.Diagnostics.AddError("Error adding Governance Group members", err.Error())
			return
		}
	}

	state, diags := r.readState(ctx, workgroupID)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Created Governance Group members", map[string]interface{}{"governance_group_id": workgroupID})

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *governanceGroupMembersResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state GovernanceGroupMembersModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	workgroupID := state.GovernanceGroupId.ValueString()

	tflog.Debug(ctx, "Reading Governance Group members", map[string]interface{}{"governance_group_id": workgroupID})

	// If the parent governance group itself is gone, its members are gone too.
	if _, httpResp, err := r.client.Beta.GovernanceGroupsAPI.GetWorkgroup(ctx, workgroupID).Execute(); err != nil {
		if httpResp != nil && httpResp.StatusCode == 404 {
			tflog.Warn(ctx, "Parent Governance Group not found, removing members resource from state", map[string]interface{}{"governance_group_id": workgroupID})
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading parent Governance Group", errDetail(err, httpResp))
		return
	}

	newState, diags := r.readState(ctx, workgroupID)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *governanceGroupMembersResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan GovernanceGroupMembersModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state GovernanceGroupMembersModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	workgroupID := plan.GovernanceGroupId.ValueString()

	desired, diags := setToStrings(ctx, plan.MemberIds)
	resp.Diagnostics.Append(diags...)
	current, diags := setToStrings(ctx, state.MemberIds)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	toAdd, toRemove := diffMemberIds(current, desired)

	tflog.Debug(ctx, "Updating Governance Group members", map[string]interface{}{
		"governance_group_id": workgroupID,
		"to_add":              len(toAdd),
		"to_remove":           len(toRemove),
	})

	if len(toAdd) > 0 {
		if _, err := r.addMembers(ctx, workgroupID, toAdd); err != nil {
			resp.Diagnostics.AddError("Error adding Governance Group members", err.Error())
			return
		}
	}
	if len(toRemove) > 0 {
		if _, err := r.removeMembers(ctx, workgroupID, toRemove); err != nil {
			resp.Diagnostics.AddError("Error removing Governance Group members", err.Error())
			return
		}
	}

	newState, diags := r.readState(ctx, workgroupID)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Updated Governance Group members", map[string]interface{}{"governance_group_id": workgroupID})

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *governanceGroupMembersResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state GovernanceGroupMembersModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	workgroupID := state.GovernanceGroupId.ValueString()
	current, diags := setToStrings(ctx, state.MemberIds)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Deleting Governance Group members", map[string]interface{}{"governance_group_id": workgroupID, "count": len(current)})

	if len(current) > 0 {
		httpResp, err := r.removeMembers(ctx, workgroupID, current)
		if err != nil {
			// A 404 here means the parent governance group (and thus its
			// members) is already gone - not an error for Delete.
			if httpResp != nil && httpResp.StatusCode == 404 {
				tflog.Warn(ctx, "Governance Group already absent on member delete", map[string]interface{}{"governance_group_id": workgroupID})
				return
			}
			resp.Diagnostics.AddError("Error removing Governance Group members", err.Error())
			return
		}
	}

	tflog.Info(ctx, "Deleted Governance Group members", map[string]interface{}{"governance_group_id": workgroupID})
}

// addMembers bulk-adds identityIDs to workgroupID. Per-item statuses (e.g.
// 409 "already a member") are logged but not treated as fatal - the desired
// end state (identity is a member) is already satisfied in that case.
func (r *governanceGroupMembersResource) addMembers(ctx context.Context, workgroupID string, identityIDs []string) (*http.Response, error) {
	items := bulkMemberItems(identityIDs)

	results, httpResp, err := r.client.Beta.GovernanceGroupsAPI.
		UpdateWorkgroupMembers(ctx, workgroupID).
		BulkWorkgroupMembersRequestInner(items).
		Execute()
	if err != nil {
		return httpResp, fmt.Errorf("%s", errDetail(err, httpResp))
	}

	for _, item := range results {
		if item.Status != 201 && item.Status != 409 {
			desc := ""
			if item.Description != nil {
				desc = *item.Description
			}
			return httpResp, fmt.Errorf("failed to add member %q to Governance Group %q: HTTP %d %s", item.Id, workgroupID, item.Status, desc)
		}
	}
	return httpResp, nil
}

// removeMembers bulk-removes identityIDs from workgroupID. A per-item 404
// ("not a member") is treated as already-satisfied, not fatal.
func (r *governanceGroupMembersResource) removeMembers(ctx context.Context, workgroupID string, identityIDs []string) (*http.Response, error) {
	items := bulkMemberItems(identityIDs)

	results, httpResp, err := r.client.Beta.GovernanceGroupsAPI.
		DeleteWorkgroupMembers(ctx, workgroupID).
		BulkWorkgroupMembersRequestInner(items).
		Execute()
	if err != nil {
		return httpResp, fmt.Errorf("%s", errDetail(err, httpResp))
	}

	for _, item := range results {
		if item.Status != 204 && item.Status != 404 {
			desc := ""
			if item.Description != nil {
				desc = *item.Description
			}
			return httpResp, fmt.Errorf("failed to remove member %q from Governance Group %q: HTTP %d %s", item.Id, workgroupID, item.Status, desc)
		}
	}
	return httpResp, nil
}

// bulkMemberItems builds the bulk-add/bulk-delete request body shared by
// addMembers/removeMembers - both endpoints accept the identical
// BulkWorkgroupMembersRequestInner shape (type=IDENTITY, id).
func bulkMemberItems(identityIDs []string) []api_beta.BulkWorkgroupMembersRequestInner {
	items := make([]api_beta.BulkWorkgroupMembersRequestInner, 0, len(identityIDs))
	for _, id := range identityIDs {
		item := api_beta.NewBulkWorkgroupMembersRequestInnerWithDefaults()
		memberType := "IDENTITY"
		item.Type = &memberType
		memberID := id
		item.Id = &memberID
		items = append(items, *item)
	}
	return items
}

// readState lists workgroupID's current membership from the API (paginating
// through the full result set) and returns the resulting Terraform state.
func (r *governanceGroupMembersResource) readState(ctx context.Context, workgroupID string) (GovernanceGroupMembersModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	var memberIDs []string

	const pageLimit = 50 // GET .../members' documented maximum limit is 50, unlike most other v1 list endpoints (250).
	var offset int32
	for {
		page, httpResp, err := r.client.Beta.GovernanceGroupsAPI.
			ListWorkgroupMembers(ctx, workgroupID).
			Offset(offset).
			Limit(pageLimit).
			Execute()
		if err != nil {
			diags.AddError("Error listing Governance Group members", errDetail(err, httpResp))
			return GovernanceGroupMembersModel{}, diags
		}
		for _, m := range page {
			if m.Id != nil {
				memberIDs = append(memberIDs, *m.Id)
			}
		}
		if len(page) < pageLimit {
			break
		}
		offset += pageLimit
	}

	memberSet, d := types.SetValueFrom(ctx, types.StringType, memberIDs)
	diags.Append(d...)
	if diags.HasError() {
		return GovernanceGroupMembersModel{}, diags
	}

	return GovernanceGroupMembersModel{
		Id:                types.StringValue(workgroupID),
		GovernanceGroupId: types.StringValue(workgroupID),
		MemberIds:         memberSet,
	}, diags
}

// setToStrings extracts a []string from a types.Set of strings.
func setToStrings(ctx context.Context, s types.Set) ([]string, diag.Diagnostics) {
	var out []string
	if s.IsNull() || s.IsUnknown() {
		return out, nil
	}
	diags := s.ElementsAs(ctx, &out, false)
	return out, diags
}

// diffMemberIds returns the identity IDs present in desired but not current
// (toAdd), and present in current but not desired (toRemove).
func diffMemberIds(current, desired []string) (toAdd, toRemove []string) {
	currentSet := make(map[string]struct{}, len(current))
	for _, id := range current {
		currentSet[id] = struct{}{}
	}
	desiredSet := make(map[string]struct{}, len(desired))
	for _, id := range desired {
		desiredSet[id] = struct{}{}
	}

	for _, id := range desired {
		if _, ok := currentSet[id]; !ok {
			toAdd = append(toAdd, id)
		}
	}
	for _, id := range current {
		if _, ok := desiredSet[id]; !ok {
			toRemove = append(toRemove, id)
		}
	}
	return toAdd, toRemove
}
