package validator_test

import (
	"testing"

	"bikegeo/pkg/model"
	"bikegeo/pkg/validator"
)

func TestValidateBinaryHavok(t *testing.T) {
	binaryHavok := model.BikeGeometry{
		ID:           "binary-havok-m",
		Manufacturer: "Binary",
		Model:        "Havok",
		Size:         "M",
		Reach:        398.0,
		Stack:        656.0,
		HtaDeg:       69.0,
		StaDeg:       74.0,
		Htl:          150.0,
		BbDrop:       70.0,
		ForkRake:     44.0,
		Wheelbase:    1150.0,
		ForkA2c:      521.0,
		CupStack:     12.0,
	}

	report := validator.ValidateBikeGeometry(binaryHavok)
	if !report.IsValid {
		t.Errorf("Expected valid bike geometry, got invalid")
	}
	if report.GeometricA2c != 482.6 {
		t.Errorf("Expected GeometricA2c 482.6, got %.1f", report.GeometricA2c)
	}
	if report.TotalStructuralA2c != 494.6 {
		t.Errorf("Expected TotalStructuralA2c 494.6, got %.1f", report.TotalStructuralA2c)
	}
}

func TestValidateMissingRequired(t *testing.T) {
	incomplete := model.BikeGeometry{
		ID:    "incomplete",
		Reach: 450.0,
		// missing stack, hta, etc.
	}

	report := validator.ValidateBikeGeometry(incomplete)
	if report.IsValid {
		t.Errorf("Expected invalid for missing fields, got valid")
	}
}

func TestValidateCrossConfig_ChumbaSendero(t *testing.T) {
	sendero := model.FrameModel{
		ID:             "chumba-sendero",
		Manufacturer:   "Chumba",
		Model:          "Sendero",
		Sizes:          []string{"S", "M", "L", "XL"},
		DefaultSize:    "L",
		HeadsetCupType: model.CupEC44,
		CupStack:       12.0,
		Configurations: []model.FrameConfiguration{
			{
				ID:           "fork-120",
				Name:         "120mm Stock",
				IsBaseline:   true,
				ForkTravelMm: 120,
				ForkA2c:      531,
				Geometries: map[string]model.SizeGeometry{
					"L": {Reach: 475, Stack: 645, HtaDeg: 67.0, StaDeg: 74.5, Htl: 125, BbDrop: 60, ForkRake: 44, Wheelbase: 1200, ChainstayLength: 430},
				},
			},
			{
				ID:           "fork-130",
				Name:         "130mm Trail",
				IsBaseline:   false,
				ForkTravelMm: 130,
				ForkA2c:      541,
				Geometries: map[string]model.SizeGeometry{
					"L": {Reach: 470, Stack: 649, HtaDeg: 66.5, StaDeg: 74.0, Htl: 125, BbDrop: 57, ForkRake: 44, Wheelbase: 1203, ChainstayLength: 430},
				},
			},
		},
	}

	crossVal := validator.ValidateCrossConfig(sendero, "L", "fork-130")
	if crossVal == nil {
		t.Fatalf("Expected non-nil CrossConfigValidation")
	}
	if !crossVal.IsCoherent {
		t.Errorf("Expected coherent cross-config validation, got max discrepancy: %.2f", crossVal.MaxDiscrepancy)
	}
	if crossVal.DeltaA2c != 10.0 {
		t.Errorf("Expected DeltaA2c 10.0, got %.1f", crossVal.DeltaA2c)
	}
}

func TestValidateCrossConfig_Discrepancy(t *testing.T) {
	badFrame := model.FrameModel{
		ID:             "flawed-test-frame",
		Manufacturer:   "Flawed",
		Model:          "Frame",
		Sizes:          []string{"L"},
		DefaultSize:    "L",
		HeadsetCupType: model.CupZS44,
		Configurations: []model.FrameConfiguration{
			{
				ID:           "baseline",
				IsBaseline:   true,
				ForkTravelMm: 120,
				ForkA2c:      530,
				Geometries: map[string]model.SizeGeometry{
					"L": {Reach: 475, Stack: 630, HtaDeg: 66.0, StaDeg: 75.0, Htl: 110, BbDrop: 60, ForkRake: 44, Wheelbase: 1200, ChainstayLength: 430},
				},
			},
			{
				ID:           "alternate-bad-cad",
				IsBaseline:   false,
				ForkTravelMm: 140,
				ForkA2c:      550, // +20mm fork
				Geometries: map[string]model.SizeGeometry{
					// Deliberately impossible CAD: Stack decreased instead of increased!
					"L": {Reach: 475, Stack: 610, HtaDeg: 66.0, StaDeg: 75.0, Htl: 110, BbDrop: 60, ForkRake: 44, Wheelbase: 1200, ChainstayLength: 430},
				},
			},
		},
	}

	crossVal := validator.ValidateCrossConfig(badFrame, "L", "alternate-bad-cad")
	if crossVal == nil {
		t.Fatalf("Expected non-nil CrossConfigValidation")
	}
	if crossVal.IsCoherent {
		t.Errorf("Expected flawed config to fail coherence check")
	}
	if crossVal.MaxDiscrepancy < 10.0 {
		t.Errorf("Expected large discrepancy, got %.1f", crossVal.MaxDiscrepancy)
	}
}

