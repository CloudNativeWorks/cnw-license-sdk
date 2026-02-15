package cnwlicense

import (
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
func NewOfflineValidator(opts ...OfflineOption) *OfflineValidator {
	v := &OfflineValidator{}
	for _, opt := range opts {
		opt(v)
	}
	return v
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

	// Determine which public key to use
	pubKeyBase64 := file.PublicKey
	if v.trustedPublicKey != "" {
		pubKeyBase64 = v.trustedPublicKey
	}
	if pubKeyBase64 == "" {
		return nil, ErrPublicKeyInvalid
	}

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

	// Verify the signature over the raw license JSON bytes.
	// The server signs json.Marshal(OfflineLicenseData), so we verify
	// against the raw JSON bytes of the "license" field.
	pubKey := ed25519.PublicKey(pubKeyBytes)
	if !ed25519.Verify(pubKey, file.License, sigBytes) {
		return nil, ErrSignatureInvalid
	}

	// Parse the license data
	var data OfflineLicenseData
	if err := json.Unmarshal(file.License, &data); err != nil {
		return nil, fmt.Errorf("%w: parse license data: %v", ErrLicenseFileInvalid, err)
	}

	// Check expiration — return data alongside the error so callers can
	// still access plan, features, license_key etc. for expired licenses.
	if !data.ExpiresAt.IsZero() && data.ExpiresAt.Before(time.Now()) {
		return &data, ErrLicenseExpired
	}

	return &data, nil
}
