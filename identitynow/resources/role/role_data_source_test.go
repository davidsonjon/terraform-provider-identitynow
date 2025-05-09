package role_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/davidsonjon/terraform-provider-identitynow/identitynow"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestAccExampleDataAccessProfile(t *testing.T) {
	identityId, ok := os.LookupEnv("ACC_TEST_IDENTITY_ID")
	if !ok {
		t.Skip("Skipping TestAcc_Project: ACC_TEST_IDENTITY_ID not specified")
	}

	entitlementId, ok := os.LookupEnv("ACC_TEST_ENTITLEMENT_ID")
	if !ok {
		t.Skip("Skipping TestAcc_Project: ACC_TEST_ENTITLEMENT_ID not specified")
	}

	accessProfileId, ok := os.LookupEnv("ACC_TEST_ACCESS_PROFILE_ID")
	if !ok {
		t.Skip("Skipping TestAcc_Project: ACC_TEST_ACCESS_PROFILE_ID not specified")
	}

	resource.Test(t, resource.TestCase{
		PreCheck: func() {},
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"identitynow": providerserver.NewProtocol6WithError(identitynow.New("test")()),
		},
		Steps: []resource.TestStep{
			// Read testing
			{
				Config: configAccessProfileData(identityId, entitlementId, accessProfileId, "TF-ROLE-TEST", "TF testing", false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.identitynow_role.role", "name", "TF-ROLE-TEST"),
				),
			},
		},
	})
}

func configAccessProfileData(identityId, entitlementId, accessProfileId, roleName, roleDescription string, roleEnabled bool) string {
	return fmt.Sprintf(`
data "identitynow_identity" "test" {
	id = "%s"
}

data "identitynow_entitlement" "entitlement" {
	id = "%s"
  }

data "identitynow_access_profile" "access_profile" {
  id = "%s"
}

resource "identitynow_role" "role" {
  name        = "%s"
  description = "%s"
  enabled     = %t
  requestable = true
  owner = {
    id = data.identitynow_identity.test.id
    type = "IDENTITY"
  }
  access_request_config = {
    approval_schemes = [
      {
        approver_type = "MANAGER",
        approver_id   = null
      }
    ]
    comments_required        = true
    denial_comments_required = true
  }
  entitlements = [
    {
      id   = data.identitynow_entitlement.entitlement.id,
      type = "ENTITLEMENT",
    },
  ]
  access_profiles = [
    {
      id   = data.identitynow_access_profile.access_profile.id,
      type = "ACCESS_PROFILE",
    },
  ]
}

data "identitynow_role" "role" {
  id = identitynow_role.role.id
}
`, identityId, entitlementId, accessProfileId, roleName, roleDescription, roleEnabled)
}
