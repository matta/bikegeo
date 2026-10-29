package catalog

import (
	"testing"

	"bikegeo/pkg/model"
)

func TestGetCatalogFork(t *testing.T) {
	// Backup original catalog and restore after test
	originalCatalog := CommonForkCatalog
	t.Cleanup(func() {
		CommonForkCatalog = originalCatalog
	})

	// Set up dummy catalog
	CommonForkCatalog = []model.ForkCatalogEntry{
		{ID: "fox36_factory", Name: "Fox 36 Factory", TravelMm: 160},
		{ID: "lyrik_ult", Name: "RockShox Lyrik Ultimate", TravelMm: 150},
		{ID: "rigid_1", Name: "Generic Rigid", TravelMm: 0},
	}

	tests := []struct {
		name      string
		searchID  string
		wantID    string
		wantFound bool
	}{
		{
			name:      "Exact ID match",
			searchID:  "fox36_factory",
			wantID:    "fox36_factory",
			wantFound: true,
		},
		{
			name:      "Case-insensitive ID match",
			searchID:  "FOX36_Factory",
			wantID:    "fox36_factory",
			wantFound: true,
		},
		{
			name:      "Exact Name match",
			searchID:  "RockShox Lyrik Ultimate",
			wantID:    "lyrik_ult",
			wantFound: true,
		},
		{
			name:      "Case-insensitive Name match",
			searchID:  "rockshox lyrik ultimate",
			wantID:    "lyrik_ult",
			wantFound: true,
		},
		{
			name:      "Non-existent fork",
			searchID:  "not_a_fork",
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, found := GetCatalogFork(tt.searchID)
			if found != tt.wantFound {
				t.Errorf("GetCatalogFork() found = %v, want %v", found, tt.wantFound)
			}
			if found && got.ID != tt.wantID {
				t.Errorf("GetCatalogFork() got ID = %v, want %v", got.ID, tt.wantID)
			}
		})
	}
}
