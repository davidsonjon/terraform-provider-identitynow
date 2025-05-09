package metadataattribute_test

import (
	"fmt"
	"testing"

	"github.com/davidsonjon/terraform-provider-identitynow/identitynow"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestAccExampleDataAccessModelMetata(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() {},
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"identitynow": providerserver.NewProtocol6WithError(identitynow.New("test")()),
		},
		Steps: []resource.TestStep{
			// Read testing
			{
				Config: configSource(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.identitynow_access_model_metadata.access_model_metadata", "name", "Access Type"),
				),
			},
		},
	})
}

func configSource() string {
	return fmt.Sprintf(`
	// built-in key iscAccessType
	data "identitynow_access_model_metadata" "access_model_metadata" {
		key = "iscAccessType"
	}
	`,
	)
}
