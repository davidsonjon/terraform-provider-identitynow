package provider

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

// testAccProtoV6ProviderFactories is shared by every acceptance test
// (TF_ACC=1 go test ... / `make testacc`) in this package and its
// subpackages' resource/data source implementations.
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"identitynow": providerserver.NewProtocol6WithError(New()()),
}

// testAccPreCheck verifies the environment variables required to authenticate
// against a real IdentityNow tenant are set before running any acceptance
// test. It intentionally does not print their values. Run acceptance tests
// with `make testacc` (TF_ACC=1) against a sandbox tenant only - never
// production.
func testAccPreCheck(t *testing.T) {
	for _, envVar := range []string{"SAIL_BASE_URL", "SAIL_CLIENT_ID", "SAIL_CLIENT_SECRET"} {
		if os.Getenv(envVar) == "" {
			t.Fatalf("%s must be set for acceptance tests", envVar)
		}
	}
}
