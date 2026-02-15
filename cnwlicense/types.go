package cnwlicense

import (
	"encoding/json"
	"time"
)

// ValidateRequest is the request body for the /v1/validate endpoint.
// Fields match api/internal/service/activation_service.go.
type ValidateRequest struct {
	LicenseKey  string `json:"license_key"`
	Fingerprint string `json:"fingerprint,omitempty"`
	Version     string `json:"version,omitempty"`
}

// ValidateResponse is the response from the /v1/validate endpoint.
// The server returns this directly (not wrapped in {data: ...}).
type ValidateResponse struct {
	Valid               bool                   `json:"valid"`
	Reason              string                 `json:"reason,omitempty"`
	Plan                string                 `json:"plan,omitempty"`
	ExpiresAt           *time.Time             `json:"expires_at,omitempty"`
	Features            map[string]interface{} `json:"features,omitempty"`
	ActivationRemaining int                    `json:"activation_remaining"`
}

// ActivateRequest is the request body for the /v1/activate endpoint.
type ActivateRequest struct {
	LicenseKey  string `json:"license_key"`
	Fingerprint string `json:"fingerprint"`
	Hostname    string `json:"hostname"`
	IP          string `json:"ip,omitempty"`
	OS          string `json:"os,omitempty"`
}

// ActivateResponse is the activation record returned by the server.
// The server wraps this in {data: ...} via the Success() helper.
type ActivateResponse struct {
	ID          string    `json:"id"`
	LicenseID   string    `json:"license_id"`
	Fingerprint string    `json:"fingerprint"`
	Hostname    string    `json:"hostname"`
	IP          string    `json:"ip"`
	OS          string    `json:"os,omitempty"`
	ActivatedAt time.Time `json:"activated_at"`
	LastSeenAt  time.Time `json:"last_seen_at"`
}

// OfflineLicenseFile represents the JSON structure of a signed offline license file.
// The License field is kept as json.RawMessage to preserve the exact bytes for
// signature verification (matching server's crypto.SignJSON behavior).
type OfflineLicenseFile struct {
	License   json.RawMessage `json:"license"`
	Signature string          `json:"signature"`
	PublicKey string          `json:"public_key"`
}

// OfflineLicenseData contains the license information embedded in an offline license file.
// Matches api/internal/service/offline_service.go.
type OfflineLicenseData struct {
	LicenseKey string                 `json:"license_key"`
	CompanyID  string                 `json:"company_id"`
	AppID      string                 `json:"app_id"`
	Plan       string                 `json:"plan"`
	Features   map[string]interface{} `json:"features"`
	ExpiresAt  time.Time              `json:"expires_at"`
	IssuedAt   time.Time              `json:"issued_at"`
}

// LicenseInfo is the unified result returned by the Manager after validation and enforcement.
type LicenseInfo struct {
	Valid       bool                   `json:"valid"`
	LicenseKey  string                 `json:"license_key"`
	Plan        string                 `json:"plan,omitempty"`
	Features    map[string]interface{} `json:"features,omitempty"`
	ExpiresAt   *time.Time             `json:"expires_at,omitempty"`
	Fingerprint string                 `json:"fingerprint"`
}

// HardwareLimits holds the hardware constraints extracted from a license's features map.
type HardwareLimits struct {
	MaxCPUPerNode int // 0 = unlimited
	MaxNodes      int // 0 = unlimited
}
