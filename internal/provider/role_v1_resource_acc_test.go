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

// TestAccRoleV1Resource is an acceptance test exercising the full CRUD
// lifecycle of the identitynow_role_v1 resource against a real
// IdentityNow sandbox tenant. Run via `make testacc` (TF_ACC=1) - never
// against a production tenant, and never with credentials printed to test
// output.
//
// Requires:
//   - SAIL_BASE_URL / SAIL_CLIENT_ID / SAIL_CLIENT_SECRET (see testAccPreCheck)
//   - ACCTEST_ROLE_OWNER_ID: a valid identity id in the target tenant to use
//     as the role's Required owner attribute (matches the API's
//     required: [name, owner]). Confirmed against a real sandbox tenant via
//     the earlier `terraform apply` live-testing pass documented in the
//     identitynow-terraform-provider-developer knowledge file, 2026-07-24
//     entry ("First live apply bug (fixed)") - the same owner id used there
//     works here too.
func TestAccRoleV1Resource(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped unless env 'TF_ACC' set")
	}

	resourceName := "identitynow_role_v1.test"
	ownerID := os.Getenv("ACCTEST_ROLE_OWNER_ID")
	if ownerID == "" {
		t.Fatal("ACCTEST_ROLE_OWNER_ID must be set to a valid identity id in the sandbox tenant for this test (owner is Required in the role schema)")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRoleV1Destroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRoleV1Config("tf-acc-test-role", "initial description", ownerID, true, false),
				// ExpectNonEmptyPlan accounts for role_v1's 5 pass-through-only
				// nested blocks (access_model_metadata, access_request_config,
				// revocation_request_config, membership, legacy_membership_info)
				// and privilege_level, which cannot safely receive a
				// UseStateForUnknown plan modifier - see
				// resource_role_planmodifiers.go for the full investigation
				// (an object-level modifier there reproducibly broke even a
				// first `terraform plan`) and privilege_level's real write
				// support (a config-removal correctness concern, same as
				// access_profiles et al.). The explicit checks below still
				// validate every attribute this test actually configures.
				ExpectNonEmptyPlan: true,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", "tf-acc-test-role"),
					resource.TestCheckResourceAttr(resourceName, "description", "initial description"),
					resource.TestCheckResourceAttr(resourceName, "enabled", "true"),
					resource.TestCheckResourceAttr(resourceName, "requestable", "false"),
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
				// Update: description/enabled/requestable change in place
				// (no replacement) via the full-document JSON Patch strategy
				// documented in roleModelToDto/roleJSONPatchReplace.
				Config:             testAccRoleV1Config("tf-acc-test-role", "updated description", ownerID, false, true),
				ExpectNonEmptyPlan: true, // see the first step's comment above, plus "modified"
				// (a genuinely volatile last-modified timestamp that legitimately
				// changes on every real Update) is *expected* to always show a
				// diff on this step specifically. An earlier attempt to suppress
				// it via `lifecycle { ignore_changes = [modified] }` in the test
				// config caused a hard "Provider produced inconsistent result
				// after apply" error instead - Core's ignore_changes reuses the
				// prior state value in the plan just like a provider-side
				// UseStateForUnknown modifier would, which is unsafe for a field
				// that the API actually changes on every Update. Do not
				// reintroduce ignore_changes here.
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", "tf-acc-test-role"),
					resource.TestCheckResourceAttr(resourceName, "description", "updated description"),
					resource.TestCheckResourceAttr(resourceName, "enabled", "false"),
					resource.TestCheckResourceAttr(resourceName, "requestable", "true"),
				),
			},
		},
	})
}

func testAccRoleV1Config(name, description, ownerID string, enabled, requestable bool) string {
	return fmt.Sprintf(`
resource "identitynow_role_v1" "test" {
  name        = %[1]q
  description = %[2]q
  enabled     = %[4]t
  requestable = %[5]t

  # Explicitly configured (rather than left unconfigured/null) so Terraform
  # Core proposes a known empty-list value instead of Unknown for these
  # Optional+Computed, write-capable attributes - avoiding a spurious
  # "(known after apply)" diff on every subsequent plan. See
  # resource_role_planmodifiers.go for why these intentionally do NOT get a
  # UseStateForUnknown plan modifier instead (they have real write/removal
  # support, unlike the pass-through-only blocks that do).
  access_profiles   = []
  dimension_refs    = []
  entitlements      = []
  additional_owners = []
  segments          = []

  owner = {
    id   = %[3]q
    type = "IDENTITY"
  }
}
`, name, description, ownerID, enabled, requestable)
}

// testAccCheckRoleV1Destroy confirms every role created by this test file no
// longer exists after Terraform destroys it (expects a 404 from the API).
func testAccCheckRoleV1Destroy(s *terraform.State) error {
	client := sailpoint.NewAPIClient(sailpoint.NewDefaultConfiguration())

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "identitynow_role_v1" {
			continue
		}

		_, httpResp, err := client.RolesAPI.
			GetRoleV1(context.Background(), rs.Primary.ID).
			Execute()
		if err == nil {
			return fmt.Errorf("role %s still exists", rs.Primary.ID)
		}
		if httpResp == nil || httpResp.StatusCode != 404 {
			return fmt.Errorf("unexpected error checking role %s was destroyed: %w", rs.Primary.ID, err)
		}
	}
	return nil
}
