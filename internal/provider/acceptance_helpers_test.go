package provider

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"testing"
	"time"

	sailpoint "github.com/sailpoint-oss/golang-sdk/v2"
	"github.com/sailpoint-oss/golang-sdk/v2/api_beta"
)

const (
	testAccFixtureOwnerID           = "00001078bd9c497a8122c6fc3f3571b1"
	testAccFixtureSourceID          = "01f28e7f21804bef8565673ed668f36e"
	testAccFixtureGovernanceGroupID = "007c4fda-c531-436b-8290-44b2789ee58c"
	testAccRetryAttempts            = 10
	testAccRetryDelay               = 2 * time.Second
)

var (
	_ = testAccCreateApplicationFixture
	_ = testAccDeleteApplicationFixture
	_ = testAccListApplicationAccessProfileIDs
)

func testAccAPIClient() *sailpoint.APIClient {
	return sailpoint.NewAPIClient(sailpoint.NewDefaultConfiguration())
}

func testAccUniqueName(prefix string) string {
	return fmt.Sprintf("%s-%s", prefix, strconv.FormatInt(time.Now().UnixNano(), 36))
}

func testAccRetry(description string, fn func() (bool, error)) error {
	var lastErr error

	for attempt := 0; attempt < testAccRetryAttempts; attempt++ {
		if attempt > 0 {
			time.Sleep(testAccRetryDelay)
		}

		ok, err := fn()
		if err == nil && ok {
			return nil
		}
		if err != nil {
			lastErr = err
			continue
		}
		lastErr = fmt.Errorf("%s not yet observed", description)
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("%s not yet observed", description)
	}
	return fmt.Errorf("%s: %w", description, lastErr)
}

func testAccContainsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func testAccCreateRoleFixture(t *testing.T, client *sailpoint.APIClient, name, description string) string {
	t.Helper()

	ctx := context.Background()
	ownerType := "IDENTITY"
	ownerID := testAccFixtureOwnerID
	dto := api_beta.NewRole(name, api_beta.OwnerReference{
		Id:   &ownerID,
		Type: &ownerType,
	})
	dto.SetDescription(description)
	dto.SetEnabled(true)
	dto.SetRequestable(false)
	dto.SetAccessProfiles([]api_beta.AccessProfileRef{})
	dto.SetDimensionRefs([]api_beta.DimensionRef{})
	dto.SetEntitlements([]api_beta.EntitlementRef{})
	dto.SetAdditionalOwners([]api_beta.AdditionalOwnerRef{})
	dto.SetSegments([]string{})

	created, httpResp, err := client.Beta.RolesAPI.CreateRole(ctx).Role(*dto).Execute()
	if err != nil {
		t.Fatalf("creating role fixture %q: %s", name, testAccHTTPError(err, httpResp))
	}
	if created == nil || created.Id == nil || *created.Id == "" {
		t.Fatalf("creating role fixture %q returned no id", name)
	}
	return *created.Id
}

func testAccDeleteRoleFixture(t *testing.T, client *sailpoint.APIClient, roleID string) {
	t.Helper()
	if roleID == "" {
		return
	}

	httpResp, err := client.Beta.RolesAPI.DeleteRole(context.Background(), roleID).Execute()
	if err != nil && (httpResp == nil || httpResp.StatusCode != http.StatusNotFound) {
		t.Fatalf("deleting role fixture %s: %s", roleID, testAccHTTPError(err, httpResp))
	}
}

