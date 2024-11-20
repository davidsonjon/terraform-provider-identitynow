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

func TestAccSegmentAccessResource(t *testing.T) {

	accessProfileId, ok := os.LookupEnv("ACC_TEST_ACCESS_PROFILE_ID")
	if !ok {
		t.Skip("Skipping TestAcc_Project: ACC_TEST_ACCESS_PROFILE_ID not specified")
	}

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
				Config: configSegmentAccessResource(segmentName, accessProfileId),
			},
			// ImportState testing
			{
				ResourceName:      "identitynow_access_profile.access_profile",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func configSegmentAccessResource(name, id string) string {
	return fmt.Sprintf(`
resource "identitynow_segment" "segment" {
	name        = "%s"
	description = "test desc"
	active      = false
	visibility_criteria = {}
}
	
resource "identitynow_segment_access" "segment_access" {
	segment_id        = identitynow_segment.segment.id
	assignments = [
		{
			type = "ACCESS_PROFILE"
			id   = "%s"
		},
	]
}
`, name, id)
}
