package catalog

import (
	"testing"

	"bikegeo/pkg/model"
)

func TestGetStandardForkForTravel(t *testing.T) {
	// Backup original catalog and restore after test
	originalCatalog := CommonForkCatalog
	defer func() { CommonForkCatalog = originalCatalog }()

	// Setup mock catalog
	CommonForkCatalog = []model.ForkCatalogEntry{
		{
			ID:       "fork-100-std",
			Name:     "Standard 100mm",
			TravelMm: 100.0,
			Standard: true,
		},
		{
			ID:       "fork-100-nonstd",
			Name:     "Non-Standard 100mm",
			TravelMm: 100.0,
			Standard: false,
		},
		{
			ID:       "fork-120-nonstd1",
			Name:     "Non-Standard 120mm 1",
			TravelMm: 120.0,
			Standard: false,
		},
		{
			ID:       "fork-120-nonstd2",
			Name:     "Non-Standard 120mm 2",
			TravelMm: 120.0,
			Standard: false,
		},
		{
			ID:       "fork-130.05-std",
			Name:     "Standard 130.05mm",
			TravelMm: 130.05,
			Standard: true,
		},
	}

	tests := []struct {
		name         string
		travelMm     float64
		expectedID   string
		expectedBool bool
	}{
		{
			name:         "Exact match standard fork",
			travelMm:     100.0,
			expectedID:   "fork-100-std",
			expectedBool: true,
		},
		{
			name:         "Match standard fork with slight float difference",
			travelMm:     100.05,
			expectedID:   "fork-100-std",
			expectedBool: true,
		},
		{
			name:         "Fallback to non-standard fork when no standard exists",
			travelMm:     120.0,
			expectedID:   "fork-120-nonstd1", // should pick first matching
			expectedBool: true,
		},
		{
			name:         "Match standard fork with another float difference",
			travelMm:     130.0,
			expectedID:   "fork-130.05-std", // difference is 0.05 < 0.1
			expectedBool: true,
		},
		{
			name:         "No match found",
			travelMm:     150.0,
			expectedID:   "",
			expectedBool: false,
		},
		{
			name:         "No match due to difference >= 0.1",
			travelMm:     130.2, // 130.2 - 130.05 = 0.15 > 0.1
			expectedID:   "",
			expectedBool: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fork, found := GetStandardForkForTravel(tt.travelMm)

			if found != tt.expectedBool {
				t.Errorf("expected found: %v, got: %v", tt.expectedBool, found)
			}

			if found && fork.ID != tt.expectedID {
				t.Errorf("expected fork ID: %s, got: %s", tt.expectedID, fork.ID)
			}
		})
	}
}
