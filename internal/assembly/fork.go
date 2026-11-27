package assembly

import (
	"fmt"

	"github.com/matta/bikegeo/internal/pose"
)

// ForkLocal has origin (0, 0) at the crown race seat (mating LowerCupBottom),
// +X pointing down the steerer axis, and +Y pointing 90° CCW (forward
// perpendicular to the steerer in the rake direction).
var ForkLocal = pose.NewCoordinateSystem("part:fork")

// LowerBore identifies the head tube's lower bore, which sets the lower
// headset cup standards the frame accepts.
type LowerBore string

const (
	Bore34Pressed    LowerBore = "34_pressed"
	Bore44Pressed    LowerBore = "44_pressed"
	Bore56Pressed    LowerBore = "56_pressed"
	Bore42Integrated LowerBore = "42_integrated"
	Bore52Integrated LowerBore = "52_integrated"
)

// Steerer identifies a fork's steerer tube type. SteererBasis is used only by
// BasisFork and chart-basis SHIS checks.
type Steerer string

const (
	SteererStraight118 Steerer = "straight_118"
	SteererTapered     Steerer = "tapered"
	SteererBasis       Steerer = "basis"
)

// PhysicalSteerer identifies the physical steerer tube of a real fork
// (RigidFork or SuspensionFork), excluding the chart-basis sentinel.
type PhysicalSteerer string

const (
	PhysicalSteererStraight118 PhysicalSteerer = "straight_118"
	PhysicalSteererTapered     PhysicalSteerer = "tapered"
)

// Valid reports whether s is a recognized physical steerer tube type.
func (s PhysicalSteerer) Valid() bool {
	switch s {
	case PhysicalSteererStraight118, PhysicalSteererTapered:
		return true
	default:
		return false
	}
}

// Steerer converts s into the corresponding Steerer constant for SHIS checks.
func (s PhysicalSteerer) Steerer() Steerer {
	switch s {
	case PhysicalSteererStraight118:
		return SteererStraight118
	case PhysicalSteererTapered:
		return SteererTapered
	default:
		return Steerer(s)
	}
}

// LowerCupStandard identifies the SHIS lower headset cup interface marrying a
// frame's head tube to a fork steerer (or BareHeadTube for an uncupped basis chart).
type LowerCupStandard string

const (
	LowerCupEC34         LowerCupStandard = "EC34"
	LowerCupEC44         LowerCupStandard = "EC44"
	LowerCupZS44         LowerCupStandard = "ZS44"
	LowerCupZS56         LowerCupStandard = "ZS56"
	LowerCupIS42         LowerCupStandard = "IS42"
	LowerCupIS52         LowerCupStandard = "IS52"
	LowerCupBareHeadTube LowerCupStandard = "bare_head_tube"
)

// LowerCupSetup specifies the lower headset cup standard and axial stack height
// marrying a frame's head tube to a fork steerer.
type LowerCupSetup struct {
	Standard LowerCupStandard
	StackMm  float64
}

// Validate checks that the lower headset cup standard is recognized and its
// stack height satisfies the physical constraint for that standard.
func (c LowerCupSetup) Validate() error {
	if !isFinite(c.StackMm) {
		return invalidInput(InputLowerCup, "lower headset cup must be finite, got %v mm", c.StackMm)
	}
	switch c.Standard {
	case LowerCupEC34, LowerCupEC44:
		if c.StackMm <= 0 {
			return invalidInput(InputLowerCup, "%s external lower headset cup must have positive stack, got %v mm", c.Standard, c.StackMm)
		}
		return nil
	case LowerCupZS44, LowerCupZS56, LowerCupIS42, LowerCupIS52:
		if c.StackMm < 0 {
			return invalidInput(InputLowerCup, "%s lower headset cup must be non-negative, got %v mm", c.Standard, c.StackMm)
		}
		return nil
	case LowerCupBareHeadTube:
		if c.StackMm != 0 {
			return invalidInput(InputLowerCup, "bare head tube has no lower headset cup (must be 0 mm), got %v mm", c.StackMm)
		}
		return nil
	default:
		return invalidInput(InputLowerCup, "unknown or missing lower headset cup standard %q", c.Standard)
	}
}

// cupBore returns the head tube bore a physical lower cup standard fits, and
// false for the bare head tube (which fits any bore).
func cupBore(std LowerCupStandard) (LowerBore, bool) {
	switch std {
	case LowerCupEC34:
		return Bore34Pressed, true
	case LowerCupEC44, LowerCupZS44:
		return Bore44Pressed, true
	case LowerCupZS56:
		return Bore56Pressed, true
	case LowerCupIS42:
		return Bore42Integrated, true
	case LowerCupIS52:
		return Bore52Integrated, true
	case LowerCupBareHeadTube:
		return "", false
	default:
		return "", false
	}
}

