package assembly

import (
	"fmt"
	"math"

	"bikegeo/internal/reboot/pose"
)

func isFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

func isFinitePoint(p pose.Point) bool {
	return isFinite(p.X) && isFinite(p.Y)
}

func validatePose(name string, p pose.Pose) error {
	if !isFinitePoint(p.Origin()) || !isFinite(float64(p.HeadingDeg())) {
		return fmt.Errorf("%s pose must be finite, got (%v, %v) at %v°",
			name, p.Origin().X, p.Origin().Y, p.HeadingDeg())
	}
	return nil
}

func validatePlacementPoses(component string, from pose.Pose, outputs ...pose.Pose) error {
	for _, p := range outputs {
		if !isFinitePoint(p.Origin()) {
			return fmt.Errorf("%s placement overflow from (%v, %v)",
				component, from.Origin().X, from.Origin().Y)
		}
	}
	return nil
}
