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

// TestAccEntitlementV1Resource exercises the adopt-existing lifecycle of
// identitynow_entitlement_v1 against a real IdentityNow sandbox tenant.
// Unlike create/delete-capable resources, this test requires a dedicated,
// pre-existing entitlement fixture because the API exposes no create or
// delete endpoint for entitlements.
//
// Requires:
//   - SAIL_BASE_URL / SAIL_CLIENT_ID / SAIL_CLIENT_SECRET (see testAccPreCheck)
//   - ACCTEST_ENTITLEMENT_ID: an existing, disposable entitlement id in the
//     target sandbox tenant. The update step mutates description/requestable,
//     and Delete only removes Terraform state, so use a fixture reserved for
//     acceptance testing rather than a shared production-like entitlement.
//   - ACCTEST_ENTITLEMENT_SOURCE_ID / ACCTEST_ENTITLEMENT_VALUE: the matching
//     source.id and value for that same entitlement, used to exercise the
//     adopt-by-lookup (source_id + value) path as well as direct adoption by id.
func TestAccEntitlementV1Resource(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped unless env 'TF_ACC' set")
	}

	entitlementID := os.Getenv("ACCTEST_ENTITLEMENT_ID")
	if entitlementID == "" {
		t.Fatal("ACCTEST_ENTITLEMENT_ID must be set to a valid entitlement id in the sandbox tenant for this test")
	}
	sourceID := os.Getenv("ACCTEST_ENTITLEMENT_SOURCE_ID")
	if sourceID == "" {
		t.Fatal("ACCTEST_ENTITLEMENT_SOURCE_ID must be set to the matching source id for ACCTEST_ENTITLEMENT_ID in the sandbox tenant for this test")
	}
	value := os.Getenv("ACCTEST_ENTITLEMENT_VALUE")
	if value == "" {
		t.Fatal("ACCTEST_ENTITLEMENT_VALUE must be set to the matching entitlement value for ACCTEST_ENTITLEMENT_ID in the sandbox tenant for this test")
	}

	resourceName := "identitynow_entitlement_v1.test"
	lookupResourceName := "identitynow_entitlement_v1.lookup"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckEntitlementV1StillExists,
		Steps: []resource.TestStep{
			{
				Config: testAccEntitlementV1ConfigByID(entitlementID, "Terraform acceptance test description", false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "id", entitlementID),
					resource.TestCheckResourceAttr(resourceName, "description", "Terraform acceptance test description"),
					resource.TestCheckResourceAttr(resourceName, "requestable", "false"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccEntitlementV1ConfigByID(entitlementID, "Terraform acceptance test description updated", true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "id", entitlementID),
					resource.TestCheckResourceAttr(resourceName, "description", "Terraform acceptance test description updated"),
					resource.TestCheckResourceAttr(resourceName, "requestable", "true"),
				),
			},
			{
				Config: testAccEntitlementV1ConfigByLookup(sourceID, value),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(lookupResourceName, "id", entitlementID),
					resource.TestCheckResourceAttr(lookupResourceName, "source_id", sourceID),
					resource.TestCheckResourceAttr(lookupResourceName, "value", value),
				),
			},
		},
	})
}

func testAccEntitlementV1ConfigByID(entitlementID, description string, requestable bool) string {
	return fmt.Sprintf(`
resource "identitynow_entitlement_v1" "test" {
  id          = %[1]q
  description = %[2]q
  requestable = %[3]t
}
`, entitlementID, description, requestable)
}

func testAccEntitlementV1ConfigByLookup(sourceID, value string) string {
	return fmt.Sprintf(`
resource "identitynow_entitlement_v1" "lookup" {
  source_id = %[1]q
  value     = %[2]q
}
`, sourceID, value)
}

// testAccCheckEntitlementV1StillExists verifies Delete only removes
// Terraform state: the upstream entitlement must remain readable after
// `terraform destroy` because the API has no delete endpoint.
func testAccCheckEntitlementV1StillExists(s *terraform.State) error {
	client := sailpoint.NewAPIClient(sailpoint.NewDefaultConfiguration())

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "identitynow_entitlement_v1" {
			continue
		}

		dto, httpResp, err := client.Beta.EntitlementsAPI.
			GetEntitlement(context.Background(), rs.Primary.ID).
			Execute()
		if err != nil {
			if httpResp != nil && httpResp.StatusCode == 404 {
				return fmt.Errorf("entitlement %s no longer exists after Terraform destroy; Delete should only remove state", rs.Primary.ID)
			}
			return fmt.Errorf("unexpected error checking entitlement %s still exists after destroy: %w", rs.Primary.ID, err)
		}
		if dto.Id == nil || *dto.Id != rs.Primary.ID {
			return fmt.Errorf("GetEntitlement returned id %v, want %s", dto.Id, rs.Primary.ID)
		}
	}

	return nil
}
