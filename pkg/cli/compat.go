package cli

import (
	"bikegeo/pkg/compare"
	"bikegeo/pkg/model"
)

// Forwarding names kept so existing callers and documentation stay valid; the
// implementations live in pkg/compare.

// ResolveFrameByToken finds a frame by ID, size-suffixed ID, name, or unique substring.
func ResolveFrameByToken(frames []model.FrameModel, token string) (*model.FrameModel, string, error) {
	return compare.ResolveFrameByToken(frames, token)
}

// ParseComparisonSpecs parses "frame[:size][@mod]..." into comparison setups.
func ParseComparisonSpecs(frames []model.FrameModel, spec string) ([]model.ComparisonSetup, error) {
	return compare.ParseComparisonSpecs(frames, spec)
}

// DefaultComparisonState returns the compiled baseline comparison configuration.
func DefaultComparisonState() model.SavedComparisonState {
	return compare.DefaultComparisonState()
}
