package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	sailpoint "github.com/sailpoint-oss/golang-sdk/v3"
)

type testAccApplicationAccessAssociationFixture struct {
	ApplicationID          string
	ManagedAccessProfile1  string
	ManagedAccessProfile2  string
	OutOfBandAccessProfile string
}

// TestAccApplicationAccessAssociationV1Resource exercises the additive
// subset-scoped lifecycle of identitynow_application_access_association_v1
// against a dedicated disposable Application plus three disposable Access
// Profiles created directly in the sandbox tenant. The Application remains
// external to Terraform so CheckDestroy can verify that destroying the helper
// resource removes only its own tracked ids while preserving an out-of-band
// access profile on the live Application.
func TestAccApplicationAccessAssociationV1Resource(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped unless env 'TF_ACC' set")
	}

	testAccPreCheck(t)
	client := testAccAPIClient()
	fixture := testAccCreateApplicationAccessAssociationFixture(t, client)
	t.Cleanup(func() {
		testAccDeleteApplicationFixture(t, client, fixture.ApplicationID)
		testAccDeleteAccessProfileFixture(t, client, fixture.ManagedAccessProfile1)
		testAccDeleteAccessProfileFixture(t, client, fixture.ManagedAccessProfile2)
		testAccDeleteAccessProfileFixture(t, client, fixture.OutOfBandAccessProfile)
	})

	resourceName := "identitynow_application_access_association_v1.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			return testAccCheckApplicationAccessAssociationV1Destroy(client, fixture)
		},
		Steps: []resource.TestStep{
			{
				Config: testAccApplicationAccessAssociationV1Config(fixture.ApplicationID, fixture.ManagedAccessProfile1, fixture.ManagedAccessProfile2),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "id", fixture.ApplicationID),
					resource.TestCheckResourceAttr(resourceName, "application_id", fixture.ApplicationID),
					resource.TestCheckResourceAttr(resourceName, "access_profile_ids.#", "2"),
					testAccCheckApplicationAccessAssociationState(resourceName, 2),
					testAccCheckApplicationHasAccessProfiles(client, fixture.ApplicationID, fixture.ManagedAccessProfile1, fixture.ManagedAccessProfile2, fixture.OutOfBandAccessProfile),
				),
			},
			{
				ResourceName: resourceName,
				ImportState:  true,
				ImportStateIdFunc: func(_ *terraform.State) (string, error) {
					return fmt.Sprintf("%s,%s/%s", fixture.ApplicationID, fixture.ManagedAccessProfile1, fixture.ManagedAccessProfile2), nil
				},
				ImportStateCheck: func(states []*terraform.InstanceState) error {
					if len(states) != 1 {
						return fmt.Errorf("expected 1 imported state, got %d", len(states))
					}
					state := states[0]
					if got := state.Attributes["id"]; got != fixture.ApplicationID {
						return fmt.Errorf("imported id = %q, want %q", got, fixture.ApplicationID)
					}
					if got := state.Attributes["application_id"]; got != fixture.ApplicationID {
						return fmt.Errorf("imported application_id = %q, want %q", got, fixture.ApplicationID)
					}
					if got := state.Attributes["access_profile_ids.#"]; got != "2" {
						return fmt.Errorf("imported access_profile_ids count = %q, want 2", got)
					}
					return nil
				},
			},
			{
				Config: testAccApplicationAccessAssociationV1Config(fixture.ApplicationID, fixture.ManagedAccessProfile1),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "access_profile_ids.#", "1"),
					testAccCheckApplicationAccessAssociationState(resourceName, 1),
					testAccCheckApplicationHasAccessProfiles(client, fixture.ApplicationID, fixture.ManagedAccessProfile1, fixture.OutOfBandAccessProfile),
					testAccCheckApplicationLacksAccessProfiles(client, fixture.ApplicationID, fixture.ManagedAccessProfile2),
				),
			},
			{
				Config:  testAccApplicationAccessAssociationV1Config(fixture.ApplicationID, fixture.ManagedAccessProfile1),
				Destroy: true,
			},
		},
	})
}

