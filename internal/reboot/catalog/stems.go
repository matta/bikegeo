package catalog

import (
	"fmt"
	"math"
	"sort"

	"bikegeo/internal/reboot/assembly"
	"bikegeo/internal/reboot/pose"
)

// LengthAvailability is how widely trail/XC-rated stems of one length are sold (stems.yaml).
type LengthAvailability string

const (
	LengthAbundant LengthAvailability = "abundant"
	LengthCommon   LengthAvailability = "common"
	LengthLimited  LengthAvailability = "limited"
	LengthSparse   LengthAvailability = "sparse"
)

// AngleAvailability is how widely one stem angle is sold at a given length (stems.yaml).
type AngleAvailability string

const (
	AngleDominant AngleAvailability = "dominant"
	AngleCommon   AngleAvailability = "common"
	AngleNiche    AngleAvailability = "niche"
)

// Stem is one concrete stem: a length and a signed angle (negative drops,
// matching assembly.StemSetup.AngleDeg), its availability, and its Q58
// composite 0–100 score.
type Stem struct {
	LengthMm           float64
	AngleDeg           pose.Deg
	LengthAvailability LengthAvailability
	AngleAvailability  AngleAvailability
	Score              float64
}

// stemAngle is one entry of a stems.yaml angle list. Reversible entries
// ("+/-X") are flip-flop stems sold as both −X° and +X°.
type stemAngle struct {
	Deg        pose.Deg
	Reversible bool
}

func flip(deg pose.Deg) stemAngle  { return stemAngle{Deg: deg, Reversible: true} }
func fixed(deg pose.Deg) stemAngle { return stemAngle{Deg: deg} }

// stemLength is one stems_by_length entry of docs/reboot/stems.yaml (catalog).
type stemLength struct {
	LengthMm                float64
	Availability            LengthAvailability
	Dominant, Common, Niche []stemAngle
}

var stemLengths = []stemLength{
	{
		LengthMm:     40,
		Availability: LengthAbundant,
		Dominant:     []stemAngle{fixed(0)},
		Common:       []stemAngle{flip(6)},
	},
	{
		LengthMm:     50,
		Availability: LengthAbundant,
		Dominant:     []stemAngle{fixed(0)},
		Common:       []stemAngle{flip(6)},
		Niche:        []stemAngle{flip(5), fixed(-17), fixed(-20)},
	},
	{
		LengthMm:     60,
		Availability: LengthAbundant,
		Dominant:     []stemAngle{fixed(0), flip(6)},
		Common:       []stemAngle{flip(7)},
		Niche:        []stemAngle{flip(12), fixed(-12), flip(17), fixed(-17), fixed(-20), flip(30)},
	},
	{
		LengthMm:     70,
		Availability: LengthAbundant,
		Dominant:     []stemAngle{flip(6), flip(7), fixed(0)},
		Common:       []stemAngle{flip(10), flip(17)},
		Niche:        []stemAngle{flip(12), fixed(-12), flip(15), fixed(-17), fixed(-20), flip(30)},
	},
	{
		LengthMm:     80,
		Availability: LengthAbundant,
		Dominant:     []stemAngle{flip(6), flip(7)},
		Common:       []stemAngle{fixed(0), flip(10), flip(17)},
		Niche:        []stemAngle{flip(12), fixed(-12), fixed(-17), fixed(-20), flip(30)},
	},
	{
		LengthMm:     90,
		Availability: LengthCommon,
		Dominant:     []stemAngle{flip(6), flip(7)},
		Common:       []stemAngle{fixed(0), flip(10), flip(17)},
		Niche:        []stemAngle{fixed(-12), flip(15), fixed(-17), fixed(-20), flip(30)},
	},
	{
		LengthMm:     100,
		Availability: LengthCommon,
		Dominant:     []stemAngle{flip(6), flip(7)},
		Common:       []stemAngle{fixed(0), flip(10), flip(17)},
		Niche:        []stemAngle{fixed(-12), fixed(-17), fixed(-20), flip(30)},
	},
	{
		LengthMm:     110,
		Availability: LengthLimited,
		Dominant:     []stemAngle{flip(6), flip(7)},
		Common:       []stemAngle{fixed(0), flip(10), flip(17)},
		Niche:        []stemAngle{fixed(-12), fixed(-20), flip(30)},
	},
	{
		LengthMm:     120,
		Availability: LengthSparse,
		Dominant:     []stemAngle{flip(6), flip(7)},
		Common:       []stemAngle{flip(10), flip(17)},
		Niche:        []stemAngle{fixed(0), flip(12), fixed(-12), fixed(-20), flip(30)},
	},
}

// Q58 composite stem score sub-attribute weights (sum 100).
const (
	stemLengthAvailabilityWeight = 35.0
	stemAngleAvailabilityWeight  = 30.0
	stemLengthRoomWeight         = 20.0
	stemAngleRoomWeight          = 15.0
)

