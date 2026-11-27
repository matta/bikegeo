package bounds

import (
	"fmt"
	"math"
)

// InputField identifies the physical input rejected by validation.
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

// NewInputError returns an *InputError attributing a formatted validation
// message to field.
func NewInputError(field InputField, format string, args ...any) error {
	return &InputError{Field: field, message: fmt.Sprintf(format, args...)}
}

func isFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}
