package catalog

import (
	"fmt"

	"bikegeo/internal/assembly"
)

// Seatpost is one seatpost the workbench can install. Its insertion comes from
// the workbench state (Q27).
type Seatpost struct {
	ID   string
	Name string
	Spec assembly.SeatpostSpec
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

// genericSeatpost is an illustrative generic post with the Ritchey-based
// length and insertion limits above.
func genericSeatpost(id string, setbackMm float64) Seatpost {
	return Seatpost{
		ID:   id,
		Name: fmt.Sprintf("Generic %.0f mm setback post", setbackMm),
		Spec: assembly.SeatpostSpec{
			TotalLengthMm:     genericPostLengthMm,
			MaxInsertionMm:    genericPostMaxInsertionMm,
			MinInsertionMm:    genericPostMinInsertionMm,
			SetbackMm:         setbackMm,
			RailClampLengthMm: 30.0, // measured
		},
	}
}

// Seatposts returns the three generic setback posts and the Redshift ShockStop
// PRO (Q28, Q75c).
func Seatposts() []Seatpost {
	return []Seatpost{
		genericSeatpost("setback_0", 0),
		genericSeatpost("setback_15", 15),
		genericSeatpost("setback_25", 25),
		// owner-reported 2026-11-19: 12 mm setback when sagged (Q28). The fit
		// report was transferred with this post unsagged (docs/RIDER_FIT.md).
		{ID: "redshift_shockstop_pro", Name: "Redshift ShockStop PRO (12 mm setback, sagged)", Spec: assembly.SeatpostSpec{
			TotalLengthMm:     350.0, // catalog
			MaxInsertionMm:    250.0, // illustrative: 100 mm linkage stack height (Q75c)
			MinInsertionMm:    100.0, // catalog: matches 100 mm Cane Creek ST27316 shim (Q75c)
			SetbackMm:         12.0,  // owner-reported (sagged, Q28)
			RailClampLengthMm: 30.0,  // illustrative (Q75c)
		}},
	}
}

// DefaultSeatpostID is the default seatpost for / and /bike (Q62, Q78).
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
