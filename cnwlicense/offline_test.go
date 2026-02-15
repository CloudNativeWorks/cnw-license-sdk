package cnwlicense

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// signLicenseData mimics the server's crypto.SignJSON behavior:
// marshal to JSON, then sign the raw bytes.
func signLicenseData(priv ed25519.PrivateKey, data OfflineLicenseData) (json.RawMessage, []byte) {
	raw, _ := json.Marshal(data)
	sig := ed25519.Sign(priv, raw)
	return json.RawMessage(raw), sig
}

func TestOfflineValidator_Verify_Success(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)

	data := OfflineLicenseData{
		LicenseKey: "CNW-TEST-1234",
		CompanyID:  "comp-001",
		AppID:      "app-001",
		Plan:       "enterprise",
		Features:   map[string]interface{}{"max_nodes": float64(10)},
		ExpiresAt:  time.Now().Add(24 * time.Hour),
		IssuedAt:   time.Now(),
	}

	rawLicense, sig := signLicenseData(priv, data)

	file := OfflineLicenseFile{
		License:   rawLicense,
		Signature: base64.StdEncoding.EncodeToString(sig),
		PublicKey: base64.StdEncoding.EncodeToString(pub),
	}
	fileJSON, _ := json.Marshal(file)

	v := NewOfflineValidator()
	result, err := v.Verify(fileJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.LicenseKey != "CNW-TEST-1234" {
		t.Errorf("expected license key CNW-TEST-1234, got %s", result.LicenseKey)
	}
	if result.Plan != "enterprise" {
		t.Errorf("expected plan enterprise, got %s", result.Plan)
	}
}

func TestOfflineValidator_Verify_WithTrustedKey(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)

	data := OfflineLicenseData{
		LicenseKey: "CNW-TRUSTED",
		ExpiresAt:  time.Now().Add(24 * time.Hour),
		IssuedAt:   time.Now(),
	}

	rawLicense, sig := signLicenseData(priv, data)

	file := OfflineLicenseFile{
		License:   rawLicense,
		Signature: base64.StdEncoding.EncodeToString(sig),
		PublicKey: "", // no embedded key
	}
	fileJSON, _ := json.Marshal(file)

	v := NewOfflineValidator(WithTrustedPublicKey(base64.StdEncoding.EncodeToString(pub)))
	result, err := v.Verify(fileJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.LicenseKey != "CNW-TRUSTED" {
		t.Errorf("expected license key CNW-TRUSTED, got %s", result.LicenseKey)
	}
}

func TestOfflineValidator_Verify_TrustedKeyOverridesEmbedded(t *testing.T) {
	trustedPub, trustedPriv, _ := ed25519.GenerateKey(rand.Reader)
	otherPub, _, _ := ed25519.GenerateKey(rand.Reader)

	data := OfflineLicenseData{
		LicenseKey: "CNW-OVERRIDE",
		ExpiresAt:  time.Now().Add(24 * time.Hour),
		IssuedAt:   time.Now(),
	}

	rawLicense, sig := signLicenseData(trustedPriv, data)

	file := OfflineLicenseFile{
		License:   rawLicense,
		Signature: base64.StdEncoding.EncodeToString(sig),
		PublicKey: base64.StdEncoding.EncodeToString(otherPub), // wrong embedded key
	}
	fileJSON, _ := json.Marshal(file)

	// Trusted key should override the embedded one
	v := NewOfflineValidator(WithTrustedPublicKey(base64.StdEncoding.EncodeToString(trustedPub)))
	result, err := v.Verify(fileJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.LicenseKey != "CNW-OVERRIDE" {
		t.Errorf("expected license key CNW-OVERRIDE, got %s", result.LicenseKey)
	}
}

func TestOfflineValidator_Verify_Expired(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)

	data := OfflineLicenseData{
		LicenseKey: "CNW-EXPIRED",
		ExpiresAt:  time.Now().Add(-24 * time.Hour), // expired yesterday
		IssuedAt:   time.Now().Add(-48 * time.Hour),
	}

	rawLicense, sig := signLicenseData(priv, data)

	file := OfflineLicenseFile{
		License:   rawLicense,
		Signature: base64.StdEncoding.EncodeToString(sig),
		PublicKey: base64.StdEncoding.EncodeToString(pub),
	}
	fileJSON, _ := json.Marshal(file)

	v := NewOfflineValidator()
	_, err := v.Verify(fileJSON)
	if !errors.Is(err, ErrLicenseExpired) {
		t.Errorf("expected ErrLicenseExpired, got %v", err)
	}
}

