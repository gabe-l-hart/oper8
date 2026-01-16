package dag

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewCompletionState(t *testing.T) {
	cs := NewCompletionState()

	require.NotNil(t, cs)
	require.Empty(t, cs.VerifiedNodes)
	require.Empty(t, cs.UnverifiedNodes)
	require.Empty(t, cs.FailedNodes)
	require.Empty(t, cs.UnstartedNodes)
	require.Empty(t, cs.DisabledNodes)
	require.Nil(t, cs.Exception)
}

func TestCompletionState_IsSuccessful(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(*CompletionState)
		successful bool
	}{
		{
			name: "all verified",
			setup: func(cs *CompletionState) {
				cs.VerifiedNodes = []string{"node1", "node2"}
			},
			successful: true,
		},
		{
			name: "has unverified nodes",
			setup: func(cs *CompletionState) {
				cs.VerifiedNodes = []string{"node1"}
				cs.UnverifiedNodes = []string{"node2"}
			},
			successful: false,
		},
		{
			name: "has failed nodes",
			setup: func(cs *CompletionState) {
				cs.VerifiedNodes = []string{"node1"}
				cs.FailedNodes = []string{"node2"}
			},
			successful: false,
		},
		{
			name: "has exception",
			setup: func(cs *CompletionState) {
				cs.VerifiedNodes = []string{"node1"}
				cs.Exception = errors.New("something failed")
			},
			successful: false,
		},
		{
			name: "empty state",
			setup: func(cs *CompletionState) {
				// No changes
			},
			successful: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cs := NewCompletionState()
			tt.setup(cs)

			result := cs.IsSuccessful()
			require.Equal(t, tt.successful, result)
		})
	}
}

func TestCompletionState_IsComplete(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*CompletionState)
		complete bool
	}{
		{
			name: "has failed nodes",
			setup: func(cs *CompletionState) {
				cs.FailedNodes = []string{"node1"}
			},
			complete: true,
		},
		{
			name: "has unstarted nodes",
			setup: func(cs *CompletionState) {
				cs.UnstartedNodes = []string{"node1"}
			},
			complete: true,
		},
		{
			name: "has exception",
			setup: func(cs *CompletionState) {
				cs.Exception = errors.New("error")
			},
			complete: true,
		},
		{
			name: "verified without unverified",
			setup: func(cs *CompletionState) {
				cs.VerifiedNodes = []string{"node1", "node2"}
			},
			complete: true,
		},
		{
			name: "only unverified nodes",
			setup: func(cs *CompletionState) {
				cs.UnverifiedNodes = []string{"node1"}
			},
			complete: false,
		},
		{
			name: "empty state",
			setup: func(cs *CompletionState) {
				// No changes
			},
			complete: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cs := NewCompletionState()
			tt.setup(cs)

			result := cs.IsComplete()
			require.Equal(t, tt.complete, result)
		})
	}
}

func TestCompletionState_HasFailures(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(*CompletionState)
		hasFailures bool
	}{
		{
			name: "has failed nodes",
			setup: func(cs *CompletionState) {
				cs.FailedNodes = []string{"node1"}
			},
			hasFailures: true,
		},
		{
			name: "has exception",
			setup: func(cs *CompletionState) {
				cs.Exception = errors.New("error")
			},
			hasFailures: true,
		},
		{
			name: "both failed and exception",
			setup: func(cs *CompletionState) {
				cs.FailedNodes = []string{"node1"}
				cs.Exception = errors.New("error")
			},
			hasFailures: true,
		},
		{
			name: "only verified nodes",
			setup: func(cs *CompletionState) {
				cs.VerifiedNodes = []string{"node1", "node2"}
			},
			hasFailures: false,
		},
		{
			name: "only unverified nodes",
			setup: func(cs *CompletionState) {
				cs.UnverifiedNodes = []string{"node1"}
			},
			hasFailures: false,
		},
		{
			name: "empty state",
			setup: func(cs *CompletionState) {
				// No changes
			},
			hasFailures: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cs := NewCompletionState()
			tt.setup(cs)

			result := cs.HasFailures()
			require.Equal(t, tt.hasFailures, result)
		})
	}
}