// cupTakesTaperedSteerer reports whether a physical cup standard accepts a
// tapered steerer (every standard accepts a straight one).
func cupTakesTaperedSteerer(std LowerCupStandard) bool {
	switch std {
	case LowerCupEC44, LowerCupZS56, LowerCupIS52:
		return true
	case LowerCupEC34, LowerCupZS44, LowerCupIS42, LowerCupBareHeadTube:
		return false
	default:
		return false
	}
}

// CheckSHIS is the canonical SHIS lower-headset compatibility rule: it checks
// that bore and steerer are recognized, cup satisfies its stack constraint,
// physical cups match bore, physical forks use physical cups, and tapered
// steerers pair only with tapered-capable cups. A bare head tube fits any
// recognized bore only with SteererBasis, and SteererBasis fits any valid cup
// matching bore.
func CheckSHIS(bore LowerBore, cup LowerCupSetup, steerer Steerer) error {
	switch bore {
	case Bore34Pressed, Bore44Pressed, Bore56Pressed, Bore42Integrated, Bore52Integrated:
	default:
		return invalidInput(InputLowerCup, "unknown or missing head tube lower bore %q", bore)
	}
	switch steerer {
	case SteererStraight118, SteererTapered, SteererBasis:
	default:
		return invalidInput(InputLowerCup, "unknown or missing fork steerer %q", steerer)
	}
	if err := cup.Validate(); err != nil {
		return err
	}
	cupFits, physical := cupBore(cup.Standard)
	if !physical {
		if steerer != SteererBasis {
			return invalidInput(InputLowerCup, "a %s steerer fork needs a physical lower cup, not %s", steerer, cup.Standard)
		}
		return nil
	}
	if cupFits != bore {
		return invalidInput(InputLowerCup, "%s lower cup needs a %s head tube bore, but the frame has %s", cup.Standard, cupFits, bore)
	}
	switch steerer {
	case SteererStraight118, SteererBasis:
		return nil
	case SteererTapered:
		if !cupTakesTaperedSteerer(cup.Standard) {
			return invalidInput(InputLowerCup, "%s lower cup does not fit a tapered steerer", cup.Standard)
		}
		return nil
	default:
		return nil
	}
}

// ForkTravel is the closed set of supported fork suspension travel classes.
type ForkTravel int

const (
	TravelRigid ForkTravel = 0
	Travel100   ForkTravel = 100
	Travel120   ForkTravel = 120
	Travel130   ForkTravel = 130
)

// Valid reports whether t is one of the supported fork travel values.
func (t ForkTravel) Valid() bool {
	switch t {
	case TravelRigid, Travel100, Travel120, Travel130:
		return true
	default:
		return false
	}
}

// Mm returns the travel in millimetres (0 for rigid or unknown travel).
func (t ForkTravel) Mm() float64 {
	switch t {
	case TravelRigid:
		return 0
	case Travel100:
		return 100
	case Travel120:
		return 120
	case Travel130:
		return 130
	default:
		return 0
	}
}

// IsSuspension reports whether t is a suspension fork travel.
func (t ForkTravel) IsSuspension() bool {
	switch t {
	case Travel100, Travel120, Travel130:
		return true
	case TravelRigid:
		return false
	default:
		return false
	}
}

// SagRange returns the travel class's recommended [minPct, maxPct] sag range.
func (t ForkTravel) SagRange() (minPct, maxPct float64, ok bool) {
	switch t {
	case Travel100:
		return 15, 20, true
	case Travel120:
		return 15, 25, true
	case Travel130:
		return 20, 25, true
	case TravelRigid:
		return 0, 0, false
	default:
		return 0, 0, false
	}
}

// SagPcts returns a fresh slice of the standard sag percentages evaluated for t.
func (t ForkTravel) SagPcts() []float64 {
	switch t {
	case Travel100:
		return []float64{15, 20}
	case Travel120:
		return []float64{15, 20, 25}
	case Travel130:
		return []float64{20, 25}
	case TravelRigid:
		return nil
	default:
		return nil
	}
}

// DefaultSagPct returns the default sag percentage for t when sag is omitted.
func (t ForkTravel) DefaultSagPct() float64 {
	switch t {
	case TravelRigid:
		return 0
	case Travel100:
		return 20
	case Travel120:
		return 25
	case Travel130:
		return 25
	default:
		return 0
	}
}

