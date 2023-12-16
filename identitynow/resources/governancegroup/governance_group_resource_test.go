package governancegroup_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/davidsonjon/terraform-provider-identitynow/identitynow"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestAccGovernanceGroupResource(t *testing.T) {
	govGroupIdentityId1, ok := os.LookupEnv("ACC_TEST_GOVGROUP_IDENTITY_ID1")
	if !ok {
		t.Skip("Skipping TestAcc_Project: ACC_TEST_GOVGROUP_IDENTITY_ID1 not specified")
	}

	govGroupIdentityId2, ok := os.LookupEnv("ACC_TEST_GOVGROUP_IDENTITY_ID2")
	if !ok {
		t.Skip("Skipping TestAcc_Project: ACC_TEST_GOVGROUP_IDENTITY_ID2 not specified")
	}

	govGroupName, ok := os.LookupEnv("ACC_TEST_GOVGROUP_NAME")
	if !ok {
		t.Skip("Skipping TestAcc_Project: ACC_TEST_GOVGROUP_NAME not specified")
	}

	resource.Test(t, resource.TestCase{
		PreCheck: func() {},
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"identitynow": providerserver.NewProtocol6WithError(identitynow.New("test")()),
		},
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: configGovernanceGroupResource(govGroupIdentityId1, govGroupIdentityId2, govGroupName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("identitynow_governance_group.test", "name", govGroupName),
				),
			},
			// ImportState testing
			{
				ResourceName:      "identitynow_governance_group.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Update and Read testing
			{
				Config: configGovernanceGroupResource(govGroupIdentityId1, govGroupIdentityId2, govGroupName+"-2"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("identitynow_governance_group.test", "name", govGroupName+"-2"),
				),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func configGovernanceGroupResource(id1, id2, name string) string {
	return fmt.Sprintf(`
	data "identitynow_identity" "test" {
		id = "%s"
	  }

	  data "identitynow_identity" "test2" {
		id = "%s"
	  }

resource "identitynow_governance_group" "test" {
  name = "%s"
  description = "test"
  owner = {
    id = data.identitynow_identity.test.id
  }
  membership = [
	{
		id = data.identitynow_identity.test.id
	},
	{
		id = data.identitynow_identity.test2.id
	}
  ]
}
`, id1, id2, name)
}
