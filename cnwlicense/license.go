package cnwlicense

import (
	"context"
	"fmt"
)

// Manager is the top-level orchestrator that combines online/offline validation
// and hardware checks into a unified API.
type Manager struct {
	client  *OnlineClient
	offline *OfflineValidator
}

// ManagerOption configures a Manager.
type ManagerOption func(*Manager)

// WithOnlineClient sets the online client for server-based validation.
func WithOnlineClient(c *OnlineClient) ManagerOption {
	return func(m *Manager) {
		m.client = c
	}
}

// WithOfflineValidator sets the offline validator for air-gapped environments.
func WithOfflineValidator(v *OfflineValidator) ManagerOption {
	return func(m *Manager) {
		m.offline = v
	}
}

// NewManager creates a new license Manager.
func NewManager(opts ...ManagerOption) *Manager {
	m := &Manager{}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// resolveFingerprint returns the client-level fingerprint if set,
// otherwise falls back to GenerateFingerprint().
func (m *Manager) resolveFingerprint() (string, error) {
	if m.client != nil {
		if fp := m.client.Fingerprint(); fp != "" {
			return fp, nil
		}
	}
	return GenerateFingerprint()
}

// ValidateAndEnforce performs full license validation with hardware enforcement:
//  1. Resolves a machine fingerprint (client-level or auto-generated)
//  2. Validates the license via the online client
//  3. Extracts hardware limits from features
//  4. Checks CPU limits on this machine
func (m *Manager) ValidateAndEnforce(ctx context.Context, licenseKey string) (*LicenseInfo, error) {
	if m.client == nil {
		return nil, fmt.Errorf("online client is required for ValidateAndEnforce")
	}

	// 1. Resolve fingerprint
	fingerprint, err := m.resolveFingerprint()
	if err != nil {
		return nil, fmt.Errorf("resolve fingerprint: %w", err)
	}

	// 2. Validate license
	resp, err := m.client.Validate(ctx, ValidateRequest{
		LicenseKey:  licenseKey,
		Fingerprint: fingerprint,
	})
	if err != nil {
		return nil, fmt.Errorf("validate license: %w", err)
	}
	if !resp.Valid {
		return &LicenseInfo{
			Valid:       false,
			LicenseKey:  licenseKey,
			Fingerprint: fingerprint,
		}, nil
	}

	// 3. Extract hardware limits
	limits := ExtractHardwareLimits(resp.Features)

	// 4. Check CPU
	if err := CheckCPU(limits); err != nil {
		return nil, err
	}

	return &LicenseInfo{
		Valid:       true,
		LicenseKey:  licenseKey,
		Plan:        resp.Plan,
		Features:    resp.Features,
		ExpiresAt:   resp.ExpiresAt,
		Fingerprint: fingerprint,
	}, nil
}

// ActivateNode activates this machine with the license server.
func (m *Manager) ActivateNode(ctx context.Context, licenseKey string) (*ActivateResponse, error) {
	if m.client == nil {
		return nil, fmt.Errorf("online client is required for ActivateNode")
	}

	fingerprint, err := m.resolveFingerprint()
	if err != nil {
		return nil, fmt.Errorf("resolve fingerprint: %w", err)
	}

	return m.client.Activate(ctx, ActivateRequest{
		LicenseKey:  licenseKey,
		Fingerprint: fingerprint,
	})
}
