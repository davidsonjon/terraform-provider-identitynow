package service_desk_integration_v1

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/sailpoint-oss/golang-sdk/v3/service_desk_integration"
)

// This file works around a confirmed upstream defect that was present in
// github.com/sailpoint-oss/golang-sdk/v2 (in the pinned v2.5.1 and still
// present in the last v2 release, v2.7.106, as of 2026-07-24 - see the
// identitynow-terraform-provider-developer knowledge file for the full repro):
// the managedResourceRefs inner type's Type/Id/Name were declared as
// map[string]interface{} instead of string, even though the real API always
// returns them as strings. Any Service Desk Integration with a non-empty
// provisioningConfig.managedResourceRefs therefore failed to unmarshal inside
// the SDK's own Execute() call (a 200 HTTP response still surfaced as a
// non-nil Go error), breaking Read/Import/Create/Update for any such object.
//
// NOTE (golang-sdk v3 migration, 2026): v3 FIXES this upstream - the type is
// now service_desk_integration.ServiceDeskSource with Type/Id/Name declared as
// *string. This fallback is therefore now a dead code path on v3 (the SDK's
// own Execute() unmarshals managedResourceRefs cleanly and the bug-detection
// below never fires). It is retained as harmless defensive code; it can be
// removed as a follow-up cleanup once verified against a live tenant. See the
// SDK issues log (issue #1, marked Resolved-in-v3).
//
// This project's hand-written CRUD code never reads
// provisioningConfig.managedResourceRefs (see the package doc comment - it is
// treated as pass-through only), so the fallback below simply re-decodes the
// response body with locally-defined, correctly-typed structs and discards
// managedResourceRefs entirely, rather than attempting to fix or vendor the
// upstream type.

// rawServiceDeskIntegrationDto mirrors the real API response shape for a
// Service Desk Integration, with a correctly-typed provisioningConfig (see
// rawProvisioningConfig) in place of service_desk_integration.ProvisioningConfig.
type rawServiceDeskIntegrationDto struct {
	Name                   string                                              `json:"name"`
	Description            string                                              `json:"description"`
	Type                   string                                              `json:"type"`
	OwnerRef               *service_desk_integration.OwnerDto                  `json:"ownerRef,omitempty"`
	ClusterRef             *service_desk_integration.SourceClusterDto          `json:"clusterRef,omitempty"`
	Cluster                *string                                             `json:"cluster,omitempty"`
	ManagedSources         []string                                            `json:"managedSources,omitempty"`
	ProvisioningConfig     *rawProvisioningConfig                              `json:"provisioningConfig,omitempty"`
	Attributes             map[string]interface{}                              `json:"attributes"`
	BeforeProvisioningRule *service_desk_integration.BeforeProvisioningRuleDto `json:"beforeProvisioningRule,omitempty"`
}

// rawProvisioningConfig mirrors service_desk_integration.ProvisioningConfig but intentionally
// omits managedResourceRefs (the field this project's code never reads, and
// the field whose element type is mistyped upstream - see file doc comment).
type rawProvisioningConfig struct {
	UniversalManager              *bool                                                             `json:"universalManager,omitempty"`
	PlanInitializerScript         *service_desk_integration.ProvisioningConfigPlanInitializerScript `json:"planInitializerScript,omitempty"`
	NoProvisioningRequests        *bool                                                             `json:"noProvisioningRequests,omitempty"`
	ProvisioningRequestExpiration *int32                                                            `json:"provisioningRequestExpiration,omitempty"`
}

// knownServiceDeskIntegrationJSONFields lists every field explicitly modeled
// by rawServiceDeskIntegrationDto, used to compute AdditionalProperties the
// same way service_desk_integration.ServiceDeskIntegrationDto.UnmarshalJSON does (notably
// capturing "id", which isn't a declared field on either struct).
var knownServiceDeskIntegrationJSONFields = []string{
	"name", "description", "type", "ownerRef", "clusterRef", "cluster",
	"managedSources", "provisioningConfig", "attributes", "beforeProvisioningRule",
}

