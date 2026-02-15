package cnwlicense

import (
	"errors"
	"runtime"
	"testing"
)

func TestExtractHardwareLimits(t *testing.T) {
	tests := []struct {
		name     string
		features map[string]interface{}
		want     HardwareLimits
	}{
		{
			name:     "nil features",
			features: nil,
			want:     HardwareLimits{},
		},
		{
			name:     "empty features",
			features: map[string]interface{}{},
			want:     HardwareLimits{},
		},
		{
			name: "float64 values (JSON default)",
			features: map[string]interface{}{
				"max_cpu_per_node": float64(8),
				"max_nodes":        float64(3),
			},
			want: HardwareLimits{MaxCPUPerNode: 8, MaxNodes: 3},
		},
		{
			name: "int values",
			features: map[string]interface{}{
				"max_cpu_per_node": 16,
				"max_nodes":        5,
			},
			want: HardwareLimits{MaxCPUPerNode: 16, MaxNodes: 5},
		},
		{
			name: "only max_cpu_per_node",
			features: map[string]interface{}{
				"max_cpu_per_node": float64(4),
			},
			want: HardwareLimits{MaxCPUPerNode: 4, MaxNodes: 0},
		},
		{
			name: "only max_nodes",
			features: map[string]interface{}{
				"max_nodes": float64(10),
			},
			want: HardwareLimits{MaxCPUPerNode: 0, MaxNodes: 10},
		},
		{
			name: "unrelated features ignored",
			features: map[string]interface{}{
				"custom_feature": "enabled",
				"max_nodes":      float64(2),
			},
			want: HardwareLimits{MaxCPUPerNode: 0, MaxNodes: 2},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractHardwareLimits(tt.features)
			if got != tt.want {
				t.Errorf("ExtractHardwareLimits() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestCheckCPU(t *testing.T) {
	numCPU := runtime.NumCPU()

	tests := []struct {
		name    string
		limits  HardwareLimits
		wantErr bool
	}{
		{
			name:    "unlimited (0)",
			limits:  HardwareLimits{MaxCPUPerNode: 0},
			wantErr: false,
		},
		{
			name:    "within limit",
			limits:  HardwareLimits{MaxCPUPerNode: numCPU + 10},
			wantErr: false,
		},
		{
			name:    "exact limit",
			limits:  HardwareLimits{MaxCPUPerNode: numCPU},
			wantErr: false,
		},
		{
			name:    "exceeded",
			limits:  HardwareLimits{MaxCPUPerNode: 1},
			wantErr: numCPU > 1, // only fails if machine has >1 CPU
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckCPU(tt.limits)
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckCPU() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && !errors.Is(err, ErrCPULimitExceeded) {
				t.Errorf("expected ErrCPULimitExceeded, got %v", err)
			}
		})
	}
}

func TestCheckNodeCount(t *testing.T) {
	tests := []struct {
		name         string
		limits       HardwareLimits
		currentNodes int
		wantErr      bool
	}{
		{
			name:         "unlimited (0)",
			limits:       HardwareLimits{MaxNodes: 0},
			currentNodes: 100,
			wantErr:      false,
		},
		{
			name:         "within limit",
			limits:       HardwareLimits{MaxNodes: 5},
			currentNodes: 3,
			wantErr:      false,
		},
		{
			name:         "at limit",
			limits:       HardwareLimits{MaxNodes: 5},
			currentNodes: 5,
			wantErr:      false,
		},
		{
			name:         "exceeded",
			limits:       HardwareLimits{MaxNodes: 5},
			currentNodes: 6,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckNodeCount(tt.limits, tt.currentNodes)
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckNodeCount() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && !errors.Is(err, ErrNodeLimitExceeded) {
				t.Errorf("expected ErrNodeLimitExceeded, got %v", err)
			}
		})
	}
}
