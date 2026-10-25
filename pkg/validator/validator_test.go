package validator_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"bikegeo/pkg/geometry"
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

func TestValidateForkRef_ValidAndSuggestions(t *testing.T) {
	// 1. Valid fork ref passes with zero issues
	validRef := model.ForkRef{
		Fork:   "fox34_120",
		SagPct: model.Float64Ptr(20.0),
		A2cMm:  506.0,
		RakeMm: 44.0,
	}
	issues := validator.ValidateForkRef("test-frame", "cad_fork", validRef)
	if len(issues) != 0 {
		t.Fatalf("Expected valid ForkRef to have 0 issues, got %d: %+v", len(issues), issues)
	}

	// 2. Missing fork catalog ID generates suggestions based on A2C and Rake
	missingForkRef := model.ForkRef{
		A2cMm:  506.0,
		RakeMm: 44.0,
	}
	issues = validator.ValidateForkRef("test-frame", "cad_fork", missingForkRef)
	if len(issues) == 0 {
		t.Fatalf("Expected error for missing fork catalog ID")
	}
	if issues[0].ID != "missing_cad_fork_fork" {
		t.Errorf("Expected issue missing_cad_fork_fork, got %s", issues[0].ID)
	}
	if !strings.Contains(issues[0].Message, "fox34_120") {
		t.Errorf("Expected suggestions to include fox34_120, got message: %s", issues[0].Message)
	}

	// 3. Missing sag_pct on suspension fork
	missingSagRef := model.ForkRef{
		Fork:   "fox34_120",
		A2cMm:  506.0,
		RakeMm: 44.0,
	}
	issues = validator.ValidateForkRef("test-frame", "cad_fork", missingSagRef)
	if len(issues) == 0 || issues[0].ID != "missing_cad_fork_sag_pct" {
		t.Errorf("Expected missing_cad_fork_sag_pct issue, got %+v", issues)
	}

	// 4. Mismatched A2C triggers error and suggests matching A2C
	mismatchedA2cRef := model.ForkRef{
		Fork:   "fox34_120",
		SagPct: model.Float64Ptr(20.0),
		A2cMm:  520.0,
		RakeMm: 44.0,
	}
	issues = validator.ValidateForkRef("test-frame", "cad_fork", mismatchedA2cRef)
	if len(issues) == 0 || issues[0].ID != "invalid_cad_fork_invariant" {
		t.Fatalf("Expected invalid_cad_fork_invariant issue, got %+v", issues)
	}
	if !strings.Contains(issues[0].Message, "506.0") {
		t.Errorf("Expected message to suggest expected A2C 506.0, got: %s", issues[0].Message)
	}

	// 5. Unsupported rake triggers error listing available rakes
	unsupportedRakeRef := model.ForkRef{
		Fork:   "fox34_120",
		SagPct: model.Float64Ptr(20.0),
		A2cMm:  506.0,
		RakeMm: 51.0,
	}
	issues = validator.ValidateForkRef("test-frame", "cad_fork", unsupportedRakeRef)
	if len(issues) == 0 || issues[0].ID != "unsupported_cad_fork_rake" {
		t.Errorf("Expected unsupported_cad_fork_rake issue, got %+v", issues)
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
		Sizes:            []string{"S", "M", "L", "XL"},
		DefaultSize:      "L",
		CadFork: model.ForkRef{
			Fork:   "sid_120",
			SagPct: model.Float64Ptr(20.0),
			A2cMm:  510.0,
			RakeMm: 44.0,
		},
		DefaultFork: model.ForkRef{
			Fork:   "sid_120",
			SagPct: model.Float64Ptr(20.0),
			A2cMm:  510.0,
			RakeMm: 44.0,
		},
		RigidFork: &model.ForkRef{
			Fork:   "rigid_510",
			A2cMm:  510.0,
			RakeMm: 44.0,
		},
		ForkRange: []model.ForkRef{
			{Fork: "sid_100", A2cMm: 506.0, RakeMm: 44.0},
			{Fork: "fox34_130", A2cMm: 541.0, RakeMm: 44.0},
		},
		Geometries: map[string]model.SizeGeometry{
			"S":  {Reach: 435, Stack: 599, HtaDeg: 67.5, StaDeg: 75.0, Htl: 86, BbDrop: 68, Wheelbase: 1121, ChainstayLength: 420, SeatTubeLength: 374, StandoverHeight: 700},
			"M":  {Reach: 455, Stack: 607, HtaDeg: 67.5, StaDeg: 75.0, Htl: 96, BbDrop: 68, Wheelbase: 1146, ChainstayLength: 420, SeatTubeLength: 420, StandoverHeight: 735},
			"L":  {Reach: 474, Stack: 615, HtaDeg: 67.5, StaDeg: 75.0, Htl: 106, BbDrop: 68, Wheelbase: 1167, ChainstayLength: 420, SeatTubeLength: 455, StandoverHeight: 770},
			"XL": {Reach: 499, Stack: 627, HtaDeg: 67.5, StaDeg: 75.0, Htl: 116, BbDrop: 68, Wheelbase: 1199, ChainstayLength: 420, SeatTubeLength: 460, StandoverHeight: 790},
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
		Sizes:              []string{"S", "M", "L", "XL"},
		DefaultSize:        "L",
		CadFork: model.ForkRef{
			Fork:   "sunliner_440",
			A2cMm:  440.0,
			RakeMm: 55.0,
		},
		DefaultFork: model.ForkRef{
			Fork:   "sunliner_440",
			A2cMm:  440.0,
			RakeMm: 55.0,
		},
		RigidFork: &model.ForkRef{
			Fork:   "sunliner_440",
			A2cMm:  440.0,
			RakeMm: 55.0,
		},
		ForkRange: []model.ForkRef{
			{Fork: "sunliner_440", A2cMm: 440.0, RakeMm: 55.0},
			{Fork: "sunliner_440", A2cMm: 440.0, RakeMm: 55.0},
		},
		Geometries: map[string]model.SizeGeometry{
			"S":  {Reach: 420, Stack: 590, HtaDeg: 69, StaDeg: 73, Htl: 130, BbDrop: 65, Wheelbase: 1131, ChainstayLength: 456, SeatTubeLength: 380, StandoverHeight: 765},
			"M":  {Reach: 434, Stack: 608, HtaDeg: 69, StaDeg: 73, Htl: 150, BbDrop: 65, Wheelbase: 1153, ChainstayLength: 456, SeatTubeLength: 430, StandoverHeight: 795},
			"L":  {Reach: 447, Stack: 632, HtaDeg: 69, StaDeg: 73, Htl: 175, BbDrop: 65, Wheelbase: 1175, ChainstayLength: 456, SeatTubeLength: 480, StandoverHeight: 835},
			"XL": {Reach: 457, Stack: 664, HtaDeg: 69, StaDeg: 73, Htl: 210, BbDrop: 65, Wheelbase: 1197, ChainstayLength: 456, SeatTubeLength: 545, StandoverHeight: 875},
		},
	}

	report := validator.ValidateFrame(sunliner)
	if !report.IsValid {
		t.Fatalf("Expected Tumbleweed Sunliner to pass validation: %s", report.ErrorSummary())
	}
	if !sunliner.IsFixedRigidOnly() {
		t.Errorf("Expected Sunliner to be fixed rigid only")
	}
}

func TestValidateForkRange(t *testing.T) {
	// Frame with inverted fork range (min static A2C > max static A2C)
	invalidRangeFrame := model.FrameModel{
		ID:                 "bad-range-frame",
		Manufacturer:       "Test",
		Model:              "BadRange",
		WheelSize:          "29",
		HeadsetCupType:     model.CupEC44,
		GeoHeadsetSpacerMm: model.Float64Ptr(10.0),
		Sizes:              []string{"L"},
		DefaultSize:        "L",
		CadFork: model.ForkRef{
			Fork:   "fox34_120",
			SagPct: model.Float64Ptr(20.0),
			A2cMm:  506.0,
			RakeMm: 44.0,
		},
		DefaultFork: model.ForkRef{
			Fork:   "fox34_120",
			SagPct: model.Float64Ptr(20.0),
			A2cMm:  506.0,
			RakeMm: 44.0,
		},
		ForkRange: []model.ForkRef{
			{Fork: "fox34_130", A2cMm: 541.0, RakeMm: 44.0},
			{Fork: "fox34_120", A2cMm: 530.0, RakeMm: 44.0},
		},
		Geometries: map[string]model.SizeGeometry{
			"L": {Reach: 456, Stack: 657, HtaDeg: 67, StaDeg: 74.5, Htl: 145, BbDrop: 60, Wheelbase: 1208, ChainstayLength: 455},
		},
	}

	report := validator.ValidateFrame(invalidRangeFrame)
	if report.IsValid {
		t.Fatalf("Expected frame with inverted fork range to fail validation")
	}

	foundRangeIssue := false
	for _, iss := range report.Issues {
		if iss.ID == "inverted_fork_range" {
			foundRangeIssue = true
			break
		}
	}
	if !foundRangeIssue {
		t.Errorf("Expected issue 'inverted_fork_range', got: %+v", report.Issues)
	}
}

func TestAllFrameData(t *testing.T) {
	files, err := filepath.Glob("../../data/bikes/*.json")
	if err != nil {
		t.Fatalf("Failed to glob data/bikes/*.json: %v", err)
	}
	if len(files) == 0 {
		t.Fatalf("Expected at least one frame file in data/bikes, found 0")
	}

	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("Failed to read %s: %v", path, err)
			continue
		}

		var frame model.FrameModel
		if err := json.Unmarshal(data, &frame); err != nil {
			t.Errorf("Failed to unmarshal %s: %v", path, err)
			continue
		}

		// 1. Verify filename matches frame ID
		expectedFilename := frame.ID + ".json"
		if filepath.Base(path) != expectedFilename {
			t.Errorf("Filename mismatch: %s has ID %q (expected filename %s)", path, frame.ID, expectedFilename)
		}

		// 2. Verify FrameModel validation passes
		report := validator.ValidateFrame(frame)
		if !report.IsValid {
			t.Errorf("Frame %s failed validation: %s", frame.ID, report.ErrorSummary())
		}

		// 3. Verify all declared sizes materialize and validate
		for _, sz := range frame.GetSizes() {
			bike, err := geometry.MaterializeBikeGeometry(frame, sz)
			if err != nil {
				t.Errorf("Frame %s failed to materialize size %s: %v", frame.ID, sz, err)
				continue
			}
			bikeRep := validator.ValidateBikeGeometry(bike)
			if !bikeRep.IsValid {
				t.Errorf("Frame %s (size %s) failed bike validation: %+v", frame.ID, sz, bikeRep.Issues)
			}
		}

		// 4. Verify CAD and Default fork specifications are populated
		if frame.CadFork.Fork == "" || frame.CadFork.A2cMm <= 0 {
			t.Errorf("Frame %s has invalid cad_fork: %+v", frame.ID, frame.CadFork)
		}
		if frame.DefaultFork.Fork == "" || frame.DefaultFork.A2cMm <= 0 {
			t.Errorf("Frame %s has invalid default_fork: %+v", frame.ID, frame.DefaultFork)
		}

		// 5. Verify fork range
		if minF, maxF, ok := frame.GetForkRange(); ok {
			if minF.StaticA2c() > maxF.StaticA2c() {
				t.Errorf("Frame %s has inverted fork range: min static %.1f > max static %.1f", frame.ID, minF.StaticA2c(), maxF.StaticA2c())
			}
		} else {
			t.Errorf("Frame %s missing fork_range", frame.ID)
		}

		// 6. Verify rigid capability matches field
		if frame.SupportsRigid() && frame.GetRigidA2c() <= 0 {
			t.Errorf("Frame %s supports rigid but has rigid A2C <= 0", frame.ID)
		}

		// 7. Verify adjustable chainstay ranges if present
		if frame.HasAdjustableChainstay() {
			if frame.GetMinChainstay() >= frame.GetMaxChainstay() {
				t.Errorf("Frame %s has invalid chainstay range [%.1f, %.1f]", frame.ID, frame.GetMinChainstay(), frame.GetMaxChainstay())
			}
		}
	}
}

func TestSizeResiduals(t *testing.T) {
	files, err := filepath.Glob("../../data/bikes/*.json")
	if err != nil || len(files) == 0 {
		t.Fatalf("Failed to glob data/bikes/*.json")
	}

	// Test SizeResiduals on the first available real frame
	data, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatalf("Failed to read %s: %v", files[0], err)
	}

	var frame model.FrameModel
	if err := json.Unmarshal(data, &frame); err != nil {
		t.Fatalf("Failed to unmarshal %s: %v", files[0], err)
	}

	residuals, err := validator.SizeResiduals(frame)
	if err != nil {
		t.Fatalf("SizeResiduals failed on %s: %v", frame.ID, err)
	}
	if len(residuals) != len(frame.Sizes) {
		t.Errorf("Expected %d residuals for %s, got %d", len(frame.Sizes), frame.ID, len(residuals))
	}
	for _, r := range residuals {
		if r.ImpliedStructuralA2c <= 0 {
			t.Errorf("Expected positive ImpliedStructuralA2c for size %s, got %.2f", r.Size, r.ImpliedStructuralA2c)
		}
	}
}
