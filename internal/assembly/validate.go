package assembly

import (
	"fmt"
	"math"

	"bikegeo/internal/pose"
)

// InputField identifies the physical input rejected by assembly validation.
type InputField string

const (
	InputForkA2c           InputField = "fork A2C"
	InputForkSag           InputField = "fork sag"
	InputLowerCup          InputField = "lower headset cup"
	InputStemSpacers       InputField = "stem spacers"
	InputStemLength        InputField = "stem length"
	InputStemAngle         InputField = "stem angle"
	InputHandlebarRoll     InputField = "handlebar roll"
	InputSeatpostInsertion InputField = "seatpost insertion"
	InputSeatpostSetback   InputField = "seatpost setback"
	InputSaddleRailOffset  InputField = "saddle rail offset"
	InputSaddleRailPitch   InputField = "saddle rail pitch"
	InputFitTargetBRP      InputField = "BRP fit target"
	InputFitTargetGrip     InputField = "HandGrip fit target"
)

// InputError carries a field identity independently of its human-readable message.
type InputError struct {
	Field   InputField
	message string
}

func (e *InputError) Error() string { return e.message }

func invalidInput(field InputField, format string, args ...any) error {
	return &InputError{Field: field, message: fmt.Sprintf(format, args...)}
}

func isFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

func isFinitePoint(p pose.Point[DynamicWorld]) bool {
	return isFinite(p.X) && isFinite(p.Y)
}

func validatePose(name string, p pose.Pose[FrameAxis, DynamicWorld]) error {
	if !isFinitePoint(p.Origin()) || !isFinite(float64(p.HeadingDeg())) {
		return fmt.Errorf("%s pose must be finite, got (%v, %v) at %v°",
			name, p.Origin().X, p.Origin().Y, p.HeadingDeg())
	}
	return nil
}

func validatePlacementPoses(component string, from pose.Pose[FrameAxis, DynamicWorld], outputs ...pose.Pose[FrameAxis, DynamicWorld]) error {
	for _, p := range outputs {
		if !isFinitePoint(p.Origin()) {
			return fmt.Errorf("%s placement overflow from (%v, %v)",
				component, from.Origin().X, from.Origin().Y)
		}
	}
	return nil
}
