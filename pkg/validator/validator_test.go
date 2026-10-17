package validator_test

import (
	"testing"

	"bikegeo/pkg/model"
	"bikegeo/pkg/validator"
)

func TestValidateBinaryHavok(t *testing.T) {
	binaryHavok := model.BikeGeometry{
		ID:                 "binary-havok-l",
		Manufacturer:       "Binary",
		Model:              "Havok",
		Size:               "L",
		Reach:              398.0,
		Stack:              656.0,
		HtaDeg:             69.0,
		StaDeg:             74.0,
		Htl:                150.0,
		BbDrop:             70.0,
		ForkRake:           45.0,
		Wheelbase:          1111.0,
		ForkA2c:            485.0,
		HeadsetCupType:     model.CupEC44,
		GeoHeadsetSpacerMm: model.Float64Ptr(10.0),
	}

	report := validator.ValidateBikeGeometry(binaryHavok)
	if !report.IsValid {
		t.Errorf("Expected valid bike geometry, got invalid: %+v", report.Issues)
	}
	if report.GeometricA2c != 485.0 {
		t.Errorf("Expected GeometricA2c 485.0, got %.1f", report.GeometricA2c)
	}
	if report.TotalStructuralA2c != 495.0 {
		t.Errorf("Expected TotalStructuralA2c 495.0, got %.1f", report.TotalStructuralA2c)
	}
}

func TestValidateOtsoFenrirSpacerNoAutoDetection(t *testing.T) {
	// With explicit geo_headset_spacer_mm = 0 and rear Tuning Chip geometry, Fenrir is valid within ≤1.0mm default slop
	fenrirZero := model.BikeGeometry{
		ID:                 "otso-fenrir-stainless-l",
		Manufacturer:       "Otso",
		Model:              "Fenrir Stainless",
		Size:               "L",
		Reach:              428.0,
		Stack:              642.0,
		HtaDeg:             68.25,
		StaDeg:             75.25,
		Htl:                140.0,
		BbDrop:             72.0,
		ForkRake:           54.0,
		Wheelbase:          1158.0,
		ChainstayLength:    450.0,
		ForkA2c:            495.0,
		HeadsetCupType:     model.CupEC44,
		GeoHeadsetSpacerMm: model.Float64Ptr(0.0),
	}

	repZero := validator.ValidateBikeGeometry(fenrirZero)
	if !repZero.IsValid {
		t.Errorf("Expected Fenrir with explicit 0mm CAD spacer to be valid, got issues: %+v", repZero.Issues)
	}
	hasMismatchWarning := false
	for _, iss := range repZero.Issues {
		if iss.ID == "geo_spacer_cup_mismatch" && iss.Severity == model.SeverityWarning {
			hasMismatchWarning = true
		}
	}
	if !hasMismatchWarning {
		t.Errorf("Expected geo_spacer_cup_mismatch warning when geo_headset_spacer_mm is 0 on EC44 frame")
	}

	// Old Fenrir numbers (bb_drop 68, hta 68.0, rake 51 -> 4.7mm A2C slop) must fail under default 1.0mm slop
	fenrirSloppy := fenrirZero
	fenrirSloppy.BbDrop = 68.0
	fenrirSloppy.HtaDeg = 68.0
	fenrirSloppy.ForkRake = 51.0
	fenrirSloppy.ChainstayLength = 440.0
	fenrirSloppy.Wheelbase = 1155.0
	if validator.ValidateBikeGeometry(fenrirSloppy).IsValid {
		t.Errorf("Expected 4.7mm A2C slop and 5.4mm WB slop to fail under default 1.0mm geo_slop_mm")
	}

	// Explicitly relaxing geo_slop_mm allows the sloppy geometry to pass
	fenrirRelaxed := fenrirSloppy
	fenrirRelaxed.GeoSlopMm = &model.GeoSlopMm{
		A2c:       model.Float64Ptr(5.0),
		Wheelbase: model.Float64Ptr(6.0),
	}
	if !validator.ValidateBikeGeometry(fenrirRelaxed).IsValid {
		t.Errorf("Expected relaxed geo_slop_mm (a2c=5.0, wheelbase=6.0) to pass validation")
	}

	// With geo_headset_spacer_mm = 12, auto-detection is gone so Fenrir fails Master Stack Equation
	fenrirTwelve := fenrirZero
	fenrirTwelve.GeoHeadsetSpacerMm = model.Float64Ptr(12.0)
	repTwelve := validator.ValidateBikeGeometry(fenrirTwelve)
	if repTwelve.IsValid {
		t.Errorf("Expected Fenrir with 12mm CAD spacer to fail validation without auto-detection")
	}

	// Missing geo_headset_spacer_mm (nil) must fail validation
	fenrirMissing := fenrirZero
	fenrirMissing.GeoHeadsetSpacerMm = nil
	repMissing := validator.ValidateBikeGeometry(fenrirMissing)
	if repMissing.IsValid {
		t.Errorf("Expected missing geo_headset_spacer_mm to fail validation")
	}
}

