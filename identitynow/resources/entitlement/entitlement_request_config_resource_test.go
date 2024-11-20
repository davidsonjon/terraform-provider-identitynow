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

func TestAccEntitlementRequestConfigResource(t *testing.T) {

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
			// Create and Read testing
			{
				Config: configEntitlementRequestConfigResource(entitlementId, "false", "false"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("identitynow_entitlement_request_config.entitlement_request_config", "access_request_config.comments_required", "false"),
					resource.TestCheckResourceAttr("identitynow_entitlement_request_config.entitlement_request_config", "access_request_config.denial_comments_required", "false"),
				),
			},
			// ImportState testing
			{
				ResourceName:      "identitynow_entitlement_request_config.entitlement_request_config",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateId:     entitlementId,
			},
			// Update and Read testing
			{
				Config: configEntitlementRequestConfigResource(entitlementId, "true", "true"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("identitynow_entitlement_request_config.entitlement_request_config", "access_request_config.comments_required", "true"),
					resource.TestCheckResourceAttr("identitynow_entitlement_request_config.entitlement_request_config", "access_request_config.denial_comments_required", "true"),
				),
			},
			// Delete testing automatically occurs in TestCase

		},
	})
}

func configEntitlementRequestConfigResource(id, commentsReq, denyCommentsReq string) string {
	return fmt.Sprintf(`
	resource "identitynow_entitlement" "entitlement_id" {
		id         = "%s"
	}

	resource "identitynow_entitlement_request_config" "entitlement_request_config" {
		id         = identitynow_entitlement.entitlement_id.id
		access_request_config = {
		  approval_schemes = [
			{
			  approver_type = "MANAGER",
			  approver_id   = null
			},
		  ]
		  comments_required        = %s
		  denial_comments_required = %s
		}
	  }
`, id, commentsReq, denyCommentsReq)
}
