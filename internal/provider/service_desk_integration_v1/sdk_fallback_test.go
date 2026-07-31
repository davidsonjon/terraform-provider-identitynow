package service_desk_integration_v1

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestIsManagedResourceRefsTypeBug(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil error", nil, false},
		{"unrelated error", errors.New("connection reset by peer"), false},
		{
			"matching error",
			errors.New(`json: cannot unmarshal object into Go struct field ProvisioningConfigManagedResourceRefsInner.provisioningConfig.managedResourceRefs.type of type string`),
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isManagedResourceRefsTypeBug(tt.err); got != tt.want {
				t.Errorf("isManagedResourceRefsTypeBug(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestDecodeServiceDeskIntegrationFallback(t *testing.T) {
	body := []byte(`{
		"id": "3c33c67491a940018bc8e93bce5ba239",
		"name": "test-sdi",
		"description": "a test integration",
		"type": "WEBHOOK",
		"ownerRef": {"type": "IDENTITY", "id": "owner-id", "name": "owner-name"},
		"clusterRef": {"type": "CLUSTER", "id": "cluster-id", "name": "cluster-name"},
		"cluster": "cluster-name",
		"managedSources": ["source-1", "source-2"],
		"attributes": {},
		"provisioningConfig": {
			"universalManager": true,
			"noProvisioningRequests": false,
			"provisioningRequestExpiration": 5,
			"managedResourceRefs": [
				{"type": "SOURCE", "id": "res-1", "name": "res-name-1"},
				{"type": "SOURCE", "id": "res-2", "name": "res-name-2"}
			]
		}
	}`)

	dto, err := decodeServiceDeskIntegrationFallback(body)
	if err != nil {
		t.Fatalf("decodeServiceDeskIntegrationFallback returned error: %v", err)
	}

	if dto.Name != "test-sdi" {
		t.Errorf("Name = %q, want %q", dto.Name, "test-sdi")
	}
	if dto.Description != "a test integration" {
		t.Errorf("Description = %q, want %q", dto.Description, "a test integration")
	}
	if dto.Type != "WEBHOOK" {
		t.Errorf("Type = %q, want %q", dto.Type, "WEBHOOK")
	}
	if dto.OwnerRef == nil || dto.OwnerRef.Id == nil || *dto.OwnerRef.Id != "owner-id" {
		t.Errorf("OwnerRef.Id = %v, want %q", dto.OwnerRef, "owner-id")
	}
	if dto.ClusterRef == nil || dto.ClusterRef.Id == nil || *dto.ClusterRef.Id != "cluster-id" {
		t.Errorf("ClusterRef.Id = %v, want %q", dto.ClusterRef, "cluster-id")
	}
	if len(dto.ManagedSources) != 2 || dto.ManagedSources[0] != "source-1" {
		t.Errorf("ManagedSources = %v, want [source-1 source-2]", dto.ManagedSources)
	}
	if dto.ProvisioningConfig == nil {
		t.Fatal("ProvisioningConfig is nil, want populated")
	}
	if dto.ProvisioningConfig.UniversalManager == nil || !*dto.ProvisioningConfig.UniversalManager {
		t.Errorf("ProvisioningConfig.UniversalManager = %v, want true", dto.ProvisioningConfig.UniversalManager)
	}
	if dto.ProvisioningConfig.ProvisioningRequestExpiration == nil || *dto.ProvisioningConfig.ProvisioningRequestExpiration != 5 {
		t.Errorf("ProvisioningConfig.ProvisioningRequestExpiration = %v, want 5", dto.ProvisioningConfig.ProvisioningRequestExpiration)
	}
	// managedResourceRefs must never survive the fallback decode - that's the
	// whole point of this workaround (the upstream field is mistyped and this
	// project never reads it).
	if dto.ProvisioningConfig.ManagedResourceRefs != nil {
		t.Errorf("ProvisioningConfig.ManagedResourceRefs = %v, want nil (must be dropped)", dto.ProvisioningConfig.ManagedResourceRefs)
	}

	// "id" is not a declared field on ServiceDeskIntegrationDto - it must land
	// in AdditionalProperties, mirroring the SDK's own UnmarshalJSON behavior.
	id, ok := dto.AdditionalProperties["id"].(string)
	if !ok || id != "3c33c67491a940018bc8e93bce5ba239" {
		t.Errorf("AdditionalProperties[id] = %v, want %q", dto.AdditionalProperties["id"], "3c33c67491a940018bc8e93bce5ba239")
	}
	// Known fields must not leak into AdditionalProperties.
	for _, known := range knownServiceDeskIntegrationJSONFields {
		if _, ok := dto.AdditionalProperties[known]; ok {
			t.Errorf("AdditionalProperties unexpectedly contains known field %q", known)
		}
	}
}

func TestDecodeServiceDeskIntegrationFallback_InvalidJSON(t *testing.T) {
	_, err := decodeServiceDeskIntegrationFallback([]byte(`not json`))
	if err == nil {
		t.Fatal("expected an error decoding invalid JSON, got nil")
	}
}

func TestWithManagedResourceRefsFallback(t *testing.T) {
	sdkBugErr := errors.New(`json: cannot unmarshal object into Go struct field ProvisioningConfigManagedResourceRefsInner.provisioningConfig.managedResourceRefs.type of type string`)
	unrelatedErr := errors.New("network error")
	body := `{"name": "test-sdi", "description": "d", "type": "WEBHOOK", "attributes": {}}`

	t.Run("nil error passes through unchanged", func(t *testing.T) {
		resp := &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}
		dto, gotResp, err := withManagedResourceRefsFallback(context.Background(), nil, resp, nil)
		if err != nil || dto != nil || gotResp != resp {
			t.Errorf("expected passthrough, got dto=%v resp=%v err=%v", dto, gotResp, err)
		}
	})

	t.Run("nil httpResp passes through unchanged", func(t *testing.T) {
		dto, gotResp, err := withManagedResourceRefsFallback(context.Background(), nil, nil, sdkBugErr)
		if err != sdkBugErr || dto != nil || gotResp != nil {
			t.Errorf("expected passthrough of original error, got dto=%v resp=%v err=%v", dto, gotResp, err)
		}
	})

	t.Run("non-2xx status passes through unchanged", func(t *testing.T) {
		resp := &http.Response{StatusCode: 500, Body: io.NopCloser(strings.NewReader(body))}
		dto, _, err := withManagedResourceRefsFallback(context.Background(), nil, resp, sdkBugErr)
		if err != sdkBugErr || dto != nil {
			t.Errorf("expected passthrough on 5xx, got dto=%v err=%v", dto, err)
		}
	})

	t.Run("unrelated error passes through unchanged", func(t *testing.T) {
		resp := &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}
		dto, _, err := withManagedResourceRefsFallback(context.Background(), nil, resp, unrelatedErr)
		if err != unrelatedErr || dto != nil {
			t.Errorf("expected passthrough of unrelated error, got dto=%v err=%v", dto, err)
		}
	})

	t.Run("known bug on 2xx triggers fallback decode", func(t *testing.T) {
		resp := &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}
		dto, _, err := withManagedResourceRefsFallback(context.Background(), nil, resp, sdkBugErr)
		if err != nil {
			t.Fatalf("expected fallback to succeed with nil error, got %v", err)
		}
		if dto == nil || dto.Name != "test-sdi" {
			t.Errorf("expected fallback-decoded dto with Name=test-sdi, got %v", dto)
		}
	})
}
