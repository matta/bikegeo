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
		ExtendedRearRailMm:     7.5,
		RailCenterToBrpMm:      23.0,
		RailCenterToSitBonesMm: -50.0,
	},
	{
		ID:                     "berthoud-aspin-open",
		Name:                   "Berthoud Aspin Open",
		RailLengthMm:           64.0,
		ExtendedRearRailMm:     0.0,
		RailCenterToBrpMm:      43.5,
		RailCenterToSitBonesMm: -44.5,
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
