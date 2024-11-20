package segment_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/davidsonjon/terraform-provider-identitynow/identitynow"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestAccSegmentResource(t *testing.T) {

	segmentName, ok := os.LookupEnv("ACC_TEST_SEGMENT_NAME")
	if !ok {
		t.Skip("Skipping TestAcc_Project: ACC_TEST_SEGMENT_NAME not specified")
	}

	resource.Test(t, resource.TestCase{
		PreCheck: func() {},
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"identitynow": providerserver.NewProtocol6WithError(identitynow.New("test")()),
		},
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: configSegmentResource(segmentName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("identitynow_access_profile.segment", "name", segmentName),
					// resource.TestCheckResourceAttr("identitynow_access_profile.test", "id", "example-id"),
				),
			},
			// ImportState testing
			{
				ResourceName:      "identitynow_access_profile.segment",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Update and Read testing
			{
				Config: configSegmentResource(segmentName + "2"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("identitynow_access_profile.segment", "name", segmentName+"2"),
				),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func configSegmentResource(name string) string {
	return fmt.Sprintf(`
	resource "identitynow_segment" "segment" {
		name        = "%s"
		description = "test desc"
		active      = false
		visibility_criteria = {}
	  }
`, name)
}
