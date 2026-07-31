package provider

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	sailpoint "github.com/sailpoint-oss/golang-sdk/v2"
)

// TestAccAccessModelMetadataAttributeV1Resource is an acceptance test
// exercising the full CRUD lifecycle of the
// identitynow_access_model_metadata_attribute_v1 resource against a real
// IdentityNow sandbox tenant. Run via `make testacc` (TF_ACC=1) - never
// against a production tenant, and never with credentials printed to test
// output.
//
// Delete note: `DELETE /access-model-metadata/attributes/{key}` is a real,
// working endpoint that SailPoint's published OpenAPI spec omits (confirmed
// 2026-07-26 via a captured browser network call) - the resource's Delete()
// hand-rolls this HTTP call rather than using the golang-sdk (which has no
// generated method for it). testAccCheckAccessModelMetadataAttributeV1Destroy
// below asserts a normal 404 after destroy, same as every other resource's
// CheckDestroy in this provider.
func TestAccAccessModelMetadataAttributeV1Resource(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped unless env 'TF_ACC' set")
	}

	resourceName := "identitynow_access_model_metadata_attribute_v1.test"
	// Include a timestamp-derived suffix so repeated test runs don't collide
	// on "key" (which must be unique).
	key := fmt.Sprintf("tfAccTest%d", time.Now().UnixNano())

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAccessModelMetadataAttributeV1Destroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAccessModelMetadataAttributeV1Config(key, "TF Acc Test Attribute", "initial description"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "key", key),
					resource.TestCheckResourceAttr(resourceName, "name", "TF Acc Test Attribute"),
					resource.TestCheckResourceAttr(resourceName, "description", "initial description"),
					resource.TestCheckResourceAttr(resourceName, "multiselect", "false"),
					resource.TestCheckResourceAttr(resourceName, "object_types.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "object_types.0", "all"),
					resource.TestCheckResourceAttr(resourceName, "values.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "values.0.value", "low"),
					resource.TestCheckResourceAttr(resourceName, "values.0.name", "Low"),
					resource.TestCheckResourceAttrSet(resourceName, "status"),
					resource.TestCheckResourceAttrSet(resourceName, "type"),
				),
			},
			{
				// ImportState verification: the resource must be importable
				// by its own "key" (there is no separate "id" field) and
				// produce an equivalent state.
				ResourceName:                         resourceName,
				ImportState:                          true,
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "key",
			},
			{
				// Update: per the API's own documentation, only
				// "name"/"description"/"multiselect"/"values" are patchable
				// - "key"/"object_types"/"type" are held constant across
				// steps deliberately (changing them would trigger
				// RequiresReplace(), a separate scenario not covered here).
				Config: testAccAccessModelMetadataAttributeV1Config(key, "TF Acc Test Attribute Updated", "updated description"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "key", key),
					resource.TestCheckResourceAttr(resourceName, "name", "TF Acc Test Attribute Updated"),
					resource.TestCheckResourceAttr(resourceName, "description", "updated description"),
					resource.TestCheckResourceAttr(resourceName, "values.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "values.0.value", "low"),
				),
			},
		},
	})
}

func testAccAccessModelMetadataAttributeV1Config(key, name, description string) string {
	return fmt.Sprintf(`
resource "identitynow_access_model_metadata_attribute_v1" "test" {
  key          = %[1]q
  name         = %[2]q
  description  = %[3]q
  multiselect  = false
  object_types = ["all"]

  values = [
    {
      value = "low"
      name  = "Low"
    },
  ]
}
`, key, name, description)
}

// testAccCheckAccessModelMetadataAttributeV1Destroy confirms every Access
// Model Metadata Attribute created by this test file no longer exists after
// Terraform destroys it (expects a 404 from the API) - Delete() hand-rolls a
// real HTTP DELETE call (see resource_access_model_metadata_attribute_delete.go),
// so this is a normal, real deletion, not a state-only removal.
func testAccCheckAccessModelMetadataAttributeV1Destroy(s *terraform.State) error {
	client := sailpoint.NewAPIClient(sailpoint.NewDefaultConfiguration())

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "identitynow_access_model_metadata_attribute_v1" {
			continue
		}

		key := rs.Primary.Attributes["key"]
		_, httpResp, err := client.Beta.AccessModelMetadataAPI.
			GetAccessModelMetadataAttribute(context.Background(), key).
			Execute()
		if err == nil {
			return fmt.Errorf("access model metadata attribute %s still exists", key)
		}
		if httpResp == nil || httpResp.StatusCode != 404 {
			return fmt.Errorf("unexpected error checking access model metadata attribute %s was destroyed: %w", key, err)
		}
	}
	return nil
}
