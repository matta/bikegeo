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

func TestHandlebarCatalog(t *testing.T) {
	// Test DefaultHandlebar
	def := DefaultHandlebar()
	if def.ID != "persuader20_800" {
		t.Errorf("DefaultHandlebar ID: expected 'persuader20_800', got %q", def.ID)
	}

	// Test AllHandlebars returns non-empty slice
	all := AllHandlebars()
	if len(all) < 8 {
		t.Errorf("AllHandlebars: expected at least 8 entries, got %d", len(all))
	}

	// Test GetHandlebarPreset with exact and normalized keys
	presetKeys := []string{
		"persuader20_800",
		"persuader20_840",
		"sqlab3OX_16_30",
		"sqlab3OX_16_45",
		"sqlab3OX_12_30",
		"sqlab311_20_16_25",
		"sqlab311_20_16_50",
		"sqlab311_FLX_16_30",
	}

	for _, k := range presetKeys {
		spec, found := GetHandlebarPreset(k)
		if !found {
			t.Errorf("GetHandlebarPreset(%q) not found", k)
		}
		if spec.ID != k {
			t.Errorf("GetHandlebarPreset(%q) ID mismatch: got %q", k, spec.ID)
		}
	}

	// Test normalized lookup (dashes, case-insensitivity)
	spec, found := GetHandlebar("sqlab3ox-16-30")
	if !found || spec.ID != "sqlab3OX_16_30" {
		t.Errorf("GetHandlebar('sqlab3ox-16-30') failed, got %+v, found=%t", spec, found)
	}

	// Test substring lookup
	specSub, foundSub := GetHandlebar("Persuader 20° (840mm)")
	if !foundSub || specSub.ID != "persuader20_840" {
		t.Errorf("GetHandlebar substring match failed, got %+v, found=%t", specSub, foundSub)
	}

	// Test fallback on empty string returns default
	specEmpty, foundEmpty := GetHandlebar("")
	if !foundEmpty || specEmpty.ID != def.ID {
		t.Errorf("GetHandlebar('') expected default, got %+v", specEmpty)
	}

	// Test non-existent handlebar
	_, foundNone := GetHandlebar("non_existent_handlebar_12345")
	if foundNone {
		t.Errorf("Expected not found for non_existent_handlebar_12345")
	}
}
