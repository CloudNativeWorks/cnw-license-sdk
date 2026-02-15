package cnwlicense

import (
	"crypto/sha256"
	"fmt"
	"net"
	"os"
	"runtime"
	"sort"
	"strings"
)

// GenerateFingerprint produces a deterministic, reboot-safe machine identifier.
// It combines hostname, MAC addresses, OS, architecture, and machine-id (Linux)
// into a SHA-256 hex string.
//
// In container environments where MAC addresses may not be available,
// the fingerprint falls back to hostname + OS + arch + machine-id.
// For Kubernetes pods, consider setting a stable HOSTNAME env var or
// using the CNW_FINGERPRINT environment variable to override entirely.
func GenerateFingerprint() (string, error) {
	// Allow explicit override via environment variable
	if fp := os.Getenv("CNW_FINGERPRINT"); fp != "" {
		return fp, nil
	}

	var parts []string

	// Hostname
	hostname, err := os.Hostname()
	if err != nil {
		return "", fmt.Errorf("get hostname: %w", err)
	}
	parts = append(parts, hostname)

	// MAC addresses (sorted for determinism, best-effort)
	macs, err := getMACAddresses()
	if err == nil && len(macs) > 0 {
		parts = append(parts, macs...)
	}

	// OS and architecture
	parts = append(parts, runtime.GOOS, runtime.GOARCH)

	// Machine ID (Linux only, best-effort)
	if machineID, err := os.ReadFile("/etc/machine-id"); err == nil {
		parts = append(parts, strings.TrimSpace(string(machineID)))
	}

	h := sha256.New()
	h.Write([]byte(strings.Join(parts, "|")))
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// getMACAddresses returns sorted, non-loopback hardware MAC addresses.
func getMACAddresses() ([]string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	var macs []string
	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		mac := iface.HardwareAddr.String()
		if mac != "" {
			macs = append(macs, mac)
		}
	}
	sort.Strings(macs)
	return macs, nil
}
