package catalog

import (
	"fmt"

	"github.com/matta/bikegeo/internal/assembly"
	"github.com/matta/bikegeo/internal/bounds"
	"github.com/matta/bikegeo/internal/pose"
)

// SeatpostPart is the validated seatpost contract carried by catalog.Seatpost
// (satisfied by assembly.RigidPost and assembly.LinkagePost).
type SeatpostPart interface {
	TotalLengthMm() float64
	InsertionLimits() bounds.BoundedRange
	RailClamp(state bounds.LoadState) assembly.RailClampPort
}

// Seatpost is one seatpost the workbench can install. Its insertion comes from
// the workbench state.
type Seatpost struct {
	ID   string
	Name string
	Part SeatpostPart
}

// TotalLengthMm returns the post's nominal length from bottom tip to clamp pivot.
func (p Seatpost) TotalLengthMm() float64 {
	return p.Part.TotalLengthMm()
}

// SetbackMm returns the post's dynamic rearward clamp offset from the shaft centerline.
func (p Seatpost) SetbackMm() float64 {
	return p.Part.RailClamp(bounds.Dynamic).CradleInParent.OriginInDst().Y()
}

// RailClampLengthMm returns the fore-aft rail clamping footprint.
func (p Seatpost) RailClampLengthMm() float64 {
	return p.Part.RailClamp(bounds.Static).ClampLengthMm
}

const (
	// Model the generic zero-offset post with the Ritchey Comp Trail Zero's
	// nominal length and minimum insertion. The workbench uses a simplified clamp.
	// https://ritcheylogic.com/bike/seatposts/comp-trail-zero-seatpost
	// illustrative: the 365 mm maximum insertion (35 mm head stack height)
	genericPostLengthMm       = 400.0
	genericPostMaxInsertionMm = 365.0
	genericPostMinInsertionMm = 100.0
)

func mustRigidPost(total, minIns, maxIns, setback, clampLen float64) assembly.RigidPost {
	p, err := assembly.NewRigidPost(total, minIns, maxIns, setback, clampLen)
	if err != nil {
		panic(fmt.Sprintf("catalog: invalid rigid seatpost: %v", err))
	}
	return p
}

func mustLinkagePost(total, minIns, maxIns, staticSetback, clampLen float64, sagStep pose.Vector) assembly.LinkagePost {
	p, err := assembly.NewLinkagePost(total, minIns, maxIns, staticSetback, clampLen, sagStep)
	if err != nil {
		panic(fmt.Sprintf("catalog: invalid linkage seatpost: %v", err))
	}
	return p
}

// genericSeatpost is an illustrative generic post with the Ritchey-based
// length and insertion limits above.
func genericSeatpost(id string, setbackMm float64) Seatpost {
	return Seatpost{
		ID:   id,
		Name: fmt.Sprintf("Generic %.0f mm setback post", setbackMm),
		Part: mustRigidPost(
			genericPostLengthMm,
			genericPostMinInsertionMm,
			genericPostMaxInsertionMm,
			setbackMm,
			30.0, // measured
		),
	}
}

// Seatposts returns the three generic setback posts and the Redshift ShockStop PRO.
func Seatposts() []Seatpost {
	return []Seatpost{
		genericSeatpost("setback_0", 0),
		genericSeatpost("setback_15", 15),
		genericSeatpost("setback_25", 25),
		// owner-reported 2026-11-19: 12 mm setback when sagged. Kept at 12 mm
		// with zero sagStep during this behavior-preserving refactor so all
		// existing numerical pins match; Milestone 6 switches to 7 mm static +
		// (-6, +5) mm sagStep after the Fenrir verification check.
		{
			ID:   "redshift_shockstop_pro",
			Name: "Redshift ShockStop PRO (12 mm setback, sagged)",
			Part: mustLinkagePost(
				350.0, // catalog
				100.0, // catalog: matches 100 mm Cane Creek ST27316 shim
				250.0, // illustrative: 100 mm linkage stack height
				12.0,  // owner-reported (sagged)
				30.0,  // illustrative
				pose.NewVector(assembly.SeatpostLocal, 0, 0),
			),
		},
	}
}

// DefaultSeatpostID is the default seatpost for / and /bike.
const DefaultSeatpostID = "redshift_shockstop_pro"

// SeatpostByID returns the seatpost with the exact ID.
func SeatpostByID(id string) (Seatpost, bool) {
	for _, p := range Seatposts() {
		if p.ID == id {
			return p, true
		}
	}
	return Seatpost{}, false
}
