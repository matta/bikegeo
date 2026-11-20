package catalog

import "bikegeo/internal/reboot/assembly"

// Saddle is one saddle model and its setup.
type Saddle struct {
	ID    string
	Name  string
	Setup assembly.SaddleSetup // RailOffsetMm and RailPitchDeg are 0
}

// DefaultSaddle returns the rider's SQlab 610 (docs/RIDER_FIT.md).
func DefaultSaddle() Saddle {
	return Saddle{
		ID:   "sqlab_610",
		Name: "SQlab 610 Ergolux Active 2.1",
		Setup: assembly.SaddleSetup{
			RailCenterToBrpMm:      32.0,
			RailCenterToSitBonesMm: -48.0,
			// illustrative: 55 mm effective saddle height less the 10 mm measured SIT elevation (Q75b)
			RailToBrpHeightMm:   45.0,
			SitBonesElevationMm: 10.0,
			RailLengthMm:        60.0, // measured
			RailPitchDeg:        0.0,
			RailOffsetMm:        0.0,
		},
	}
}