func testAccCreateAccessProfileFixture(t *testing.T, client *sailpoint.APIClient, name, description string) string {
	t.Helper()

	ctx := context.Background()
	ownerType := "IDENTITY"
	ownerID := testAccFixtureOwnerID
	sourceType := "SOURCE"
	sourceID := testAccFixtureSourceID

	dto := api_beta.NewAccessProfile(name, api_beta.OwnerReference{
		Id:   &ownerID,
		Type: &ownerType,
	}, api_beta.AccessProfileSourceRef{
		Id:   &sourceID,
		Type: &sourceType,
	})
	dto.SetDescription(description)
	dto.SetEnabled(false)
	dto.SetRequestable(false)
	dto.SetEntitlements([]api_beta.EntitlementRef{})
	dto.SetAdditionalOwners([]api_beta.AdditionalOwnerRef{})
	dto.SetSegments([]string{})

	created, httpResp, err := client.Beta.AccessProfilesAPI.CreateAccessProfile(ctx).AccessProfile(*dto).Execute()
	if err != nil {
		t.Fatalf("creating access profile fixture %q: %s", name, testAccHTTPError(err, httpResp))
	}
	if created == nil || created.Id == nil || *created.Id == "" {
		t.Fatalf("creating access profile fixture %q returned no id", name)
	}
	return *created.Id
}

func testAccDeleteAccessProfileFixture(t *testing.T, client *sailpoint.APIClient, accessProfileID string) {
	t.Helper()
	if accessProfileID == "" {
		return
	}

	httpResp, err := client.Beta.AccessProfilesAPI.DeleteAccessProfile(context.Background(), accessProfileID).Execute()
	if err != nil && (httpResp == nil || httpResp.StatusCode != http.StatusNotFound) {
		t.Fatalf("deleting access profile fixture %s: %s", accessProfileID, testAccHTTPError(err, httpResp))
	}
}

func testAccCreateSegmentFixture(t *testing.T, client *sailpoint.APIClient, name, description string) string {
	t.Helper()

	dto := api_beta.NewSegmentWithDefaults()
	dto.SetName(name)
	dto.SetDescription(description)
	dto.SetActive(true)

	operator := "EQUALS"
	attribute := "uid"
	valueType := "STRING"
	value := testAccUniqueName("does-not-exist")
	dto.SetVisibilityCriteria(api_beta.VisibilityCriteria{
		Expression: &api_beta.Expression{
			Operator:  &operator,
			Attribute: *api_beta.NewNullableString(&attribute),
			Value: *api_beta.NewNullableValue(&api_beta.Value{
				Type:  *api_beta.NewNullableString(&valueType),
				Value: &value,
			}),
		},
	})

	created, httpResp, err := client.Beta.SegmentsAPI.CreateSegment(context.Background()).Segment(*dto).Execute()
	if err != nil {
		t.Fatalf("creating segment fixture %q: %s", name, testAccHTTPError(err, httpResp))
	}
	if created == nil || created.Id == nil || *created.Id == "" {
		t.Fatalf("creating segment fixture %q returned no id", name)
	}
	return *created.Id
}

func testAccDeleteSegmentFixture(t *testing.T, client *sailpoint.APIClient, segmentID string) {
	t.Helper()
	if segmentID == "" {
		return
	}

	httpResp, err := client.Beta.SegmentsAPI.DeleteSegment(context.Background(), segmentID).Execute()
	if err != nil && (httpResp == nil || httpResp.StatusCode != http.StatusNotFound) {
		t.Fatalf("deleting segment fixture %s: %s", segmentID, testAccHTTPError(err, httpResp))
	}
}

