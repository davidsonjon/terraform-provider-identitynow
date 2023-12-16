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

func TestAccExampleDataEntitlement(t *testing.T) {
	entitlementId, ok := os.LookupEnv("ACC_TEST_ENTITLEMENT_ID")
	if !ok {
		t.Skip("Skipping TestAcc_Project: ACC_TEST_ENTITLEMENT_ID not specified")
	}

	resource.Test(t, resource.TestCase{
		PreCheck: func() {},
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"identitynow": providerserver.NewProtocol6WithError(identitynow.New("test")()),
		},
		Steps: []resource.TestStep{
			// Read testing
			{
				Config: configEntitlementData(entitlementId),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.identitynow_entitlement.entitlement", "name", "AWS TEST GROUP - SWS"),
				),
			},
		},
	})
}

func configEntitlementData(id string) string {
	return fmt.Sprintf(`
	data "identitynow_entitlement" "entitlement" {
		id = "%s"
	  }
`, id)
}
