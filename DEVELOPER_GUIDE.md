# CNW License SDK - Developer Guide

Go SDK for integrating license validation and hardware enforcement into your applications.

```
go get github.com/CloudNativeWorks/cnw-license-sdk/cnwlicense
```

## Table of Contents

- [Architecture Overview](#architecture-overview)
- [Quick Start](#quick-start)
- [Online License Validation](#online-license-validation)
- [Offline License Validation](#offline-license-validation)
- [Hardware Enforcement](#hardware-enforcement)
- [Machine Fingerprinting](#machine-fingerprinting)
- [Manager (Full Orchestration)](#manager-full-orchestration)
- [Error Handling](#error-handling)
- [Integration Patterns](#integration-patterns)
- [API Reference](#api-reference)

---

## Architecture Overview

The SDK has two layers you can use independently or together:

```
+--------------------------------------+
|         Manager (Orchestrator)       |
|   ValidateAndEnforce / ActivateNode  |
+----------+----------------+----------+
           |                |
    +------+------+  +------+------+
    | OnlineClient|  | Offline     |
    | (HTTP API)  |  | Validator   |
    +------+------+  +------+------+
           |                |
    +------+------+  +------+------+
    | Hardware    |  | Ed25519     |
    | Limits      |  | Signatures  |
    +-------------+  +-------------+
```

| Component | Use Case |
|-----------|----------|
| `OnlineClient` | SaaS apps with internet access |
| `OfflineValidator` | Air-gapped / on-premise environments |
| `Manager` | All-in-one: combines validation + hardware enforcement |

---

## Quick Start

### Simplest possible integration (4 lines)

```go
package main

import (
    "context"
    "log"

    "github.com/CloudNativeWorks/cnw-license-sdk/cnwlicense"
)

func main() {
    client := cnwlicense.NewOnlineClient("https://license.example.com", "your-api-key")
    resp, err := client.Validate(context.Background(), cnwlicense.ValidateRequest{
        LicenseKey: "CNW-XXXX-YYYY-ZZZZ",
    })
    if err != nil {
        log.Fatal(err)
    }
    if !resp.Valid {
        log.Fatalf("License invalid: %s", resp.Reason)
    }
    log.Printf("License valid, expires: %v", resp.ExpiresAt)
}
```

---

## Online License Validation

### Creating a Client

```go
// Basic
client := cnwlicense.NewOnlineClient("https://license.example.com", "your-api-key")

// With custom timeout
client := cnwlicense.NewOnlineClient(serverURL, apiKey,
    cnwlicense.WithTimeout(5 * time.Second),
)

// With a pre-generated fingerprint (e.g., read from DB)
client := cnwlicense.NewOnlineClient(serverURL, apiKey,
    cnwlicense.WithFingerprint(savedFingerprint),
)

// With custom HTTP client (e.g., for proxies, TLS config)
httpClient := &http.Client{
    Transport: &http.Transport{
        TLSClientConfig: &tls.Config{
            RootCAs: customCertPool,
        },
    },
    Timeout: 15 * time.Second,
}
client := cnwlicense.NewOnlineClient(serverURL, apiKey,
    cnwlicense.WithHTTPClient(httpClient),
    cnwlicense.WithUserAgent("my-app/1.0"),
)
```

### Validating a License

```go
resp, err := client.Validate(ctx, cnwlicense.ValidateRequest{
    LicenseKey:  "CNW-XXXX-YYYY-ZZZZ",
    Fingerprint: "optional-machine-id",  // optional
    Version:     "2.1.0",                // optional
})
if err != nil {
    // Network error or server error (see Error Handling)
    return err
}

if !resp.Valid {
    // License exists but is invalid
    log.Printf("License rejected: %s", resp.Reason)
    // resp.Reason can be: "license not found", "license is suspended", "license expired"
    return fmt.Errorf("invalid license: %s", resp.Reason)
}

// License is valid
log.Printf("Expires: %v", resp.ExpiresAt)
log.Printf("Remaining activations: %d", resp.ActivationRemaining)
log.Printf("Features: %v", resp.Features)
```

### Activating a Machine

Activation registers a specific machine with the license server. Use this for per-machine licensing.

```go
resp, err := client.Activate(ctx, cnwlicense.ActivateRequest{
    LicenseKey:  "CNW-XXXX-YYYY-ZZZZ",
    Fingerprint: fingerprint,  // unique machine ID
    Hostname:    "web-server-01",
    IP:          "10.0.1.50",  // optional
    OS:          "linux",      // optional
})
if err != nil {
    switch {
    case errors.Is(err, cnwlicense.ErrActivationLimit):
        log.Fatal("All activation slots are used")
    case errors.Is(err, cnwlicense.ErrLicenseNotFound):
        log.Fatal("Invalid license key")
    case errors.Is(err, cnwlicense.ErrLicenseExpired):
        log.Fatal("License has expired")
    default:
        log.Fatal(err)
    }
}
log.Printf("Activated: %s (ID: %s)", resp.Fingerprint, resp.ID)
```

---

## Offline License Validation

For air-gapped environments without internet access. The license server generates signed JSON files
that can be verified locally using Ed25519 signatures.

### Offline License File Format

The server generates files in this format:

```json
{
  "license": {
    "license_key": "CNW-XXXX-YYYY-ZZZZ",
    "company_id": "...",
    "app_id": "...",
    "plan": "enterprise",
    "features": {"max_nodes": 10, "max_cpu_per_node": 16},
    "expires_at": "2026-12-31T00:00:00Z",
    "issued_at": "2026-01-15T10:00:00Z"
  },
  "signature": "base64-encoded-ed25519-signature",
  "public_key": "base64-encoded-public-key"
}
```

### Verifying from File

```go
// Using the public key embedded in the license file
v := cnwlicense.NewOfflineValidator()
data, err := v.VerifyFile("/etc/myapp/license.json")
if err != nil {
    switch {
    case errors.Is(err, cnwlicense.ErrSignatureInvalid):
        log.Fatal("License file has been tampered with")
    case errors.Is(err, cnwlicense.ErrLicenseExpired):
        log.Fatal("Offline license has expired")
    case errors.Is(err, cnwlicense.ErrLicenseFileInvalid):
        log.Fatal("Corrupt or malformed license file")
    default:
        log.Fatal(err)
    }
}
log.Printf("License: %s, Plan: %s", data.LicenseKey, data.Plan)
```

### Using a Trusted Public Key (Recommended for Production)

Embedding the server's public key in your binary prevents key substitution attacks:

```go
// Embed the public key at build time
const trustedPubKey = "base64-encoded-server-public-key"

v := cnwlicense.NewOfflineValidator(
    cnwlicense.WithTrustedPublicKey(trustedPubKey),
)
data, err := v.VerifyFile("/etc/myapp/license.json")
```

You can get the server's public key from the admin API (`GET /v1/licenses/{id}/offline-file`).

### Verifying from Bytes

Useful when the license is embedded in config or fetched from a non-file source:

```go
licenseJSON := []byte(`{"license": {...}, "signature": "...", "public_key": "..."}`)

v := cnwlicense.NewOfflineValidator(cnwlicense.WithTrustedPublicKey(pubKey))
data, err := v.Verify(licenseJSON)
```

---

## Hardware Enforcement

Check that the current machine meets the license's hardware constraints.

### Extracting Limits from Features

```go
// Features come from ValidateResponse or OfflineLicenseData
features := resp.Features  // map[string]interface{}

limits := cnwlicense.ExtractHardwareLimits(features)
// limits.MaxCPUPerNode = 8   (0 = unlimited)
// limits.MaxNodes      = 3   (0 = unlimited)
```

Recognized feature keys:
- `max_cpu_per_node` - maximum CPU cores per machine
- `max_nodes` - maximum number of nodes in a cluster

### Checking CPU Limits

```go
if err := cnwlicense.CheckCPU(limits); err != nil {
    // errors.Is(err, cnwlicense.ErrCPULimitExceeded) == true
    log.Fatalf("This machine exceeds the CPU limit: %v", err)
    // Example: "CPU limit exceeded: machine has 16 CPUs, limit is 8"
}
```

### Checking Node Count

```go
currentNodes := 5 // get from your node registry or cluster info
if err := cnwlicense.CheckNodeCount(limits, currentNodes); err != nil {
    // errors.Is(err, cnwlicense.ErrNodeLimitExceeded) == true
    log.Fatalf("Cluster exceeds node limit: %v", err)
}
```

---

## Machine Fingerprinting

Generate a deterministic, reboot-safe machine identifier:

```go
fingerprint, err := cnwlicense.GenerateFingerprint()
// Returns a 64-character hex string (SHA-256)
// Example: "a1b2c3d4e5f6..."
```

The fingerprint is derived from:
- Hostname
- MAC addresses (sorted, non-loopback)
- OS and CPU architecture
- `/etc/machine-id` (Linux only, best-effort)

The same machine always produces the same fingerprint.

---

## Manager (Full Orchestration)

The `Manager` combines online validation and hardware checks into a single call.

### Basic Setup

```go
client := cnwlicense.NewOnlineClient(serverURL, apiKey)

mgr := cnwlicense.NewManager(
    cnwlicense.WithOnlineClient(client),
)

info, err := mgr.ValidateAndEnforce(ctx, "CNW-XXXX-YYYY-ZZZZ")
if err != nil {
    log.Fatal(err)
}
log.Printf("License valid: %v, Plan: %s, Fingerprint: %s", info.Valid, info.Plan, info.Fingerprint)
```

### With Client-Level Fingerprint

```go
// Read fingerprint from DB, or generate and persist it
client := cnwlicense.NewOnlineClient(serverURL, apiKey,
    cnwlicense.WithFingerprint(savedFingerprint),
)
mgr := cnwlicense.NewManager(
    cnwlicense.WithOnlineClient(client),
)

// ValidateAndEnforce does ALL of these automatically:
// 1. Resolve fingerprint (from client or auto-generate)
// 2. Validate license via API
// 3. Extract hardware limits from features
// 4. Check CPU count on this machine
info, err := mgr.ValidateAndEnforce(ctx, "CNW-XXXX-YYYY-ZZZZ")
if err != nil {
    log.Fatal(err)
}

// Node count is managed by the application, not the SDK
limits := cnwlicense.ExtractHardwareLimits(info.Features)
myNodeCount := getActiveNodeCountFromDB(info.LicenseKey)
if err := cnwlicense.CheckNodeCount(limits, myNodeCount); err != nil {
    log.Fatal(err)
}

// Activate this machine
activation, err := mgr.ActivateNode(ctx, "CNW-XXXX-YYYY-ZZZZ")
```

---

## Error Handling

### Sentinel Errors

All sentinel errors can be checked with `errors.Is()`:

```go
err := doSomething()

switch {
case errors.Is(err, cnwlicense.ErrLicenseNotFound):
    // License key doesn't exist on the server
case errors.Is(err, cnwlicense.ErrLicenseInactive):
    // License is suspended or revoked
case errors.Is(err, cnwlicense.ErrLicenseExpired):
    // License has passed its expiration date
case errors.Is(err, cnwlicense.ErrActivationLimit):
    // All activation slots are taken
case errors.Is(err, cnwlicense.ErrSignatureInvalid):
    // Offline license signature doesn't match (tampered)
case errors.Is(err, cnwlicense.ErrPublicKeyInvalid):
    // Ed25519 public key is malformed
case errors.Is(err, cnwlicense.ErrLicenseFileInvalid):
    // License file JSON is malformed
case errors.Is(err, cnwlicense.ErrCPULimitExceeded):
    // Machine has more CPUs than the license allows
case errors.Is(err, cnwlicense.ErrNodeLimitExceeded):
    // Cluster has more nodes than the license allows
}
```

### Server Errors

Non-mapped server errors are returned as `*cnwlicense.ServerError`:

```go
var serverErr *cnwlicense.ServerError
if errors.As(err, &serverErr) {
    log.Printf("Server returned %d: [%s] %s",
        serverErr.StatusCode, serverErr.Code, serverErr.Message)
}
```

### Error Mapping

The SDK automatically maps known server error codes to sentinel errors:

| Server Code | HTTP Status | SDK Error |
|-------------|-------------|-----------|
| `NOT_FOUND` | 404 | `ErrLicenseNotFound` |
| `FORBIDDEN` (message: "license expired") | 403 | `ErrLicenseExpired` |
| `FORBIDDEN` (other) | 403 | `ErrLicenseInactive` |
| `ACTIVATION_LIMIT` | 409 | `ErrActivationLimit` |
| Others | varies | `*ServerError` |

---

## Integration Patterns

### Pattern 1: Startup Gate

Block application startup until a valid license is confirmed:

```go
func main() {
    client := cnwlicense.NewOnlineClient(os.Getenv("LICENSE_SERVER"), os.Getenv("API_KEY"))

    resp, err := client.Validate(context.Background(), cnwlicense.ValidateRequest{
        LicenseKey: os.Getenv("LICENSE_KEY"),
    })
    if err != nil {
        log.Fatalf("License check failed: %v", err)
    }
    if !resp.Valid {
        log.Fatalf("Invalid license: %s", resp.Reason)
    }

    // Application starts here
    startServer()
}
```

### Pattern 2: Periodic Background Check

Validate periodically so the app can react to license revocations:

```go
func startLicenseChecker(ctx context.Context, client *cnwlicense.OnlineClient, licenseKey string) {
    ticker := time.NewTicker(1 * time.Hour)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            resp, err := client.Validate(ctx, cnwlicense.ValidateRequest{
                LicenseKey: licenseKey,
            })
            if err != nil {
                log.Printf("License check error: %v", err)
                continue // don't kill the app on transient errors
            }
            if !resp.Valid {
                log.Fatalf("License revoked: %s", resp.Reason)
            }
        }
    }
}
```

### Pattern 3: Kubernetes Operator with License Enforcement

For a Kubernetes operator that must enforce licensing:

```go
func main() {
    ctx := context.Background()

    // Use a stable fingerprint for the pod (e.g., from a ConfigMap or PV)
    client := cnwlicense.NewOnlineClient(
        os.Getenv("LICENSE_SERVER"),
        os.Getenv("API_KEY"),
        cnwlicense.WithFingerprint(os.Getenv("NODE_FINGERPRINT")),
        cnwlicense.WithTimeout(5*time.Second),
    )
    mgr := cnwlicense.NewManager(cnwlicense.WithOnlineClient(client))

    // Validate + enforce hardware limits
    info, err := mgr.ValidateAndEnforce(ctx, os.Getenv("LICENSE_KEY"))
    if err != nil {
        log.Fatalf("License enforcement failed: %v", err)
    }
    log.Printf("License valid, plan: %s", info.Plan)

    // Node count is managed externally (e.g., via your own DB)
    limits := cnwlicense.ExtractHardwareLimits(info.Features)
    nodeCount := countActiveNodesInDB(info.LicenseKey)
    if err := cnwlicense.CheckNodeCount(limits, nodeCount); err != nil {
        log.Fatalf("Node limit exceeded: %v", err)
    }

    // Start periodic re-validation
    go func() {
        ticker := time.NewTicker(30 * time.Second)
        defer ticker.Stop()
        for range ticker.C {
            mgr.ValidateAndEnforce(ctx, os.Getenv("LICENSE_KEY"))
        }
    }()

    // Graceful shutdown
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
    <-sigCh
}
```

### Pattern 4: Air-gapped On-Premise Deployment

For environments with no internet access. The customer receives a signed license file from the admin panel.

```go
import _ "embed"

//go:embed server_public_key.txt
var serverPubKey string

func checkOfflineLicense() (*cnwlicense.OfflineLicenseData, error) {
    v := cnwlicense.NewOfflineValidator(
        cnwlicense.WithTrustedPublicKey(serverPubKey),
    )

    data, err := v.VerifyFile("/etc/myapp/license.json")
    if err != nil {
        return nil, err
    }

    // Optionally enforce hardware limits
    limits := cnwlicense.ExtractHardwareLimits(data.Features)
    if err := cnwlicense.CheckCPU(limits); err != nil {
        return nil, err
    }

    return data, nil
}
```

### Pattern 5: Feature-Gated Functionality

Use features map to gate specific functionality:

```go
resp, _ := client.Validate(ctx, cnwlicense.ValidateRequest{LicenseKey: key})

// Check for specific features
if enabled, ok := resp.Features["advanced_analytics"].(bool); ok && enabled {
    enableAnalyticsDashboard()
}

// Check for plan-based features
if maxUsers, ok := resp.Features["max_users"].(float64); ok {
    setUserLimit(int(maxUsers))
}
```

---

## API Reference

### Package `cnwlicense`

```
import "github.com/CloudNativeWorks/cnw-license-sdk/cnwlicense"
```

#### Client

| Function / Method | Description |
|---|---|
| `NewOnlineClient(serverURL, apiKey, ...ClientOption)` | Create HTTP client |
| `client.Validate(ctx, ValidateRequest)` | Check license validity |
| `client.Activate(ctx, ActivateRequest)` | Register machine activation |
| `client.Fingerprint()` | Get the client-level fingerprint |

#### Client Options

| Option | Description |
|---|---|
| `WithHTTPClient(*http.Client)` | Custom HTTP client |
| `WithTimeout(time.Duration)` | Request timeout (default: 10s) |
| `WithUserAgent(string)` | User-Agent header |
| `WithFingerprint(string)` | Client-level fingerprint (auto-used in requests) |

#### Offline Validator

| Function / Method | Description |
|---|---|
| `NewOfflineValidator(...OfflineOption)` | Create offline validator |
| `validator.VerifyFile(path)` | Verify license from file |
| `validator.Verify([]byte)` | Verify license from bytes |
| `WithTrustedPublicKey(base64)` | Pin server's public key |

#### Hardware

| Function | Description |
|---|---|
| `ExtractHardwareLimits(features)` | Parse limits from features map |
| `CheckCPU(limits)` | Verify CPU count |
| `CheckNodeCount(limits, count)` | Verify node count |
| `GenerateFingerprint()` | Generate machine ID (SHA-256) |

#### Manager

| Function / Method | Description |
|---|---|
| `NewManager(...ManagerOption)` | Create orchestrator |
| `mgr.ValidateAndEnforce(ctx, key)` | Full validation + enforcement pipeline |
| `mgr.ActivateNode(ctx, key)` | Activate machine |

#### Manager Options

| Option | Description |
|---|---|
| `WithOnlineClient(client)` | Set online client |
| `WithOfflineValidator(v)` | Set offline validator |

