package cnwlicense

import (
	"fmt"
	"runtime"
)

// ExtractHardwareLimits extracts hardware limits from a license features map.
// JSON numbers are float64 by default, so this handles the conversion.
// A value of 0 means unlimited.
func ExtractHardwareLimits(features map[string]interface{}) HardwareLimits {
	var limits HardwareLimits
	if features == nil {
		return limits
	}
	if v, ok := features["max_cpu_per_node"]; ok {
		limits.MaxCPUPerNode = toInt(v)
	}
	if v, ok := features["max_nodes"]; ok {
		limits.MaxNodes = toInt(v)
	}
	return limits
}

// CheckCPU verifies that the current machine's CPU count does not exceed the limit.
// Returns nil if the limit is 0 (unlimited) or the CPU count is within bounds.
func CheckCPU(limits HardwareLimits) error {
	if limits.MaxCPUPerNode <= 0 {
		return nil
	}
	cpuCount := runtime.NumCPU()
	if cpuCount > limits.MaxCPUPerNode {
		return fmt.Errorf("%w: machine has %d CPUs, limit is %d", ErrCPULimitExceeded, cpuCount, limits.MaxCPUPerNode)
	}
	return nil
}

// CheckNodeCount verifies that the current node count does not exceed the limit.
// Returns nil if the limit is 0 (unlimited) or the count is within bounds.
func CheckNodeCount(limits HardwareLimits, currentNodes int) error {
	if limits.MaxNodes <= 0 {
		return nil
	}
	if currentNodes > limits.MaxNodes {
		return fmt.Errorf("%w: %d nodes active, limit is %d", ErrNodeLimitExceeded, currentNodes, limits.MaxNodes)
	}
	return nil
}

// toInt converts a JSON number (float64) or integer to int.
func toInt(v interface{}) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	default:
		return 0
	}
}
