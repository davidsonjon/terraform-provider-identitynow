package metadataattribute_test

import (
	"fmt"
	"testing"

	"github.com/davidsonjon/terraform-provider-identitynow/identitynow"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestAccMetadataAttributeResource(t *testing.T) {

	resource.Test(t, resource.TestCase{
		PreCheck: func() {},
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"identitynow": providerserver.NewProtocol6WithError(identitynow.New("test")()),
		},
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: configAccessProfileResource("TFTEST", "Test Description"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("identitynow_access_model_metadata.access_model_metadata", "name", "TFTEST"),
					resource.TestCheckResourceAttr("identitynow_access_model_metadata.access_model_metadata", "key", "entTFTEST"),
				),
			},
			// ImportState testing
			{
				ResourceName:      "identitynow_access_model_metadata.access_model_metadata",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateId:     "key",
			},
			// Update and Read testing
			{
				Config: configAccessProfileResource("TFTEST", "Test Description2"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("identitynow_access_model_metadata.access_model_metadata", "description", "Test Description2"),
				),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func configAccessProfileResource(name, description string) string {
	return fmt.Sprintf(`
resource "identitynow_access_model_metadata" "access_model_metadata" {
  name         = "%s"
  object_types = ["entitlement"]
  description  = "%s"
  values = [
    {
      name  = "def"
      value = "def"
    },
    {
      name  = "xyz"
      value = "xyz"
  }]
}
`, name, description)
}
