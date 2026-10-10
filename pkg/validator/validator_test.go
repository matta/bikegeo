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
