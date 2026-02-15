// Package cnwlicense provides a Go client library for the CNW License Server.
//
// Install with:
//
//	go get github.com/CloudNativeWorks/cnw-license-sdk/cnwlicense
//
// It supports three modes of license validation:
//
//   - Online validation via the CNW License Server HTTP API
//   - Offline validation using Ed25519-signed license files
//   - Distributed node management with MongoDB or PostgreSQL registries
//
// # Quick Start
//
// For simple online license validation:
//
//	client := cnwlicense.NewOnlineClient("https://license.example.com", "your-api-key")
//	resp, err := client.Validate(ctx, cnwlicense.ValidateRequest{
//	    LicenseKey: "CNW-XXXX-YYYY-ZZZZ",
//	})
//
// # Distributed Systems
//
// For distributed systems that need node-level enforcement:
//
//	registry, _ := noderegistry.NewMongoRegistry(ctx, mongoDB)
//	mgr := cnwlicense.NewManager(
//	    cnwlicense.WithOnlineClient(client),
//	    cnwlicense.WithNodeRegistry(registry),
//	)
//	info, err := mgr.ValidateAndEnforce(ctx, "CNW-XXXX-YYYY-ZZZZ")
//	defer mgr.Shutdown(ctx)
//
// # Offline (Air-gapped)
//
// For air-gapped environments with Ed25519-signed license files:
//
//	v := cnwlicense.NewOfflineValidator(cnwlicense.WithTrustedPublicKey(pubKeyBase64))
//	data, err := v.VerifyFile("/etc/myapp/license.json")
package cnwlicense
