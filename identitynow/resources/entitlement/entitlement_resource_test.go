package entitlement_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/davidsonjon/terraform-provider-identitynow/identitynow"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestAccEntitlementResource(t *testing.T) {

	entitlementId, ok := os.LookupEnv("ACC_TEST_ENTITLEMENT_ID")
	if !ok {
		t.Skip("Skipping TestAcc_Project: ACC_TEST_ENTITLEMENT_ID not specified")
	}

	entitlementValue, ok := os.LookupEnv("ACC_TEST_ENTITLEMENT_VALUE")
	if !ok {
		t.Skip("Skipping TestAcc_Project: ACC_TEST_ENTITLEMENT_VALUE not specified")
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
				Config: configEntitlementResource(entitlementId, "false", sourceId, entitlementValue),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("identitynow_entitlement.entitlement_id", "privileged", "false"),
					resource.TestCheckResourceAttr("identitynow_entitlement.entitlement_value", "privileged", "false"),
				),
			},
			// ImportState testing
			{
				ResourceName:      "identitynow_entitlement.entitlement_id",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateId:     entitlementId,
			},
			// Update and Read testing
			{
				Config: configEntitlementResource(entitlementId, "false", sourceId, entitlementValue),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("identitynow_entitlement.entitlement_id", "privileged", "false"),
					resource.TestCheckResourceAttr("identitynow_entitlement.entitlement_value", "privileged", "false"),
				),
			},
			// Delete testing automatically occurs in TestCase

		},
	})
}

func configEntitlementResource(id, priv, sourceId, value string) string {
	return fmt.Sprintf(`
	resource "identitynow_entitlement" "entitlement_id" {
		id         = "%s"
		privileged = %s
	}

	resource "identitynow_entitlement" "entitlement_value" {
		source_id = "%s"
		value = "%s"
		privileged = %s
	}
`, id, priv, sourceId, value, priv)
}
