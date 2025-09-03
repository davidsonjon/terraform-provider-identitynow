package identity_test

import (
	"fmt"
	"testing"

	"github.com/davidsonjon/terraform-provider-identitynow/identitynow"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestAccExampleDataIdentities(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() {},
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"identitynow": providerserver.NewProtocol6WithError(identitynow.New("test")()),
		},
		Steps: []resource.TestStep{
			{
				Config: testAccIdentitiesDataSourceConfig(`alias sw "alice"`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.identitynow_identities.test", "identities.#"),
				),
			},
		},
	})
}

func TestAccExampleDataIdentitiesWithLimit(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() {},
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"identitynow": providerserver.NewProtocol6WithError(identitynow.New("test")()),
		},
		Steps: []resource.TestStep{
			{
				Config: testAccIdentitiesDataSourceConfigWithLimit(`email co "@"`, 10),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.identitynow_identities.test", "identities.#"),
				),
			},
		},
	})
}

func TestAccExampleDataIdentitiesEmailFilter(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() {},
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"identitynow": providerserver.NewProtocol6WithError(identitynow.New("test")()),
		},
		Steps: []resource.TestStep{
			{
				Config: testAccIdentitiesDataSourceConfig(`email sw "alice"`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.identitynow_identities.test", "identities.#"),
				),
			},
		},
	})
}

func TestAccExampleDataIdentitiesNoFilter(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() {},
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"identitynow": providerserver.NewProtocol6WithError(identitynow.New("test")()),
		},
		Steps: []resource.TestStep{
			{
				Config: testAccIdentitiesDataSourceConfigNoFilter(5),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.identitynow_identities.test", "identities.#"),
				),
			},
		},
	})
}

func testAccIdentitiesDataSourceConfig(filter string) string {
	return fmt.Sprintf(`
data "identitynow_identities" "test" {
  filters = "%s"
}
`, filter)
}

func testAccIdentitiesDataSourceConfigWithLimit(filter string, limit int) string {
	return fmt.Sprintf(`
data "identitynow_identities" "test" {
  filters = "%s"
  limit   = %d
}
`, filter, limit)
}

func testAccIdentitiesDataSourceConfigNoFilter(limit int) string {
	return fmt.Sprintf(`
data "identitynow_identities" "test" {
  limit = %d
}
`, limit)
}