// Stems returns the 107 stems from docs/reboot/stems.yaml, ordered by length
// and then by signed angle, both ascending.
func Stems() []Stem {
	var out []Stem
	for _, sl := range stemLengths {
		byAngle := make(map[pose.Deg]AngleAvailability)
		addTier := func(tier AngleAvailability, entries []stemAngle) {
			for _, e := range entries {
				if e.Reversible {
					for _, deg := range []pose.Deg{-e.Deg, e.Deg} {
						if _, exists := byAngle[deg]; !exists {
							byAngle[deg] = tier
						}
					}
				} else {
					if _, exists := byAngle[e.Deg]; !exists {
						byAngle[e.Deg] = tier
					}
				}
			}
		}
		addTier(AngleDominant, sl.Dominant)
		addTier(AngleCommon, sl.Common)
		addTier(AngleNiche, sl.Niche)

		hasMs := false
		var minMs, maxMs pose.Deg
		for deg, tier := range byAngle {
			if tier == AngleDominant || tier == AngleCommon {
				if !hasMs {
					minMs, maxMs = deg, deg
					hasMs = true
				} else {
					if deg < minMs {
						minMs = deg
					}
					if deg > maxMs {
						maxMs = deg
					}
				}
			}
		}
		if !hasMs {
			panic(fmt.Sprintf("catalog: stem length %.0f mm has no dominant or common angles", sl.LengthMm))
		}

		for deg, tier := range byAngle {
			out = append(out, Stem{
				LengthMm:           sl.LengthMm,
				AngleDeg:           deg,
				LengthAvailability: sl.Availability,
				AngleAvailability:  tier,
				Score:              stemScore(sl.LengthMm, sl.Availability, deg, tier, minMs, maxMs),
			})
		}
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].LengthMm != out[j].LengthMm {
			return out[i].LengthMm < out[j].LengthMm
		}
		return out[i].AngleDeg < out[j].AngleDeg
	})
	return out
}

// stemScore returns the Q58 composite 0–100 score.
func stemScore(lengthMm float64, length LengthAvailability, angle pose.Deg, tier AngleAvailability, minMs, maxMs pose.Deg) float64 {
	var lenAvail float64
	switch length {
	case LengthAbundant:
		lenAvail = 100
	case LengthCommon:
		lenAvail = 80
	case LengthLimited:
		lenAvail = 35
	case LengthSparse:
		lenAvail = 0
	default:
		panic(fmt.Sprintf("catalog: unknown stem length availability %q", length))
	}

	var angAvail float64
	switch tier {
	case AngleDominant:
		angAvail = 100
	case AngleCommon:
		angAvail = 80
	case AngleNiche:
		angAvail = 0
	default:
		panic(fmt.Sprintf("catalog: unknown stem angle availability %q", tier))
	}

	lenRoom := 100.0 * clamp01(min(lengthMm-40.0, 100.0-lengthMm)/20.0)
	angRoom := 100.0 * clamp01(float64(min(angle-minMs, maxMs-angle))/10.0)

	return (stemLengthAvailabilityWeight*lenAvail +
		stemAngleAvailabilityWeight*angAvail +
		stemLengthRoomWeight*lenRoom +
		stemAngleRoomWeight*angRoom) / 100.0
}

func clamp01(x float64) float64 {
	if x < 0 {
		return 0
	}
	if x > 1 {
		return 1
	}
	return x
}

// StemOptions returns the 107 catalog stems in assembly.StemOption form for
// build fitting (Q97, Q100).
func StemOptions() []assembly.StemOption {
	stems := Stems()
	out := make([]assembly.StemOption, len(stems))
	for i, s := range stems {
		out[i] = assembly.StemOption{
			LengthMm: s.LengthMm,
			AngleDeg: s.AngleDeg,
			Score:    s.Score,
		}
	}
	return out
}

// SpacerStepMm is the steerer spacer increment build fitting enumerates (Q82).
const SpacerStepMm = 5.0

// DefaultStemSetup returns the baseline stem dimensions used to place builds.
// UpperStackMm (11 mm) and MaxSpacersMm (30 mm) are Q82 constants; ClampHeightMm
// (40 mm) and the continuous-fit length range (40–130 mm) are illustrative.
// Build fitting overwrites LengthMm, AngleDeg, and SpacersMm from StemOptions().
func DefaultStemSetup() assembly.StemSetup {
	return assembly.StemSetup{
		UpperStackMm:  assembly.DefaultUpperStackMm,
		SpacersMm:     20.0,
		ClampHeightMm: 40.0,
		LengthMm:      60.0,
		AngleDeg:      -6.0,
		MaxSpacersMm:  assembly.DefaultMaxSpacersMm,
		MinLengthMm:   40.0,
		MaxLengthMm:   130.0,
	}
}

// StemByDimensions returns the catalog stem matching lengthMm and angleDeg
// within 1e-9, or false if the stem is off-catalog or non-finite (Q110).
func StemByDimensions(lengthMm float64, angleDeg pose.Deg) (Stem, bool) {
	if math.IsNaN(lengthMm) || math.IsInf(lengthMm, 0) ||
		math.IsNaN(float64(angleDeg)) || math.IsInf(float64(angleDeg), 0) {
		return Stem{}, false
	}
	const tol = 1e-9
	for _, s := range Stems() {
		if math.Abs(s.LengthMm-lengthMm) <= tol && math.Abs(float64(s.AngleDeg-angleDeg)) <= tol {
			return s, true
		}
	}
	return Stem{}, false
}
