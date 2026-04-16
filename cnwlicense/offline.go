package cnwlicense

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// OfflineValidator verifies Ed25519-signed offline license files.
// It is compatible with the server's crypto.SignJSON signing format.
type OfflineValidator struct {
	trustedPublicKey string // base64-encoded Ed25519 public key
}

// NewOfflineValidator creates a new offline license validator.
// A trusted public key is required to prevent key substitution attacks.
// Get the key from the Settings page or GET /v1/settings/public-key.
func NewOfflineValidator(trustedPublicKey string, opts ...OfflineOption) (*OfflineValidator, error) {
	if trustedPublicKey == "" {
		return nil, fmt.Errorf("%w: trusted public key is required", ErrPublicKeyInvalid)
	}
	v := &OfflineValidator{trustedPublicKey: trustedPublicKey}
	for _, opt := range opts {
		opt(v)
	}
	return v, nil
}

// VerifyFile reads a license file from disk and verifies its signature.
func (v *OfflineValidator) VerifyFile(filePath string) (*OfflineLicenseData, error) {
	raw, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read license file: %w", err)
	}
	return v.Verify(raw)
}

// Verify verifies a raw JSON license file and returns the license data.
//
// The verification process matches the server's crypto.SignJSON format:
//  1. Parse the outer envelope (license as raw JSON, signature, public_key)
//  2. Decode the public key and signature from base64
//  3. Verify ed25519.Verify(pubKey, rawLicenseBytes, signature)
//  4. Parse and validate the license data (expiration check)
func (v *OfflineValidator) Verify(raw []byte) (*OfflineLicenseData, error) {
	var file OfflineLicenseFile
	if err := json.Unmarshal(raw, &file); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrLicenseFileInvalid, err)
	}

	if len(file.License) == 0 || file.Signature == "" {
		return nil, ErrLicenseFileInvalid
	}

	// Always use the trusted public key — never fall back to the embedded key.
	pubKeyBase64 := v.trustedPublicKey

	pubKeyBytes, err := base64.StdEncoding.DecodeString(pubKeyBase64)
	if err != nil {
		return nil, fmt.Errorf("%w: base64 decode: %v", ErrPublicKeyInvalid, err)
	}
	if len(pubKeyBytes) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("%w: key length %d, expected %d", ErrPublicKeyInvalid, len(pubKeyBytes), ed25519.PublicKeySize)
	}

	sigBytes, err := base64.StdEncoding.DecodeString(file.Signature)
	if err != nil {
		return nil, fmt.Errorf("%w: signature decode: %v", ErrSignatureInvalid, err)
	}

	// Normalize license JSON to compact form before verification.
	// The server signs compact json.Marshal() output, but the file may have
	// been pretty-printed (e.g. jq, editors). json.Compact removes whitespace
	// so the bytes match regardless of formatting.
	var compacted bytes.Buffer
	if err := json.Compact(&compacted, file.License); err != nil {
		return nil, fmt.Errorf("%w: compact license JSON: %v", ErrLicenseFileInvalid, err)
	}
	licenseBytes := compacted.Bytes()

	pubKey := ed25519.PublicKey(pubKeyBytes)
	if !ed25519.Verify(pubKey, licenseBytes, sigBytes) {
		return nil, ErrSignatureInvalid
	}

	// Parse the license data from the verified bytes
	var data OfflineLicenseData
	if err := json.Unmarshal(licenseBytes, &data); err != nil {
		return nil, fmt.Errorf("%w: parse license data: %v", ErrLicenseFileInvalid, err)
	}

	// Check expiration — return data alongside the error so callers can
	// still access plan, features, license_key etc. for expired licenses.
	if !data.ExpiresAt.IsZero() && data.ExpiresAt.Before(time.Now()) {
		return &data, ErrLicenseExpired
	}

	return &data, nil
}
