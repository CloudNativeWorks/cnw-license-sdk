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

	v, err := NewOfflineValidator(base64.StdEncoding.EncodeToString(pub))
	if err != nil {
		t.Fatalf("unexpected error creating validator: %v", err)
	}
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

	v, _ := NewOfflineValidator(base64.StdEncoding.EncodeToString(pub))
	result, err := v.Verify(fileJSON)
	if !errors.Is(err, ErrLicenseExpired) {
		t.Errorf("expected ErrLicenseExpired, got %v", err)
	}
	// Data should still be returned for expired licenses
	if result == nil {
		t.Fatal("expected non-nil data for expired license")
	}
	if result.LicenseKey != "CNW-EXPIRED" {
		t.Errorf("expected license key CNW-EXPIRED, got %s", result.LicenseKey)
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

	v, _ := NewOfflineValidator(base64.StdEncoding.EncodeToString(pub))
	_, err := v.Verify(fileJSON)
	if !errors.Is(err, ErrSignatureInvalid) {
		t.Errorf("expected ErrSignatureInvalid, got %v", err)
	}
}

func TestOfflineValidator_Verify_KeySubstitution(t *testing.T) {
	// Attacker generates their own key pair
	attackerPub, attackerPriv, _ := ed25519.GenerateKey(rand.Reader)
	// Server's real key pair
	serverPub, _, _ := ed25519.GenerateKey(rand.Reader)

	data := OfflineLicenseData{
		LicenseKey: "CNW-FAKE",
		Plan:       "enterprise",
		Features:   map[string]interface{}{"max_nodes": float64(9999)},
		ExpiresAt:  time.Now().Add(24 * time.Hour),
		IssuedAt:   time.Now(),
	}

	// Attacker signs with their own key
	rawLicense, sig := signLicenseData(attackerPriv, data)

	// Attacker embeds their own public key in the file
	file := OfflineLicenseFile{
		License:   rawLicense,
		Signature: base64.StdEncoding.EncodeToString(sig),
		PublicKey: base64.StdEncoding.EncodeToString(attackerPub),
	}
	fileJSON, _ := json.Marshal(file)

	// SDK uses the server's trusted key → attacker's signature must fail
	v, _ := NewOfflineValidator(base64.StdEncoding.EncodeToString(serverPub))
	_, err := v.Verify(fileJSON)
	if !errors.Is(err, ErrSignatureInvalid) {
		t.Errorf("expected ErrSignatureInvalid for key substitution attack, got %v", err)
	}
}

func TestOfflineValidator_Verify_InvalidJSON(t *testing.T) {
	v, _ := NewOfflineValidator("dGVzdGtleXRlc3RrZXl0ZXN0a2V5dGVzdGtleXk=") // 32 bytes base64
	_, err := v.Verify([]byte("not json"))
	if !errors.Is(err, ErrLicenseFileInvalid) {
		t.Errorf("expected ErrLicenseFileInvalid, got %v", err)
	}
}

func TestOfflineValidator_Verify_MissingFields(t *testing.T) {
	v, _ := NewOfflineValidator("dGVzdGtleXRlc3RrZXl0ZXN0a2V5dGVzdGtleXk=")
	_, err := v.Verify([]byte(`{"license": {}, "signature": ""}`))
	if !errors.Is(err, ErrLicenseFileInvalid) {
		t.Errorf("expected ErrLicenseFileInvalid, got %v", err)
	}
}

func TestOfflineValidator_RequiresTrustedKey(t *testing.T) {
	_, err := NewOfflineValidator("")
	if !errors.Is(err, ErrPublicKeyInvalid) {
		t.Errorf("expected ErrPublicKeyInvalid for empty key, got %v", err)
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

	v, _ := NewOfflineValidator(base64.StdEncoding.EncodeToString(pub))
	result, err := v.VerifyFile(filePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.LicenseKey != "CNW-FILE-TEST" {
		t.Errorf("expected license key CNW-FILE-TEST, got %s", result.LicenseKey)
	}
}

func TestOfflineValidator_Verify_PrettyPrintedFile(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)

	data := OfflineLicenseData{
		LicenseKey: "CNW-PRETTY",
		CompanyID:  "comp-001",
		Plan:       "enterprise",
		Features:   map[string]interface{}{"max_nodes": float64(5), "max_cpu_per_node": float64(8)},
		ExpiresAt:  time.Now().Add(24 * time.Hour),
		IssuedAt:   time.Now(),
	}

	// Server signs compact JSON
	rawLicense, sig := signLicenseData(priv, data)

	// Simulate user pretty-printing the file (e.g. jq .)
	var prettyLicense json.RawMessage
	json.MarshalIndent(json.RawMessage(rawLicense), "    ", "    ")
	// MarshalIndent on RawMessage just re-formats it
	indented, _ := json.MarshalIndent(json.RawMessage(rawLicense), "", "    ")
	prettyLicense = json.RawMessage(indented)

	file := OfflineLicenseFile{
		License:   prettyLicense,
		Signature: base64.StdEncoding.EncodeToString(sig),
		PublicKey: base64.StdEncoding.EncodeToString(pub),
	}
	// Pretty-print the whole file too
	fileJSON, _ := json.MarshalIndent(file, "", "  ")

	v, _ := NewOfflineValidator(base64.StdEncoding.EncodeToString(pub))
	result, err := v.Verify(fileJSON)
	if err != nil {
		t.Fatalf("pretty-printed file should verify successfully, got: %v", err)
	}
	if result.LicenseKey != "CNW-PRETTY" {
		t.Errorf("expected license key CNW-PRETTY, got %s", result.LicenseKey)
	}
}

func TestOfflineValidator_VerifyFile_NotFound(t *testing.T) {
	v, _ := NewOfflineValidator("dGVzdGtleXRlc3RrZXl0ZXN0a2V5dGVzdGtleXk=")
	_, err := v.VerifyFile("/nonexistent/license.json")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}
