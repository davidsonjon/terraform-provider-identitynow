package provider

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"testing"
	"time"

	sailpoint "github.com/sailpoint-oss/golang-sdk/v3"
	"github.com/sailpoint-oss/golang-sdk/v3/access_profiles"
	"github.com/sailpoint-oss/golang-sdk/v3/apps"
	"github.com/sailpoint-oss/golang-sdk/v3/roles"
	"github.com/sailpoint-oss/golang-sdk/v3/segments"
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
	dto := roles.NewRole(name, *roles.NewNullableOwnerReference(&roles.OwnerReference{
		Id:   &ownerID,
		Type: &ownerType,
	}))
	dto.SetDescription(description)
	dto.SetEnabled(true)
	dto.SetRequestable(false)
	dto.SetAccessProfiles([]roles.AccessProfileRef{})
	dto.SetDimensionRefs([]roles.DimensionRef{})
	dto.SetEntitlements([]roles.EntitlementRef{})
	dto.SetAdditionalOwners([]roles.AdditionalOwnerRef{})
	dto.SetSegments([]string{})

	created, httpResp, err := client.RolesAPI.CreateRoleV1(ctx).Role(*dto).Execute()
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

	httpResp, err := client.RolesAPI.DeleteRoleV1(context.Background(), roleID).Execute()
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

	dto := access_profiles.NewAccessProfile(name, *access_profiles.NewNullableOwnerReference(&access_profiles.OwnerReference{
		Id:   &ownerID,
		Type: &ownerType,
	}), access_profiles.AccessProfileSourceRef{
		Id:   &sourceID,
		Type: &sourceType,
	})
	dto.SetDescription(description)
	dto.SetEnabled(false)
	dto.SetRequestable(false)
	dto.SetEntitlements([]access_profiles.EntitlementRef{})
	dto.SetAdditionalOwners([]access_profiles.AdditionalOwnerRef{})
	dto.SetSegments([]string{})

	created, httpResp, err := client.AccessProfilesAPI.CreateAccessProfileV1(ctx).AccessProfile(*dto).Execute()
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

	httpResp, err := client.AccessProfilesAPI.DeleteAccessProfileV1(context.Background(), accessProfileID).Execute()
	if err != nil && (httpResp == nil || httpResp.StatusCode != http.StatusNotFound) {
		t.Fatalf("deleting access profile fixture %s: %s", accessProfileID, testAccHTTPError(err, httpResp))
	}
}

func testAccCreateSegmentFixture(t *testing.T, client *sailpoint.APIClient, name, description string) string {
	t.Helper()

	dto := segments.NewSegmentWithDefaults()
	dto.SetName(name)
	dto.SetDescription(description)
	dto.SetActive(true)

	operator := "EQUALS"
	attribute := "uid"
	valueType := "STRING"
	value := testAccUniqueName("does-not-exist")
	dto.SetVisibilityCriteria(segments.SegmentVisibilityCriteria{
		Expression: &segments.Expression{
			Operator:  &operator,
			Attribute: *segments.NewNullableString(&attribute),
			Value: *segments.NewNullableValue(&segments.Value{
				Type:  &valueType,
				Value: &value,
			}),
		},
	})

	created, httpResp, err := client.SegmentsAPI.CreateSegmentV1(context.Background()).Segment(*dto).Execute()
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

	httpResp, err := client.SegmentsAPI.DeleteSegmentV1(context.Background(), segmentID).Execute()
	if err != nil && (httpResp == nil || httpResp.StatusCode != http.StatusNotFound) {
		t.Fatalf("deleting segment fixture %s: %s", segmentID, testAccHTTPError(err, httpResp))
	}
}

func testAccCreateApplicationFixture(t *testing.T, client *sailpoint.APIClient, name, description string, accessProfileIDs []string) string {
	t.Helper()

	ctx := context.Background()
	source := apps.NewSourceAppCreateDtoAccountSource(testAccFixtureSourceID)
	sourceType := "SOURCE"
	source.SetType(sourceType)
	dto := apps.NewSourceAppCreateDto(name, description, *source)

	created, httpResp, err := client.AppsAPI.CreateSourceAppV1(ctx).SourceAppCreateDto(*dto).Execute()
	if err != nil {
		t.Fatalf("creating application fixture %q: %s", name, testAccHTTPError(err, httpResp))
	}
	if created == nil || created.Id == nil || *created.Id == "" {
		t.Fatalf("creating application fixture %q returned no id", name)
	}

	patch := make([]apps.JsonPatchOperation, 0, 5)
	ownerType := "IDENTITY"
	ownerMap := map[string]interface{}{
		"id":   testAccFixtureOwnerID,
		"type": ownerType,
	}
	patch = append(patch, apps.JsonPatchOperation{
		Op:   "replace",
		Path: "/owner",
		Value: func() *apps.JsonPatchOperationValue {
			v := apps.MapmapOfStringAnyAsJsonPatchOperationValue(&ownerMap)
			return &v
		}(),
	})

	arr := make([]apps.ArrayInner, 0, len(accessProfileIDs))
	for _, accessProfileID := range accessProfileIDs {
		id := accessProfileID
		arr = append(arr, apps.ArrayInner{String: &id})
	}
	patch = append(patch, apps.JsonPatchOperation{
		Op:   "replace",
		Path: "/accessProfiles",
		Value: func() *apps.JsonPatchOperationValue {
			v := apps.ArrayOfArrayInnerAsJsonPatchOperationValue(&arr)
			return &v
		}(),
	})

	enabled := true
	provisionRequestEnabled := false
	appCenterEnabled := true
	patch = append(patch,
		apps.JsonPatchOperation{
			Op:   "replace",
			Path: "/enabled",
			Value: func() *apps.JsonPatchOperationValue {
				v := apps.BoolAsJsonPatchOperationValue(&enabled)
				return &v
			}(),
		},
		apps.JsonPatchOperation{
			Op:   "replace",
			Path: "/provisionRequestEnabled",
			Value: func() *apps.JsonPatchOperationValue {
				v := apps.BoolAsJsonPatchOperationValue(&provisionRequestEnabled)
				return &v
			}(),
		},
		apps.JsonPatchOperation{
			Op:   "replace",
			Path: "/appCenterEnabled",
			Value: func() *apps.JsonPatchOperationValue {
				v := apps.BoolAsJsonPatchOperationValue(&appCenterEnabled)
				return &v
			}(),
		},
	)

	_, httpResp, err = client.AppsAPI.PatchSourceAppV1(ctx, *created.Id).JsonPatchOperation(patch).Execute()
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

	_, httpResp, err := client.AppsAPI.DeleteSourceAppV1(context.Background(), applicationID).Execute()
	if err != nil && (httpResp == nil || httpResp.StatusCode != http.StatusNotFound) {
		t.Fatalf("deleting application fixture %s: %s", applicationID, testAccHTTPError(err, httpResp))
	}
}

func testAccListApplicationAccessProfileIDs(client *sailpoint.APIClient, applicationID string) ([]string, error) {
	ids := make([]string, 0)

	var offset int32
	for {
		page, httpResp, err := client.AppsAPI.
			ListAccessProfilesForSourceAppV1(context.Background(), applicationID).
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
