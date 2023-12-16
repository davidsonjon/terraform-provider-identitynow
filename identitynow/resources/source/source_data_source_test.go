package source_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/davidsonjon/terraform-provider-identitynow/identitynow"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestAccExampleDataSource(t *testing.T) {

	sourceId, ok := os.LookupEnv("ACC_TEST_SOURCE_ID")
	if !ok {
		t.Skip("Skipping TestAcc_Project: ACC_TEST_SOURCE_ID not specified")
	}

	sourceName, ok := os.LookupEnv("ACC_TEST_SOURCE_NAME")
	if !ok {
		t.Skip("Skipping TestAcc_Project: ACC_TEST_SOURCE_NAME not specified")
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"identitynow": providerserver.NewProtocol6WithError(identitynow.New("test")()),
		},
		Steps: []resource.TestStep{
			// Read testing
			{
				Config: configSource(sourceId),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.identitynow_source.source", "name", sourceName),
				),
			},
		},
	})
}

func configSource(id string) string {
	return fmt.Sprintf(`
	data "identitynow_source" "source" {
		id = "%s"
	  }
	`,
		id,
	)
}
