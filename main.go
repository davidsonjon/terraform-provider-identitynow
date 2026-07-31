package main

import (
	"context"
	"log"

	"terraform-provider-identitynow/internal/provider"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
)

func main() {
	opts := providerserver.ServeOpts{
		// Must be the full hostname/namespace/type address (not just
		// namespace/type) - providerserver.Serve fatally rejects anything
		// shorter, which would otherwise crash the binary on startup for
		// every real user installing this from the registry.
		Address: "registry.terraform.io/davidsonjon/identitynow",
	}

	err := providerserver.Serve(context.Background(), provider.New(), opts)
	if err != nil {
		log.Fatal(err.Error())
	}
}
