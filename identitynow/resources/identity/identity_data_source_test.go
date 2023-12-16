package identity_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/davidsonjon/terraform-provider-identitynow/identitynow"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestAccExampleDataIdentity(t *testing.T) {
	identityId, ok := os.LookupEnv("ACC_TEST_IDENTITY_ID")
	if !ok {
		t.Skip("Skipping TestAcc_Project: ACC_TEST_IDENTITY_ID not specified")
	}

	identityName, ok := os.LookupEnv("ACC_TEST_IDENTITY_NAME")
	if !ok {
		t.Skip("Skipping TestAcc_Project: ACC_TEST_IDENTITY_NAME not specified")
	}

	resource.Test(t, resource.TestCase{
		PreCheck: func() {},
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"identitynow": providerserver.NewProtocol6WithError(identitynow.New("test")()),
		},
		Steps: []resource.TestStep{
			// Read testing
			{
				Config: configSource(identityId),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.identitynow_identity.test", "name", identityName),
				),
			},
		},
	})
}

func configSource(id string) string {
	return fmt.Sprintf(`
	data "identitynow_identity" "test" {
		id = "%s"
	  }
	`,
		id,
	)
}
