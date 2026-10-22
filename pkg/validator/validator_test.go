package validator_test

import (
	"encoding/json"
	"os"
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
		Model:              "Fenrir",
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

func TestValidateRitcheyUltra(t *testing.T) {
	ultra := model.FrameModel{
		ID:                 "ritchey-ultra",
		Manufacturer:       "Ritchey",
		Model:              "Ultra",
		Sizes:              []string{"S", "M", "L", "XL"},
		DefaultSize:        "L",
		HeadsetCupType:     model.CupIS52,
		GeoHeadsetSpacerMm: model.Float64Ptr(0.0),
		GeoSlopMm:          &model.GeoSlopMm{A2c: model.Float64Ptr(3.0)},
		Configurations: []model.FrameConfiguration{
			{
				ID:         "stock",
				IsBaseline: true,
				Fork: model.ForkSpec{
					TravelMm: 120,
					A2c:      530,
					Rake:     44,
				},
				SagPercentagePublished: 20,
				Geometries: map[string]model.SizeGeometry{
					"S":  {Reach: 417, Stack: 606, HtaDeg: 68.0, StaDeg: 74.0, Htl: 100, BbDrop: 58, Wheelbase: 1121, ChainstayLength: 439},
					"M":  {Reach: 430, Stack: 613, HtaDeg: 68.5, StaDeg: 74.0, Htl: 105, BbDrop: 58, Wheelbase: 1131, ChainstayLength: 439},
					"L":  {Reach: 445, Stack: 617, HtaDeg: 68.5, StaDeg: 73.5, Htl: 110, BbDrop: 58, Wheelbase: 1148, ChainstayLength: 439},
					"XL": {Reach: 460, Stack: 627, HtaDeg: 68.5, StaDeg: 73.5, Htl: 120, BbDrop: 58, Wheelbase: 1167, ChainstayLength: 439},
				},
			},
		},
	}

	report := validator.ValidateFrame(ultra)
	if !report.IsValid {
		t.Fatalf("Expected Ritchey Ultra to pass validation: %s", report.ErrorSummary())
	}
}

func TestValidateEskerHayduke(t *testing.T) {
	hayduke := model.FrameModel{
		ID:                 "esker-hayduke",
		Manufacturer:       "Esker",
		Model:              "Hayduke",
		Sizes:              []string{"S1", "S2", "S3", "S4"},
		DefaultSize:        "S3",
		HeadsetCupType:     model.CupEC44,
		GeoHeadsetSpacerMm: model.Float64Ptr(12.0),
		GeoSlopMm: &model.GeoSlopMm{
			A2c:       model.Float64Ptr(3.0),
			Wheelbase: model.Float64Ptr(6.0),
		},
		ChainstayRangeMm:   []float64{440, 454},
		Configurations: []model.FrameConfiguration{
			{
				ID:         "stock",
				IsBaseline: true,
				Fork: model.ForkSpec{
					TravelMm: 120,
					A2c:      531,
					Rake:     44,
				},
				SagPercentagePublished: 30,
				Geometries: map[string]model.SizeGeometry{
					"S1": {Reach: 420, Stack: 615, HtaDeg: 67.5, StaDeg: 74.0, Htl: 105, BbDrop: 65, Wheelbase: 1130, ChainstayLength: 440},
					"S2": {Reach: 450, Stack: 620, HtaDeg: 67.5, StaDeg: 74.0, Htl: 110, BbDrop: 65, Wheelbase: 1160, ChainstayLength: 440},
					"S3": {Reach: 470, Stack: 630, HtaDeg: 67.5, StaDeg: 74.0, Htl: 120, BbDrop: 65, Wheelbase: 1183, ChainstayLength: 440},
					"S4": {Reach: 500, Stack: 645, HtaDeg: 67.5, StaDeg: 74.0, Htl: 140, BbDrop: 65, Wheelbase: 1218, ChainstayLength: 440},
				},
			},
		},
	}

	report := validator.ValidateFrame(hayduke)
	if !report.IsValid {
		t.Fatalf("Expected Esker Hayduke to pass validation: %s", report.ErrorSummary())
	}
}

func TestValidateRevelElJefe(t *testing.T) {
	jefe := model.FrameModel{
		ID:                 "revel-el-jefe",
		Manufacturer:       "Revel",
		Model:              "El Jefe",
		HeadsetCupType:     model.CupIS52,
		GeoHeadsetSpacerMm: model.Float64Ptr(0.0),
		GeoSlopMm: &model.GeoSlopMm{
			A2c:       model.Float64Ptr(6.0),
			Wheelbase: model.Float64Ptr(6.5),
		},
		ActualStaDeg:     73.3,
		ChainstayRangeMm: []float64{420, 435},
		Configurations: []model.FrameConfiguration{
			{
				ID:         "stock",
				IsBaseline: true,
				Fork: model.ForkSpec{
					TravelMm: 120,
					A2c:      534,
					Rake:     44,
				},
				SagPercentagePublished: 20,
				Geometries: map[string]model.SizeGeometry{
					"S":  {Reach: 435, Stack: 599, HtaDeg: 67.5, StaDeg: 75.0, Htl: 86, BbDrop: 68, Wheelbase: 1121, ChainstayLength: 420, SeatTubeLength: 374, StandoverHeight: 700},
					"M":  {Reach: 455, Stack: 607, HtaDeg: 67.5, StaDeg: 75.0, Htl: 96, BbDrop: 68, Wheelbase: 1146, ChainstayLength: 420, SeatTubeLength: 420, StandoverHeight: 735},
					"L":  {Reach: 474, Stack: 615, HtaDeg: 67.5, StaDeg: 75.0, Htl: 106, BbDrop: 68, Wheelbase: 1167, ChainstayLength: 420, SeatTubeLength: 455, StandoverHeight: 770},
					"XL": {Reach: 499, Stack: 627, HtaDeg: 67.5, StaDeg: 75.0, Htl: 116, BbDrop: 68, Wheelbase: 1199, ChainstayLength: 420, SeatTubeLength: 460, StandoverHeight: 790},
				},
			},
		},
	}

	report := validator.ValidateFrame(jefe)
	if !report.IsValid {
		t.Fatalf("Expected Revel El Jefe to pass validation: %s", report.ErrorSummary())
	}
	if !jefe.HasBentSeatTube() {
		t.Fatalf("Expected Revel El Jefe to have bent seat tube")
	}

	// Verify invalid actual_sta_deg fails validation
	invalidJefe := jefe
	invalidJefe.ActualStaDeg = 92.0
	invReport := validator.ValidateFrame(invalidJefe)
	if invReport.IsValid {
		t.Errorf("Expected invalid actual_sta_deg (92.0°) to fail validation")
	}
}

func TestValidateTumbleweedSunliner(t *testing.T) {
	sunliner := model.FrameModel{
		ID:                 "tumbleweed-sunliner",
		Manufacturer:       "Tumbleweed",
		Model:              "Sunliner",
		Category:           model.CategoryRigid,
		HeadsetCupType:     model.CupEC44,
		GeoHeadsetSpacerMm: model.Float64Ptr(13.0),
		RigidA2cMm:         440.0,
		DefaultFork:        "rigid",
		Sizes:              []string{"S", "M", "L", "XL"},
		DefaultSize:        "L",
		Configurations: []model.FrameConfiguration{
			{
				ID:         "stock",
				IsBaseline: true,
				Fork: model.ForkSpec{
					TravelMm: 0,
					A2c:      440,
					Rake:     55,
				},
				SagPercentagePublished: 0,
				Geometries: map[string]model.SizeGeometry{
					"S":  {Reach: 420, Stack: 590, HtaDeg: 69, StaDeg: 73, Htl: 130, BbDrop: 65, Wheelbase: 1131, ChainstayLength: 456, SeatTubeLength: 380, StandoverHeight: 765},
					"M":  {Reach: 434, Stack: 608, HtaDeg: 69, StaDeg: 73, Htl: 150, BbDrop: 65, Wheelbase: 1153, ChainstayLength: 456, SeatTubeLength: 430, StandoverHeight: 795},
					"L":  {Reach: 447, Stack: 632, HtaDeg: 69, StaDeg: 73, Htl: 175, BbDrop: 65, Wheelbase: 1175, ChainstayLength: 456, SeatTubeLength: 480, StandoverHeight: 835},
					"XL": {Reach: 457, Stack: 664, HtaDeg: 69, StaDeg: 73, Htl: 210, BbDrop: 65, Wheelbase: 1197, ChainstayLength: 456, SeatTubeLength: 545, StandoverHeight: 875},
				},
			},
		},
	}

	report := validator.ValidateFrame(sunliner)
	if !report.IsValid {
		t.Fatalf("Expected Tumbleweed Sunliner to pass validation: %s", report.ErrorSummary())
	}
	if !sunliner.IsFixedRigidOnly() {
		t.Errorf("Expected Sunliner to be fixed rigid only")
	}
	if !sunliner.IsFixedFork() {
		t.Errorf("Expected Sunliner to be fixed fork")
	}
}

func TestValidateSurlyKrampus(t *testing.T) {
	krampus := model.FrameModel{
		ID:                 "surly-krampus",
		Manufacturer:       "Surly",
		Model:              "Krampus",
		Category:           model.CategoryHardtail,
		HeadsetCupType:     model.CupEC44,
		GeoHeadsetSpacerMm: model.Float64Ptr(12.0),
		GeoSlopMm: &model.GeoSlopMm{
			A2c: model.Float64Ptr(2.0),
		},
		RigidA2cMm:        483.0,
		DefaultFork:       "rigid",
		SupportedTravelMm: []float64{100, 110, 120},
		Sizes:             []string{"S", "M", "L", "XL"},
		DefaultSize:       "L",
		Configurations: []model.FrameConfiguration{
			{
				ID:         "stock",
				IsBaseline: true,
				Fork: model.ForkSpec{
					TravelMm: 0,
					A2c:      483,
					Rake:     47,
				},
				SagPercentagePublished: 0,
				Geometries: map[string]model.SizeGeometry{
					"S":  {Reach: 404, Stack: 593, HtaDeg: 69, StaDeg: 73, Htl: 90, BbDrop: 65, Wheelbase: 1087, ChainstayLength: 435, SeatTubeLength: 368, StandoverHeight: 770},
					"M":  {Reach: 426, Stack: 602, HtaDeg: 69, StaDeg: 73, Htl: 100, BbDrop: 65, Wheelbase: 1113, ChainstayLength: 435, SeatTubeLength: 419, StandoverHeight: 797},
					"L":  {Reach: 448, Stack: 612, HtaDeg: 69, StaDeg: 73, Htl: 110, BbDrop: 65, Wheelbase: 1138, ChainstayLength: 435, SeatTubeLength: 470, StandoverHeight: 825},
					"XL": {Reach: 470, Stack: 621, HtaDeg: 69, StaDeg: 73, Htl: 120, BbDrop: 65, Wheelbase: 1164, ChainstayLength: 435, SeatTubeLength: 521, StandoverHeight: 853},
				},
			},
		},
	}

	report := validator.ValidateFrame(krampus)
	if !report.IsValid {
		t.Fatalf("Expected Surly Krampus to pass validation: %s", report.ErrorSummary())
	}
	if !krampus.SupportsRigid() {
		t.Errorf("Expected Krampus to support rigid fork")
	}
	if !krampus.SupportsTravel(120) {
		t.Errorf("Expected Krampus to support 120mm travel fork")
	}
}

func TestValidateBinaryKinetik(t *testing.T) {
	kinetik := model.FrameModel{
		ID:                 "binary-kinetik",
		Manufacturer:       "Binary",
		Model:              "Kinetik",
		Year:               2026,
		Category:           model.CategoryHardtail,
		HeadsetCupType:     model.CupEC44,
		GeoHeadsetSpacerMm: model.Float64Ptr(10.0),
		GeoSlopMm: &model.GeoSlopMm{
			A2c:       model.Float64Ptr(1.5),
			Wheelbase: model.Float64Ptr(1.5),
		},
		RigidA2cMm:        500.0,
		DefaultFork:       "120mm",
		SupportedTravelMm: []float64{100, 110, 120},
		ChainstayRangeMm:  []float64{455, 472},
		Sizes:             []string{"S", "SMD", "M", "ML", "L", "XL"},
		DefaultSize:       "L",
		Configurations: []model.FrameConfiguration{
			{
				ID:         "stock",
				IsBaseline: true,
				Fork: model.ForkSpec{
					TravelMm:           120,
					A2c:                530,
					Rake:               44,
					StandardSagPercent: 15,
				},
				SagPercentagePublished: 15,
				Geometries: map[string]model.SizeGeometry{
					"S":   {Reach: 417, Stack: 616, HtaDeg: 67, StaDeg: 74, Htl: 100, BbDrop: 60, Wheelbase: 1151, ChainstayLength: 455, FrontCenter: 703, SeatTubeLength: 390, StandoverHeight: 771},
					"SMD": {Reach: 433, Stack: 616, HtaDeg: 67, StaDeg: 74, Htl: 100, BbDrop: 60, Wheelbase: 1167, ChainstayLength: 455, FrontCenter: 719, SeatTubeLength: 415, StandoverHeight: 792},
					"M":   {Reach: 434, Stack: 625, HtaDeg: 67, StaDeg: 74, Htl: 110, BbDrop: 60, Wheelbase: 1172, ChainstayLength: 455, FrontCenter: 724, SeatTubeLength: 420, StandoverHeight: 821},
					"ML":  {Reach: 441, Stack: 643, HtaDeg: 67, StaDeg: 74, Htl: 130, BbDrop: 60, Wheelbase: 1187, ChainstayLength: 455, FrontCenter: 738, SeatTubeLength: 440, StandoverHeight: 840},
					"L":   {Reach: 456, Stack: 657, HtaDeg: 67, StaDeg: 74.5, Htl: 145, BbDrop: 60, Wheelbase: 1208, ChainstayLength: 455, FrontCenter: 759, SeatTubeLength: 480, StandoverHeight: 867},
					"XL":  {Reach: 465, Stack: 671, HtaDeg: 67, StaDeg: 74.5, Htl: 160, BbDrop: 60, Wheelbase: 1223, ChainstayLength: 455, FrontCenter: 774, SeatTubeLength: 508, StandoverHeight: 887},
				},
			},
		},
	}

	report := validator.ValidateFrame(kinetik)
	if !report.IsValid {
		t.Fatalf("Expected Binary Kinetik to pass validation: %s", report.ErrorSummary())
	}
	if !kinetik.SupportsRigid() {
		t.Errorf("Expected Kinetik to support rigid fork (500mm)")
	}
	if !kinetik.SupportsTravel(100) || !kinetik.SupportsTravel(120) {
		t.Errorf("Expected Kinetik to support 100mm and 120mm travel forks")
	}
	if !kinetik.HasAdjustableChainstay() {
		t.Errorf("Expected Kinetik to have adjustable chainstays")
	}
	if kinetik.GetMinChainstay() != 455 || kinetik.GetMaxChainstay() != 472 {
		t.Errorf("Expected chainstay range 455-472mm, got %.1f-%.1f", kinetik.GetMinChainstay(), kinetik.GetMaxChainstay())
	}
}

func TestValidateImplausibleForkA2c(t *testing.T) {
	// A 120mm 29er fork with 512mm static A2C is physically implausible (standard is 520-545mm)
	// and drops to 488mm effective A2C at 20% sag.
	implausibleFrame := model.FrameModel{
		ID:                 "bad-fork-frame",
		Manufacturer:       "Test",
		Model:              "BadFork",
		WheelSize:          "29",
		HeadsetCupType:     model.CupEC44,
		GeoHeadsetSpacerMm: model.Float64Ptr(10.0),
		SupportedTravelMm: []float64{120},
		Sizes:             []string{"L"},
		DefaultSize:       "L",
		Configurations: []model.FrameConfiguration{
			{
				ID:         "stock",
				IsBaseline: true,
				Fork: model.ForkSpec{
					TravelMm: 120,
					A2c:      512, // Implausible static A2C for 120mm 29er
					Rake:     44,
				},
				SagPercentagePublished: 0,
				Geometries: map[string]model.SizeGeometry{
					"L": {Reach: 456, Stack: 657, HtaDeg: 67, StaDeg: 74.5, Htl: 145, BbDrop: 60, Wheelbase: 1208, ChainstayLength: 455, FrontCenter: 759},
				},
			},
		},
	}

	report := validator.ValidateFrame(implausibleFrame)
	if report.IsValid {
		t.Fatalf("Expected frame with 120mm fork and 512mm static A2C to fail validation")
	}

	foundIssue := false
	for _, iss := range report.Issues {
		if iss.ID == "implausible_fork_a2c" || iss.ID == "implausible_supported_travel_a2c" {
			foundIssue = true
			break
		}
	}
	if !foundIssue {
		for _, szRep := range report.SizeReports {
			for _, iss := range szRep.Issues {
				if iss.ID == "implausible_fork_a2c" {
					foundIssue = true
					break
				}
			}
		}
	}
	if !foundIssue {
		t.Errorf("Expected issue 'implausible_fork_a2c' or 'implausible_supported_travel_a2c', got: %+v", report.Issues)
	}
}

func TestValidateTurnerVennTi(t *testing.T) {
	venn := model.FrameModel{
		ID:                 "turner-venn-ti",
		Manufacturer:       "Turner",
		Model:              "Venn Ti",
		Year:               2024,
		Category:           model.CategoryHardtail,
		HeadsetCupType:     model.CupZS56,
		GeoHeadsetSpacerMm: model.Float64Ptr(0.0),
		GeoSlopMm: &model.GeoSlopMm{
			A2c:       model.Float64Ptr(2.5),
			Wheelbase: model.Float64Ptr(2.0),
		},
		DefaultFork:       "130mm",
		SupportedTravelMm: []float64{120, 130, 140},
		Sizes:             []string{"S", "M", "L", "XL"},
		DefaultSize:       "L",
		Configurations: []model.FrameConfiguration{
			{
				ID:         "stock",
				Name:       "Stock 130mm Suspension Build",
				IsBaseline: true,
				Fork: model.ForkSpec{
					TravelMm:           130,
					A2c:                541,
					Rake:               44,
					StandardSagPercent: 20,
				},
				SagPercentagePublished: 0,
				Geometries: map[string]model.SizeGeometry{
					"S": {
						Reach:           410,
						Stack:           628,
						HtaDeg:          67.0,
						StaDeg:          74.0,
						Htl:             90,
						BbDrop:          65,
						Wheelbase:       1137,
						ChainstayLength: 445,
						SeatTubeLength:  410,
					},
					"M": {
						Reach:           434,
						Stack:           645,
						HtaDeg:          67.0,
						StaDeg:          74.0,
						Htl:             110,
						BbDrop:          65,
						Wheelbase:       1170,
						ChainstayLength: 445,
						SeatTubeLength:  450,
					},
					"L": {
						Reach:           448,
						Stack:           662,
						HtaDeg:          67.0,
						StaDeg:          73.0,
						Htl:             130,
						BbDrop:          62,
						Wheelbase:       1190,
						ChainstayLength: 445,
						SeatTubeLength:  490,
					},
					"XL": {
						Reach:           470,
						Stack:           690,
						HtaDeg:          67.0,
						StaDeg:          73.0,
						Htl:             160,
						BbDrop:          62,
						Wheelbase:       1224,
						ChainstayLength: 445,
						SeatTubeLength:  530,
					},
				},
			},
		},
	}

	report := validator.ValidateFrame(venn)
	if !report.IsValid {
		t.Fatalf("Expected Turner Venn Ti to pass validation: %s", report.ErrorSummary())
	}
	if venn.SupportsRigid() {
		t.Errorf("Expected Venn Ti to not support rigid fork")
	}
	if !venn.SupportsTravel(120) || !venn.SupportsTravel(130) || !venn.SupportsTravel(140) {
		t.Errorf("Expected Venn Ti to support 120mm, 130mm, and 140mm travel forks")
	}
	if venn.SupportsTravel(100) || venn.SupportsTravel(150) {
		t.Errorf("Expected Venn Ti to reject 100mm and 150mm travel forks")
	}
	if venn.HasAdjustableChainstay() {
		t.Errorf("Expected Venn Ti to have fixed chainstays (445mm)")
	}
}

func TestValidateFairlightHolt(t *testing.T) {
	holt := model.FrameModel{
		ID:                 "fairlight-holt-2",
		Manufacturer:       "Fairlight",
		Model:              "Holt 2.0",
		Year:               2025,
		Category:           model.CategoryHardtail,
		WheelSize:          "29",
		HeadsetCupType:     model.CupEC44,
		GeoHeadsetSpacerMm: model.Float64Ptr(12.0),
		ActualStaDeg:       74.3,
		SupportedTravelMm:  []float64{100, 110, 120},
		DefaultFork:        "120mm",
		Sizes:              []string{"S", "M", "L", "XL"},
		DefaultSize:        "L",
		Configurations: []model.FrameConfiguration{
			{
				ID:         "stock",
				Name:       "Stock 120mm Suspension Build",
				IsBaseline: true,
				Fork: model.ForkSpec{
					ID:                 "sid_120",
					Name:               "RockShox SID 120mm",
					TravelMm:           120,
					A2c:                531,
					Rake:               44,
					StandardSagPercent: 20,
				},
				SagPercentagePublished: 20,
				Geometries: map[string]model.SizeGeometry{
					"S": {
						Reach:           425,
						Stack:           616,
						HtaDeg:          67.0,
						StaDeg:          75.3,
						Htl:             100,
						BbDrop:          63,
						Wheelbase:       1138,
						ChainstayLength: 435,
						SeatTubeLength:  385,
						StandoverHeight: 742,
					},
					"M": {
						Reach:           450,
						Stack:           616,
						HtaDeg:          67.0,
						StaDeg:          75.3,
						Htl:             100,
						BbDrop:          63,
						Wheelbase:       1163,
						ChainstayLength: 435,
						SeatTubeLength:  425,
						StandoverHeight: 765,
					},
					"L": {
						Reach:           470,
						Stack:           625,
						HtaDeg:          67.0,
						StaDeg:          75.3,
						Htl:             110,
						BbDrop:          63,
						Wheelbase:       1187,
						ChainstayLength: 435,
						SeatTubeLength:  465,
						StandoverHeight: 791,
					},
					"XL": {
						Reach:           495,
						Stack:           643,
						HtaDeg:          67.0,
						StaDeg:          75.2,
						Htl:             130,
						BbDrop:          63,
						Wheelbase:       1220,
						ChainstayLength: 435,
						SeatTubeLength:  510,
						StandoverHeight: 824,
					},
				},
			},
		},
	}

	report := validator.ValidateFrame(holt)
	if !report.IsValid {
		t.Fatalf("Expected Fairlight Holt 2.0 to pass validation: %s", report.ErrorSummary())
	}
	if holt.SupportsRigid() {
		t.Errorf("Expected Holt 2.0 to not support rigid fork")
	}
	if !holt.SupportsTravel(100) || !holt.SupportsTravel(110) || !holt.SupportsTravel(120) {
		t.Errorf("Expected Holt 2.0 to support 100mm, 110mm, and 120mm travel forks")
	}
	if holt.SupportsTravel(80) || holt.SupportsTravel(130) {
		t.Errorf("Expected Holt 2.0 to reject 80mm and 130mm travel forks")
	}
	if holt.HasAdjustableChainstay() {
		t.Errorf("Expected Holt 2.0 to have fixed chainstays (435mm)")
	}
}

func TestValidateSklarPBJ(t *testing.T) {
	pbj := model.FrameModel{
		ID:                 "sklar-pbj",
		Manufacturer:       "Sklar",
		Model:              "PBJ",
		Year:               2025,
		Category:           model.CategoryHardtail,
		WheelSize:          "29",
		HeadsetCupType:     model.CupEC44,
		GeoHeadsetSpacerMm: model.Float64Ptr(12.0),
		RigidA2cMm:         485.0,
		DefaultFork:        "rigid",
		SupportedTravelMm:  []float64{100, 110, 120},
		ChainstayRangeMm:   []float64{432, 452},
		Sizes:              []string{"XS", "S", "M", "L", "XL"},
		DefaultSize:        "M",
		GeoSlopMm: &model.GeoSlopMm{
			A2c:         model.Float64Ptr(3.5),
			FrontCenter: model.Float64Ptr(3.0),
			Wheelbase:   model.Float64Ptr(5.0),
		},
		Configurations: []model.FrameConfiguration{
			{
				ID:         "stock",
				Name:       "Stock Rigid 485mm Build",
				IsBaseline: true,
				Fork: model.ForkSpec{
					ID:       "rigid_485",
					Name:     "Sklar Chromoly Steel 485mm",
					TravelMm: 0,
					A2c:      485,
					Rake:     52,
				},
				SagPercentagePublished: 0,
				Geometries: map[string]model.SizeGeometry{
					"XS": {
						Reach:           411,
						Stack:           574,
						HtaDeg:          68.0,
						StaDeg:          74.5,
						Htl:             70,
						BbDrop:          68,
						Wheelbase:       1098,
						FrontCenter:     675,
						ChainstayLength: 432,
						SeatTubeLength:  360,
					},
					"S": {
						Reach:           433,
						Stack:           588,
						HtaDeg:          68.5,
						StaDeg:          74.0,
						Htl:             85,
						BbDrop:          68,
						Wheelbase:       1124,
						FrontCenter:     698,
						ChainstayLength: 432,
						SeatTubeLength:  410,
					},
					"M": {
						Reach:           461,
						Stack:           615,
						HtaDeg:          69.0,
						StaDeg:          74.0,
						Htl:             110,
						BbDrop:          68,
						Wheelbase:       1156,
						FrontCenter:     730,
						ChainstayLength: 432,
						SeatTubeLength:  440,
					},
					"L": {
						Reach:           475,
						Stack:           641,
						HtaDeg:          69.0,
						StaDeg:          74.0,
						Htl:             140,
						BbDrop:          68,
						Wheelbase:       1182,
						FrontCenter:     755,
						ChainstayLength: 432,
						SeatTubeLength:  505,
					},
					"XL": {
						Reach:           508,
						Stack:           670,
						HtaDeg:          69.0,
						StaDeg:          74.0,
						Htl:             170,
						BbDrop:          68,
						Wheelbase:       1222,
						FrontCenter:     795,
						ChainstayLength: 432,
						SeatTubeLength:  530,
					},
				},
			},
		},
	}

	report := validator.ValidateFrame(pbj)
	if !report.IsValid {
		t.Fatalf("Expected Sklar PBJ to pass validation: %s", report.ErrorSummary())
	}
	if !pbj.SupportsRigid() {
		t.Errorf("Expected Sklar PBJ to support rigid fork (485mm)")
	}
	if pbj.GetRigidA2c() != 485.0 {
		t.Errorf("Expected Sklar PBJ rigid A2C to be 485mm, got %.1f", pbj.GetRigidA2c())
	}
	if !pbj.SupportsTravel(100) || !pbj.SupportsTravel(110) || !pbj.SupportsTravel(120) {
		t.Errorf("Expected Sklar PBJ to support 100mm, 110mm, and 120mm travel forks")
	}
	if pbj.SupportsTravel(80) || pbj.SupportsTravel(130) {
		t.Errorf("Expected Sklar PBJ to reject 80mm and 130mm travel forks")
	}
	if !pbj.HasAdjustableChainstay() {
		t.Errorf("Expected Sklar PBJ to have adjustable chainstays")
	}
	if pbj.GetMinChainstay() != 432 || pbj.GetMaxChainstay() != 452 {
		t.Errorf("Expected chainstay range 432-452mm (20mm adjustment), got %.1f-%.1f", pbj.GetMinChainstay(), pbj.GetMaxChainstay())
	}
	if pbj.GetMaxChainstay()-pbj.GetMinChainstay() != 20 {
		t.Errorf("Expected exactly 20mm of chainstay adjustment, got %.1f", pbj.GetMaxChainstay()-pbj.GetMinChainstay())
	}
}

func TestValidateSevenExpedition926f(t *testing.T) {
	data, err := os.ReadFile("../../data/bikes/seven-expedition-926f.json")
	if err != nil {
		t.Fatalf("Failed to read seven-expedition-926f.json: %v", err)
	}
	var seven model.FrameModel
	if err := json.Unmarshal(data, &seven); err != nil {
		t.Fatalf("Failed to parse seven-expedition-926f.json: %v", err)
	}

	report := validator.ValidateFrame(seven)
	if !report.IsValid {
		t.Fatalf("Expected Seven Cycles Expedition 926f to pass validation: %s", report.ErrorSummary())
	}
	if len(seven.Sizes) != 24 {
		t.Errorf("Expected 24 sizes (12 Standard Fit + 12 Endurance Fit), got %d", len(seven.Sizes))
	}
	if !seven.SupportsRigid() {
		t.Errorf("Expected Seven Expedition 926f to support rigid fork (472mm)")
	}
	if seven.GetRigidA2c() != 472.0 {
		t.Errorf("Expected Seven Expedition 926f rigid A2C to be 472mm, got %.1f", seven.GetRigidA2c())
	}
	if !seven.SupportsTravel(100) || !seven.SupportsTravel(110) || !seven.SupportsTravel(120) {
		t.Errorf("Expected Seven Expedition 926f to support 100mm, 110mm, and 120mm travel forks")
	}
	if seven.SupportsTravel(80) || seven.SupportsTravel(130) {
		t.Errorf("Expected Seven Expedition 926f to reject 80mm and 130mm travel forks")
	}
	if seven.HasAdjustableChainstay() {
		t.Errorf("Expected Seven Expedition 926f to have fixed chainstays")
	}
}

func TestValidateSurlyECR(t *testing.T) {
	data, err := os.ReadFile("../../data/bikes/surly-ecr.json")
	if err != nil {
		t.Fatalf("Failed to read surly-ecr.json: %v", err)
	}
	var ecr model.FrameModel
	if err := json.Unmarshal(data, &ecr); err != nil {
		t.Fatalf("Failed to parse surly-ecr.json: %v", err)
	}

	report := validator.ValidateFrame(ecr)
	if !report.IsValid {
		t.Fatalf("Expected Surly ECR to pass validation: %s", report.ErrorSummary())
	}
	if len(ecr.Sizes) != 3 {
		t.Errorf("Expected 3 sizes (M, L, XL), got %d", len(ecr.Sizes))
	}
	if !ecr.SupportsRigid() {
		t.Errorf("Expected Surly ECR to support rigid fork (468mm)")
	}
	if ecr.GetRigidA2c() != 468.0 {
		t.Errorf("Expected Surly ECR rigid A2C to be 468mm, got %.1f", ecr.GetRigidA2c())
	}
	if !ecr.IsFixedRigidOnly() {
		t.Errorf("Expected Surly ECR to be fixed rigid only")
	}
	if !ecr.IsFixedFork() {
		t.Errorf("Expected Surly ECR to be fixed fork")
	}
	if ecr.SupportsTravel(100) || ecr.SupportsTravel(120) {
		t.Errorf("Expected Surly ECR to reject suspension travel forks")
	}
	if ecr.HeadsetCupType != model.CupEC34 {
		t.Errorf("Expected Surly ECR to have EC34 headset cup, got %s", ecr.HeadsetCupType)
	}
}


