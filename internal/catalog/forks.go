package catalog

import (
	"fmt"
	"strconv"

	"bikegeo/internal/assembly"
)

// ForkVariant is one published axle-to-crown and rake configuration of a Fork (Q128).
type ForkVariant struct {
	StaticA2cMm float64
	RakeMm      float64
}

// Fork is a catalog fork model with its published A2C and rake variants (Q128).
type Fork struct {
	ID      string
	Name    string
	Travel  assembly.ForkTravel
	Steerer assembly.Steerer
	// RigidClass groups rigid forks by effective length for compatibility
	// (Q135); suspension forks have RigidClassNone.
	RigidClass RigidClass
	Variants   []ForkVariant
}

// RigidClass is a hand-assigned length class of commercial rigid forks (Q135).
type RigidClass string

const (
	RigidClassNone RigidClass = ""
	RigidClass440  RigidClass = "440"
	RigidClass470  RigidClass = "470"
	RigidClass485  RigidClass = "485"
	RigidClass495  RigidClass = "495"
)

// ValidFor reports whether c is a legal class for a fork of the given travel:
// a rigid fork has one of the four classes and a suspension fork has none.
func (c RigidClass) ValidFor(travel assembly.ForkTravel) bool {
	switch travel {
	case assembly.TravelRigid:
		switch c {
		case RigidClass440, RigidClass470, RigidClass485, RigidClass495:
			return true
		case RigidClassNone:
			return false
		default:
			return false
		}
	case assembly.Travel100, assembly.Travel120, assembly.Travel130:
		return c == RigidClassNone
	default:
		return false
	}
}

// standInRigidClasses returns the rigid classes a listed suspension fork of the
// given travel also unlocks (Q135). The stand-in is one way: a rigid fork never
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

// IsMultiVariant reports whether f has more than one published variant (Q131).
func (f Fork) IsMultiVariant() bool {
	return len(f.Variants) > 1
}

// VariantID returns the Q131 query identifier for variant v of f: bare f.ID
// on single-variant models, or "<f.ID>-<StaticA2cMm>x<RakeMm>" without
// trailing ".0" on multi-variant models.
func (f Fork) VariantID(v ForkVariant) string {
	if !f.IsMultiVariant() {
		return f.ID
	}
	return fmt.Sprintf("%s-%sx%s", f.ID, formatDim(v.StaticA2cMm), formatDim(v.RakeMm))
}

func formatDim(x float64) string {
	return strconv.FormatFloat(x, 'f', -1, 64)
}

// ForkOption is one concrete fork variant (model + A2C/rake) offered by a frame (Q128).
type ForkOption struct {
	Fork    Fork
	Variant ForkVariant
	// LowerCup is the typical cup this fork installs with on the offering
	// frame's head tube (Q136).
	LowerCup assembly.LowerCupSetup
}

// ID returns the concrete Q131 query identifier for o.
func (o ForkOption) ID() string {
	return o.Fork.VariantID(o.Variant)
}

// Name returns the display name for o, appending "(<A2C>/<Rake>)" for multi-variant models.
func (o ForkOption) Name() string {
	if !o.Fork.IsMultiVariant() {
		return o.Fork.Name
	}
	return fmt.Sprintf("%s (%s/%s)", o.Fork.Name, formatDim(o.Variant.StaticA2cMm), formatDim(o.Variant.RakeMm))
}

// Spec returns the installed fork's assembly specification.
func (o ForkOption) Spec() assembly.ForkSpec {
	return assembly.ForkSpec{
		StaticA2cMm: o.Variant.StaticA2cMm,
		Travel:      o.Fork.Travel,
		RakeMm:      o.Variant.RakeMm,
		Steerer:     o.Fork.Steerer,
	}
}

