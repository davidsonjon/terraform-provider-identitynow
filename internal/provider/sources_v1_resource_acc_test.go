package provider

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	sailpoint "github.com/sailpoint-oss/golang-sdk/v2"
)

// TestAccSourceV1Resource is an acceptance test exercising the full CRUD
// lifecycle of the identitynow_source_v1 resource against a real IdentityNow
// sandbox tenant. Run via `make testacc` (TF_ACC=1) - never against a
// production tenant, and never with credentials printed to test output.
//
// Requires:
//   - SAIL_BASE_URL / SAIL_CLIENT_ID / SAIL_CLIENT_SECRET (see testAccPreCheck)
//   - ACCTEST_SOURCE_OWNER_ID: a valid identity id in the target tenant to
//     use as the source's "owner" (Required in the source schema).
//   - ACCTEST_SOURCE_CONNECTOR: a valid connector script name in the target
//     tenant to create a source against. A "delimited-file" connector is
//     recommended (e.g. "delimited-file-<suffix>") since it requires no
//     external system connectivity and its "cluster" is always null,
//     avoiding a second tenant-specific reference id for this test.
//
// The update step changes connector_attributes itself (not just an unrelated
// scalar field), specifically to exercise the live-merge fix in Update()
// (see mergeConnectorAttributes in resource_source.go): confirms a genuine,
// intentional connector_attributes change applies cleanly rather than
// triggering the API's "Illegal attempt to modify \"healthy\" field" false
// conflict that an unconditional, configured-subset-only PATCH used to cause.
func TestAccSourceV1Resource(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped unless env 'TF_ACC' set")
	}

	resourceName := "identitynow_source_v1.test"
	ownerID := os.Getenv("ACCTEST_SOURCE_OWNER_ID")
	if ownerID == "" {
		t.Fatal("ACCTEST_SOURCE_OWNER_ID must be set to a valid identity id in the sandbox tenant for this test (owner is Required in the source schema)")
	}
	connector := os.Getenv("ACCTEST_SOURCE_CONNECTOR")
	if connector == "" {
		t.Fatal("ACCTEST_SOURCE_CONNECTOR must be set to a valid connector script name in the sandbox tenant for this test (a delimited-file connector is recommended)")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckSourceV1Destroy,
		Steps: []resource.TestStep{
			{
				Config: testAccSourceV1Config("tf-acc-test-source", "initial description", connector, ownerID, "Local"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", "tf-acc-test-source"),
					resource.TestCheckResourceAttr(resourceName, "description", "initial description"),
					resource.TestCheckResourceAttr(resourceName, "connector", connector),
					resource.TestCheckResourceAttr(resourceName, "owner.id", ownerID),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
				),
			},
			{
				// ImportState verification: the resource must be importable
				// by its own id and produce an equivalent state.
				// "connector_attributes" is ignored here: the live API
				// enriches connectorAttributes with extra
				// server/connector-computed keys (healthy, since, status,
				// connectionType, etc. - duplicates of top-level attributes)
				// that aren't part of what the practitioner configured, so a
				// fresh import (which has no prior practitioner-configured
				// value to fall back to) always reads back that enriched
				// superset rather than the original subset - a known,
				// documented limitation (see dtoToModel's ConnectorAttributes
				// handling in resource_source.go and the resource docs'
				// Known Limitations section), not a defect in this test.
				ResourceName:            resourceName,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"connector_attributes"},
			},
			{
				// Update: a genuine connector_attributes change, exercising
				// the live-merge fix in Update() (see package doc above).
				Config: testAccSourceV1Config("tf-acc-test-source", "updated description", connector, ownerID, "Attachment"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", "tf-acc-test-source"),
					resource.TestCheckResourceAttr(resourceName, "description", "updated description"),
				),
			},
		},
	})
}

func testAccSourceV1Config(name, description, connector, ownerID, fileLocationType string) string {
	return fmt.Sprintf(`
resource "identitynow_source_v1" "test" {
  name        = %[1]q
  description = %[2]q
  connector   = %[3]q

  owner = {
    id   = %[4]q
    type = "IDENTITY"
  }

  connector_attributes = jsonencode({
    fileLocationType = %[5]q
  })
}
`, name, description, connector, ownerID, fileLocationType)
}

// testAccCheckSourceV1Destroy confirms every source created by this test file
// no longer exists after Terraform destroys it (expects a 404 from the API).
// Sources were confirmed live to have a short eventual-consistency delay
// between a successful DELETE and the GET-by-id endpoint reflecting it (a
// GET immediately following DELETE can still briefly return 200) - a small
// retry/backoff accommodates that sandbox-side lag without masking a real
// dangling resource, which would still fail after these retries are exhausted.
func testAccCheckSourceV1Destroy(s *terraform.State) error {
	client := sailpoint.NewAPIClient(sailpoint.NewDefaultConfiguration())

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "identitynow_source_v1" {
			continue
		}

		var lastErr error
		for attempt := 0; attempt < 5; attempt++ {
			if attempt > 0 {
				time.Sleep(2 * time.Second)
			}
			_, httpResp, err := client.Beta.SourcesAPI.
				GetSource(context.Background(), rs.Primary.ID).
				Execute()
			if err == nil {
				lastErr = fmt.Errorf("source %s still exists", rs.Primary.ID)
				continue
			}
			if httpResp == nil || httpResp.StatusCode != 404 {
				return fmt.Errorf("unexpected error checking source %s was destroyed: %w", rs.Primary.ID, err)
			}
			lastErr = nil
			break
		}
		if lastErr != nil {
			return lastErr
		}
	}
	return nil
}
