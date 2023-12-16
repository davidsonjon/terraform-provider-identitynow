package application_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/davidsonjon/terraform-provider-identitynow/identitynow"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestAccExampleDataApplication(t *testing.T) {

	applicationName, ok := os.LookupEnv("ACC_TEST_APPLICATION_NAME")
	if !ok {
		t.Skip("Skipping TestAcc_Project: ACC_TEST_APPLICATION_NAME not specified")
	}
	applicationName = applicationName + "-datasource"

	identityId, ok := os.LookupEnv("ACC_TEST_IDENTITY_ID")
	if !ok {
		t.Skip("Skipping TestAcc_Project: ACC_TEST_IDENTITY_ID not specified")
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
			// Read testing
			{
				Config: configApplicationData(applicationName, sourceId, "true", identityId),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.identitynow_application.application", "name", applicationName),
				),
			},
		},
	})
}

func configApplicationData(name, sourceId, enabled, userId string) string {
	return fmt.Sprintf(`
	data "identitynow_identity" "test" {
		id = "%s"
	}
	  
	data "identitynow_source" "source" {
		id = "%s"
	}
	  
	resource "identitynow_application" "application" {
		name        = "%s"
		description = "new test application"
	  
		owner = {
		  id = data.identitynow_identity.test.cc_id
		}
	  
		account_service_id = data.identitynow_source.source.connector_attributes.cloud_external_id
		launchpad_enabled  = %s
	  
	}
	  
	data "identitynow_application" "application" {
		id = identitynow_application.application.id
	}
`, userId, sourceId, name, enabled)
}
