package catalog

import (
	"fmt"
	"strconv"

	"github.com/matta/bikegeo/internal/assembly"
	"github.com/matta/bikegeo/internal/pose"
)

// ForkVariant is one published axle-to-crown and rake configuration of a Fork.
type ForkVariant struct {
	StaticA2cMm float64
	RakeMm      float64
}

// Fork is a catalog fork model with its published A2C and rake variants.
type Fork struct {
	ID      string
	Name    string
	Travel  assembly.ForkTravel
	Steerer assembly.PhysicalSteerer
	// RigidClass groups rigid forks by effective length for compatibility;
	// suspension forks have RigidClassNone.
	RigidClass RigidClass
	Variants   []ForkVariant
}

// RigidClass is a hand-assigned length class of commercial rigid forks.
type RigidClass string

const (
	RigidClassNone RigidClass = ""
	RigidClass440  RigidClass = "440"
	RigidClass470  RigidClass = "470"
	RigidClass485  RigidClass = "485"
	RigidClass495  RigidClass = "495"
)

// standInRigidClasses returns the rigid classes a listed suspension fork of the
// given travel also unlocks. The stand-in is one way: a rigid fork never
// unlocks a suspension travel.
func standInRigidClasses(travel assembly.ForkTravel) []RigidClass {
	switch travel {
	case assembly.Travel100:
		return []RigidClass{RigidClass485}
	case assembly.Travel120:
		return []RigidClass{RigidClass495}
	case assembly.TravelRigid, assembly.Travel130:
		return nil
	default:
		return nil
	}
}

// IsMultiVariant reports whether f has more than one published variant.
func (f Fork) IsMultiVariant() bool {
	return len(f.Variants) > 1
}

// VariantID returns the query identifier for variant v of f: bare f.ID on
// single-variant models, or "<f.ID>-<StaticA2cMm>x<RakeMm>" without trailing
// ".0" on multi-variant models.
func (f Fork) VariantID(v ForkVariant) string {
	if !f.IsMultiVariant() {
		return f.ID
	}
	return fmt.Sprintf("%s-%sx%s", f.ID, formatDim(v.StaticA2cMm), formatDim(v.RakeMm))
}

func formatDim(x float64) string {
	return strconv.FormatFloat(x, 'f', -1, 64)
}

// PhysicalFork is the concrete physical fork contract returned by ForkOption
// (satisfied by assembly.RigidFork and assembly.SuspensionFork, excluding
// assembly.BasisFork).
type PhysicalFork interface {
	CheckHeadset(bore assembly.LowerBore, cup assembly.LowerCupSetup) error
	AxleInFork(state assembly.LoadState) pose.Point
	Steerer() assembly.PhysicalSteerer
}

// ForkOption is one concrete fork variant (model + A2C/rake) offered by a frame.
type ForkOption struct {
	Fork    Fork
	Variant ForkVariant
	// LowerCup is the typical cup this fork installs with on the offering
	// frame's head tube.
	LowerCup assembly.LowerCupSetup
}

// ID returns the concrete query identifier for o.
func (o ForkOption) ID() string {
	return o.Fork.VariantID(o.Variant)
}

// Name returns the display name for o with "(<A2C>/<Rake>)" appended.
func (o ForkOption) Name() string {
	return fmt.Sprintf("%s (%s/%s)", o.Fork.Name, formatDim(o.Variant.StaticA2cMm), formatDim(o.Variant.RakeMm))
}

// PhysicalFork constructs the concrete assembly.RigidFork or
// assembly.SuspensionFork for o at sagPct using o's published StaticA2cMm.
func (o ForkOption) PhysicalFork(sagPct float64) (PhysicalFork, error) {
	return o.PhysicalForkWithA2c(o.Variant.StaticA2cMm, sagPct)
}

// PhysicalForkWithA2c constructs the concrete assembly.RigidFork or
// assembly.SuspensionFork for o with staticA2cMm and sagPct (allowing /bike
// custom fork_a2c overrides while rejecting non-zero sag on rigid forks).
func (o ForkOption) PhysicalForkWithA2c(staticA2cMm, sagPct float64) (PhysicalFork, error) {
	if o.Fork.Travel.IsSuspension() {
		return assembly.NewSuspensionFork(staticA2cMm, o.Variant.RakeMm, o.Fork.Travel, o.Fork.Steerer, sagPct)
	}
	fork, err := assembly.NewRigidFork(staticA2cMm, o.Variant.RakeMm, o.Fork.Steerer)
	if err != nil {
		return nil, err
	}
	if sagPct != 0 {
		return nil, assembly.NewInputError(assembly.InputForkSag, "rigid fork cannot sag (sag must be 0%%), got %v%%", sagPct)
	}
	return fork, nil
}