func TestCompletionState_TotalNodes(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*CompletionState)
		total int
	}{
		{
			name: "empty state",
			setup: func(cs *CompletionState) {
			},
			total: 0,
		},
		{
			name: "only verified",
			setup: func(cs *CompletionState) {
				cs.VerifiedNodes = []string{"n1", "n2", "n3"}
			},
			total: 3,
		},
		{
			name: "mixed states",
			setup: func(cs *CompletionState) {
				cs.VerifiedNodes = []string{"n1", "n2"}
				cs.UnverifiedNodes = []string{"n3"}
				cs.FailedNodes = []string{"n4"}
				cs.UnstartedNodes = []string{"n5", "n6"}
				cs.DisabledNodes = []string{"n7"}
			},
			total: 7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cs := NewCompletionState()
			tt.setup(cs)

			result := cs.TotalNodes()
			require.Equal(t, tt.total, result)
		})
	}
}

func TestCompletionState_String(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*CompletionState)
		contains []string
	}{
		{
			name: "empty state",
			setup: func(cs *CompletionState) {
			},
			contains: []string{"empty"},
		},
		{
			name: "verified nodes",
			setup: func(cs *CompletionState) {
				cs.VerifiedNodes = []string{"n1", "n2"}
			},
			contains: []string{"Verified: 2"},
		},
		{
			name: "unverified nodes",
			setup: func(cs *CompletionState) {
				cs.UnverifiedNodes = []string{"n1"}
			},
			contains: []string{"Unverified: 1"},
		},
		{
			name: "failed nodes",
			setup: func(cs *CompletionState) {
				cs.FailedNodes = []string{"n1", "n2"}
			},
			contains: []string{"Failed: 2"},
		},
		{
			name: "unstarted nodes",
			setup: func(cs *CompletionState) {
				cs.UnstartedNodes = []string{"n1"}
			},
			contains: []string{"Unstarted: 1"},
		},
		{
			name: "disabled nodes",
			setup: func(cs *CompletionState) {
				cs.DisabledNodes = []string{"n1", "n2", "n3"}
			},
			contains: []string{"Disabled: 3"},
		},
		{
			name: "exception",
			setup: func(cs *CompletionState) {
				cs.Exception = errors.New("test error")
			},
			contains: []string{"Exception:", "test error"},
		},
		{
			name: "mixed states",
			setup: func(cs *CompletionState) {
				cs.VerifiedNodes = []string{"n1"}
				cs.FailedNodes = []string{"n2"}
				cs.Exception = errors.New("error")
			},
			contains: []string{"Verified: 1", "Failed: 1", "Exception:"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cs := NewCompletionState()
			tt.setup(cs)

			result := cs.String()
			for _, substr := range tt.contains {
				require.Contains(t, result, substr)
			}
		})
	}
}

func TestCompletionState_DetailedString(t *testing.T) {
	cs := NewCompletionState()
	cs.VerifiedNodes = []string{"node1", "node2"}
	cs.UnverifiedNodes = []string{"node3"}
	cs.FailedNodes = []string{"node4"}
	cs.UnstartedNodes = []string{"node5", "node6"}
	cs.DisabledNodes = []string{"node7"}
	cs.Exception = errors.New("test exception")

	result := cs.DetailedString()

	// Should contain node names
	require.Contains(t, result, "node1")
	require.Contains(t, result, "node2")
	require.Contains(t, result, "node3")
	require.Contains(t, result, "node4")
	require.Contains(t, result, "node5")
	require.Contains(t, result, "node6")
	require.Contains(t, result, "node7")

	// Should contain categories
	require.Contains(t, result, "Verified:")
	require.Contains(t, result, "Unverified:")
	require.Contains(t, result, "Failed:")
	require.Contains(t, result, "Unstarted:")
	require.Contains(t, result, "Disabled:")
	require.Contains(t, result, "Exception:")

	// Should contain exception message
	require.Contains(t, result, "test exception")
}