// The 16 fork models and their published variants (Q128, Q129, Q133).
var (
	sunliner440 = Fork{
		ID: "sunliner_440", Name: "Tumbleweed Chromoly Steel Unicrown 440mm", Steerer: assembly.SteererStraight118, RigidClass: RigidClass440,
		Variants: []ForkVariant{{StaticA2cMm: 440, RakeMm: 55}},
	} // catalog
	surlyEcr468 = Fork{
		ID: "surly_ecr_468", Name: "Surly ECR 4130 Chromoly 468mm", Steerer: assembly.SteererStraight118, RigidClass: RigidClass470,
		Variants: []ForkVariant{{StaticA2cMm: 468, RakeMm: 43}},
	} // catalog
	firestarterDeluxe483 = Fork{
		ID: "firestarter_deluxe_483", Name: "Salsa Firestarter / Cutthroat Carbon Deluxe 483mm", Steerer: assembly.SteererTapered, RigidClass: RigidClass485,
		Variants: []ForkVariant{{StaticA2cMm: 483, RakeMm: 51}},
	} // catalog
	surlyKrampus483 = Fork{
		ID: "surly_krampus_483", Name: "Surly Krampus 4130 Chromoly 483mm", Steerer: assembly.SteererStraight118, RigidClass: RigidClass485,
		Variants: []ForkVariant{{StaticA2cMm: 483, RakeMm: 47}},
	} // catalog
	sklarSteel485 = Fork{
		ID: "sklar_steel_485", Name: "Sklar Chromoly Steel 485mm", Steerer: assembly.SteererStraight118, RigidClass: RigidClass485,
		Variants: []ForkVariant{{StaticA2cMm: 485, RakeMm: 52}},
	} // catalog
	enveMountainBoost = Fork{
		ID: "enve_mountain_boost", Name: "ENVE Mountain Carbon Boost", Steerer: assembly.SteererTapered, RigidClass: RigidClass495,
		Variants: []ForkVariant{{StaticA2cMm: 490, RakeMm: 44}, {StaticA2cMm: 492, RakeMm: 52}},
	} // published (S5)
	lithicCarbonMountain = Fork{
		ID: "lithic_carbon_mountain", Name: "Lithic Carbon Mountain", Steerer: assembly.SteererTapered, RigidClass: RigidClass495,
		Variants: []ForkVariant{{StaticA2cMm: 495, RakeMm: 44}, {StaticA2cMm: 498, RakeMm: 51.5}},
	} // published (S6)
	sid100 = Fork{
		ID: "sid_100", Name: "RockShox SID 100mm", Travel: assembly.Travel100, Steerer: assembly.SteererTapered,
		Variants: []ForkVariant{{StaticA2cMm: 506, RakeMm: 44}, {StaticA2cMm: 506, RakeMm: 51}},
	} // catalog
	fox34SL120 = Fork{
		ID: "fox34_sl_120", Name: "Fox 34 SL 120mm", Travel: assembly.Travel120, Steerer: assembly.SteererTapered,
		Variants: []ForkVariant{{StaticA2cMm: 530.5, RakeMm: 44}},
	} // published (fork_fox_34_sl.md)
	sid120 = Fork{
		ID: "sid_120", Name: "RockShox SID 120mm", Travel: assembly.Travel120, Steerer: assembly.SteererTapered,
		Variants: []ForkVariant{{StaticA2cMm: 530, RakeMm: 44}, {StaticA2cMm: 534, RakeMm: 44}},
	} // catalog
	pike120 = Fork{
		ID: "pike_120", Name: "RockShox Pike 120mm", Travel: assembly.Travel120, Steerer: assembly.SteererTapered,
		Variants: []ForkVariant{{StaticA2cMm: 531, RakeMm: 44}},
	} // catalog
	fox34_120 = Fork{
		ID: "fox34_120", Name: "Fox Float 34 120mm", Travel: assembly.Travel120, Steerer: assembly.SteererTapered,
		Variants: []ForkVariant{{StaticA2cMm: 534, RakeMm: 44}, {StaticA2cMm: 534, RakeMm: 51}},
	} // published (fork_fox_34.md)
	fox36SL120 = Fork{
		ID: "fox36_sl_120", Name: "Fox 36 SL 120mm", Travel: assembly.Travel120, Steerer: assembly.SteererTapered,
		Variants: []ForkVariant{{StaticA2cMm: 536, RakeMm: 44}},
	} // published (fork_fox_36_sl.md)
	fox34SL130 = Fork{
		ID: "fox34_sl_130", Name: "Fox 34 SL 130mm", Travel: assembly.Travel130, Steerer: assembly.SteererTapered,
		Variants: []ForkVariant{{StaticA2cMm: 540.5, RakeMm: 44}},
	} // published (fork_fox_34_sl.md)
	fox34_130 = Fork{
		ID: "fox34_130", Name: "Fox Float 34 130mm", Travel: assembly.Travel130, Steerer: assembly.SteererTapered,
		Variants: []ForkVariant{{StaticA2cMm: 544, RakeMm: 44}, {StaticA2cMm: 544, RakeMm: 51}},
	} // published (fork_fox_34.md)
	fox36SL130 = Fork{
		ID: "fox36_sl_130", Name: "Fox 36 SL 130mm", Travel: assembly.Travel130, Steerer: assembly.SteererTapered,
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

// ForkByID returns the catalog fork model with the exact ID.
func ForkByID(id string) (Fork, bool) {
	for _, f := range Forks() {
		if f.ID == id {
			return f, true
		}
	}
	return Fork{}, false
}
