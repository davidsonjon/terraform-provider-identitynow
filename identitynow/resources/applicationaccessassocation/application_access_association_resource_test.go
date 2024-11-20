package applicationaccessassocation_test

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/davidsonjon/terraform-provider-identitynow/identitynow"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestAccApplicationAccessAssociationResource(t *testing.T) {

	accessProfileName, ok := os.LookupEnv("ACC_TEST_ACCESS_PROFILE_NAME")
	if !ok {
		t.Skip("Skipping TestAcc_Project: ACC_TEST_ACCESS_PROFILE_NAME not specified")
	}

	accessProfileName = accessProfileName + "-ACC"

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

	applicationName, ok := os.LookupEnv("ACC_TEST_APPLICATION_NAME")
	if !ok {
		t.Skip("Skipping TestAcc_Project: ACC_TEST_APPLICATION_NAME not specified")
	}

	applicationName = applicationName + "-ACC"

	resource.Test(t, resource.TestCase{
		PreCheck: func() {},
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"identitynow": providerserver.NewProtocol6WithError(identitynow.New("test")()),
		},
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccApplicationAccessAssociationResourceConfig(identityId, sourceId, entitlementId, accessProfileName, applicationName, []string{
					"identitynow_access_profile.access_profile1.id",
					"identitynow_access_profile.access_profile2.id",
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("identitynow_application_access_association.application_access_association", "access_profile_ids.0"),
					resource.TestCheckResourceAttrSet("identitynow_application_access_association.application_access_association", "access_profile_ids.1"),
				),
			},
			// Update and Read testing
			{
				Config: testAccApplicationAccessAssociationResourceConfig(identityId, sourceId, entitlementId, accessProfileName, applicationName, []string{
					"identitynow_access_profile.access_profile2.id",
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("identitynow_application_access_association.application_access_association", "access_profile_ids.0"),
				),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func testAccApplicationAccessAssociationResourceConfig(userId, sourceId, entitlementId, accessProfileName, appName string, configurableAttribute []string) string {
	// semiformat := fmt.Sprintf("%q\n", configurableAttribute) // Turn the slice into a string that looks like ["one" "two" "three"]
	// test := strings.Split(semiformat, " ")                   // Split this string by spaces

	tf := fmt.Sprintf(`
	data "identitynow_identity" "test" {
		id = "%s"
	}
	
	data "identitynow_source" "source" {
		id = "%s"
	}
	
	data "identitynow_entitlement" "entitlement" {
		id = "%s"
	  }
	
	resource "identitynow_access_profile" "access_profile1" {
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

	  resource "identitynow_access_profile" "access_profile2" {
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

resource "identitynow_application" "application" {
	name = %q
	description = "new test application"
	owner_external_id = data.identitynow_identity.test.id
	account_service_id = data.identitynow_source.source.connector_attributes.cloud_external_id
	launchpad_enabled = false
	provision_request_enabled = false
	access_profile_ids = []
	lifecycle {
		ignore_changes = [
			access_profile_ids,
		]
	}
}

resource "identitynow_application_access_association" "application_access_association" {
	application_id = identitynow_application.application.id
	access_profile_ids = [%v]
}
`, userId, sourceId, entitlementId, accessProfileName, (accessProfileName + "2"), appName, strings.Join(configurableAttribute, ", "))

	return tf
}
