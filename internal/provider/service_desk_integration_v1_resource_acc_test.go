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

// TestAccServiceDeskIntegrationV1Resource is an acceptance test exercising the
// full CRUD lifecycle of the identitynow_service_desk_integration_v1
// resource against a real IdentityNow sandbox tenant. Run via `make testacc`
// (TF_ACC=1) - never against a production tenant, and never with credentials
// printed to test output.
//
// Requires:
//   - SAIL_BASE_URL / SAIL_CLIENT_ID / SAIL_CLIENT_SECRET (see testAccPreCheck)
//   - ACCTEST_SDI_CLUSTER_ID: a valid managed cluster id in the target tenant.
//     Confirmed via direct API testing that the "clusterRef" attribute is
//     required by the real API for at least the "ServiceNowSDIM"/"Generic
//     SDIM" integration types, despite the OpenAPI spec not marking it
//     required (see the identitynow-terraform-provider-developer knowledge
//     file, 2026-07-24 entry) - modelToDto was fixed to treat this and the
//     other Optional+Computed ref attributes' Unknown plan values as "not
//     specified" rather than erroring, as part of adding this test.
//   - The target tenant must have a licensed/enabled "ServiceNowSDIM" service
//     desk integration connector. A sandbox lacking that license will fail
//     Create with a 400 ("Application template ... was not found"); this is a
//     tenant configuration/licensing gap, not a provider code defect - the
//     Create code path was directly confirmed to reach the API and correctly
//     surface the real validation error in that case.
func TestAccServiceDeskIntegrationV1Resource(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped unless env 'TF_ACC' set")
	}

	resourceName := "identitynow_service_desk_integration_v1.test"
	clusterID := os.Getenv("ACCTEST_SDI_CLUSTER_ID")
	if clusterID == "" {
		t.Fatal("ACCTEST_SDI_CLUSTER_ID must be set to a valid managed cluster id in the sandbox tenant for this test (the ServiceNowSDIM integration type requires clusterRef, despite the OpenAPI spec not marking it required)")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckServiceDeskIntegrationV1Destroy,
		Steps: []resource.TestStep{
			{
				Config: testAccServiceDeskIntegrationV1Config("tf-acc-test-sdi", "initial description", clusterID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", "tf-acc-test-sdi"),
					resource.TestCheckResourceAttr(resourceName, "description", "initial description"),
					resource.TestCheckResourceAttr(resourceName, "type", "ServiceNowSDIM"),
					resource.TestCheckResourceAttr(resourceName, "cluster_ref.id", clusterID),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
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
				// Update: description changes in place (no replacement).
				Config: testAccServiceDeskIntegrationV1Config("tf-acc-test-sdi", "updated description", clusterID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", "tf-acc-test-sdi"),
					resource.TestCheckResourceAttr(resourceName, "description", "updated description"),
				),
			},
		},
	})
}

func testAccServiceDeskIntegrationV1Config(name, description, clusterID string) string {
	return fmt.Sprintf(`
resource "identitynow_service_desk_integration_v1" "test" {
  name        = %[1]q
  description = %[2]q
  type        = "ServiceNowSDIM"

  cluster_ref = {
    id = %[3]q
  }

  attributes = {}
}
`, name, description, clusterID)
}

// testAccCheckServiceDeskIntegrationV1Destroy confirms every service desk
// integration created by this test file no longer exists after Terraform
// destroys it (expects a 404 from the API).
func testAccCheckServiceDeskIntegrationV1Destroy(s *terraform.State) error {
	client := sailpoint.NewAPIClient(sailpoint.NewDefaultConfiguration())

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "identitynow_service_desk_integration_v1" {
			continue
		}

		_, httpResp, err := client.ServiceDeskIntegrationAPI.
			GetServiceDeskIntegrationV1(context.Background(), rs.Primary.ID).
			Execute()
		if err == nil {
			return fmt.Errorf("service desk integration %s still exists", rs.Primary.ID)
		}
		if httpResp == nil || httpResp.StatusCode != 404 {
			return fmt.Errorf("unexpected error checking service desk integration %s was destroyed: %w", rs.Primary.ID, err)
		}
	}
	return nil
}
