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

// TestAccSegmentV1Resource is an acceptance test exercising the full CRUD
// lifecycle of the identitynow_segment_v1 resource against a real
// IdentityNow sandbox tenant, plus its paired singular (by-id and by-name)
// and plural data sources. Run via `make testacc` (TF_ACC=1) - never against
// a production tenant, and never with credentials printed to test output.
//
// Confirmed live via a manual `terraform apply`/plan/import/destroy pass in
// test/segment/main.tf against the real sandbox (2026-07-31) before writing
// this test - see the identitynow-terraform-provider-developer knowledge
// file's segment_v1 entry for the full investigation, including the two
// bugs found and fixed during that pass (plural data source CustomType
// staleness, missing UseStateForUnknown plan modifiers).
func TestAccSegmentV1Resource(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped unless env 'TF_ACC' set")
	}

	resourceName := "identitynow_segment_v1.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckSegmentV1Destroy,
		Steps: []resource.TestStep{
			{
				Config: testAccSegmentV1Config("tf-acc-test-segment", "initial description", false, "Employee"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", "tf-acc-test-segment"),
					resource.TestCheckResourceAttr(resourceName, "description", "initial description"),
					resource.TestCheckResourceAttr(resourceName, "active", "false"),
					resource.TestCheckResourceAttr(resourceName, "visibility_criteria.expression.operator", "AND"),
					resource.TestCheckResourceAttr(resourceName, "visibility_criteria.expression.children.0.attribute", "workerType"),
					resource.TestCheckResourceAttr(resourceName, "visibility_criteria.expression.children.0.value.value", "Employee"),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttrSet(resourceName, "created"),
					resource.TestCheckResourceAttrSet(resourceName, "modified"),
					resource.TestCheckResourceAttrPair("data.identitynow_segment_v1.by_id", "id", resourceName, "id"),
					resource.TestCheckResourceAttrPair("data.identitynow_segment_v1.by_name", "id", resourceName, "id"),
					resource.TestCheckResourceAttrPair("data.identitynow_segment_v1.by_id", "active", resourceName, "active"),
				),
			},
			{
				// ImportState verification: the resource must be importable
				// by its own id and produce an equivalent state.
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
				// The plural data source block isn't part of the imported
				// resource's own config/state and the singular data sources'
				// "by_name"/"by_id" instances are separate resource
				// addresses (not re-created on import), so none of them
				// interfere with ImportStateVerify's state comparison here.
			},
			{
				// Update: description/active/visibility_criteria value change
				// in place (no replacement) via the conditional JSON-Patch
				// strategy documented in segmentPatchOps.
				Config: testAccSegmentV1Config("tf-acc-test-segment", "updated description", true, "Contractor"),
				// "modified" is a genuinely volatile last-modified timestamp
				// that legitimately changes on every real Update (the API
				// bumps it), so it is intentionally excluded from
				// resource_segment_planmodifiers.go's UseStateForUnknown
				// scope and is *expected* to show a diff on this step only.
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "description", "updated description"),
					resource.TestCheckResourceAttr(resourceName, "active", "true"),
					resource.TestCheckResourceAttr(resourceName, "visibility_criteria.expression.children.0.value.value", "Contractor"),
				),
			},
		},
	})
}

func testAccSegmentV1Config(name, description string, active bool, workerType string) string {
	return fmt.Sprintf(`
resource "identitynow_segment_v1" "test" {
  name        = %[1]q
  description = %[2]q
  active      = %[3]t

  visibility_criteria = {
    expression = {
      operator = "AND"
      children = [
        {
          operator  = "EQUALS"
          attribute = "workerType"
          value = {
            type  = "STRING"
            value = %[4]q
          }
        },
        {
          operator  = "EQUALS"
          attribute = "uid"
          value = {
            type  = "STRING"
            value = "does-not-exist-tf-acc-test"
          }
        }
      ]
    }
  }
}

data "identitynow_segment_v1" "by_id" {
  id = identitynow_segment_v1.test.id
}

data "identitynow_segment_v1" "by_name" {
  name = identitynow_segment_v1.test.name
}
`, name, description, active, workerType)
}

// testAccCheckSegmentV1Destroy confirms every segment created by this test
// file no longer exists after Terraform destroys it (expects a 404 from the
// API).
func testAccCheckSegmentV1Destroy(s *terraform.State) error {
	client := sailpoint.NewAPIClient(sailpoint.NewDefaultConfiguration())

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "identitynow_segment_v1" {
			continue
		}

		_, httpResp, err := client.Beta.SegmentsAPI.
			GetSegment(context.Background(), rs.Primary.ID).
			Execute()
		if err == nil {
			return fmt.Errorf("segment %s still exists", rs.Primary.ID)
		}
		if httpResp == nil || httpResp.StatusCode != 404 {
			return fmt.Errorf("unexpected error checking segment %s was destroyed: %w", rs.Primary.ID, err)
		}
	}
	return nil
}