func TestValidateGeoSlopMmAnd3DFrontCenter(t *testing.T) {
	// Binary Havok size M has a 1.4mm A2C diff and 1.2mm Wheelbase diff, while 3D Front-Center (657mm) is within 0.5mm
	havokM := model.BikeGeometry{
		ID:                 "binary-havok-m",
		Manufacturer:       "Binary",
		Model:              "Havok",
		Size:               "M",
		Reach:              386.0,
		Stack:              627.0,
		HtaDeg:             68.5,
		StaDeg:             74.0,
		Htl:                120.0,
		BbDrop:             70.0,
		ForkRake:           45.0,
		Wheelbase:          1092.0,
		ChainstayLength:    445.0,
		FrontCenter:        657.0,
		ForkA2c:            485.0,
		HeadsetCupType:     model.CupEC44,
		GeoHeadsetSpacerMm: model.Float64Ptr(10.0),
	}

	// Without geo_slop_mm, default 1.0mm slop rejects size M's 1.4mm A2C and 1.2mm Wheelbase discrepancy
	repDefault := validator.ValidateBikeGeometry(havokM)
	if repDefault.IsValid {
		t.Errorf("Expected Havok M to fail default 1.0mm slop without geo_slop_mm override")
	}

	// With geo_slop_mm {a2c: 1.5, wheelbase: 1.5} (leaving front_center at default 1.0mm), it passes
	havokM.GeoSlopMm = &model.GeoSlopMm{
		A2c:       model.Float64Ptr(1.5),
		Wheelbase: model.Float64Ptr(1.5),
	}
	repRelaxed := validator.ValidateBikeGeometry(havokM)
	if !repRelaxed.IsValid {
		t.Errorf("Expected Havok M with geo_slop_mm={a2c:1.5, wheelbase:1.5} to be valid, got issues: %+v", repRelaxed.Issues)
	}
}

