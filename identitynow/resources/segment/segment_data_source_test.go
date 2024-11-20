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

func TestAccExampleDataSegment(t *testing.T) {
	segmentName, ok := os.LookupEnv("ACC_TEST_SEGMENT_NAME")
	if !ok {
		t.Skip("Skipping TestAcc_Project: ACC_TEST_SEGMENT_NAME not specified")
	}

	segmentId, ok := os.LookupEnv("ACC_TEST_SEGMENT_ID")
	if !ok {
		t.Skip("Skipping TestAcc_Project: ACC_TEST_SEGMENT_ID not specified")
	}

	resource.Test(t, resource.TestCase{
		PreCheck: func() {},
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"identitynow": providerserver.NewProtocol6WithError(identitynow.New("test")()),
		},
		Steps: []resource.TestStep{
			// Read testing
			{
				Config: configSegmentData(segmentId, segmentName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.identitynow_segment.segment", "name", segmentName),
				),
			},
			{
				Config: configSegmentData(segmentId, segmentName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.identitynow_segment.segment_name", "id", segmentId),
				),
			},
		},
	})
}

func configSegmentData(id, name string) string {
	return fmt.Sprintf(`
	data "identitynow_segment" "segment" {
		id = "%s"
	  }

	data "identitynow_segment" "segment_name" {
		name = "%s"
	}
`, id, name)
}
