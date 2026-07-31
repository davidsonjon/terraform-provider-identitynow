package access_model_metadata_attribute_v1

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	sailpoint "github.com/sailpoint-oss/golang-sdk/v2"
	"github.com/sailpoint-oss/golang-sdk/v2/api_beta"
)

// deleteAccessModelMetadataAttribute issues a raw DELETE
// /access-model-metadata/attributes/{key} request against the beta API
// surface.
//
// Why this exists instead of a generated SDK call: DELETE
// /access-model-metadata/attributes/{key} is a REAL, working endpoint in
// IdentityNow/ISC (confirmed 2026-07-26 via a captured browser network call
// from the Admin > Access Model > Metadata UI itself), but it is MISSING
// from SailPoint's published `api-specs` OpenAPI document for this service
// (both the legacy `beta` spec and the newer per-service `v1` spec omit it),
// and therefore `golang-sdk`'s generated `AccessModelMetadataAPIService` has
// no `DeleteAccessModelMetadataAttribute` method at all - this is a
// spec/SDK documentation gap, NOT a real API limitation (contrast with the
// genuinely-undocumented-by-design `ProvisioningConfigManagedResourceRefs`
// SDK typing defect noted elsewhere in this provider's agent knowledge base,
// where the upstream defect is in a field type, not a whole missing
// operation).
//
// This hand-rolls the HTTP call using only the SDK's already-exported
// configuration surface (github.com/sailpoint-oss/golang-sdk/v2/api_beta's
// `Configuration.BaseURL`/`ClientId`/`ClientSecret`/`TokenURL`/`Token`/
// `HTTPClient`, all exported fields) so it stays a legitimate, no-fork
// extension of the SDK rather than vendoring or forking any generated code.
// It deliberately mirrors api_beta's own unexported `prepareRequest`/
// `callAPI`/`getAccessToken` request-shape and auth-token-caching logic
// (same header names, same client-credentials form-POST shape, same
// cfg.Token read/write caching pattern) so behavior stays consistent with
// every other generated call this resource also makes.
//
// If SailPoint ever adds this operation to the published OpenAPI spec (and
// therefore a future golang-sdk release generates a real
// `DeleteAccessModelMetadataAttribute` method), this helper should be
// deleted and replaced with the generated call - check for that on any
// future `golang-sdk` version bump for this target.
func deleteAccessModelMetadataAttribute(ctx context.Context, client *sailpoint.APIClient, key string) (*http.Response, error) {
	cfg := client.Beta.GetConfig()

	token, err := betaBearerToken(cfg)
	if err != nil {
		return nil, fmt.Errorf("could not obtain bearer token: %w", err)
	}

	reqURL := strings.TrimSuffix(cfg.BaseURL, "/") + "/access-model-metadata/attributes/" + url.PathEscape(key)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("could not build DELETE request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	httpClient := cfg.HTTPClient.StandardClient()
	return httpClient.Do(req)
}

// betaBearerToken returns a valid bearer token for the Beta API client,
// reusing the SDK's own cached cfg.Token if one has already been fetched by
// a prior generated-SDK call on this same client (the common case, since
// Delete() always runs after at least one Create/Read/Update on the same
// resource), and otherwise fetching+caching a fresh one via the standard
// OAuth2 client-credentials flow - the exact request shape used internally
// by api_beta's own unexported getAccessToken.
func betaBearerToken(cfg *api_beta.Configuration) (string, error) {
	if cfg.Token != "" {
		return cfg.Token, nil
	}
	if cfg.ClientId == "" || cfg.ClientSecret == "" || cfg.TokenURL == "" {
		return "", fmt.Errorf("no cached token and no client credentials available to fetch one")
	}

	form := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {cfg.ClientId},
		"client_secret": {cfg.ClientSecret},
	}
	req, err := http.NewRequest(http.MethodPost, cfg.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var tok struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &tok); err != nil {
		return "", fmt.Errorf("could not parse token response: %w", err)
	}
	if tok.AccessToken == "" {
		return "", fmt.Errorf("token endpoint returned an empty access_token (status %s)", resp.Status)
	}

	cfg.Token = tok.AccessToken
	return cfg.Token, nil
}
