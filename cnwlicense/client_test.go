package cnwlicense

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestOnlineClient_Validate_Success(t *testing.T) {
	expires := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/validate" {
			t.Errorf("expected /v1/validate, got %s", r.URL.Path)
		}
		if r.Header.Get("X-API-Key") != "test-key" {
			t.Errorf("expected X-API-Key: test-key, got %s", r.Header.Get("X-API-Key"))
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type: application/json, got %s", r.Header.Get("Content-Type"))
		}

		var req ValidateRequest
		json.NewDecoder(r.Body).Decode(&req)
		if req.LicenseKey != "CNW-TEST-1234" {
			t.Errorf("expected license key CNW-TEST-1234, got %s", req.LicenseKey)
		}

		// Server returns validate response directly (not wrapped)
		resp := ValidateResponse{
			Valid:               true,
			ExpiresAt:           &expires,
			Features:            map[string]interface{}{"max_nodes": float64(5)},
			ActivationRemaining: 3,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewOnlineClient(server.URL, "test-key")
	resp, err := client.Validate(context.Background(), ValidateRequest{
		LicenseKey: "CNW-TEST-1234",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Valid {
		t.Error("expected valid=true")
	}
	if resp.ActivationRemaining != 3 {
		t.Errorf("expected 3 remaining, got %d", resp.ActivationRemaining)
	}
	if resp.ExpiresAt == nil || !resp.ExpiresAt.Equal(expires) {
		t.Errorf("expected expires_at %v, got %v", expires, resp.ExpiresAt)
	}
}

func TestOnlineClient_Validate_Invalid(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := ValidateResponse{
			Valid:  false,
			Reason: "license not found",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewOnlineClient(server.URL, "test-key")
	resp, err := client.Validate(context.Background(), ValidateRequest{
		LicenseKey: "INVALID",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Valid {
		t.Error("expected valid=false")
	}
	if resp.Reason != "license not found" {
		t.Errorf("expected reason 'license not found', got %q", resp.Reason)
	}
}

func TestOnlineClient_Activate_Success(t *testing.T) {
	activatedAt := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/activate" {
			t.Errorf("expected /v1/activate, got %s", r.URL.Path)
		}

		var req ActivateRequest
		json.NewDecoder(r.Body).Decode(&req)
		if req.Fingerprint != "abc123" {
			t.Errorf("expected fingerprint abc123, got %s", req.Fingerprint)
		}

		// Server wraps activate response in {data: ...}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": ActivateResponse{
				ID:          "act-001",
				Fingerprint: req.Fingerprint,
				Hostname:    req.Hostname,
				ActivatedAt: activatedAt,
				LastSeenAt:  activatedAt,
			},
		})
	}))
	defer server.Close()

	client := NewOnlineClient(server.URL, "test-key")
	resp, err := client.Activate(context.Background(), ActivateRequest{
		LicenseKey:  "CNW-TEST-1234",
		Fingerprint: "abc123",
		Hostname:    "node-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ID != "act-001" {
		t.Errorf("expected ID act-001, got %s", resp.ID)
	}
	if resp.Fingerprint != "abc123" {
		t.Errorf("expected fingerprint abc123, got %s", resp.Fingerprint)
	}
}

func TestOnlineClient_Activate_LimitReached(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]string{
				"code":    "ACTIVATION_LIMIT",
				"message": "activation limit reached",
			},
		})
	}))
	defer server.Close()

	client := NewOnlineClient(server.URL, "test-key")
	_, err := client.Activate(context.Background(), ActivateRequest{
		LicenseKey:  "CNW-TEST-1234",
		Fingerprint: "abc123",
		Hostname:    "node-1",
	})
	if !errors.Is(err, ErrActivationLimit) {
		t.Errorf("expected ErrActivationLimit, got %v", err)
	}

	// Mapped errors should also expose ServerError details via errors.As
	var se *ServerError
	if !errors.As(err, &se) {
		t.Fatal("expected errors.As to return ServerError for mapped error")
	}
	if se.StatusCode != http.StatusConflict {
		t.Errorf("expected status %d, got %d", http.StatusConflict, se.StatusCode)
	}
	if se.Code != "ACTIVATION_LIMIT" {
		t.Errorf("expected code ACTIVATION_LIMIT, got %s", se.Code)
	}
}

func TestOnlineClient_Activate_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]string{
				"code":    "NOT_FOUND",
				"message": "license not found",
			},
		})
	}))
	defer server.Close()

	client := NewOnlineClient(server.URL, "test-key")
	_, err := client.Activate(context.Background(), ActivateRequest{
		LicenseKey:  "INVALID",
		Fingerprint: "abc123",
		Hostname:    "node-1",
	})
	if !errors.Is(err, ErrLicenseNotFound) {
		t.Errorf("expected ErrLicenseNotFound, got %v", err)
	}
}

func TestOnlineClient_Activate_Expired(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]string{
				"code":    "FORBIDDEN",
				"message": "license expired",
			},
		})
	}))
	defer server.Close()

	client := NewOnlineClient(server.URL, "test-key")
	_, err := client.Activate(context.Background(), ActivateRequest{
		LicenseKey:  "EXPIRED",
		Fingerprint: "abc123",
		Hostname:    "node-1",
	})
	if !errors.Is(err, ErrLicenseExpired) {
		t.Errorf("expected ErrLicenseExpired, got %v", err)
	}
}

func TestOnlineClient_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewOnlineClient(server.URL, "test-key", WithTimeout(50*time.Millisecond))
	_, err := client.Validate(context.Background(), ValidateRequest{
		LicenseKey: "CNW-TEST-1234",
	})
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestOnlineClient_CustomUserAgent(t *testing.T) {
	var receivedUA string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedUA = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ValidateResponse{Valid: true})
	}))
	defer server.Close()

	client := NewOnlineClient(server.URL, "test-key", WithUserAgent("my-app/2.0"))
	client.Validate(context.Background(), ValidateRequest{LicenseKey: "test"})

	if receivedUA != "my-app/2.0" {
		t.Errorf("expected User-Agent 'my-app/2.0', got %q", receivedUA)
	}
}

func TestOnlineClient_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]string{
				"code":    "INTERNAL_ERROR",
				"message": "internal server error",
			},
		})
	}))
	defer server.Close()

	client := NewOnlineClient(server.URL, "test-key")
	_, err := client.Validate(context.Background(), ValidateRequest{
		LicenseKey: "test",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	var se *ServerError
	if !errors.As(err, &se) {
		t.Fatalf("expected ServerError, got %T: %v", err, err)
	}
	if se.StatusCode != 500 {
		t.Errorf("expected status 500, got %d", se.StatusCode)
	}
	if se.Code != "INTERNAL_ERROR" {
		t.Errorf("expected code INTERNAL_ERROR, got %s", se.Code)
	}
}
