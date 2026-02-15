// Package cnwlicense provides a Go client library for the CNW License Server.
//
// Install with:
//
//	go get github.com/CloudNativeWorks/cnw-license-sdk/cnwlicense
//
// It supports two modes of license validation:
//
//   - Online validation via the CNW License Server HTTP API
//   - Offline validation using Ed25519-signed license files
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
// # Offline (Air-gapped)
//
// For air-gapped environments with Ed25519-signed license files:
//
//	v := cnwlicense.NewOfflineValidator(cnwlicense.WithTrustedPublicKey(pubKeyBase64))
//	data, err := v.VerifyFile("/etc/myapp/license.json")
package cnwlicense
