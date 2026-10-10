package canonicalizer_test

import (
	"testing"

	"bikegeo/pkg/canonicalizer"
	"bikegeo/pkg/model"
)

func TestCanonicalizeBikes(t *testing.T) {
	bikes := []model.BikeGeometry{
		{
			ID:           "test-bike",
			Manufacturer: "Test",
			Model:        "HT",
			Size:         "L",
			Reach:        450,
			Stack:        630,
			HtaDeg:       66.0,
			StaDeg:       75.0,
			Htl:          110,
			BbDrop:       60,
			ForkRake:     44,
			Wheelbase:    1180,
			ForkA2c:      541,
		},
	}

	config := model.CanonicalConfig{
		TargetA2c:  515.0,
		CupStack:   2.0,
		SeatHeight: 760.0,
		SagPercent: 20.0,
	}

	rows := canonicalizer.CanonicalizeBikes(bikes, config)
	if len(rows) != 1 {
		t.Fatalf("Expected 1 row, got %d", len(rows))
	}

	row := rows[0]
	if row.Canonical.ActiveReach <= 0 {
		t.Errorf("Expected positive active reach, got %.1f", row.Canonical.ActiveReach)
	}
	if row.Baseline.Reach != 450 {
		t.Errorf("Expected baseline reach 450, got %.1f", row.Baseline.Reach)
	}
}

func TestResolveForkSpec(t *testing.T) {
	spec := model.ForkSpec{ID: "fox34_130"}
	resolved := canonicalizer.ResolveForkSpec(spec)
	if resolved.A2c != 541 {
		t.Errorf("Expected A2c 541, got %.1f", resolved.A2c)
	}
	if resolved.Rake != 44 {
		t.Errorf("Expected Rake 44, got %.1f", resolved.Rake)
	}
	if resolved.TravelMm != 130 {
		t.Errorf("Expected TravelMm 130, got %.1f", resolved.TravelMm)
	}
	if !resolved.IsInferred {
		t.Errorf("Expected IsInferred to be true")
	}
}
