package catalog

import (
	"fmt"

	"github.com/matta/bikegeo/internal/assembly"
)

// Saddle is one saddle model and its validated assembly.Saddle geometry.
type Saddle struct {
	ID   string
	Name string
	Part assembly.Saddle
}

func mustSaddle(dims assembly.SaddleDimensions) assembly.Saddle {
	s, err := assembly.NewSaddle(dims)
	if err != nil {
		panic(fmt.Sprintf("catalog: invalid saddle: %v", err))
	}
	return s
}

// DefaultSaddle returns the rider's SQlab 610 (docs/RIDER_FIT.md).
func DefaultSaddle() Saddle {
	return Saddle{
		ID:   "sqlab_610",
		Name: "SQlab 610 Ergolux Active 2.1",
		Part: mustSaddle(assembly.SaddleDimensions{
			RailCenterToBrpMm:      32.0,
			RailCenterToSitBonesMm: -48.0,
			// illustrative: 55 mm effective saddle height less the 10 mm measured SIT elevation
			RailToBrpHeightMm:   45.0,
			SitBonesElevationMm: 10.0,
			RailLengthMm:        60.0, // measured
		}),
	}
}
