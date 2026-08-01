package provider

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	sailpoint "github.com/sailpoint-oss/golang-sdk/v3"
)

// TestAccApplicationV1Resource is an acceptance test exercising the full
// CRUD lifecycle of the identitynow_application_v1 resource against a real
// IdentityNow sandbox tenant. Run via `make testacc` (TF_ACC=1) - never
// against a production tenant, and never with credentials printed to test
// output.
//
// Requires:
//   - SAIL_BASE_URL / SAIL_CLIENT_ID / SAIL_CLIENT_SECRET (see testAccPreCheck)
//   - ACCTEST_APPLICATION_OWNER_ID: a valid identity id in the target
//     tenant to use as the application's "owner" (not accepted by the create
//     API - set via an immediate follow-up PATCH, see resource_application.go).
//   - ACCTEST_APPLICATION_SOURCE_ID: a valid source id in the target tenant
//     to use as "account_source.id" (Required by the create API).
//
// This test does not exercise access_profile_ids: doing so live requires a
// real, tenant-specific access profile id purely for test plumbing, and the
// attribute's read/write path (a separate ListAccessProfilesForSourceApp
// call plus a JSON Patch replace on /accessProfiles) is already covered by
// the unit tests in internal/provider/application_v1/resource_application_test.go
// (TestApplicationCreatePatchOps, TestApplicationDtoToModel_RoundTrip,
// TestStringSetToArrayInner) and was directly confirmed via a real
// `terraform apply` during this resource's live-testing pass (see the
// application_v1 dated entry in
// .github/agents/identitynow-terraform-provider-developer.knowledge.md).
func TestAccApplicationV1Resource(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped unless env 'TF_ACC' set")
	}

	resourceName := "identitynow_application_v1.test"
	ownerID := os.Getenv("ACCTEST_APPLICATION_OWNER_ID")
	if ownerID == "" {
		t.Fatal("ACCTEST_APPLICATION_OWNER_ID must be set to a valid identity id in the sandbox tenant for this test")
	}
	sourceID := os.Getenv("ACCTEST_APPLICATION_SOURCE_ID")
	if sourceID == "" {
		t.Fatal("ACCTEST_APPLICATION_SOURCE_ID must be set to a valid source id in the sandbox tenant for this test (account_source.id is Required)")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckApplicationV1Destroy,
		Steps: []resource.TestStep{
			{
				Config: testAccApplicationV1Config("tf-acc-test-application", "initial description", sourceID, ownerID, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", "tf-acc-test-application"),
					resource.TestCheckResourceAttr(resourceName, "description", "initial description"),
					resource.TestCheckResourceAttr(resourceName, "enabled", "true"),
					resource.TestCheckResourceAttr(resourceName, "account_source.id", sourceID),
					resource.TestCheckResourceAttr(resourceName, "owner.id", ownerID),
					resource.TestCheckResourceAttr(resourceName, "owner.type", "IDENTITY"),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttrSet(resourceName, "created"),
				),
			},
			{
				// ImportState verification: the resource must be importable
				// by its own id and produce an equivalent state.
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				// Update: description/enabled change in place (no
				// replacement) via the minimal JSON Patch diff computed by
				// applicationUpdatePatchOps.
				Config: testAccApplicationV1Config("tf-acc-test-application", "updated description", sourceID, ownerID, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", "tf-acc-test-application"),
					resource.TestCheckResourceAttr(resourceName, "description", "updated description"),
					resource.TestCheckResourceAttr(resourceName, "enabled", "false"),
				),
			},
		},
	})
}

func testAccApplicationV1Config(name, description, sourceID, ownerID string, enabled bool) string {
	return fmt.Sprintf(`
resource "identitynow_application_v1" "test" {
  name        = %[1]q
  description = %[2]q
  enabled     = %[5]t

  account_source = {
    id = %[3]q
  }

  owner = {
    id   = %[4]q
    type = "IDENTITY"
  }
}
`, name, description, sourceID, ownerID, enabled)
}

// testAccCheckApplicationV1Destroy confirms every application created by
// this test file no longer exists after Terraform destroys it (expects a
// 404 from the API).
func testAccCheckApplicationV1Destroy(s *terraform.State) error {
	client := sailpoint.NewAPIClient(sailpoint.NewDefaultConfiguration())

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "identitynow_application_v1" {
			continue
		}

		_, httpResp, err := client.AppsAPI.
			GetSourceAppV1(context.Background(), rs.Primary.ID).
			Execute()
		if err == nil {
			return fmt.Errorf("application %s still exists", rs.Primary.ID)
		}
		if httpResp == nil || httpResp.StatusCode != 404 {
			return fmt.Errorf("unexpected error checking application %s was destroyed: %w", rs.Primary.ID, err)
		}
	}
	return nil
}