func TestValidateInconsistentStackFails(t *testing.T) {
	flawed := model.BikeGeometry{
		ID:                 "flawed-hardtail",
		Manufacturer:       "Flawed",
		Model:              "Demo",
		Size:               "M",
		Reach:              440.0,
		Stack:              590.0, // Impossible stack with 541mm fork and 120mm HTL
		HtaDeg:             66.5,
		StaDeg:             75.0,
		Htl:                120.0,
		BbDrop:             65.0,
		ForkRake:           44.0,
		Wheelbase:          1170.0,
		ForkA2c:            541.0,
		HeadsetCupType:     model.CupEC44,
		GeoHeadsetSpacerMm: model.Float64Ptr(12.0),
	}

	report := validator.ValidateBikeGeometry(flawed)
	if report.IsValid {
		t.Errorf("Expected flawed stack/A2C geometry to fail validation, got valid")
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
		ID:                 "chumba-sendero",
		Manufacturer:       "Chumba",
		Model:              "Sendero",
		Sizes:              []string{"S", "M", "L", "XL"},
		DefaultSize:        "L",
		HeadsetCupType:     model.CupEC44,
		GeoHeadsetSpacerMm: model.Float64Ptr(12.0),
		Configurations: []model.FrameConfiguration{
			{
				ID:         "fork-120",
				Name:       "120mm Stock",
				IsBaseline: true,
				Fork:       model.ForkSpec{TravelMm: 120, A2c: 531, Rake: 44},
				Geometries: map[string]model.SizeGeometry{
					"L": {Reach: 475, Stack: 645, HtaDeg: 67.0, StaDeg: 74.5, Htl: 125, BbDrop: 60, Wheelbase: 1200, ChainstayLength: 430},
				},
			},
			{
				ID:         "fork-130",
				Name:       "130mm Trail",
				IsBaseline: false,
				Fork:       model.ForkSpec{TravelMm: 130, A2c: 541, Rake: 44},
				Geometries: map[string]model.SizeGeometry{
					"L": {Reach: 470, Stack: 649, HtaDeg: 66.5, StaDeg: 74.0, Htl: 125, BbDrop: 57, Wheelbase: 1203, ChainstayLength: 430},
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
		ID:                 "flawed-test-frame",
		Manufacturer:       "Flawed",
		Model:              "Frame",
		Sizes:              []string{"L"},
		DefaultSize:        "L",
		HeadsetCupType:     model.CupZS44,
		GeoHeadsetSpacerMm: model.Float64Ptr(2.0),
		Configurations: []model.FrameConfiguration{
			{
				ID:         "baseline",
				IsBaseline: true,
				Fork:       model.ForkSpec{TravelMm: 120, A2c: 530, Rake: 44},
				Geometries: map[string]model.SizeGeometry{
					"L": {Reach: 475, Stack: 630, HtaDeg: 66.0, StaDeg: 75.0, Htl: 110, BbDrop: 60, Wheelbase: 1200, ChainstayLength: 430},
				},
			},
			{
				ID:         "alternate-bad-cad",
				IsBaseline: false,
				Fork:       model.ForkSpec{TravelMm: 140, A2c: 550, Rake: 44}, // +20mm fork
				Geometries: map[string]model.SizeGeometry{
					// Deliberately impossible CAD: Stack decreased instead of increased!
					"L": {Reach: 475, Stack: 610, HtaDeg: 66.0, StaDeg: 75.0, Htl: 110, BbDrop: 60, Wheelbase: 1200, ChainstayLength: 430},
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

func TestValidateSourPastaParty(t *testing.T) {
	pastaParty := model.FrameModel{
		ID:                 "sour-pasta-party",
		Manufacturer:       "Sour",
		Model:              "Pasta Party",
		Sizes:              []string{"S", "M", "L", "XL"},
		DefaultSize:        "L",
		HeadsetCupType:     model.CupEC44,
		GeoHeadsetSpacerMm: model.Float64Ptr(12.0),
		GeoSlopMm:          &model.GeoSlopMm{A2c: model.Float64Ptr(1.5)},
		Configurations: []model.FrameConfiguration{
			{
				ID:         "stock",
				IsBaseline: true,
				Fork: model.ForkSpec{
					TravelMm: 120,
					A2c:      530,
					Rake:     44,
				},
				SagPercentagePublished: 15,
				Geometries: map[string]model.SizeGeometry{
					"S":  {Reach: 415, Stack: 607, HtaDeg: 66.6, StaDeg: 74.0, Htl: 85, BbDrop: 65, Wheelbase: 1128, ChainstayLength: 435},
					"M":  {Reach: 437, Stack: 615, HtaDeg: 66.6, StaDeg: 74.0, Htl: 95, BbDrop: 65, Wheelbase: 1153, ChainstayLength: 435},
					"L":  {Reach: 467, Stack: 619, HtaDeg: 66.6, StaDeg: 74.0, Htl: 100, BbDrop: 65, Wheelbase: 1185, ChainstayLength: 435},
					"XL": {Reach: 491, Stack: 630, HtaDeg: 66.6, StaDeg: 74.0, Htl: 110, BbDrop: 65, Wheelbase: 1214, ChainstayLength: 435},
				},
			},
		},
	}

	report := validator.ValidateFrame(pastaParty)
	if !report.IsValid {
		t.Fatalf("Expected Sour Pasta Party to pass validation: %s", report.ErrorSummary())
	}
}

