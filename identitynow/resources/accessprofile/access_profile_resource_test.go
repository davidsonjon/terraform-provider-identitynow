package accessprofile_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/davidsonjon/terraform-provider-identitynow/identitynow"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestAccAccessProfileResource(t *testing.T) {

	accessProfileName, ok := os.LookupEnv("ACC_TEST_ACCESS_PROFILE_NAME")
	if !ok {
		t.Skip("Skipping TestAcc_Project: ACC_TEST_ACCESS_PROFILE_NAME not specified")
	}

	identityId, ok := os.LookupEnv("ACC_TEST_IDENTITY_ID")
	if !ok {
		t.Skip("Skipping TestAcc_Project: ACC_TEST_IDENTITY_ID not specified")
	}

	entitlementId, ok := os.LookupEnv("ACC_TEST_ENTITLEMENT_ID")
	if !ok {
		t.Skip("Skipping TestAcc_Project: ACC_TEST_ENTITLEMENT_ID not specified")
	}

	sourceId, ok := os.LookupEnv("ACC_TEST_SOURCE_ID")
	if !ok {
		t.Skip("Skipping TestAcc_Project: ACC_TEST_SOURCE_ID not specified")
	}

	resource.Test(t, resource.TestCase{
		PreCheck: func() {},
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"identitynow": providerserver.NewProtocol6WithError(identitynow.New("test")()),
		},
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: configAccessProfileResource(accessProfileName, identityId, sourceId, entitlementId),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("identitynow_access_profile.access_profile", "name", accessProfileName),
					// resource.TestCheckResourceAttr("identitynow_access_profile.test", "id", "example-id"),
				),
			},
			// ImportState testing
			{
				ResourceName:      "identitynow_access_profile.access_profile",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Update and Read testing
			{
				Config: configAccessProfileResource(accessProfileName+"2", identityId, sourceId, entitlementId),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("identitynow_access_profile.access_profile", "name", accessProfileName+"2"),
				),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func configAccessProfileResource(name, userId, sourceId, entitlementId string) string {
	return fmt.Sprintf(`
data "identitynow_identity" "test" {
	id = "%s"
}

data "identitynow_source" "source" {
	id = "%s"
}

data "identitynow_entitlement" "entitlement" {
	id = "%s"
  }

resource "identitynow_access_profile" "access_profile" {
	name        = "%s"
	description = "Terraform Test"
	requestable = true
	enabled     = true
	owner = {
	  id   = data.identitynow_identity.test.id
	  name = data.identitynow_identity.test.name
	  type = "IDENTITY"
	}
	source = {
	  id   = data.identitynow_source.source.id
	  name = data.identitynow_source.source.name
	  type = "SOURCE"
	}
	entitlements = [
	  {
		id   = data.identitynow_entitlement.entitlement.id,
		type = "ENTITLEMENT",
	  },
	]
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
  }
`, userId, sourceId, entitlementId, name)
}