func testAccCreateApplicationAccessAssociationFixture(t *testing.T, client *sailpoint.APIClient) testAccApplicationAccessAssociationFixture {
	t.Helper()

	suffix := testAccUniqueName("application-access-association")
	managed1 := testAccCreateAccessProfileFixture(t, client, "tf-acc-test-app-access-assoc-ap-1-"+suffix, "Terraform acceptance test managed access profile 1.")
	managed2 := testAccCreateAccessProfileFixture(t, client, "tf-acc-test-app-access-assoc-ap-2-"+suffix, "Terraform acceptance test managed access profile 2.")
	outOfBand := testAccCreateAccessProfileFixture(t, client, "tf-acc-test-app-access-assoc-ap-3-"+suffix, "Terraform acceptance test out-of-band access profile.")
	appID := testAccCreateApplicationFixture(t, client, "tf-acc-test-app-access-assoc-"+suffix, "Terraform acceptance test application_access_association_v1 application.", []string{outOfBand})

	return testAccApplicationAccessAssociationFixture{
		ApplicationID:          appID,
		ManagedAccessProfile1:  managed1,
		ManagedAccessProfile2:  managed2,
		OutOfBandAccessProfile: outOfBand,
	}
}

func testAccApplicationAccessAssociationV1Config(applicationID string, accessProfileIDs ...string) string {
	quotedIDs := ""
	for _, accessProfileID := range accessProfileIDs {
		quotedIDs += fmt.Sprintf("    %q,\n", accessProfileID)
	}

	return fmt.Sprintf(`
resource "identitynow_application_access_association_v1" "test" {
  application_id = %q
  access_profile_ids = [
%s  ]
}
`, applicationID, quotedIDs)
}

func testAccCheckApplicationAccessAssociationState(resourceName string, want int) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource %s not found in state", resourceName)
		}
		if got := rs.Primary.Attributes["access_profile_ids.#"]; got != fmt.Sprintf("%d", want) {
			return fmt.Errorf("access_profile_ids count = %q, want %d", got, want)
		}
		return nil
	}
}

func testAccCheckApplicationHasAccessProfiles(client *sailpoint.APIClient, applicationID string, wantIDs ...string) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		return testAccRetry(fmt.Sprintf("application %s access profile union", applicationID), func() (bool, error) {
			ids, err := testAccListApplicationAccessProfileIDs(client, applicationID)
			if err != nil {
				return false, err
			}
			if len(ids) != len(wantIDs) {
				return false, nil
			}
			for _, wantID := range wantIDs {
				if !testAccContainsString(ids, wantID) {
					return false, nil
				}
			}
			return true, nil
		})
	}
}

func testAccCheckApplicationLacksAccessProfiles(client *sailpoint.APIClient, applicationID string, absentIDs ...string) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		return testAccRetry(fmt.Sprintf("application %s missing managed ids", applicationID), func() (bool, error) {
			ids, err := testAccListApplicationAccessProfileIDs(client, applicationID)
			if err != nil {
				return false, err
			}
			for _, absentID := range absentIDs {
				if testAccContainsString(ids, absentID) {
					return false, nil
				}
			}
			return true, nil
		})
	}
}

func testAccCheckApplicationAccessAssociationV1Destroy(client *sailpoint.APIClient, fixture testAccApplicationAccessAssociationFixture) error {
	return testAccRetry("application access association destroy verification", func() (bool, error) {
		ids, err := testAccListApplicationAccessProfileIDs(client, fixture.ApplicationID)
		if err != nil {
			return false, err
		}

		if len(ids) != 1 {
			return false, nil
		}
		if !testAccContainsString(ids, fixture.OutOfBandAccessProfile) {
			return false, nil
		}
		if testAccContainsString(ids, fixture.ManagedAccessProfile1) || testAccContainsString(ids, fixture.ManagedAccessProfile2) {
			return false, nil
		}
		return true, nil
	})
}