// String returns a human-readable label for t.
func (t ForkTravel) String() string {
	switch t {
	case TravelRigid:
		return "rigid"
	case Travel100:
		return "100mm"
	case Travel120:
		return "120mm"
	case Travel130:
		return "130mm"
	default:
		return fmt.Sprintf("%dmm", int(t))
	}
}

// RigidFork is a physical rigid fork whose front axle sits at a constant
// (StaticA2cMm, RakeMm) in ForkLocal across both Static and Dynamic states.
// Construct instances with NewRigidFork.
type RigidFork struct {
	staticA2cMm float64
	rakeMm      float64
	steerer     PhysicalSteerer
	valid       bool
}

// NewRigidFork validates and constructs a physical RigidFork.
func NewRigidFork(staticA2cMm, rakeMm float64, steerer PhysicalSteerer) (RigidFork, error) {
	if !isFinite(staticA2cMm) || staticA2cMm <= 0 {
		return RigidFork{}, invalidInput(InputForkA2c, "fork static A2C must be finite and positive, got %v mm", staticA2cMm)
	}
	if !isFinite(rakeMm) {
		return RigidFork{}, fmt.Errorf("fork rake must be finite, got %v mm", rakeMm)
	}
	if !steerer.Valid() {
		return RigidFork{}, invalidInput(InputLowerCup, "unknown or missing fork steerer %q", steerer)
	}
	return RigidFork{
		staticA2cMm: staticA2cMm,
		rakeMm:      rakeMm,
		steerer:     steerer,
		valid:       true,
	}, nil
}

func (f RigidFork) requireValid() {
	if !f.valid {
		panic("assembly: uninitialized zero-value RigidFork")
	}
}

// StaticA2cMm returns the fork's axle-to-crown length along the steerer axis.
func (f RigidFork) StaticA2cMm() float64 {
	f.requireValid()
	return f.staticA2cMm
}

// RakeMm returns the fork's perpendicular forward offset from the steerer axis.
func (f RigidFork) RakeMm() float64 {
	f.requireValid()
	return f.rakeMm
}

// Steerer returns the fork's physical steerer type.
func (f RigidFork) Steerer() PhysicalSteerer {
	f.requireValid()
	return f.steerer
}

// CheckHeadset enforces physical SHIS lower-cup compatibility with bore.
func (f RigidFork) CheckHeadset(bore LowerBore, cup LowerCupSetup) error {
	f.requireValid()
	return CheckSHIS(bore, cup, f.steerer.Steerer())
}

// AxleInFork returns the front axle position in ForkLocal. Rigid forks are
// identical across Static and Dynamic load states.
func (f RigidFork) AxleInFork(state LoadState) pose.Point {
	f.requireValid()
	switch state {
	case Static, Dynamic:
		return pose.NewPoint(ForkLocal, f.staticA2cMm, f.rakeMm)
	default:
		panic(fmt.Sprintf("assembly: invalid LoadState %q", state))
	}
}

// SuspensionFork is a physical suspension fork whose axle-to-crown compresses
// by Travel.Mm() * (SagPct / 100) when loaded from Static to Dynamic.
// Construct instances with NewSuspensionFork.
type SuspensionFork struct {
	staticA2cMm  float64
	dynamicA2cMm float64
	rakeMm       float64
	travel       ForkTravel
	steerer      PhysicalSteerer
	sagPct       float64
	valid        bool
}

// NewSuspensionFork validates and constructs a physical SuspensionFork.
func NewSuspensionFork(staticA2cMm, rakeMm float64, travel ForkTravel, steerer PhysicalSteerer, sagPct float64) (SuspensionFork, error) {
	if !isFinite(staticA2cMm) || staticA2cMm <= 0 {
		return SuspensionFork{}, invalidInput(InputForkA2c, "fork static A2C must be finite and positive, got %v mm", staticA2cMm)
	}
	if !travel.IsSuspension() {
		return SuspensionFork{}, fmt.Errorf("suspension fork requires a suspension travel class, got %d mm", travel)
	}
	if !steerer.Valid() {
		return SuspensionFork{}, invalidInput(InputLowerCup, "unknown or missing fork steerer %q", steerer)
	}
	if !isFinite(sagPct) || sagPct < 0 || sagPct > 100 {
		return SuspensionFork{}, invalidInput(InputForkSag, "fork sag must be in [0%%, 100%%], got %v%%", sagPct)
	}
	dynamicA2c := staticA2cMm - travel.Mm()*(sagPct/100.0)
	if !isFinite(dynamicA2c) || dynamicA2c <= 0 {
		return SuspensionFork{}, invalidInput(InputForkA2c, "fork effective A2C must be finite and positive, got %v mm", dynamicA2c)
	}
	if !isFinite(rakeMm) {
		return SuspensionFork{}, fmt.Errorf("fork rake must be finite, got %v mm", rakeMm)
	}
	return SuspensionFork{
		staticA2cMm:  staticA2cMm,
		dynamicA2cMm: dynamicA2c,
		rakeMm:       rakeMm,
		travel:       travel,
		steerer:      steerer,
		sagPct:       sagPct,
		valid:        true,
	}, nil
}

