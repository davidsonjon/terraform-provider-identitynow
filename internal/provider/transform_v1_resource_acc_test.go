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

// TestAccTransformV1Resource is an acceptance test exercising the full CRUD
// lifecycle of the identitynow_transform_v1 resource against a real
// IdentityNow sandbox tenant. Run via `make testacc` (TF_ACC=1) - never
// against a production tenant, and never with credentials printed to test
// output.
//
// Unlike role_v1/service_desk_integration_v1, this resource needs no extra
// ACCTEST_* environment variable: "upper" is a transform type with no
// required "attributes" sub-properties (see
// https://developer.sailpoint.com/docs/extensibility/transforms/operations),
// so the test config is fully self-contained. This test config, including the
// "attributes" JSON string content used across both steps, mirrors the
// manual apply/update/destroy cycle already live-verified against the real
// sandbox tenant during the 2026-07-24 transform_v1 implementation (see the
// identitynow-terraform-provider-developer knowledge file) - this test
// automates that same lifecycle for CI-independent regression coverage.
func TestAccTransformV1Resource(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped unless env 'TF_ACC' set")
	}

	resourceName := "identitynow_transform_v1.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckTransformV1Destroy,
		Steps: []resource.TestStep{
			{
				Config: testAccTransformV1Config("tf-acc-test-transform", "upper", `{"requiresPeriodicRefresh":false}`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", "tf-acc-test-transform"),
					resource.TestCheckResourceAttr(resourceName, "type", "upper"),
					resource.TestCheckResourceAttr(resourceName, "internal", "false"),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					// jsontypes.Normalized's semantic (not textual) equality is
					// itself exercised implicitly here: the check below compares
					// against the exact config string, but the "id"/"plan"
					// pipeline round-trips it through the API's own JSON
					// marshaling first (see transformReadToModel), so a pass
					// here also confirms no spurious whitespace/key-ordering
					// diff was introduced by that round-trip.
					resource.TestCheckResourceAttr(resourceName, "attributes", `{"requiresPeriodicRefresh":false}`),
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
				// Update: per the API's own description ("Only the
				// 'attributes' field is mutable"), only "attributes" is
				// expected to actually change here - name/type are held
				// constant across steps deliberately (see
				// resource_transform.go's Update doc comment on the
				// currently-unenforced name/type immutability follow-up).
				Config: testAccTransformV1Config("tf-acc-test-transform", "upper", `{"requiresPeriodicRefresh":true}`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", "tf-acc-test-transform"),
					resource.TestCheckResourceAttr(resourceName, "type", "upper"),
					resource.TestCheckResourceAttr(resourceName, "attributes", `{"requiresPeriodicRefresh":true}`),
				),
			},
		},
	})
}

func testAccTransformV1Config(name, transformType, attributesJSON string) string {
	return fmt.Sprintf(`
resource "identitynow_transform_v1" "test" {
  name       = %[1]q
  type       = %[2]q
  attributes = %[3]q
}
`, name, transformType, attributesJSON)
}

// testAccCheckTransformV1Destroy confirms every transform created by this
// test file no longer exists after Terraform destroys it (expects a 404 from
// the API).
func testAccCheckTransformV1Destroy(s *terraform.State) error {
	client := sailpoint.NewAPIClient(sailpoint.NewDefaultConfiguration())

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "identitynow_transform_v1" {
			continue
		}

		_, httpResp, err := client.Beta.TransformsAPI.
			GetTransform(context.Background(), rs.Primary.ID).
			Execute()
		if err == nil {
			return fmt.Errorf("transform %s still exists", rs.Primary.ID)
		}
		if httpResp == nil || httpResp.StatusCode != 404 {
			return fmt.Errorf("unexpected error checking transform %s was destroyed: %w", rs.Primary.ID, err)
		}
	}
	return nil
}
