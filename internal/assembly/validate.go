package assembly

import (
	"fmt"
	"math"

	"github.com/matta/bikegeo/internal/bounds"
	"github.com/matta/bikegeo/internal/pose"
)

func invalidInput(field bounds.InputField, format string, args ...any) error {
	return bounds.NewInputError(field, format, args...)
}

func isFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

func isFinitePoint(p pose.Point) bool {
	return isFinite(p.X()) && isFinite(p.Y())
}

func validatePlacementPoints(component string, from pose.Point, outputs ...pose.Point) error {
	for _, p := range outputs {
		if !isFinitePoint(p) {
			return fmt.Errorf("%s placement overflow from (%v, %v)",
				component, from.X(), from.Y())
		}
	}
	return nil
}
