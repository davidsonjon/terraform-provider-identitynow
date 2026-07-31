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

// TestAccIdentityProfileV1Resource is an acceptance test exercising the full
// CRUD lifecycle of the identitynow_identity_profile_v1 resource against a
// real IdentityNow sandbox tenant. Run via `make testacc` (TF_ACC=1) - never
// against a production tenant, and never with credentials printed to test
// output.
//
// Requires:
//   - SAIL_BASE_URL / SAIL_CLIENT_ID / SAIL_CLIENT_SECRET (see testAccPreCheck)
//   - ACCTEST_IDENTITY_PROFILE_OWNER_ID: a valid identity id in the target
//     tenant to use as the profile's Required owner attribute.
//
// The config creates its own dedicated identitynow_source_v1 fixture and
// binds it as the profile's Required authoritative_source, mirroring the
// live-testing pass documented in the identitynow-terraform-provider-developer
// knowledge file's identity_profile_v1 entry: every real, pre-existing
// "authoritative" source in a live tenant is already 1:1 bound to an
// existing Identity Profile (the API rejects Create with a 409 reference
// conflict if reused), so only a fresh, never-before-bound source works
// here. That same entry also documents that this tenant's source_v1
// "authoritative" flag cannot be reliably set/read (a pre-existing,
// unrelated sources_v1 bug) - ignore_changes accounts for that noise so this
// test can focus on identity_profile_v1 itself. "identity_attribute_config"
// is intentionally left unconfigured: a freshly created source with no
// uploaded schema/accounts has no valid attributeName values to populate it
// with (confirmed live, HTTP 400 "Illegal value"), so only unit-test-level
// coverage exists for that dynamic JSON blob (see resource_identity_profile_test.go).
func TestAccIdentityProfileV1Resource(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped unless env 'TF_ACC' set")
	}

	resourceName := "identitynow_identity_profile_v1.test"
	ownerID := os.Getenv("ACCTEST_IDENTITY_PROFILE_OWNER_ID")
	if ownerID == "" {
		t.Fatal("ACCTEST_IDENTITY_PROFILE_OWNER_ID must be set to a valid identity id in the sandbox tenant for this test (owner is Required in the identity profile schema)")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckIdentityProfileV1Destroy,
		Steps: []resource.TestStep{
			{
				Config: testAccIdentityProfileV1Config("tf-acc-test-idprofile", "initial description", ownerID, 20),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", "tf-acc-test-idprofile"),
					resource.TestCheckResourceAttr(resourceName, "description", "initial description"),
					resource.TestCheckResourceAttr(resourceName, "priority", "20"),
					resource.TestCheckResourceAttr(resourceName, "owner.id", ownerID),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttrSet(resourceName, "authoritative_source.id"),
				),
			},
			{
				// ImportState verification: the resource must be importable
				// by its own id and produce an equivalent state.
				// "identity_attribute_config" is ignored: the live API
				// auto-populates a default mapping server-side even when
				// never configured, and a fresh import has no prior
				// practitioner-configured value to fall back to (see
				// dtoToModel's IdentityAttributeConfig handling in
				// resource_identity_profile.go).
				ResourceName:            resourceName,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"identity_attribute_config"},
			},
			{
				// Update: priority + description change in place (no
				// replacement) via the JSON Patch strategy in Update().
				Config: testAccIdentityProfileV1Config("tf-acc-test-idprofile", "updated description", ownerID, 30),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", "tf-acc-test-idprofile"),
					resource.TestCheckResourceAttr(resourceName, "description", "updated description"),
					resource.TestCheckResourceAttr(resourceName, "priority", "30"),
				),
			},
		},
	})
}

func testAccIdentityProfileV1Config(name, description, ownerID string, priority int) string {
	return fmt.Sprintf(`
resource "identitynow_source_v1" "authoritative" {
  name        = "%[1]s-authsource"
  connector   = "delimited-file"
  description = "Authoritative source fixture for the identity_profile_v1 acceptance test."

  owner = {
    id   = %[3]q
    type = "IDENTITY"
  }

  connector_attributes = jsonencode({
    fileLocationType = "Local"
  })

  # This tenant's source_v1 "authoritative" flag and several other
  # Computed attributes are known to flap/differ from what was configured
  # in this sandbox (a pre-existing sources_v1/tenant-specific quirk, not
  # related to identity_profile_v1) - ignored here so this test can focus
  # on identitynow_identity_profile_v1 itself. See the knowledge file's
  # identity_profile_v1 entry for the full investigation.
  lifecycle {
    ignore_changes = [
      authoritative, connector, connector_class, connector_id,
      connection_type, connector_implementation_id, connector_name,
      delete_threshold, features, healthy, schemas, since, status, type,
    ]
  }
}

resource "identitynow_identity_profile_v1" "test" {
  name        = %[1]q
  description = %[2]q
  priority    = %[4]d

  owner = {
    id   = %[3]q
    type = "IDENTITY"
  }

  authoritative_source = {
    id   = identitynow_source_v1.authoritative.id
    type = "SOURCE"
  }
}
`, name, description, ownerID, priority)
}

// testAccCheckIdentityProfileV1Destroy confirms every identity profile
// created by this test file no longer exists after Terraform destroys it
// (expects a 404 from the API). Delete is asynchronous server-side (polled
// internally via TaskManagementAPI in resource_identity_profile.go's
// Delete()), so by the time Terraform reports the destroy complete the
// profile should already be gone.
func testAccCheckIdentityProfileV1Destroy(s *terraform.State) error {
	client := sailpoint.NewAPIClient(sailpoint.NewDefaultConfiguration())

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "identitynow_identity_profile_v1" {
			continue
		}

		_, httpResp, err := client.Beta.IdentityProfilesAPI.
			GetIdentityProfile(context.Background(), rs.Primary.ID).
			Execute()
		if err == nil {
			return fmt.Errorf("identity profile %s still exists", rs.Primary.ID)
		}
		if httpResp == nil || httpResp.StatusCode != 404 {
			return fmt.Errorf("unexpected error checking identity profile %s was destroyed: %w", rs.Primary.ID, err)
		}
	}
	return nil
}
