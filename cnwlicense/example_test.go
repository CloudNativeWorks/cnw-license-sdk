package cnwlicense_test

import (
	"context"
	"fmt"

	"github.com/CloudNativeWorks/cnw-license-sdk/cnwlicense"
)

func ExampleNewOnlineClient() {
	client := cnwlicense.NewOnlineClient("https://license.example.com", "your-api-key")
	resp, err := client.Validate(context.Background(), cnwlicense.ValidateRequest{
		LicenseKey: "CNW-XXXX-YYYY-ZZZZ",
	})
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	fmt.Printf("Valid: %v\n", resp.Valid)
}

func ExampleNewOfflineValidator() {
	v := cnwlicense.NewOfflineValidator(
		cnwlicense.WithTrustedPublicKey("base64-encoded-public-key"),
	)
	data, err := v.VerifyFile("/etc/myapp/license.json")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	fmt.Printf("License: %s, Plan: %s\n", data.LicenseKey, data.Plan)
}

func ExampleExtractHardwareLimits() {
	features := map[string]interface{}{
		"max_cpu_per_node": float64(8),
		"max_nodes":        float64(3),
	}
	limits := cnwlicense.ExtractHardwareLimits(features)
	fmt.Printf("CPU limit: %d, Node limit: %d\n", limits.MaxCPUPerNode, limits.MaxNodes)

	if err := cnwlicense.CheckNodeCount(limits, 2); err != nil {
		fmt.Printf("Node check failed: %v\n", err)
	} else {
		fmt.Println("Node count OK")
	}
	// Output:
	// CPU limit: 8, Node limit: 3
	// Node count OK
}

func ExampleGenerateFingerprint() {
	fp, err := cnwlicense.GenerateFingerprint()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	fmt.Printf("Fingerprint length: %d\n", len(fp))
	// Output: Fingerprint length: 64
}
