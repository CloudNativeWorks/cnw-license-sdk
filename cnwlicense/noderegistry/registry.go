// Package noderegistry provides interfaces and implementations for tracking
// license nodes across distributed systems.
package noderegistry

import (
	"context"
	"time"
)

// NodeInfo represents a registered node in the license system.
type NodeInfo struct {
	Fingerprint  string    `json:"fingerprint" bson:"fingerprint"`
	Hostname     string    `json:"hostname" bson:"hostname"`
	IP           string    `json:"ip" bson:"ip"`
	OS           string    `json:"os" bson:"os"`
	LicenseKey   string    `json:"license_key" bson:"license_key"`
	RegisteredAt time.Time `json:"registered_at" bson:"registered_at"`
	LastSeenAt   time.Time `json:"last_seen_at" bson:"last_seen_at"`
}

// NodeRegistry manages node registrations for distributed license enforcement.
type NodeRegistry interface {
	// Register creates or updates a node registration (upsert by fingerprint).
	Register(ctx context.Context, node NodeInfo) (*NodeInfo, error)

	// Deregister removes a node registration (for graceful shutdown).
	Deregister(ctx context.Context, fingerprint string) error

	// Count returns the number of active nodes for a license key.
	Count(ctx context.Context, licenseKey string) (int, error)

	// List returns all registered nodes for a license key.
	List(ctx context.Context, licenseKey string) ([]NodeInfo, error)

	// Ping updates the last_seen_at timestamp for a node.
	Ping(ctx context.Context, fingerprint string) error

	// Prune removes stale nodes that haven't been seen since olderThan.
	// Returns the number of nodes removed.
	Prune(ctx context.Context, licenseKey string, olderThan time.Duration) (int, error)

	// Close releases any resources held by the registry.
	Close(ctx context.Context) error
}