// The 16 fork models and their published variants.
var (
	sunliner440 = Fork{
		ID: "sunliner_440", Name: "Tumbleweed Chromoly Steel Unicrown 440mm", Steerer: assembly.PhysicalSteererStraight118, RigidClass: RigidClass440,
		Variants: []ForkVariant{{StaticA2cMm: 440, RakeMm: 55}},
	} // catalog
	surlyEcr468 = Fork{
		ID: "surly_ecr_468", Name: "Surly ECR 4130 Chromoly 468mm", Steerer: assembly.PhysicalSteererStraight118, RigidClass: RigidClass470,
		Variants: []ForkVariant{{StaticA2cMm: 468, RakeMm: 43}},
	} // catalog
	firestarterDeluxe483 = Fork{
		ID: "firestarter_deluxe_483", Name: "Salsa Firestarter / Cutthroat Carbon Deluxe 483mm", Steerer: assembly.PhysicalSteererTapered, RigidClass: RigidClass485,
		Variants: []ForkVariant{{StaticA2cMm: 483, RakeMm: 51}},
	} // catalog
	surlyKrampus483 = Fork{
		ID: "surly_krampus_483", Name: "Surly Krampus 4130 Chromoly 483mm", Steerer: assembly.PhysicalSteererStraight118, RigidClass: RigidClass485,
		Variants: []ForkVariant{{StaticA2cMm: 483, RakeMm: 47}},
	} // catalog
	sklarSteel485 = Fork{
		ID: "sklar_steel_485", Name: "Sklar Chromoly Steel 485mm", Steerer: assembly.PhysicalSteererStraight118, RigidClass: RigidClass485,
		Variants: []ForkVariant{{StaticA2cMm: 485, RakeMm: 52}},
	} // catalog
	enveMountainBoost = Fork{
		ID: "enve_mountain_boost", Name: "ENVE Mountain Carbon Boost", Steerer: assembly.PhysicalSteererTapered, RigidClass: RigidClass495,
		Variants: []ForkVariant{{StaticA2cMm: 490, RakeMm: 44}, {StaticA2cMm: 492, RakeMm: 52}},
	} // published (S5)
	lithicCarbonMountain = Fork{
		ID: "lithic_carbon_mountain", Name: "Lithic Carbon Mountain", Steerer: assembly.PhysicalSteererTapered, RigidClass: RigidClass495,
		Variants: []ForkVariant{{StaticA2cMm: 495, RakeMm: 44}, {StaticA2cMm: 498, RakeMm: 51.5}},
	} // published (S6)
	sid100 = Fork{
		ID: "sid_100", Name: "RockShox SID 100mm", Travel: assembly.Travel100, Steerer: assembly.PhysicalSteererTapered,
		Variants: []ForkVariant{{StaticA2cMm: 506, RakeMm: 44}, {StaticA2cMm: 506, RakeMm: 51}},
	} // catalog
	fox34SL120 = Fork{
		ID: "fox34_sl_120", Name: "Fox 34 SL 120mm", Travel: assembly.Travel120, Steerer: assembly.PhysicalSteererTapered,
		Variants: []ForkVariant{{StaticA2cMm: 530.5, RakeMm: 44}},
	} // published (fork_fox_34_sl.md)
	sid120 = Fork{
		ID: "sid_120", Name: "RockShox SID 120mm", Travel: assembly.Travel120, Steerer: assembly.PhysicalSteererTapered,
		Variants: []ForkVariant{{StaticA2cMm: 530, RakeMm: 44}, {StaticA2cMm: 534, RakeMm: 44}},
	} // catalog
	pike120 = Fork{
		ID: "pike_120", Name: "RockShox Pike 120mm", Travel: assembly.Travel120, Steerer: assembly.PhysicalSteererTapered,
		Variants: []ForkVariant{{StaticA2cMm: 531, RakeMm: 44}},
	} // catalog
	fox34_120 = Fork{
		ID: "fox34_120", Name: "Fox Float 34 120mm", Travel: assembly.Travel120, Steerer: assembly.PhysicalSteererTapered,
		Variants: []ForkVariant{{StaticA2cMm: 534, RakeMm: 44}, {StaticA2cMm: 534, RakeMm: 51}},
	} // published (fork_fox_34.md)
	fox36SL120 = Fork{
		ID: "fox36_sl_120", Name: "Fox 36 SL 120mm", Travel: assembly.Travel120, Steerer: assembly.PhysicalSteererTapered,
		Variants: []ForkVariant{{StaticA2cMm: 536, RakeMm: 44}},
	} // published (fork_fox_36_sl.md)
	fox34SL130 = Fork{
		ID: "fox34_sl_130", Name: "Fox 34 SL 130mm", Travel: assembly.Travel130, Steerer: assembly.PhysicalSteererTapered,
		Variants: []ForkVariant{{StaticA2cMm: 540.5, RakeMm: 44}},
	} // published (fork_fox_34_sl.md)
	fox34_130 = Fork{
		ID: "fox34_130", Name: "Fox Float 34 130mm", Travel: assembly.Travel130, Steerer: assembly.PhysicalSteererTapered,
		Variants: []ForkVariant{{StaticA2cMm: 544, RakeMm: 44}, {StaticA2cMm: 544, RakeMm: 51}},
	} // published (fork_fox_34.md)
	fox36SL130 = Fork{
		ID: "fox36_sl_130", Name: "Fox 36 SL 130mm", Travel: assembly.Travel130, Steerer: assembly.PhysicalSteererTapered,
		Variants: []ForkVariant{{StaticA2cMm: 546, RakeMm: 44}},
	} // published (fork_fox_36_sl.md)
)

// Forks returns the 16 fork models in catalog order.
func Forks() []Fork {
	return []Fork{
		sunliner440, surlyEcr468, firestarterDeluxe483, surlyKrampus483,
		sklarSteel485, enveMountainBoost, lithicCarbonMountain,
		sid100, fox34SL120, sid120, pike120, fox34_120, fox36SL120,
		fox34SL130, fox34_130, fox36SL130,
	}
}
