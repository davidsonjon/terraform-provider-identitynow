package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestAccSourceLoadEntitlementWaitV1Resource exercises
// identitynow_source_load_entitlement_wait_v1 against a real IdentityNow
// sandbox tenant: triggering entitlement aggregation on a real source,
// waiting for the launched task to complete, an in-place update of
// wait_for_active_jobs (no re-trigger), a triggers change forcing
// replacement (re-triggering aggregation), and import.
//
// Requires:
//   - SAIL_BASE_URL / SAIL_CLIENT_ID / SAIL_CLIENT_SECRET (see testAccPreCheck)
//   - ACCTEST_SOURCE_LOAD_ENTITLEMENT_WAIT_SOURCE_ID: an existing, directly
//     connected (non-delimited-file) source id in the target sandbox tenant
//     that is known to aggregate successfully. Delimited-file sources are
//     not supported by this resource (they require a CSV file body this
//     resource never sends), and a broken/misconfigured source will make
//     Create fail with a completionStatus=ERROR diagnostic - use a
//     dedicated, healthy fixture reserved for acceptance testing.
func TestAccSourceLoadEntitlementWaitV1Resource(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped unless env 'TF_ACC' set")
	}

	sourceID := os.Getenv("ACCTEST_SOURCE_LOAD_ENTITLEMENT_WAIT_SOURCE_ID")
	if sourceID == "" {
		t.Fatal("ACCTEST_SOURCE_LOAD_ENTITLEMENT_WAIT_SOURCE_ID must be set to a healthy, directly connected source id in the sandbox tenant for this test")
	}

	resourceName := "identitynow_source_load_entitlement_wait_v1.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				// Initial create: triggers aggregation and waits for
				// completion.
				Config: testAccSourceLoadEntitlementWaitV1Config(sourceID, "acctest-v1", true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "source_id", sourceID),
					resource.TestCheckResourceAttr(resourceName, "triggers.reason", "acctest-v1"),
					resource.TestCheckResourceAttr(resourceName, "wait_for_active_jobs", "true"),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
				),
			},
			{
				// wait_for_active_jobs-only change: Update in place, no
				// re-trigger.
				Config: testAccSourceLoadEntitlementWaitV1Config(sourceID, "acctest-v1", false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "wait_for_active_jobs", "false"),
				),
			},
			{
				// triggers change: forces replacement, re-triggering
				// aggregation.
				Config: testAccSourceLoadEntitlementWaitV1Config(sourceID, "acctest-v2", false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "triggers.reason", "acctest-v2"),
				),
			},
			{
				// Import: composite id parsing. ImportStateVerify can't be
				// used here - `id` is intentionally different post-import
				// (set to source_id, since no historical task id is
				// recoverable) than the task id `id` holds after a live
				// Create, so ImportStateVerify's ID-based state
				// correlation can never match. Use ImportStateCheck to
				// directly assert on the imported attributes instead.
				ResourceName: resourceName,
				ImportState:  true,
				ImportStateIdFunc: func(_ *terraform.State) (string, error) {
					return sourceID + ",reason:acctest-v2,false", nil
				},
				ImportStateCheck: func(states []*terraform.InstanceState) error {
					if len(states) != 1 {
						return fmt.Errorf("expected 1 imported state, got %d", len(states))
					}
					s := states[0]
					if got := s.Attributes["source_id"]; got != sourceID {
						return fmt.Errorf("imported source_id = %q, want %q", got, sourceID)
					}
					if got := s.Attributes["id"]; got != sourceID {
						return fmt.Errorf("imported id = %q, want %q (source_id, since no historical task id is recoverable)", got, sourceID)
					}
					if got := s.Attributes["triggers.reason"]; got != "acctest-v2" {
						return fmt.Errorf("imported triggers.reason = %q, want %q", got, "acctest-v2")
					}
					if got := s.Attributes["wait_for_active_jobs"]; got != "false" {
						return fmt.Errorf("imported wait_for_active_jobs = %q, want %q", got, "false")
					}
					return nil
				},
			},
		},
	})
}

func testAccSourceLoadEntitlementWaitV1Config(sourceID, triggerReason string, waitForActiveJobs bool) string {
	wait := "false"
	if waitForActiveJobs {
		wait = "true"
	}

	return `
resource "identitynow_source_load_entitlement_wait_v1" "test" {
  source_id = "` + sourceID + `"

  triggers = {
    reason = "` + triggerReason + `"
  }

  wait_for_active_jobs = ` + wait + `
}
`
}
