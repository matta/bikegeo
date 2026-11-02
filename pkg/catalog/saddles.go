package catalog

import (
	"strings"

	"bikegeo/pkg/model"
)

// CommonSaddleCatalog holds standard compiled saddle models.
var CommonSaddleCatalog = []model.SaddleSpec{
	{
		ID:                     "sqlab-610-ergolux",
		Name:                   "SQlab 610 Ergolux Active 2.1",
		RailLengthMm:           60.0,
		RailCenterToBrpMm:      23.0,
		RailCenterToSitBonesMm: -50.0,
		SitBonesElevationMm:    10.0, // SQlab Ergolux step: rear sit bone platform is elevated 10mm above the 70mm BRP horizontal plane
	},
	{
		ID:                     "berthoud-aspin-open",
		Name:                   "Berthoud Aspin Open",
		RailLengthMm:           64.0,
		RailCenterToBrpMm:      43.5,
		RailCenterToSitBonesMm: -44.5,
		SitBonesElevationMm:    0.0, // Traditional flat/hammock leather touring profile (coplanar sit surface at BRP)
	},
	// SelleItaliaSlrBoostSuperflowL3 represents the 145x248mm L3 model with 7mm Manganese/TI316 rails.
	// Photogrammetrically derived from 2000x2000 orthogonal views (scale: ~6.38-6.40 px/mm).
	// Reference checkpoints from nose tip (248.0mm total length):
	//   - 70mm BRP line:       114.0mm from nose tip (molded underside BRP step & 70mm flank width)
	//   - Rail scale center:   131.7mm from nose tip (graduated scale spans [111.7mm, 151.7mm] from nose)
	//   - Sit bone support:    197.0mm from nose tip (widest 145mm flare / center of rear perforated zones)
	{
		ID:                     "selle-italia-slr-boost-superflow-l3",
		Name:                   "Selle Italia SLR Boost Superflow (L3 145mm)",
		RailLengthMm:           40.0,  // 8 x 5mm printed graduations between rear tick and front STOP mark [-20.0 to +20.0]
		RailCenterToBrpMm:      17.7,  // BRP (114.0mm from nose) is 17.7mm forward of rail scale center (131.7mm from nose)
		RailCenterToSitBonesMm: -65.3, // Sit bone line (197.0mm from nose) is 65.3mm rearward of rail scale center (83.0mm behind BRP)
		SitBonesElevationMm:    0.0,   // Flat racing profile (coplanar sit surface at BRP)
	},
}

// DefaultSaddle returns the default reference saddle (SQlab 610 Ergolux Active 2.1).
func DefaultSaddle() model.SaddleSpec {
	return CommonSaddleCatalog[0]
}

// AllSaddles returns all saddles in the compiled catalog.
func AllSaddles() []model.SaddleSpec {
	copied := make([]model.SaddleSpec, len(CommonSaddleCatalog))
	copy(copied, CommonSaddleCatalog)
	return copied
}

// GetSaddle looks up a saddle by exact ID or case-insensitive substring match.
func GetSaddle(idOrName string) (model.SaddleSpec, bool) {
	norm := strings.ToLower(strings.TrimSpace(idOrName))
	if norm == "" {
		return DefaultSaddle(), true
	}

	for _, s := range CommonSaddleCatalog {
		if strings.ToLower(s.ID) == norm {
			return s, true
		}
	}

	for _, s := range CommonSaddleCatalog {
		if strings.Contains(strings.ToLower(s.ID), norm) || strings.Contains(strings.ToLower(s.Name), norm) {
			return s, true
		}
	}

	return model.SaddleSpec{}, false
}
