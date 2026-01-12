package dag

import (
	"fmt"
	"strings"
)

// CompletionState tracks the outcome of a DAG execution.
// It categorizes nodes into different states based on execution results.
type CompletionState struct {
	// VerifiedNodes are nodes that completed successfully
	VerifiedNodes []string

	// UnverifiedNodes are nodes that completed but failed verification (non-fatal)
	UnverifiedNodes []string

	// FailedNodes are nodes that failed fatally
	FailedNodes []string

	// UnstartedNodes are nodes that never started due to upstream failures
	UnstartedNodes []string

	// DisabledNodes are nodes that were skipped because they're disabled
	DisabledNodes []string

	// Exception holds any fatal exception that occurred
	Exception error
}

// NewCompletionState creates an empty completion state.
func NewCompletionState() *CompletionState {
	return &CompletionState{
		VerifiedNodes:   []string{},
		UnverifiedNodes: []string{},
		FailedNodes:     []string{},
		UnstartedNodes:  []string{},
		DisabledNodes:   []string{},
	}
}

// IsSuccessful returns true if all nodes were verified and none failed.
func (cs *CompletionState) IsSuccessful() bool {
	return len(cs.FailedNodes) == 0 &&
		len(cs.UnverifiedNodes) == 0 &&
		cs.Exception == nil
}

// IsComplete returns true if the execution finished (successfully or not).
func (cs *CompletionState) IsComplete() bool {
	return len(cs.FailedNodes) > 0 ||
		len(cs.UnstartedNodes) > 0 ||
		cs.Exception != nil ||
		(len(cs.VerifiedNodes) > 0 && len(cs.UnverifiedNodes) == 0)
}

// HasFailures returns true if any nodes failed or an exception occurred.
func (cs *CompletionState) HasFailures() bool {
	return len(cs.FailedNodes) > 0 || cs.Exception != nil
}

// TotalNodes returns the total number of nodes tracked.
func (cs *CompletionState) TotalNodes() int {
	return len(cs.VerifiedNodes) +
		len(cs.UnverifiedNodes) +
		len(cs.FailedNodes) +
		len(cs.UnstartedNodes) +
		len(cs.DisabledNodes)
}

// String provides a human-readable summary of the completion state.
func (cs *CompletionState) String() string {
	var parts []string

	if len(cs.VerifiedNodes) > 0 {
		parts = append(parts, fmt.Sprintf("Verified: %d", len(cs.VerifiedNodes)))
	}
	if len(cs.UnverifiedNodes) > 0 {
		parts = append(parts, fmt.Sprintf("Unverified: %d", len(cs.UnverifiedNodes)))
	}
	if len(cs.FailedNodes) > 0 {
		parts = append(parts, fmt.Sprintf("Failed: %d", len(cs.FailedNodes)))
	}
	if len(cs.UnstartedNodes) > 0 {
		parts = append(parts, fmt.Sprintf("Unstarted: %d", len(cs.UnstartedNodes)))
	}
	if len(cs.DisabledNodes) > 0 {
		parts = append(parts, fmt.Sprintf("Disabled: %d", len(cs.DisabledNodes)))
	}
	if cs.Exception != nil {
		parts = append(parts, fmt.Sprintf("Exception: %v", cs.Exception))
	}

	if len(parts) == 0 {
		return "CompletionState: empty"
	}

	return "CompletionState{" + strings.Join(parts, ", ") + "}"
}

// DetailedString provides a detailed view including node names.
func (cs *CompletionState) DetailedString() string {
	var parts []string

	if len(cs.VerifiedNodes) > 0 {
		parts = append(parts, fmt.Sprintf("Verified: %v", cs.VerifiedNodes))
	}
	if len(cs.UnverifiedNodes) > 0 {
		parts = append(parts, fmt.Sprintf("Unverified: %v", cs.UnverifiedNodes))
	}
	if len(cs.FailedNodes) > 0 {
		parts = append(parts, fmt.Sprintf("Failed: %v", cs.FailedNodes))
	}
	if len(cs.UnstartedNodes) > 0 {
		parts = append(parts, fmt.Sprintf("Unstarted: %v", cs.UnstartedNodes))
	}
	if len(cs.DisabledNodes) > 0 {
		parts = append(parts, fmt.Sprintf("Disabled: %v", cs.DisabledNodes))
	}
	if cs.Exception != nil {
		parts = append(parts, fmt.Sprintf("Exception: %v", cs.Exception))
	}

	return "CompletionState{\n  " + strings.Join(parts, "\n  ") + "\n}"
}
