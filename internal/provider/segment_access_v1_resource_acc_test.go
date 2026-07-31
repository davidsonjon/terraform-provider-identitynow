package provider

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	sailpoint "github.com/sailpoint-oss/golang-sdk/v2"
)

type testAccSegmentAccessFixture struct {
	SegmentID       string
	RoleID          string
	AccessProfileID string
}

// TestAccSegmentAccessV1Resource exercises the live assign/remove lifecycle of
// identitynow_segment_access_v1 against dedicated disposable fixtures created
// directly in the sandbox tenant. The managed resource under test owns only
// the segment membership edges; the Segment, Role, and Access Profile
// themselves intentionally remain external so CheckDestroy can verify their
// `/segments` lists no longer contain this test's segment after Terraform
// destroys only the association resource.
func TestAccSegmentAccessV1Resource(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped unless env 'TF_ACC' set")
	}

	testAccPreCheck(t)
	client := testAccAPIClient()
	fixture := testAccCreateSegmentAccessFixture(t, client)
	t.Cleanup(func() {
		testAccDeleteAccessProfileFixture(t, client, fixture.AccessProfileID)
		testAccDeleteRoleFixture(t, client, fixture.RoleID)
		testAccDeleteSegmentFixture(t, client, fixture.SegmentID)
	})

	resourceName := "identitynow_segment_access_v1.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			return testAccCheckSegmentAccessV1Destroy(client, fixture)
		},
		Steps: []resource.TestStep{
			{
				Config: testAccSegmentAccessV1Config(fixture.SegmentID, fixture.RoleID, fixture.AccessProfileID, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "id", fixture.SegmentID),
					resource.TestCheckResourceAttr(resourceName, "segment_id", fixture.SegmentID),
					resource.TestCheckResourceAttr(resourceName, "assignments.#", "2"),
					testAccCheckSegmentAccessAssignmentState(resourceName, 2),
					testAccCheckRoleHasSegment(client, fixture.RoleID, fixture.SegmentID, true),
					testAccCheckAccessProfileHasSegment(client, fixture.AccessProfileID, fixture.SegmentID, true),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccSegmentAccessV1Config(fixture.SegmentID, fixture.RoleID, fixture.AccessProfileID, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "assignments.#", "1"),
					testAccCheckSegmentAccessAssignmentState(resourceName, 1),
					testAccCheckRoleHasSegment(client, fixture.RoleID, fixture.SegmentID, false),
					testAccCheckAccessProfileHasSegment(client, fixture.AccessProfileID, fixture.SegmentID, true),
				),
			},
		},
	})
}

func testAccCreateSegmentAccessFixture(t *testing.T, client *sailpoint.APIClient) testAccSegmentAccessFixture {
	t.Helper()

	suffix := testAccUniqueName("segment-access")
	return testAccSegmentAccessFixture{
		SegmentID:       testAccCreateSegmentFixture(t, client, "tf-acc-test-segment-access-"+suffix, "Terraform acceptance test segment_access_v1 segment."),
		RoleID:          testAccCreateRoleFixture(t, client, "tf-acc-test-segment-access-role-"+suffix, "Terraform acceptance test segment_access_v1 role."),
		AccessProfileID: testAccCreateAccessProfileFixture(t, client, "tf-acc-test-segment-access-ap-"+suffix, "Terraform acceptance test segment_access_v1 access profile."),
	}
}

func testAccSegmentAccessV1Config(segmentID, roleID, accessProfileID string, includeRole bool) string {
	roleAssignment := ""
	if includeRole {
		roleAssignment = fmt.Sprintf(`
    {
      type = "ROLE"
      id   = %q
    },`, roleID)
	}

	return fmt.Sprintf(`
resource "identitynow_segment_access_v1" "test" {
  segment_id = %q

  assignments = [%s
    {
      type = "ACCESS_PROFILE"
      id   = %q
    },
  ]
}
`, segmentID, roleAssignment, accessProfileID)
}

func testAccCheckSegmentAccessAssignmentState(resourceName string, want int) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource %s not found in state", resourceName)
		}
		if got := rs.Primary.Attributes["assignments.#"]; got != fmt.Sprintf("%d", want) {
			return fmt.Errorf("assignments count = %q, want %d", got, want)
		}
		return nil
	}
}

func testAccCheckRoleHasSegment(client *sailpoint.APIClient, roleID, segmentID string, want bool) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		return testAccRetry(fmt.Sprintf("role %s segment membership", roleID), func() (bool, error) {
			role, httpResp, err := client.Beta.RolesAPI.GetRole(context.Background(), roleID).Execute()
			if err != nil {
				return false, fmt.Errorf("getting role %s: %s", roleID, testAccHTTPError(err, httpResp))
			}
			return testAccContainsString(role.Segments, segmentID) == want, nil
		})
	}
}

func testAccCheckAccessProfileHasSegment(client *sailpoint.APIClient, accessProfileID, segmentID string, want bool) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		return testAccRetry(fmt.Sprintf("access profile %s segment membership", accessProfileID), func() (bool, error) {
			accessProfile, httpResp, err := client.Beta.AccessProfilesAPI.GetAccessProfile(context.Background(), accessProfileID).Execute()
			if err != nil {
				return false, fmt.Errorf("getting access profile %s: %s", accessProfileID, testAccHTTPError(err, httpResp))
			}
			return testAccContainsString(accessProfile.Segments, segmentID) == want, nil
		})
	}
}

func testAccCheckSegmentAccessV1Destroy(client *sailpoint.APIClient, fixture testAccSegmentAccessFixture) error {
	return testAccRetry("segment access destroy verification", func() (bool, error) {
		role, httpResp, err := client.Beta.RolesAPI.GetRole(context.Background(), fixture.RoleID).Execute()
		if err != nil {
			return false, fmt.Errorf("getting role %s during destroy check: %s", fixture.RoleID, testAccHTTPError(err, httpResp))
		}

		accessProfile, httpResp, err := client.Beta.AccessProfilesAPI.GetAccessProfile(context.Background(), fixture.AccessProfileID).Execute()
		if err != nil {
			return false, fmt.Errorf("getting access profile %s during destroy check: %s", fixture.AccessProfileID, testAccHTTPError(err, httpResp))
		}

		roleHasSegment := testAccContainsString(role.Segments, fixture.SegmentID)
		accessProfileHasSegment := testAccContainsString(accessProfile.Segments, fixture.SegmentID)
		if roleHasSegment || accessProfileHasSegment {
			return false, nil
		}

		return true, nil
	})
}