func (f SuspensionFork) requireValid() {
	if !f.valid {
		panic("assembly: uninitialized zero-value SuspensionFork")
	}
}

// StaticA2cMm returns the unloaded axle-to-crown length at 0% sag.
func (f SuspensionFork) StaticA2cMm() float64 {
	f.requireValid()
	return f.staticA2cMm
}

// DynamicA2cMm returns the sagged axle-to-crown length at SagPct.
func (f SuspensionFork) DynamicA2cMm() float64 {
	f.requireValid()
	return f.dynamicA2cMm
}

// RakeMm returns the fork's perpendicular forward offset from the steerer axis.
func (f SuspensionFork) RakeMm() float64 {
	f.requireValid()
	return f.rakeMm
}

// Travel returns the fork's suspension travel class.
func (f SuspensionFork) Travel() ForkTravel {
	f.requireValid()
	return f.travel
}

// Steerer returns the fork's physical steerer type.
func (f SuspensionFork) Steerer() PhysicalSteerer {
	f.requireValid()
	return f.steerer
}

// SagPct returns the fork's seated sag percentage in [0, 100].
func (f SuspensionFork) SagPct() float64 {
	f.requireValid()
	return f.sagPct
}

// CheckHeadset enforces physical SHIS lower-cup compatibility with bore.
func (f SuspensionFork) CheckHeadset(bore LowerBore, cup LowerCupSetup) error {
	f.requireValid()
	return CheckSHIS(bore, cup, f.steerer.Steerer())
}

// AxleInFork returns the front axle position in ForkLocal for state: Static
// uses StaticA2cMm; Dynamic uses DynamicA2cMm.
func (f SuspensionFork) AxleInFork(state LoadState) pose.Point {
	f.requireValid()
	switch state {
	case Static:
		return pose.NewPoint(ForkLocal, f.staticA2cMm, f.rakeMm)
	case Dynamic:
		return pose.NewPoint(ForkLocal, f.dynamicA2cMm, f.rakeMm)
	default:
		panic(fmt.Sprintf("assembly: invalid LoadState %q", state))
	}
}

// BasisFork represents a manufacturer chart-basis fork hypothesis (no physical
// steerer or suspension travel). Construct instances with NewBasisFork.
type BasisFork struct {
	basisA2cMm float64
	rakeMm     float64
	valid      bool
}

// NewBasisFork validates and constructs a BasisFork for chart-consistency and
// basis-fork inspection.
func NewBasisFork(basisA2cMm, rakeMm float64) (BasisFork, error) {
	if !isFinite(basisA2cMm) || basisA2cMm <= 0 {
		return BasisFork{}, invalidInput(InputForkA2c, "fork static A2C must be finite and positive, got %v mm", basisA2cMm)
	}
	if !isFinite(rakeMm) {
		return BasisFork{}, fmt.Errorf("fork rake must be finite, got %v mm", rakeMm)
	}
	return BasisFork{
		basisA2cMm: basisA2cMm,
		rakeMm:     rakeMm,
		valid:      true,
	}, nil
}

func (f BasisFork) requireValid() {
	if !f.valid {
		panic("assembly: uninitialized zero-value BasisFork")
	}
}

// BasisA2cMm returns the chart-basis axle-to-crown length.
func (f BasisFork) BasisA2cMm() float64 {
	f.requireValid()
	return f.basisA2cMm
}

// RakeMm returns the chart-basis fork rake.
func (f BasisFork) RakeMm() float64 {
	f.requireValid()
	return f.rakeMm
}

// CheckHeadset permits LowerCupBareHeadTube (0 mm) or any valid physical cup
// matching bore.
func (f BasisFork) CheckHeadset(bore LowerBore, cup LowerCupSetup) error {
	f.requireValid()
	return CheckSHIS(bore, cup, SteererBasis)
}

// AxleInFork returns the basis front axle position in ForkLocal (identical
// across Static and Dynamic load states).
func (f BasisFork) AxleInFork(state LoadState) pose.Point {
	f.requireValid()
	switch state {
	case Static, Dynamic:
		return pose.NewPoint(ForkLocal, f.basisA2cMm, f.rakeMm)
	default:
		panic(fmt.Sprintf("assembly: invalid LoadState %q", state))
	}
}
