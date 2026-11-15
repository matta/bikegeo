package model

import (
	"fmt"
	"maps"
	"slices"
)

type ValidationIssue struct {
	ID         string
	Field      string
	Severity   Severity
	Title      string
	Message    string
	Details    string
	Expected   string
	Actual     string
	Difference float64
}

type PublishMode string

const (
	ModeStaticCad     PublishMode = "static_cad"
	ModeSaggedCad     PublishMode = "sagged_cad"
	ModeMismatchedCad PublishMode = "mismatched_cad"
	ModeUnspecified   PublishMode = "unspecified"
)

type ValidationReport struct {
	BikeID              string
	IsValid             bool
	Issues              []ValidationIssue
	GeometricA2c        float64
	StatedA2c           float64
	ExpectedForkA2c     float64
	GeoHeadsetSpacerMm  float64
	CupStack            float64
	TotalStructuralA2c  float64
	A2cDifference       Opt[float64]
	InferredPublishMode PublishMode
	FrontCenterCalc     float64
	FrontCenterDiff     Opt[float64]
	// Rounding budgets of the three closure checks; nil when a check did not run.
	A2cBudget         *ErrorBudget
	FrontCenterBudget *ErrorBudget
	WheelbaseBudget   *ErrorBudget
}

// FrameValidationReport holds the complete validation result across all sizes of a FrameModel.
type FrameValidationReport struct {
	FrameID     string
	IsValid     bool
	Issues      []ValidationIssue
	SizeReports map[string]ValidationReport
}

// ErrorSummary returns a human-readable summary of all validation errors on the frame.
func (r FrameValidationReport) ErrorSummary() string {
	if r.IsValid {
		return ""
	}
	var msgs []string
	for _, iss := range r.Issues {
		if iss.Severity == SeverityError {
			msgs = append(msgs, fmt.Sprintf("%s: %s", iss.Title, iss.Message))
		}
	}
	for _, sz := range slices.Sorted(maps.Keys(r.SizeReports)) {
		rep := r.SizeReports[sz]
		for _, iss := range rep.Issues {
			if iss.Severity == SeverityError {
				msgs = append(msgs, fmt.Sprintf("[Size %s] %s: %s", sz, iss.Title, iss.Message))
			}
		}
	}
	if len(msgs) == 0 {
		return fmt.Sprintf("frame %q is in an invalid state", r.FrameID)
	}
	return fmt.Sprintf("frame %q failed validation: %s", r.FrameID, msgs[0])
}
