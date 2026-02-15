package noderegistry

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const defaultPostgresTable = "cnw_license_nodes"

// validIdentifier matches safe PostgreSQL identifiers (letters, digits, underscores).
var validIdentifier = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// PostgresOption configures a PostgresRegistry.
type PostgresOption func(*PostgresRegistry)

// WithTableName sets the PostgreSQL table name. Default: "cnw_license_nodes".
func WithTableName(name string) PostgresOption {
	return func(r *PostgresRegistry) {
		r.tableName = name
	}
}

// PostgresRegistry implements NodeRegistry using PostgreSQL.
type PostgresRegistry struct {
	pool      *pgxpool.Pool
	tableName string
}

// NewPostgresRegistry creates a new PostgreSQL-backed node registry.
// It auto-creates the table and indexes on initialization.
func NewPostgresRegistry(ctx context.Context, pool *pgxpool.Pool, opts ...PostgresOption) (*PostgresRegistry, error) {
	r := &PostgresRegistry{
		pool:      pool,
		tableName: defaultPostgresTable,
	}
	for _, opt := range opts {
		opt(r)
	}
	if !validIdentifier.MatchString(r.tableName) {
		return nil, fmt.Errorf("invalid table name %q: must match [a-zA-Z_][a-zA-Z0-9_]*", r.tableName)
	}
	if err := r.ensureTable(ctx); err != nil {
		return nil, fmt.Errorf("create table: %w", err)
	}
	return r, nil
}

func (r *PostgresRegistry) ensureTable(ctx context.Context) error {
	query := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			fingerprint  TEXT PRIMARY KEY,
			hostname     TEXT NOT NULL DEFAULT '',
			ip           TEXT NOT NULL DEFAULT '',
			os           TEXT NOT NULL DEFAULT '',
			license_key  TEXT NOT NULL,
			registered_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			last_seen_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE INDEX IF NOT EXISTS idx_%s_license_key_last_seen
			ON %s (license_key, last_seen_at);
	`, r.tableName, r.tableName, r.tableName)
	_, err := r.pool.Exec(ctx, query)
	return err
}

func (r *PostgresRegistry) Register(ctx context.Context, node NodeInfo) (*NodeInfo, error) {
	now := time.Now()
	query := fmt.Sprintf(`
		INSERT INTO %s (fingerprint, hostname, ip, os, license_key, registered_at, last_seen_at)
		VALUES ($1, $2, $3, $4, $5, $6, $6)
		ON CONFLICT (fingerprint) DO UPDATE SET
			hostname = EXCLUDED.hostname,
			ip = EXCLUDED.ip,
			os = EXCLUDED.os,
			license_key = EXCLUDED.license_key,
			last_seen_at = EXCLUDED.last_seen_at
		RETURNING registered_at, last_seen_at
	`, r.tableName)

	err := r.pool.QueryRow(ctx, query,
		node.Fingerprint, node.Hostname, node.IP, node.OS, node.LicenseKey, now,
	).Scan(&node.RegisteredAt, &node.LastSeenAt)
	if err != nil {
		return nil, fmt.Errorf("register node: %w", err)
	}
	return &node, nil
}

func (r *PostgresRegistry) Deregister(ctx context.Context, fingerprint string) error {
	query := fmt.Sprintf(`DELETE FROM %s WHERE fingerprint = $1`, r.tableName)
	_, err := r.pool.Exec(ctx, query, fingerprint)
	if err != nil {
		return fmt.Errorf("deregister node: %w", err)
	}
	return nil
}

func (r *PostgresRegistry) Count(ctx context.Context, licenseKey string) (int, error) {
	query := fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE license_key = $1`, r.tableName)
	var count int
	err := r.pool.QueryRow(ctx, query, licenseKey).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count nodes: %w", err)
	}
	return count, nil
}

func (r *PostgresRegistry) List(ctx context.Context, licenseKey string) ([]NodeInfo, error) {
	query := fmt.Sprintf(`
		SELECT fingerprint, hostname, ip, os, license_key, registered_at, last_seen_at
		FROM %s WHERE license_key = $1 ORDER BY registered_at
	`, r.tableName)

	rows, err := r.pool.Query(ctx, query, licenseKey)
	if err != nil {
		return nil, fmt.Errorf("list nodes: %w", err)
	}
	defer rows.Close()

	var nodes []NodeInfo
	for rows.Next() {
		var n NodeInfo
		if err := rows.Scan(&n.Fingerprint, &n.Hostname, &n.IP, &n.OS,
			&n.LicenseKey, &n.RegisteredAt, &n.LastSeenAt); err != nil {
			return nil, fmt.Errorf("scan node: %w", err)
		}
		nodes = append(nodes, n)
	}
	return nodes, rows.Err()
}

func (r *PostgresRegistry) Ping(ctx context.Context, fingerprint string) error {
	query := fmt.Sprintf(`UPDATE %s SET last_seen_at = NOW() WHERE fingerprint = $1`, r.tableName)
	_, err := r.pool.Exec(ctx, query, fingerprint)
	if err != nil {
		return fmt.Errorf("ping node: %w", err)
	}
	return nil
}

func (r *PostgresRegistry) Prune(ctx context.Context, licenseKey string, olderThan time.Duration) (int, error) {
	cutoff := time.Now().Add(-olderThan)
	query := fmt.Sprintf(`DELETE FROM %s WHERE license_key = $1 AND last_seen_at < $2`, r.tableName)
	tag, err := r.pool.Exec(ctx, query, licenseKey, cutoff)
	if err != nil {
		return 0, fmt.Errorf("prune nodes: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

func (r *PostgresRegistry) Close(_ context.Context) error {
	return nil // user manages the pgxpool.Pool lifecycle
}
