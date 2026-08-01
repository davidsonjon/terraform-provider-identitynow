package provider

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	sailpoint "github.com/sailpoint-oss/golang-sdk/v3"
	"github.com/sailpoint-oss/golang-sdk/v3/entitlements"
)

const testAccEntitlementRequestConfigEntitlementID = "00124ded10e14880b767a9c0130730b8"

type testAccEntitlementRequestConfigApprover struct {
	Type string
	ID   string
}

// TestAccEntitlementRequestConfigV1Resource exercises the adopt-existing live
// lifecycle of identitynow_entitlement_request_config_v1 against the dedicated
// sandbox entitlement fixture already used by identitynow_entitlement_v1. The
// managed resource updates only the entitlement's request-config document;
// Delete intentionally removes Terraform state only.
func TestAccEntitlementRequestConfigV1Resource(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped unless env 'TF_ACC' set")
	}

	testAccPreCheck(t)
	client := testAccAPIClient()
	resourceName := "identitynow_entitlement_request_config_v1.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckEntitlementRequestConfigV1Destroy,
		Steps: []resource.TestStep{
			{
				Config: testAccEntitlementRequestConfigV1Config(false, true, true, false, []testAccEntitlementRequestConfigApprover{
					{Type: "MANAGER"},
					{Type: "GOVERNANCE_GROUP", ID: testAccFixtureGovernanceGroupID},
				}, []testAccEntitlementRequestConfigApprover{
					{Type: "MANAGER"},
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "id", testAccEntitlementRequestConfigEntitlementID),
					resource.TestCheckResourceAttr(resourceName, "access_request_config.request_comment_required", "false"),
					resource.TestCheckResourceAttr(resourceName, "access_request_config.denial_comment_required", "true"),
					resource.TestCheckResourceAttr(resourceName, "access_request_config.reauthorization_required", "true"),
					resource.TestCheckResourceAttr(resourceName, "access_request_config.require_end_date", "false"),
					testAccCheckEntitlementRequestConfigLive(
						client,
						false,
						true,
						true,
						false,
						[]testAccEntitlementRequestConfigApprover{
							{Type: "MANAGER"},
							{Type: "GOVERNANCE_GROUP", ID: testAccFixtureGovernanceGroupID},
						},
						[]testAccEntitlementRequestConfigApprover{
							{Type: "MANAGER"},
						},
					),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccEntitlementRequestConfigV1Config(true, false, false, true, []testAccEntitlementRequestConfigApprover{
					{Type: "GOVERNANCE_GROUP", ID: testAccFixtureGovernanceGroupID},
				}, []testAccEntitlementRequestConfigApprover{
					{Type: "GOVERNANCE_GROUP", ID: testAccFixtureGovernanceGroupID},
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "access_request_config.request_comment_required", "true"),
					resource.TestCheckResourceAttr(resourceName, "access_request_config.denial_comment_required", "false"),
					resource.TestCheckResourceAttr(resourceName, "access_request_config.reauthorization_required", "false"),
					resource.TestCheckResourceAttr(resourceName, "access_request_config.require_end_date", "true"),
					testAccCheckEntitlementRequestConfigLive(
						client,
						true,
						false,
						false,
						true,
						[]testAccEntitlementRequestConfigApprover{
							{Type: "GOVERNANCE_GROUP", ID: testAccFixtureGovernanceGroupID},
						},
						[]testAccEntitlementRequestConfigApprover{
							{Type: "GOVERNANCE_GROUP", ID: testAccFixtureGovernanceGroupID},
						},
					),
				),
			},
			{
				Config: testAccEntitlementRequestConfigV1EntitlementOnlyConfig(),
				Check:  testAccCheckResourceMissingFromState(resourceName),
			},
		},
	})
}

func testAccEntitlementRequestConfigV1Config(
	requestCommentRequired, denialCommentRequired, reauthorizationRequired, requireEndDate bool,
	approvalSchemes, revocationSchemes []testAccEntitlementRequestConfigApprover,
) string {
	return fmt.Sprintf(`
resource "identitynow_entitlement_v1" "test" {
  id          = %[1]q
  requestable = true
}

resource "identitynow_entitlement_request_config_v1" "test" {
  id = identitynow_entitlement_v1.test.id

  access_request_config = {
    approval_schemes = [
%[6]s
    ]
    request_comment_required = %[2]t
    denial_comment_required  = %[3]t
    reauthorization_required = %[4]t
    require_end_date         = %[5]t
  }

  revocation_request_config = {
    revocation_approval_schemes = [
%[7]s
    ]
  }
}
`, testAccEntitlementRequestConfigEntitlementID, requestCommentRequired, denialCommentRequired, reauthorizationRequired, requireEndDate, testAccEntitlementRequestConfigApproverBlocks(approvalSchemes), testAccEntitlementRequestConfigApproverBlocks(revocationSchemes))
}

