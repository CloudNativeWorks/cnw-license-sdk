package cnwlicense

import (
	"context"
	"fmt"
	"os"
	"runtime"

	"github.com/CloudNativeWorks/cnw-license-sdk/cnwlicense/noderegistry"
)

// Manager is the top-level orchestrator that combines online/offline validation,
// hardware checks, and node registry into a unified API.
type Manager struct {
	client   *OnlineClient
	offline  *OfflineValidator
	registry noderegistry.NodeRegistry
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

// WithNodeRegistry sets the node registry for distributed node tracking.
func WithNodeRegistry(r noderegistry.NodeRegistry) ManagerOption {
	return func(m *Manager) {
		m.registry = r
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

// ValidateAndEnforce performs full license validation with hardware enforcement:
//  1. Generates a machine fingerprint
//  2. Validates the license via the online client
//  3. Extracts hardware limits from features
//  4. Checks CPU limits on this machine
//  5. Registers this node in the registry (if configured)
//  6. Checks node count limits (deregisters on failure)
func (m *Manager) ValidateAndEnforce(ctx context.Context, licenseKey string) (*LicenseInfo, error) {
	if m.client == nil {
		return nil, fmt.Errorf("online client is required for ValidateAndEnforce")
	}

	// 1. Generate fingerprint
	fingerprint, err := GenerateFingerprint()
	if err != nil {
		return nil, fmt.Errorf("generate fingerprint: %w", err)
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

	info := &LicenseInfo{
		Valid:       true,
		LicenseKey:  licenseKey,
		Features:    resp.Features,
		ExpiresAt:   resp.ExpiresAt,
		Fingerprint: fingerprint,
	}

	// 5 & 6. Node registry operations (if configured)
	if m.registry != nil {
		hostname, _ := os.Hostname()
		node := noderegistry.NodeInfo{
			Fingerprint: fingerprint,
			Hostname:    hostname,
			OS:          runtime.GOOS,
			LicenseKey:  licenseKey,
		}
		if _, err := m.registry.Register(ctx, node); err != nil {
			return nil, fmt.Errorf("register node: %w", err)
		}

		count, err := m.registry.Count(ctx, licenseKey)
		if err != nil {
			return nil, fmt.Errorf("count nodes: %w", err)
		}
		info.NodeCount = count

		if err := CheckNodeCount(limits, count); err != nil {
			// Deregister this node since we exceeded the limit
			_ = m.registry.Deregister(ctx, fingerprint)
			return nil, err
		}
	}

	return info, nil
}

// ActivateNode activates this machine with the license server and registers it
// in the node registry (if configured).
func (m *Manager) ActivateNode(ctx context.Context, licenseKey string) (*ActivateResponse, error) {
	if m.client == nil {
		return nil, fmt.Errorf("online client is required for ActivateNode")
	}

	fingerprint, err := GenerateFingerprint()
	if err != nil {
		return nil, fmt.Errorf("generate fingerprint: %w", err)
	}

	hostname, _ := os.Hostname()
	activation, err := m.client.Activate(ctx, ActivateRequest{
		LicenseKey:  licenseKey,
		Fingerprint: fingerprint,
		Hostname:    hostname,
		OS:          runtime.GOOS,
	})
	if err != nil {
		return nil, err
	}

	if m.registry != nil {
		node := noderegistry.NodeInfo{
			Fingerprint: fingerprint,
			Hostname:    hostname,
			OS:          runtime.GOOS,
			LicenseKey:  licenseKey,
		}
		if _, err := m.registry.Register(ctx, node); err != nil {
			return nil, fmt.Errorf("register node: %w", err)
		}
	}

	return activation, nil
}

// Shutdown deregisters this node from the registry for graceful shutdown.
func (m *Manager) Shutdown(ctx context.Context) error {
	if m.registry == nil {
		return nil
	}
	fingerprint, err := GenerateFingerprint()
	if err != nil {
		return fmt.Errorf("generate fingerprint: %w", err)
	}
	return m.registry.Deregister(ctx, fingerprint)
}
