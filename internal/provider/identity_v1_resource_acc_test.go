package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccIdentityV1(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped unless env 'TF_ACC' set")
	}

	identityID := os.Getenv("ACCTEST_IDENTITY_ID")
	if identityID == "" {
		t.Fatal("ACCTEST_IDENTITY_ID must be set to a valid identity id in the sandbox tenant for this test")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccIdentityV1Config(identityID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.identitynow_identity_v1.by_id", "id", identityID),
					resource.TestCheckResourceAttrSet("data.identitynow_identity_v1.by_id", "alias"),
					resource.TestCheckResourceAttrSet("data.identitynow_identity_v1.by_id", "email_address"),
					resource.TestCheckResourceAttrPair("data.identitynow_identity_v1.by_alias", "id", "data.identitynow_identity_v1.by_id", "id"),
					resource.TestCheckResourceAttrPair("data.identitynow_identity_v1.by_email", "id", "data.identitynow_identity_v1.by_id", "id"),
					resource.TestCheckResourceAttrSet("data.identitynow_identity_v1.by_alias", "alias"),
					resource.TestCheckResourceAttrSet("data.identitynow_identity_v1.by_email", "email_address"),
					resource.TestCheckResourceAttr("data.identitynow_identities_v1.filtered", "identities.#", "1"),
				),
			},
		},
	})
}

func testAccIdentityV1Config(identityID string) string {
	filter := fmt.Sprintf("id eq \"%s\"", identityID)
	return fmt.Sprintf(`
data "identitynow_identity_v1" "by_id" {
  id = %[1]q
}

data "identitynow_identity_v1" "by_alias" {
  alias = data.identitynow_identity_v1.by_id.alias
}

data "identitynow_identity_v1" "by_email" {
  email_address = data.identitynow_identity_v1.by_id.email_address
}

data "identitynow_identities_v1" "filtered" {
  filters = %[2]q
  limit   = 5
}
`, identityID, filter)
}
