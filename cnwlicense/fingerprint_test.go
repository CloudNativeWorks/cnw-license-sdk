package cnwlicense

import (
	"os"
	"testing"
)

func TestGenerateFingerprint_NotEmpty(t *testing.T) {
	fp, err := GenerateFingerprint()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fp == "" {
		t.Fatal("fingerprint should not be empty")
	}
	// SHA-256 hex = 64 chars
	if len(fp) != 64 {
		t.Errorf("expected 64 char hex string, got %d chars: %s", len(fp), fp)
	}
}

func TestGenerateFingerprint_Deterministic(t *testing.T) {
	fp1, err := GenerateFingerprint()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fp2, err := GenerateFingerprint()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fp1 != fp2 {
		t.Errorf("fingerprint should be deterministic: %s != %s", fp1, fp2)
	}
}

func TestGenerateFingerprint_EnvOverride(t *testing.T) {
	const custom = "custom-fingerprint-from-env"
	t.Setenv("CNW_FINGERPRINT", custom)

	fp, err := GenerateFingerprint()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fp != custom {
		t.Errorf("expected %q, got %q", custom, fp)
	}
}

func TestGenerateFingerprint_EnvOverrideEmpty(t *testing.T) {
	// Ensure empty env var does NOT override (falls through to real fingerprint)
	t.Setenv("CNW_FINGERPRINT", "")
	// Also clear it to make sure unset works
	os.Unsetenv("CNW_FINGERPRINT")

	fp, err := GenerateFingerprint()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fp) != 64 {
		t.Errorf("expected 64 char hex string without env override, got %d chars", len(fp))
	}
}