func testAccCreateApplicationFixture(t *testing.T, client *sailpoint.APIClient, name, description string, accessProfileIDs []string) string {
	t.Helper()

	ctx := context.Background()
	source := api_beta.NewSourceAppCreateDtoAccountSource(testAccFixtureSourceID)
	sourceType := "SOURCE"
	source.SetType(sourceType)
	dto := api_beta.NewSourceAppCreateDto(name, description, *source)

	created, httpResp, err := client.Beta.AppsAPI.CreateSourceApp(ctx).SourceAppCreateDto(*dto).Execute()
	if err != nil {
		t.Fatalf("creating application fixture %q: %s", name, testAccHTTPError(err, httpResp))
	}
	if created == nil || created.Id == nil || *created.Id == "" {
		t.Fatalf("creating application fixture %q returned no id", name)
	}

	patch := make([]api_beta.JsonPatchOperation, 0, 5)
	ownerType := "IDENTITY"
	ownerMap := map[string]interface{}{
		"id":   testAccFixtureOwnerID,
		"type": ownerType,
	}
	patch = append(patch, api_beta.JsonPatchOperation{
		Op:   "replace",
		Path: "/owner",
		Value: func() *api_beta.UpdateMultiHostSourcesRequestInnerValue {
			v := api_beta.MapmapOfStringAnyAsUpdateMultiHostSourcesRequestInnerValue(&ownerMap)
			return &v
		}(),
	})

	arr := make([]api_beta.ArrayInner, 0, len(accessProfileIDs))
	for _, accessProfileID := range accessProfileIDs {
		id := accessProfileID
		arr = append(arr, api_beta.ArrayInner{String: &id})
	}
	patch = append(patch, api_beta.JsonPatchOperation{
		Op:   "replace",
		Path: "/accessProfiles",
		Value: func() *api_beta.UpdateMultiHostSourcesRequestInnerValue {
			v := api_beta.ArrayOfArrayInnerAsUpdateMultiHostSourcesRequestInnerValue(&arr)
			return &v
		}(),
	})

	enabled := true
	provisionRequestEnabled := false
	appCenterEnabled := true
	patch = append(patch,
		api_beta.JsonPatchOperation{
			Op:   "replace",
			Path: "/enabled",
			Value: func() *api_beta.UpdateMultiHostSourcesRequestInnerValue {
				v := api_beta.BoolAsUpdateMultiHostSourcesRequestInnerValue(&enabled)
				return &v
			}(),
		},
		api_beta.JsonPatchOperation{
			Op:   "replace",
			Path: "/provisionRequestEnabled",
			Value: func() *api_beta.UpdateMultiHostSourcesRequestInnerValue {
				v := api_beta.BoolAsUpdateMultiHostSourcesRequestInnerValue(&provisionRequestEnabled)
				return &v
			}(),
		},
		api_beta.JsonPatchOperation{
			Op:   "replace",
			Path: "/appCenterEnabled",
			Value: func() *api_beta.UpdateMultiHostSourcesRequestInnerValue {
				v := api_beta.BoolAsUpdateMultiHostSourcesRequestInnerValue(&appCenterEnabled)
				return &v
			}(),
		},
	)

	_, httpResp, err = client.Beta.AppsAPI.PatchSourceApp(ctx, *created.Id).JsonPatchOperation(patch).Execute()
	if err != nil {
		t.Fatalf("patching application fixture %q: %s", name, testAccHTTPError(err, httpResp))
	}

	return *created.Id
}

func testAccDeleteApplicationFixture(t *testing.T, client *sailpoint.APIClient, applicationID string) {
	t.Helper()
	if applicationID == "" {
		return
	}

	_, httpResp, err := client.Beta.AppsAPI.DeleteSourceApp(context.Background(), applicationID).Execute()
	if err != nil && (httpResp == nil || httpResp.StatusCode != http.StatusNotFound) {
		t.Fatalf("deleting application fixture %s: %s", applicationID, testAccHTTPError(err, httpResp))
	}
}

func testAccListApplicationAccessProfileIDs(client *sailpoint.APIClient, applicationID string) ([]string, error) {
	ids := make([]string, 0)

	var offset int32
	for {
		page, httpResp, err := client.Beta.AppsAPI.
			ListAccessProfilesForSourceApp(context.Background(), applicationID).
			Limit(250).
			Offset(offset).
			Execute()
		if err != nil {
			return nil, fmt.Errorf("listing application access profiles: %s", testAccHTTPError(err, httpResp))
		}

		for i := range page {
			if page[i].Id != nil && *page[i].Id != "" {
				ids = append(ids, *page[i].Id)
			}
		}

		if len(page) < 250 {
			return ids, nil
		}
		offset += 250
	}
}

func testAccHTTPError(err error, httpResp *http.Response) string {
	if err == nil {
		return ""
	}
	if httpResp == nil {
		return err.Error()
	}
	return fmt.Sprintf("%s (status %d)", err.Error(), httpResp.StatusCode)
}