func testAccEntitlementRequestConfigV1EntitlementOnlyConfig() string {
	return fmt.Sprintf(`
resource "identitynow_entitlement_v1" "test" {
  id          = %q
  requestable = true
}
`, testAccEntitlementRequestConfigEntitlementID)
}

func testAccEntitlementRequestConfigApproverBlocks(items []testAccEntitlementRequestConfigApprover) string {
	blocks := ""
	for _, item := range items {
		if item.ID == "" {
			blocks += fmt.Sprintf(`      {
        approver_type = %q
      },
`, item.Type)
			continue
		}

		blocks += fmt.Sprintf(`      {
        approver_type = %q
        approver_id   = %q
      },
`, item.Type, item.ID)
	}
	return blocks
}

func testAccCheckEntitlementRequestConfigLive(
	client *sailpoint.APIClient,
	requestCommentRequired, denialCommentRequired, reauthorizationRequired, requireEndDate bool,
	wantApprovalSchemes, wantRevocationSchemes []testAccEntitlementRequestConfigApprover,
) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		return testAccRetry("entitlement request config live state", func() (bool, error) {
			dto, httpResp, err := client.EntitlementsAPI.
				GetEntitlementRequestConfigV1(context.Background(), testAccEntitlementRequestConfigEntitlementID).
				Execute()
			if err != nil {
				return false, fmt.Errorf("getting entitlement request config: %s", testAccHTTPError(err, httpResp))
			}

			access, ok := dto.GetAccessRequestConfigOk()
			if !ok || access == nil {
				return false, nil
			}
			if access.GetRequestCommentRequired() != requestCommentRequired ||
				access.GetDenialCommentRequired() != denialCommentRequired ||
				access.GetReauthorizationRequired() != reauthorizationRequired ||
				access.GetRequireEndDate() != requireEndDate {
				return false, nil
			}
			if !testAccApprovalSchemesMatch(access.GetApprovalSchemes(), wantApprovalSchemes) {
				return false, nil
			}

			revocation, ok := dto.GetRevocationRequestConfigOk()
			if !ok || revocation == nil {
				return false, nil
			}
			if !testAccApprovalSchemesMatch(revocation.GetApprovalSchemes(), wantRevocationSchemes) {
				return false, nil
			}

			return true, nil
		})
	}
}

func testAccApprovalSchemesMatch(got []entitlements.EntitlementApprovalScheme, want []testAccEntitlementRequestConfigApprover) bool {
	if len(got) != len(want) {
		return false
	}

	remaining := make([]testAccEntitlementRequestConfigApprover, len(want))
	copy(remaining, want)

	for _, scheme := range got {
		matched := false
		for i, wantScheme := range remaining {
			if scheme.GetApproverType() != wantScheme.Type {
				continue
			}

			gotID, ok := scheme.GetApproverIdOk()
			switch {
			case wantScheme.ID == "" && ok && gotID != nil && *gotID != "":
				continue
			case wantScheme.ID == "" && ok && gotID != nil && *gotID == "":
				// treat empty-string approver ids the same as omitted
			case wantScheme.ID != "":
				if !ok || gotID == nil || *gotID != wantScheme.ID {
					continue
				}
			}

			remaining = append(remaining[:i], remaining[i+1:]...)
			matched = true
			break
		}
		if !matched {
			return false
		}
	}

	return len(remaining) == 0
}

func testAccCheckResourceMissingFromState(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if _, ok := s.RootModule().Resources[resourceName]; ok {
			return fmt.Errorf("resource %s still present in state", resourceName)
		}
		return nil
	}
}

func testAccCheckEntitlementRequestConfigV1Destroy(s *terraform.State) error {
	for _, rs := range s.RootModule().Resources {
		if rs.Type == "identitynow_entitlement_request_config_v1" {
			return fmt.Errorf("entitlement request config resource %s still present in state after destroy", rs.Primary.ID)
		}
	}
	return nil
}