// isManagedResourceRefsTypeBug reports whether err looks like the known
// golang-sdk defect described in this file's doc comment, as opposed to some
// other, unrelated decode failure that the fallback below cannot help with.
func isManagedResourceRefsTypeBug(err error) bool {
	return err != nil && strings.Contains(err.Error(), "managedResourceRefs")
}

// decodeServiceDeskIntegrationFallback re-decodes a Service Desk Integration
// API response body, working around the upstream golang-sdk defect. It
// deliberately drops provisioningConfig.managedResourceRefs (never read by
// this project's CRUD code) rather than attempting to correctly type it.
func decodeServiceDeskIntegrationFallback(body []byte) (*service_desk_integration.ServiceDeskIntegrationDto, error) {
	var raw rawServiceDeskIntegrationDto
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("fallback decode also failed: %w", err)
	}

	additionalProperties := map[string]interface{}{}
	if err := json.Unmarshal(body, &additionalProperties); err == nil {
		for _, k := range knownServiceDeskIntegrationJSONFields {
			delete(additionalProperties, k)
		}
	}

	dto := &service_desk_integration.ServiceDeskIntegrationDto{
		Name:                   raw.Name,
		Description:            raw.Description,
		Type:                   raw.Type,
		OwnerRef:               raw.OwnerRef,
		ClusterRef:             raw.ClusterRef,
		Cluster:                *service_desk_integration.NewNullableString(raw.Cluster),
		ManagedSources:         raw.ManagedSources,
		Attributes:             raw.Attributes,
		BeforeProvisioningRule: raw.BeforeProvisioningRule,
		AdditionalProperties:   additionalProperties,
	}
	if raw.ProvisioningConfig != nil {
		dto.ProvisioningConfig = &service_desk_integration.ProvisioningConfig{
			UniversalManager:              raw.ProvisioningConfig.UniversalManager,
			PlanInitializerScript:         *service_desk_integration.NewNullableProvisioningConfigPlanInitializerScript(raw.ProvisioningConfig.PlanInitializerScript),
			NoProvisioningRequests:        raw.ProvisioningConfig.NoProvisioningRequests,
			ProvisioningRequestExpiration: raw.ProvisioningConfig.ProvisioningRequestExpiration,
			// ManagedResourceRefs is intentionally left unset - see file doc comment.
		}
	}
	return dto, nil
}

// withManagedResourceRefsFallback wraps the (dto, httpResp, err) triple
// returned by the generated SDK's ServiceDeskIntegrationAPI Execute() calls.
// If err looks like the known managedResourceRefs type bug and the HTTP call
// actually succeeded, it re-decodes the response body locally instead of
// surfacing a spurious error for what was really a 2xx response.
func withManagedResourceRefsFallback(ctx context.Context, dto *service_desk_integration.ServiceDeskIntegrationDto, httpResp *http.Response, err error) (*service_desk_integration.ServiceDeskIntegrationDto, *http.Response, error) {
	if err == nil || httpResp == nil || httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return dto, httpResp, err
	}
	if !isManagedResourceRefsTypeBug(err) {
		return dto, httpResp, err
	}

	tflog.Debug(ctx, "Working around known golang-sdk managedResourceRefs type defect; re-decoding response body", map[string]interface{}{"sdk_error": err.Error()})

	body, readErr := io.ReadAll(httpResp.Body)
	if readErr != nil {
		tflog.Warn(ctx, "Could not read response body for managedResourceRefs fallback decode; surfacing original SDK error", map[string]interface{}{"read_error": readErr.Error()})
		return dto, httpResp, err
	}
	httpResp.Body = io.NopCloser(bytes.NewBuffer(body))

	fallbackDto, fallbackErr := decodeServiceDeskIntegrationFallback(body)
	if fallbackErr != nil {
		tflog.Warn(ctx, "managedResourceRefs fallback decode also failed; surfacing original SDK error", map[string]interface{}{"fallback_error": fallbackErr.Error()})
		return dto, httpResp, err
	}
	tflog.Debug(ctx, "managedResourceRefs fallback decode succeeded", nil)
	return fallbackDto, httpResp, nil
}