func TestOfflineValidator_Verify_TamperedData(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)

	data := OfflineLicenseData{
		LicenseKey: "CNW-ORIGINAL",
		ExpiresAt:  time.Now().Add(24 * time.Hour),
		IssuedAt:   time.Now(),
	}

	_, sig := signLicenseData(priv, data)

	// Tamper: change the license key
	data.LicenseKey = "CNW-TAMPERED"
	tamperedRaw, _ := json.Marshal(data)

	file := OfflineLicenseFile{
		License:   json.RawMessage(tamperedRaw),
		Signature: base64.StdEncoding.EncodeToString(sig),
		PublicKey: base64.StdEncoding.EncodeToString(pub),
	}
	fileJSON, _ := json.Marshal(file)

	v := NewOfflineValidator()
	_, err := v.Verify(fileJSON)
	if !errors.Is(err, ErrSignatureInvalid) {
		t.Errorf("expected ErrSignatureInvalid, got %v", err)
	}
}

func TestOfflineValidator_Verify_WrongKey(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	otherPub, _, _ := ed25519.GenerateKey(rand.Reader)

	data := OfflineLicenseData{
		LicenseKey: "CNW-WRONGKEY",
		ExpiresAt:  time.Now().Add(24 * time.Hour),
		IssuedAt:   time.Now(),
	}

	rawLicense, sig := signLicenseData(priv, data)

	file := OfflineLicenseFile{
		License:   rawLicense,
		Signature: base64.StdEncoding.EncodeToString(sig),
		PublicKey: base64.StdEncoding.EncodeToString(otherPub), // wrong key
	}
	fileJSON, _ := json.Marshal(file)

	v := NewOfflineValidator()
	_, err := v.Verify(fileJSON)
	if !errors.Is(err, ErrSignatureInvalid) {
		t.Errorf("expected ErrSignatureInvalid, got %v", err)
	}
}

func TestOfflineValidator_Verify_InvalidJSON(t *testing.T) {
	v := NewOfflineValidator()
	_, err := v.Verify([]byte("not json"))
	if !errors.Is(err, ErrLicenseFileInvalid) {
		t.Errorf("expected ErrLicenseFileInvalid, got %v", err)
	}
}

func TestOfflineValidator_Verify_MissingFields(t *testing.T) {
	v := NewOfflineValidator()
	_, err := v.Verify([]byte(`{"license": {}, "signature": ""}`))
	if !errors.Is(err, ErrLicenseFileInvalid) {
		t.Errorf("expected ErrLicenseFileInvalid, got %v", err)
	}
}

func TestOfflineValidator_Verify_NoPublicKey(t *testing.T) {
	v := NewOfflineValidator()
	_, err := v.Verify([]byte(`{"license": {"license_key":"test"}, "signature": "abc"}`))
	if !errors.Is(err, ErrPublicKeyInvalid) {
		t.Errorf("expected ErrPublicKeyInvalid, got %v", err)
	}
}

func TestOfflineValidator_VerifyFile(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)

	data := OfflineLicenseData{
		LicenseKey: "CNW-FILE-TEST",
		ExpiresAt:  time.Now().Add(24 * time.Hour),
		IssuedAt:   time.Now(),
	}

	rawLicense, sig := signLicenseData(priv, data)

	file := OfflineLicenseFile{
		License:   rawLicense,
		Signature: base64.StdEncoding.EncodeToString(sig),
		PublicKey: base64.StdEncoding.EncodeToString(pub),
	}
	fileJSON, _ := json.Marshal(file)

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "license.json")
	os.WriteFile(filePath, fileJSON, 0644)

	v := NewOfflineValidator()
	result, err := v.VerifyFile(filePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.LicenseKey != "CNW-FILE-TEST" {
		t.Errorf("expected license key CNW-FILE-TEST, got %s", result.LicenseKey)
	}
}

func TestOfflineValidator_VerifyFile_NotFound(t *testing.T) {
	v := NewOfflineValidator()
	_, err := v.VerifyFile("/nonexistent/license.json")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}
