package validator_test

import (
	"math"
	"strings"
	"testing"

	"bikegeo/pkg/canonicalizer"
	"bikegeo/pkg/catalog"
	"bikegeo/pkg/frames"
	"bikegeo/pkg/geom2d"
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
		GeoHeadsetSpacerMm: 10.0,
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
		GeoHeadsetSpacerMm: 0.0,
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
		A2c:       model.Some(5.0),
		Wheelbase: model.Some(6.0),
	}
	if !validator.ValidateBikeGeometry(fenrirRelaxed).IsValid {
		t.Errorf("Expected relaxed geo_slop_mm (a2c=5.0, wheelbase=6.0) to pass validation")
	}

	// With geo_headset_spacer_mm = 12, auto-detection is gone so Fenrir fails Master Stack Equation
	fenrirTwelve := fenrirZero
	fenrirTwelve.GeoHeadsetSpacerMm = 12.0
	repTwelve := validator.ValidateBikeGeometry(fenrirTwelve)
	if repTwelve.IsValid {
		t.Errorf("Expected Fenrir with 12mm CAD spacer to fail validation without auto-detection")
	}

	// Out-of-range geo_headset_spacer_mm (-1.0) must fail validation
	fenrirInvalidSpacer := fenrirZero
	fenrirInvalidSpacer.GeoHeadsetSpacerMm = -1.0
	repInvalidSpacer := validator.ValidateBikeGeometry(fenrirInvalidSpacer)
	if repInvalidSpacer.IsValid {
		t.Errorf("Expected negative geo_headset_spacer_mm to fail validation")
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
		GeoHeadsetSpacerMm: 10.0,
	}

	// Size M's 1.4mm A2C residual comes from the integer Stack (627mm): rounding of Stack,
	// BB drop and HTL alone can produce ±1.6mm, so it passes with no geo_slop_mm at all.
	repDefault := validator.ValidateBikeGeometry(havokM)
	if !repDefault.IsValid {
		t.Fatalf("Expected Havok M to pass within its rounding budget, got issues: %+v", repDefault.Issues)
	}
	if b := repDefault.A2cBudget; b == nil || b.Tier() != model.TierLengthRounding {
		t.Errorf("Expected Havok M A2C residual to be explained by length rounding, got %+v", b)
	}

	// A declared allowance adds to the rounding budget rather than replacing it.
	havokM.GeoSlopMm = &model.GeoSlopMm{A2c: model.Some(1.5)}
	repRelaxed := validator.ValidateBikeGeometry(havokM)
	if !repRelaxed.IsValid {
		t.Errorf("Expected Havok M with geo_slop_mm={a2c:1.5} to be valid, got issues: %+v", repRelaxed.Issues)
	}
	if b := repRelaxed.A2cBudget; b == nil || b.Allowance != 1.5 || math.Abs(b.Tol()-(b.RoundingTol()+1.5)) > 1e-12 {
		t.Errorf("Expected a2c allowance 1.5 added to the rounding budget, got %+v", b)
	}

	// Negative allowances are rejected.
	havokM.GeoSlopMm = &model.GeoSlopMm{Wheelbase: model.Some(-0.5)}
	if validator.ValidateBikeGeometry(havokM).IsValid {
		t.Errorf("Expected Havok M with geo_slop_mm.wheelbase=-0.5 to fail validation")
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
		GeoHeadsetSpacerMm: 12.0,
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
		SagPct: 20.0,
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

	// 3. Missing sag_pct on suspension fork with sagged A2C
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
		SagPct: 20.0,
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
		SagPct: 20.0,
		A2cMm:  506.0,
		RakeMm: 51.0,
	}
	issues = validator.ValidateForkRef("test-frame", "cad_fork", unsupportedRakeRef)
	if len(issues) == 0 || issues[0].ID != "unsupported_cad_fork_rake" {
		t.Errorf("Expected unsupported_cad_fork_rake issue, got %+v", issues)
	}
}

func TestValidateRevelElJefe(t *testing.T) {
	jefe := frames.RevelElJefe.Clone()

	report := validator.ValidateFrame(jefe)
	if !report.IsValid {
		t.Fatalf("Expected Revel El Jefe to pass validation: %s", report.ErrorSummary())
	}
	if jefe.ActualStaDeg <= 0 {
		t.Fatalf("Expected Revel El Jefe to have bent seat tube (ActualStaDeg > 0)")
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
		GeoHeadsetSpacerMm: 13.0,
		Sizes:              []string{"S", "M", "L", "XL"},
		DefaultSize:        "L",
		CadFork: model.ForkRef{
			Fork:   "sunliner_440",
			A2cMm:  440.0,
			RakeMm: 55.0,
		},
		Forks: []model.ForkRef{
			{Fork: "sunliner_440", A2cMm: 440.0, RakeMm: 55.0},
		},
		MaxSpacerMm:   40.0,
		CanonicalStem: model.StemSpec{LengthMm: 60.0, AngleDeg: -6.0},
		Stems:         []string{"mtb"},
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

func TestValidateForks(t *testing.T) {
	// Frame with missing forks
	missingForksFrame := model.FrameModel{
		ID:                 "no-forks-frame",
		Manufacturer:       "Test",
		Model:              "NoForks",
		WheelSize:          "29",
		HeadsetCupType:     model.CupEC44,
		GeoHeadsetSpacerMm: 10.0,
		Sizes:              []string{"L"},
		DefaultSize:        "L",
		CadFork: model.ForkRef{
			Fork:   "fox34_120",
			SagPct: 20.0,
			A2cMm:  506.0,
			RakeMm: 44.0,
		},
		Forks:         nil,
		MaxSpacerMm:   40.0,
		CanonicalStem: model.StemSpec{LengthMm: 60.0, AngleDeg: -6.0},
		Stems:         []string{"mtb"},
		Geometries: map[string]model.SizeGeometry{
			"L": {Reach: 456, Stack: 657, HtaDeg: 67, StaDeg: 74.5, Htl: 145, BbDrop: 60, Wheelbase: 1208, ChainstayLength: 455},
		},
	}

	report := validator.ValidateFrame(missingForksFrame)
	if report.IsValid {
		t.Fatalf("Expected frame with missing forks to fail validation")
	}

	foundIssue := false
	for _, iss := range report.Issues {
		if iss.ID == "missing_forks" {
			foundIssue = true
			break
		}
	}
	if !foundIssue {
		t.Errorf("Expected issue 'missing_forks', got: %+v", report.Issues)
	}
}

func TestAllFrameData(t *testing.T) {
	allFrames := frames.AllFrames()
	if len(allFrames) == 0 {
		t.Fatalf("Expected at least one frame in pkg/frames, found 0")
	}

	for _, frame := range allFrames {
		// 1. Verify FrameModel validation passes
		report := validator.ValidateFrame(frame)
		if !report.IsValid {
			t.Errorf("Frame %s failed validation: %s", frame.ID, report.ErrorSummary())
		}

		// 2. Verify all declared sizes materialize and validate
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

		// 3. Verify CAD fork specification is populated and TravelMm matches catalog
		if frame.CadFork.Fork == "" || frame.CadFork.A2cMm <= 0 {
			t.Errorf("Frame %s has invalid cad_fork: %+v", frame.ID, frame.CadFork)
		}
		if entry, ok := catalog.GetCatalogFork(frame.CadFork.Fork); ok && frame.CadFork.TravelMm != entry.TravelMm {
			t.Errorf("Frame %s CadFork %q TravelMm = %.0f, catalog has %.0f", frame.ID, frame.CadFork.Fork, frame.CadFork.TravelMm, entry.TravelMm)
		}

		// 4. Verify forks
		if len(frame.Forks) == 0 {
			t.Errorf("Frame %s missing forks", frame.ID)
		}
		for _, f := range frame.Forks {
			if entry, ok := catalog.GetCatalogFork(f.Fork); ok && f.TravelMm != entry.TravelMm {
				t.Errorf("Frame %s Fork %q TravelMm = %.0f, catalog has %.0f", frame.ID, f.Fork, f.TravelMm, entry.TravelMm)
			}
		}
		if _, _, ok := frame.EffectiveA2cRange(); !ok {
			t.Errorf("Frame %s cannot derive valid A2C range", frame.ID)
		}

		// 5. Verify rigid capability matches field
		if frame.SupportsRigid() && frame.GetRigidA2c() <= 0 {
			t.Errorf("Frame %s supports rigid but has rigid A2C <= 0", frame.ID)
		}

		// 6. Verify adjustable chainstay ranges if present
		if frame.HasAdjustableChainstay() {
			if frame.GetMinChainstay() >= frame.GetMaxChainstay() {
				t.Errorf("Frame %s has invalid chainstay range [%.1f, %.1f]", frame.ID, frame.GetMinChainstay(), frame.GetMaxChainstay())
			}
		}

		// 7. Verify custom GeoSlopMm overrides provide an explicit Reason
		if frame.GeoSlopMm != nil && frame.GeoSlopMm.HasCustomSlop() && strings.TrimSpace(frame.GeoSlopMm.Reason) == "" {
			t.Errorf("Frame %s sets custom GeoSlopMm without an explicit Reason", frame.ID)
		}

		// 8. Verify non-zero StandoverHeight is physically plausible (> 600 mm)
		for _, sz := range frame.GetSizes() {
			if geo, ok := frame.Geometries[sz]; ok && geo.StandoverHeight != 0 && geo.StandoverHeight <= 600.0 {
				t.Errorf("Frame %s (size %s) has implausible StandoverHeight %.1f mm (expected > 600 mm or 0 if unpublished)", frame.ID, sz, geo.StandoverHeight)
			}
		}
	}
}

func TestSizeResiduals(t *testing.T) {
	allFrames := frames.AllFrames()
	if len(allFrames) == 0 {
		t.Fatalf("Expected at least one frame in pkg/frames")
	}

	frame := allFrames[0]
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

func TestValidateFrameStemSpacerRequirements(t *testing.T) {
	allFrames := frames.AllFrames()
	if len(allFrames) == 0 {
		t.Fatalf("Expected at least one frame in pkg/frames")
	}

	validFrame := allFrames[0]

	// 1. Valid frame passes
	rep := validator.ValidateFrame(validFrame)
	if !rep.IsValid {
		t.Fatalf("Expected valid frame to pass, got: %s", rep.ErrorSummary())
	}

	// 2. Negative MaxSpacerMm
	fNegSpacer := validFrame.Clone()
	fNegSpacer.MaxSpacerMm = -5.0
	rep = validator.ValidateFrame(fNegSpacer)
	if rep.IsValid || !strings.Contains(rep.ErrorSummary(), "Invalid Maximum Spacer Stack") {
		t.Errorf("Expected Invalid Maximum Spacer Stack error, got: %s", rep.ErrorSummary())
	}

	// 3. Zero/negative CanonicalStem length
	fBadCanon := validFrame.Clone()
	fBadCanon.CanonicalStem = model.StemSpec{LengthMm: 0, AngleDeg: -6}
	rep = validator.ValidateFrame(fBadCanon)
	if rep.IsValid || !strings.Contains(rep.ErrorSummary(), "canonical_stem length_mm must be positive") {
		t.Errorf("Expected canonical stem positive length error, got: %s", rep.ErrorSummary())
	}

	// 4. Missing Stems
	fNoStems := validFrame.Clone()
	fNoStems.Stems = nil
	rep = validator.ValidateFrame(fNoStems)
	if rep.IsValid || !strings.Contains(rep.ErrorSummary(), "Missing Stems") {
		t.Errorf("Expected Missing Stems error, got: %s", rep.ErrorSummary())
	}

	// 5. CanonicalStem not included in Stems
	fCanonNotInStems := validFrame.Clone()
	fCanonNotInStems.CanonicalStem = model.StemSpec{LengthMm: 125, AngleDeg: -17}
	rep = validator.ValidateFrame(fCanonNotInStems)
	if rep.IsValid || !strings.Contains(rep.ErrorSummary(), "Canonical Stem Not in Stems List") {
		t.Errorf("Expected Canonical Stem Not in Stems List error, got: %s", rep.ErrorSummary())
	}

	// 6. Unknown Stem Set in Stems
	fUnknownStem := validFrame.Clone()
	fUnknownStem.Stems = []string{"nonexistent_stem"}
	rep = validator.ValidateFrame(fUnknownStem)
	if rep.IsValid || !strings.Contains(rep.ErrorSummary(), "not defined in data/stems.json") {
		t.Errorf("Expected Unknown Stem Set error, got: %s", rep.ErrorSummary())
	}

	// 7. Invalid ExpectedStemRangeMm
	fBadRange := validFrame.Clone()
	fBadRange.ExpectedStemRangeMm = []float64{100, 60}
	rep = validator.ValidateFrame(fBadRange)
	if rep.IsValid || !strings.Contains(rep.ErrorSummary(), "Invalid Expected Stem Range") {
		t.Errorf("Expected Invalid Expected Stem Range error, got: %s", rep.ErrorSummary())
	}

	// 8. CanonicalStem outside ExpectedStemRangeMm
	fCanonOutsideRange := validFrame.Clone()
	fCanonOutsideRange.ExpectedStemRangeMm = []float64{70, 100}
	// validFrame has CanonicalStem 60mm
	rep = validator.ValidateFrame(fCanonOutsideRange)
	if rep.IsValid || !strings.Contains(rep.ErrorSummary(), "Canonical Stem Outside Expected Range") {
		t.Errorf("Expected Canonical Stem Outside Expected Range error, got: %s", rep.ErrorSummary())
	}

	// 9. Valid ExpectedStemRangeMm
	fValidRange := validFrame.Clone()
	fValidRange.ExpectedStemRangeMm = []float64{50, 90}
	rep = validator.ValidateFrame(fValidRange)
	if !rep.IsValid {
		t.Errorf("Expected valid frame with ExpectedStemRangeMm [50, 90], got: %s", rep.ErrorSummary())
	}
}

func TestValidateStackReachSagHint(t *testing.T) {
	// Raw Seven Cycles Sola Slang 2924 published geometry (Size S 57cm). Stack (650mm) matches a
	// 544mm static fork with a 12mm EC44 cup, but Reach (411mm) is ~19mm longer than Stack,
	// HTA, and the published horizontal Front Center (711mm) / Wheelbase (1158mm) allow.
	rawSeven := model.BikeGeometry{
		ID:                 "seven-sola-slang-raw-s57",
		Manufacturer:       "Seven",
		Model:              "Sola Slang 2924",
		Size:               "S 57cm",
		Reach:              411.0,
		Stack:              650.0,
		HtaDeg:             65.0,
		StaDeg:             76.75,
		Htl:                104.0,
		BbDrop:             70.0,
		ForkA2c:            544.0,
		ForkRake:           44.0,
		HeadsetCupType:     model.CupEC44,
		GeoHeadsetSpacerMm: 12.0,
		FrontCenter:        714.4, // 3D BB-to-front-axle equivalent of 711.0mm horizontal
		Wheelbase:          1158.0,
		ChainstayLength:    452.0,
	}

	countIssues := func(rep model.ValidationReport, id string, sev model.Severity) int {
		n := 0
		for _, issue := range rep.Issues {
			if issue.ID == id && issue.Severity == sev {
				n++
			}
		}
		return n
	}

	// Both Front Center and Wheelbase fail; the sag hint is emitted exactly once, as a warning.
	report := validator.ValidateBikeGeometry(rawSeven)
	if report.IsValid {
		t.Fatalf("Expected raw mixed-sag geometry to fail validation, but it passed")
	}
	if countIssues(report, "front_center_mismatch", model.SeverityError) != 1 {
		t.Errorf("Expected front_center_mismatch error, got issues: %+v", report.Issues)
	}
	if countIssues(report, "wheelbase_discrepancy", model.SeverityError) != 1 {
		t.Errorf("Expected wheelbase_discrepancy error, got issues: %+v", report.Issues)
	}
	if n := countIssues(report, "stack_reach_sag_hint", model.SeverityWarning); n != 1 {
		t.Errorf("Expected exactly one stack_reach_sag_hint warning, got %d: %+v", n, report.Issues)
	}

	// Without Front Center, the hint is derived from the Wheelbase residual instead.
	rawSevenNoFc := rawSeven
	rawSevenNoFc.FrontCenter = 0
	repNoFc := validator.ValidateBikeGeometry(rawSevenNoFc)
	if repNoFc.IsValid {
		t.Fatalf("Expected raw mixed-sag geometry without FrontCenter to fail via Wheelbase discrepancy")
	}
	if countIssues(repNoFc, "stack_reach_sag_hint", model.SeverityWarning) != 1 {
		t.Errorf("Expected stack_reach_sag_hint warning from Wheelbase residual, got issues: %+v", repNoFc.Issues)
	}

	// A 2mm Reach residual on an integer-degree chart (HTA 65 → ±0.5°, worth about ±6mm of
	// Front Center) is within what rounding can produce: it passes, but only by assuming the
	// head angle is rounded, and says so. This is the documented limit of a worst-case budget.
	smallResidual := rawSeven
	smallResidual.Reach = 394.0
	repSmall := validator.ValidateBikeGeometry(smallResidual)
	if !repSmall.IsValid {
		t.Fatalf("Expected 2mm Reach residual to pass within the integer-degree rounding budget, got: %+v", repSmall.Issues)
	}
	if countIssues(repSmall, "front_center_needs_angle_rounding", model.SeverityInfo) != 1 {
		t.Errorf("Expected front_center_needs_angle_rounding notice for 2mm Reach residual, got issues: %+v", repSmall.Issues)
	}

	// On a quarter-degree chart (as Seven publishes) a 3.5mm Reach residual exceeds the budget
	// and fails, but its equivalent A2C offset (< 10mm) is too small to attribute to sag.
	quarterDeg := rawSeven
	quarterDeg.Reach = 395.7
	quarterDeg.ChartPrecision.HtaDeg = 0.125
	repQuarter := validator.ValidateBikeGeometry(quarterDeg)
	if repQuarter.IsValid {
		t.Fatalf("Expected 3.5mm Reach residual on a quarter-degree chart to fail Front Center validation")
	}
	if countIssues(repQuarter, "front_center_mismatch", model.SeverityError) != 1 {
		t.Errorf("Expected front_center_mismatch error, got issues: %+v", repQuarter.Issues)
	}
	if countIssues(repQuarter, "stack_reach_sag_hint", model.SeverityWarning) != 0 {
		t.Errorf("Did not expect stack_reach_sag_hint for small residual, got issues: %+v", repQuarter.Issues)
	}

	// Internally consistent Reach passes under the 3D Front Center convention.
	correctedSeven := rawSeven
	correctedSeven.Reach = 392.2
	correctedReport := validator.ValidateBikeGeometry(correctedSeven)
	if !correctedReport.IsValid {
		t.Fatalf("Expected corrected internally consistent geometry to pass validation, got issues: %+v", correctedReport.Issues)
	}
	if countIssues(correctedReport, "stack_reach_sag_hint", model.SeverityWarning) != 0 {
		t.Errorf("Did not expect stack_reach_sag_hint for consistent geometry, got issues: %+v", correctedReport.Issues)
	}
}

func TestValidateSaggedFrameStaticA2cLabel(t *testing.T) {
	// F-p4d-1: Sagged-CAD frames must display their true static A2C (e.g. 530 mm for Revel El Jefe)
	// in both the summary table ("Stated A2C") and detail table ("Published Fork A2C (Static)").
	doc, allValid, err := validator.BuildValidationDocument([]validator.Target{
		{Frame: frames.RevelElJefe, Size: frames.RevelElJefe.DefaultSize},
		{Frame: frames.SurlyECR, Size: frames.SurlyECR.DefaultSize},
	})
	if err != nil {
		t.Fatalf("BuildValidationDocument failed: %v", err)
	}
	if !allValid {
		t.Fatalf("Expected all frames valid")
	}

	var summaryStated string
	for _, row := range doc.Tables()[0].Rows {
		if row.Cells[0].Text == "revel-el-jefe" {
			summaryStated = row.Cells[4].Text
		}
	}
	if summaryStated != "530 mm" {
		t.Errorf("Summary Stated A2C for revel-el-jefe = %q, want %q", summaryStated, "530 mm")
	}

	detailDoc, valid, err := validator.BuildValidationDocument([]validator.Target{
		{Frame: frames.RevelElJefe, Size: frames.RevelElJefe.DefaultSize},
	})
	if err != nil || !valid {
		t.Fatalf("BuildValidationDocument detail failed: valid=%v err=%v", valid, err)
	}
	var detailStated string
	for _, row := range detailDoc.Tables()[1].Rows {
		if row.Cells[0].Text == "Published Fork A2C (Static)" {
			detailStated = row.Cells[1].Text
		}
	}
	if detailStated != "530 mm" {
		t.Errorf("Detail Published Fork A2C (Static) for revel-el-jefe = %q, want %q", detailStated, "530 mm")
	}
}

func TestFrontCenterConventionConsistent(t *testing.T) {
	// F-p4d-2: Front Center validation must always use 3D convention (matching Calculated Front Center (3D)
	// and fit), never silently flipping to horizontal on sklar-pbj:XL (3D residual = 2.7mm).
	bike, err := geometry.MaterializeBikeGeometry(frames.SklarPBJ, "XL")
	if err != nil {
		t.Fatalf("MaterializeBikeGeometry failed: %v", err)
	}
	rep := validator.ValidateBikeGeometry(bike)
	if !rep.IsValid {
		t.Fatalf("Expected sklar-pbj:XL to pass, got: %+v", rep.Issues)
	}
	if fcDiff, ok := rep.FrontCenterDiff.Get(); !ok || fcDiff != 2.7 {
		t.Fatalf("Expected FrontCenterDiff 2.7mm for sklar-pbj:XL, got %v", rep.FrontCenterDiff)
	}
	var fcNotice string
	for _, iss := range rep.Issues {
		if iss.ID == "front_center_ok" {
			fcNotice = iss.Message
		}
	}
	if !strings.Contains(fcNotice, "3D") || !strings.Contains(fcNotice, "2.7mm") {
		t.Errorf("Expected 3D Front Center notice within 2.7mm, got: %q", fcNotice)
	}

	// The 2.7mm 3D residual is within the Front Center rounding budget (half-degree HTA column),
	// so it needs no Front Center allowance.
	bike.GeoSlopMm = nil
	repNoAllowance := validator.ValidateBikeGeometry(bike)
	if b := repNoAllowance.FrontCenterBudget; b == nil || !b.Within() || b.Saturation() > 1 {
		t.Fatalf("Expected sklar-pbj:XL 2.7mm 3D FC residual within its rounding budget, got %+v", b)
	}
}

func TestRoundingBudgetRows(t *testing.T) {
	// The validation proof reports each closure check's propagated rounding budget and the
	// declared allowance; there is no fixed wheelbase floor and no "cross:" tolerance.
	rowsOf := func(f model.FrameModel) map[string]string {
		t.Helper()
		doc, valid, err := validator.BuildValidationDocument([]validator.Target{{Frame: f, Size: f.DefaultSize}})
		if err != nil || !valid {
			t.Fatalf("BuildValidationDocument(%s) failed: valid=%v err=%v", f.ID, valid, err)
		}
		rows := map[string]string{}
		for _, row := range doc.Tables()[1].Rows {
			rows[row.Cells[0].Text] = row.Cells[1].Text
		}
		return rows
	}

	for _, f := range []model.FrameModel{frames.OtsoFenrirStainless, frames.SurlyECR} {
		rows := rowsOf(f)
		for _, label := range []string{"Rounding Budget: Stack / A2C", "Rounding Budget: Wheelbase"} {
			v, ok := rows[label]
			if !ok {
				t.Errorf("%s: missing %q row", f.ID, label)
				continue
			}
			if !strings.Contains(v, "% used") || strings.Contains(v, "cross:") {
				t.Errorf("%s: unexpected %q value %q", f.ID, label, v)
			}
		}
		if _, ok := rows["Allowed Slop Tolerances (geo_slop_mm)"]; ok {
			t.Errorf("%s: obsolete Allowed Slop Tolerances row present", f.ID)
		}
	}
	for _, f := range []model.FrameModel{frames.SevenExpedition926d, frames.MootsMountaineer} {
		rows := rowsOf(f)
		if v, ok := rows["Rounding Budget: Stack / A2C"]; !ok || !strings.Contains(v, "% used") {
			t.Errorf("%s: unexpected Stack / A2C budget row: %q (ok=%v)", f.ID, v, ok)
		}
		if _, ok := rows["Rounding Budget: Wheelbase"]; ok {
			t.Errorf("%s: unexpected Rounding Budget: Wheelbase when Wheelbase is omitted", f.ID)
		}
	}
	if got := rowsOf(frames.SurlyECR)["Declared Allowance (geo_slop_mm)"]; got != "none (rounding budget only)" {
		t.Errorf("surly-ecr declared allowance = %q, want none", got)
	}

	// Quarter-degree HTA (Fenrir) gives a much tighter wheelbase budget than integer HTA (ECR).
	fenrirWb := rowsOf(frames.OtsoFenrirStainless)["Rounding Budget: Wheelbase"]
	ecrWb := rowsOf(frames.SurlyECR)["Rounding Budget: Wheelbase"]
	if !strings.Contains(fenrirWb, "HTA ±1.") || !strings.Contains(ecrWb, "HTA ±5.") {
		t.Errorf("expected quarter-degree vs integer-degree HTA terms, got fenrir=%q ecr=%q", fenrirWb, ecrWb)
	}

	for _, v := range []float64{-1.5, 10.5} {
		badFrame := frames.SurlyECR.Clone()
		badFrame.GeoSlopMm = &model.GeoSlopMm{Wheelbase: model.Some(v), Reason: "out of range"}
		if validator.ValidateFrame(badFrame).IsValid {
			t.Errorf("Expected ValidateFrame to reject GeoSlopMm.Wheelbase=%.1f", v)
		}
	}
	okFrame := frames.SurlyECR.Clone()
	okFrame.GeoSlopMm = &model.GeoSlopMm{Wheelbase: model.Some(1.5), Reason: "small allowance"}
	if rep := validator.ValidateFrame(okFrame); !rep.IsValid {
		t.Errorf("Expected GeoSlopMm.Wheelbase=1.5 to be accepted now that there is no floor: %s", rep.ErrorSummary())
	}
}

func TestDeclaredModeLabel(t *testing.T) {
	// F-p4d-4: Detail table uses "CAD Convention" matching the summary table rather than "Inferred Published Mode".
	doc, valid, err := validator.BuildValidationDocument([]validator.Target{
		{Frame: frames.EskerHayduke, Size: frames.EskerHayduke.DefaultSize},
	})
	if err != nil || !valid {
		t.Fatalf("BuildValidationDocument(esker-hayduke) failed: valid=%v err=%v", valid, err)
	}
	var modeVal string
	for _, row := range doc.Tables()[1].Rows {
		if row.Cells[0].Text == "Inferred Published Mode" {
			t.Errorf("Unexpected legacy label 'Inferred Published Mode' in detail table")
		}
		if row.Cells[0].Text == "CAD Convention" {
			modeVal = row.Cells[1].Text
		}
	}
	if modeVal != "Sagged CAD" {
		t.Errorf("CAD Convention for esker-hayduke = %q, want %q", modeVal, "Sagged CAD")
	}
}

func TestSizeResidualsUsesDeclaredCadFork(t *testing.T) {
	// F-p4ab-5: SizeResiduals must evaluate FC calc and WB calc at the declared CAD fork + spacer
	// (502mm structural A2C on binary-havok) rather than Stack-implied A2C.
	residuals, err := validator.SizeResiduals(frames.BinaryHavok)
	if err != nil {
		t.Fatalf("SizeResiduals(binary-havok) failed: %v", err)
	}
	var m *validator.SizeResidual
	for i := range residuals {
		if residuals[i].Size == "M" {
			m = &residuals[i]
			break
		}
	}
	if m == nil {
		t.Fatalf("size M not found in binary-havok residuals")
	}
	if m.FcCalc != 657.1 {
		t.Errorf("binary-havok:M FcCalc = %.1f, want 657.1 (declared CAD fork 502mm)", m.FcCalc)
	}
	if m.WbCalc != 1092.7 {
		t.Errorf("binary-havok:M WbCalc = %.1f, want 1092.7 (declared CAD fork 502mm)", m.WbCalc)
	}
	if deltaFc, ok := m.DeltaFc.Get(); !ok || deltaFc != 0.1 {
		t.Errorf("binary-havok:M DeltaFc = %v, want 0.1", m.DeltaFc)
	}
	if deltaWb, ok := m.DeltaWb.Get(); !ok || deltaWb != 0.7 {
		t.Errorf("binary-havok:M DeltaWb = %v, want +0.7", m.DeltaWb)
	}
}

// TestDeclaredAllowancesAreNecessary keeps geo_slop_mm a last resort: every declared
// allowance must be needed by at least one size (the frame fails without it), and must
// not exceed the largest need beyond rounding by more than the 0.5mm rounding-up step.
func TestDeclaredAllowancesAreNecessary(t *testing.T) {
	type field struct {
		name   string
		ptr    func(*model.GeoSlopMm) *model.Opt[float64]
		budget func(model.ValidationReport) *model.ErrorBudget
	}
	fields := []field{
		{"a2c", func(s *model.GeoSlopMm) *model.Opt[float64] { return &s.A2c }, func(r model.ValidationReport) *model.ErrorBudget { return r.A2cBudget }},
		{"front_center", func(s *model.GeoSlopMm) *model.Opt[float64] { return &s.FrontCenter }, func(r model.ValidationReport) *model.ErrorBudget { return r.FrontCenterBudget }},
		{"wheelbase", func(s *model.GeoSlopMm) *model.Opt[float64] { return &s.Wheelbase }, func(r model.ValidationReport) *model.ErrorBudget { return r.WheelbaseBudget }},
	}
	for _, frame := range frames.AllFrames() {
		if frame.GeoSlopMm == nil {
			continue
		}
		for _, f := range fields {
			declared, ok := (*f.ptr(frame.GeoSlopMm)).Get()
			if !ok {
				continue
			}
			without := frame.Clone()
			slop := *frame.GeoSlopMm
			*f.ptr(&slop) = model.Opt[float64]{}
			without.GeoSlopMm = &slop
			rep := validator.ValidateFrame(without)
			if rep.IsValid {
				t.Errorf("%s: geo_slop_mm.%s = %.1fmm is unnecessary; every size passes within its rounding budget", frame.ID, f.name, declared)
				continue
			}
			need := 0.0
			for _, sr := range rep.SizeReports {
				if b := f.budget(sr); b != nil {
					need = math.Max(need, b.NeededAllowance())
				}
			}
			if declared > need+0.5+1e-9 {
				t.Errorf("%s: geo_slop_mm.%s = %.1fmm exceeds the %.2fmm needed beyond rounding by more than 0.5mm", frame.ID, f.name, declared, need)
			}
		}
	}
}

func TestRegistryGeometricInvariants(t *testing.T) {
	profile := catalog.DefaultRiderProfile()
	saddle := catalog.DefaultSaddle()

	frameSetups := func(t *testing.T, frame model.FrameModel) []model.ComparisonSetup {
		t.Helper()
		setups, err := canonicalizer.ExpandPlausibleForkSetups(frame, model.Opt[float64]{})
		if err != nil {
			t.Fatalf("ExpandPlausibleForkSetups(%s): %v", frame.ID, err)
		}
		cadRake := frame.CadFork.RakeMm
		if cadRake <= 0 {
			cadRake = 44.0
		}
		hasCad := false
		for _, s := range setups {
			if math.Abs(s.A2cMm-frame.CadFork.A2cMm) < 1e-6 && math.Abs(s.RakeMm-cadRake) < 1e-6 {
				hasCad = true
				break
			}
		}
		if !hasCad {
			setups = append(setups, model.ComparisonSetup{
				FrameID:      frame.ID,
				A2cMm:        frame.CadFork.A2cMm,
				RakeMm:       cadRake,
				SagPct:       frame.CadFork.SagPct,
				DisplayLabel: "CAD fork",
			})
		}
		return setups
	}

	t.Run("RigidDistancesInvariantUnderPitch", func(t *testing.T) {
		for _, frame := range frames.AllFrames() {
			setups := frameSetups(t, frame)
			for _, sz := range frame.GetSizes() {
				bike, err := geometry.MaterializeBikeGeometry(frame, sz)
				if err != nil {
					t.Fatalf("MaterializeBikeGeometry(%s, %s): %v", frame.ID, sz, err)
				}
				bb := geom2d.Origin
				u0 := geometry.SteererTubeDir(bike.HtaDeg)
				ht0 := geom2d.Point2{X: bike.Reach, Y: bike.Stack}
				htb0 := ht0.SubVec(u0.Scale(bike.Htl))
				ra0 := geom2d.Point2{X: -geometry.RearCenterHoriz(bike.ChainstayLength, bike.BbDrop), Y: bike.BbDrop}

				wantHtBb := ht0.DistanceTo(bb)
				wantHtbBb := htb0.DistanceTo(bb)
				wantBbRa := ra0.DistanceTo(bb)
				wantHtHtb := ht0.DistanceTo(htb0)
				wantNominalIncluded := bike.HtaDeg - bike.StaDeg

				cadRes, err := geometry.AnalyzeBikeFitWithSaddle(bike, model.Some(bike.CadFork.A2cMm), model.Some(bike.CadFork.RakeMm), profile, saddle)
				if err != nil {
					t.Fatalf("AnalyzeBikeFitWithSaddle(%s %s CAD): %v", frame.ID, sz, err)
				}
				wantEffIncluded := cadRes.ActiveHta - cadRes.ActiveSta

				for _, s := range setups {
					for _, spacer := range []float64{bike.GeoHeadsetSpacerMm, bike.GeoHeadsetSpacerMm + 12.0} {
						b := bike
						b.GeoHeadsetSpacerMm = spacer
						res, err := geometry.AnalyzeBikeFitWithSaddle(b, model.Some(s.A2cMm), model.Some(s.RakeMm), profile, saddle)
						if err != nil {
							t.Fatalf("AnalyzeBikeFitWithSaddle(%s %s %s spacer=%.1f): %v", frame.ID, sz, s.DisplayLabel, spacer, err)
						}
						uAct := geometry.SteererTubeDir(res.ActiveHta)
						htAct := geom2d.Point2{X: res.ActiveReach, Y: res.ActiveStack}
						htbAct := htAct.SubVec(uAct.Scale(bike.Htl))
						raAct := geom2d.Point2{X: -geometry.RearCenterHoriz(bike.ChainstayLength, res.ActiveBbDrop), Y: res.ActiveBbDrop}

						if d := math.Abs(htAct.DistanceTo(bb) - wantHtBb); d > 1e-6 {
							t.Errorf("%s %s (%s): |HT-BB| drifted by %.9f mm", frame.ID, sz, s.DisplayLabel, d)
						}
						if d := math.Abs(htbAct.DistanceTo(bb) - wantHtbBb); d > 1e-6 {
							t.Errorf("%s %s (%s): |HTB-BB| drifted by %.9f mm", frame.ID, sz, s.DisplayLabel, d)
						}
						if d := math.Abs(raAct.DistanceTo(bb) - wantBbRa); d > 1e-6 {
							t.Errorf("%s %s (%s): |BB-RA| drifted by %.9f mm", frame.ID, sz, s.DisplayLabel, d)
						}
						if d := math.Abs(htAct.DistanceTo(htbAct) - wantHtHtb); d > 1e-6 {
							t.Errorf("%s %s (%s): |HT-HTB| drifted by %.9f mm", frame.ID, sz, s.DisplayLabel, d)
						}
						if d := math.Abs(wantHtHtb - bike.Htl); d > 1e-9 {
							t.Errorf("%s %s (%s): |HT-HTB| != HTL by %.9f mm", frame.ID, sz, s.DisplayLabel, d)
						}
						nominalStaAct := bike.StaDeg - res.DeltaAngleDeg
						if d := ((res.ActiveHta - nominalStaAct) - wantNominalIncluded).Abs(); d > 1e-6 {
							t.Errorf("%s %s (%s): nominal (HTA-STA) drifted by %.9f deg", frame.ID, sz, s.DisplayLabel, d)
						}
						if d := ((res.ActiveHta - res.ActiveSta) - wantEffIncluded).Abs(); d > 1e-6 {
							t.Errorf("%s %s (%s): effective (HTA-STA) drifted by %.9f deg", frame.ID, sz, s.DisplayLabel, d)
						}
					}
				}
			}
		}
	})

	t.Run("CADForkReproducesPublishedChart", func(t *testing.T) {
		for _, frame := range frames.AllFrames() {
			for _, sz := range frame.GetSizes() {
				bike, err := geometry.MaterializeBikeGeometry(frame, sz)
				if err != nil {
					t.Fatalf("MaterializeBikeGeometry(%s, %s): %v", frame.ID, sz, err)
				}
				cadBike := bike
				cadBike.GeoHeadsetSpacerMm = bike.HeadsetCupType.CupStackMm()
				res, err := geometry.AnalyzeBikeFitWithSaddle(cadBike, model.Some(cadBike.CadFork.A2cMm), model.Some(cadBike.CadFork.RakeMm), profile, saddle)
				if err != nil {
					t.Fatalf("AnalyzeBikeFitWithSaddle(%s %s): %v", frame.ID, sz, err)
				}
				if d := math.Abs(res.ActiveReach - bike.Reach); d > 1e-6 {
					t.Errorf("%s %s: Reach diff %.9f mm", frame.ID, sz, d)
				}
				if d := math.Abs(res.ActiveStack - bike.Stack); d > 1e-6 {
					t.Errorf("%s %s: Stack diff %.9f mm", frame.ID, sz, d)
				}
				if d := (res.ActiveHta - bike.HtaDeg).Abs(); d > 1e-6 {
					t.Errorf("%s %s: HTA diff %.9f deg", frame.ID, sz, d)
				}
				nominalSta := bike.StaDeg - res.DeltaAngleDeg
				if d := (nominalSta - bike.StaDeg).Abs(); d > 1e-6 {
					t.Errorf("%s %s: nominal STA diff %.9f deg", frame.ID, sz, d)
				}
				if !bike.HasBentSeatTube() {
					if d := (res.ActiveSta - bike.StaDeg).Abs(); d > 1e-6 {
						t.Errorf("%s %s: ActiveSta diff %.9f deg", frame.ID, sz, d)
					}
				}
				if d := math.Abs(res.ActiveBbDrop - bike.BbDrop); d > 1e-6 {
					t.Errorf("%s %s: BbDrop diff %.9f mm", frame.ID, sz, d)
				}
				if bike.HeadsetCupType.CupStackMm() > bike.GeoHeadsetSpacerMm {
					physRes, err := geometry.AnalyzeBikeFitWithSaddle(bike, model.Some(bike.CadFork.A2cMm), model.Some(bike.CadFork.RakeMm), profile, saddle)
					if err != nil {
						t.Fatalf("AnalyzeBikeFitWithSaddle(%s %s physical cup): %v", frame.ID, sz, err)
					}
					if !(physRes.ActiveHta < bike.HtaDeg) {
						t.Errorf("%s %s: physical lower cup %.1fmm > CAD %.1fmm did not slacken HTA: %.4f -> %.4f",
							frame.ID, sz, bike.HeadsetCupType.CupStackMm(), bike.GeoHeadsetSpacerMm, bike.HtaDeg, physRes.ActiveHta)
					}
				}

				htaRad := bike.HtaDeg.Angle()
				u := geometry.SteererTubeDir(bike.HtaDeg)
				n := u.Perpendicular(geom2d.Clockwise)
				lChart := (bike.Stack - bike.BbDrop + bike.CadFork.RakeMm*htaRad.Cos()) / htaRad.Sin()
				faChart := geom2d.Point2{X: bike.Reach, Y: bike.Stack}.SubVec(u.Scale(lChart)).Add(n.Scale(bike.CadFork.RakeMm))
				ra0 := geom2d.Point2{X: -geometry.RearCenterHoriz(bike.ChainstayLength, bike.BbDrop), Y: bike.BbDrop}

				if d := math.Abs(faChart.Y - bike.BbDrop); d > 1e-9 {
					t.Errorf("%s %s: FA_chart.Y=%.9f != BbDrop=%.9f", frame.ID, sz, faChart.Y, bike.BbDrop)
				}

				a2cBudget, ok := geometry.A2cBudget(bike)
				if !ok || !a2cBudget.Within() {
					t.Errorf("%s %s: A2cBudget ok=%v within=%v (%+v)", frame.ID, sz, ok, a2cBudget.Within(), a2cBudget)
				} else {
					lDecl := bike.CadFork.A2cMm + bike.GeoHeadsetSpacerMm + bike.Htl
					if d := math.Abs((lDecl - lChart) - a2cBudget.Residual); d > 1e-9 {
						t.Errorf("%s %s: declared-fork A2C residual %.9f != A2cBudget.Residual %.9f", frame.ID, sz, lDecl-lChart, a2cBudget.Residual)
					}
				}
				if bike.FrontCenter > 0 {
					fcBudget, ok := geometry.FrontCenterBudget(bike)
					if !ok || !fcBudget.Within() {
						t.Errorf("%s %s: FrontCenterBudget ok=%v within=%v (%+v)", frame.ID, sz, ok, fcBudget.Within(), fcBudget)
					} else if d := math.Abs((faChart.DistanceToOrigin() - bike.FrontCenter) - fcBudget.Residual); d > 1e-9 {
						t.Errorf("%s %s: chart FC residual %.9f != FrontCenterBudget.Residual %.9f", frame.ID, sz, faChart.DistanceToOrigin()-bike.FrontCenter, fcBudget.Residual)
					}
				}
				if bike.Wheelbase > 0 {
					wbBudget, ok := geometry.WheelbaseBudget(bike)
					if !ok || !wbBudget.Within() {
						t.Errorf("%s %s: WheelbaseBudget ok=%v within=%v (%+v)", frame.ID, sz, ok, wbBudget.Within(), wbBudget)
					} else if d := math.Abs(((faChart.X - ra0.X) - bike.Wheelbase) - wbBudget.Residual); d > 1e-9 {
						t.Errorf("%s %s: chart WB residual %.9f != WheelbaseBudget.Residual %.9f", frame.ID, sz, (faChart.X-ra0.X)-bike.Wheelbase, wbBudget.Residual)
					}
				}
			}
		}
	})

	t.Run("CupAndA2cAreInterchangeable", func(t *testing.T) {
		const extraMm = 10.0
		for _, frame := range frames.AllFrames() {
			setups := frameSetups(t, frame)
			for _, sz := range frame.GetSizes() {
				bike, err := geometry.MaterializeBikeGeometry(frame, sz)
				if err != nil {
					t.Fatalf("MaterializeBikeGeometry(%s, %s): %v", frame.ID, sz, err)
				}
				for _, s := range setups {
					fitBase, err := geometry.AnalyzeBikeFitWithSaddle(bike, model.Some(s.A2cMm), model.Some(s.RakeMm), profile, saddle)
					if err != nil {
						t.Fatalf("AnalyzeBikeFitWithSaddle base (%s %s %s): %v", frame.ID, sz, s.DisplayLabel, err)
					}
					fitLonger, err := geometry.AnalyzeBikeFitWithSaddle(bike, model.Some(s.A2cMm+extraMm), model.Some(s.RakeMm), profile, saddle)
					if err != nil {
						t.Fatalf("AnalyzeBikeFitWithSaddle longer (%s %s %s): %v", frame.ID, sz, s.DisplayLabel, err)
					}
					if !(fitLonger.ActiveHta < fitBase.ActiveHta) {
						t.Errorf("%s %s (%s): AnalyzeBikeFitWithSaddle longer front end did not slacken HTA: %.4f -> %.4f", frame.ID, sz, s.DisplayLabel, fitBase.ActiveHta, fitLonger.ActiveHta)
					}
					if bike.GeoHeadsetSpacerMm >= extraMm {
						cupBike := bike
						cupBike.GeoHeadsetSpacerMm -= extraMm
						fitCup, err := geometry.AnalyzeBikeFitWithSaddle(cupBike, model.Some(s.A2cMm), model.Some(s.RakeMm), profile, saddle)
						if err != nil {
							t.Fatalf("AnalyzeBikeFitWithSaddle cup (%s %s %s): %v", frame.ID, sz, s.DisplayLabel, err)
						}
						if d := math.Abs(fitCup.ActiveReach - fitLonger.ActiveReach); d > 1e-9 {
							t.Errorf("%s %s (%s): AnalyzeBikeFit Reach cup vs A2C diff %.12f", frame.ID, sz, s.DisplayLabel, d)
						}
						if d := math.Abs(fitCup.ActiveStack - fitLonger.ActiveStack); d > 1e-9 {
							t.Errorf("%s %s (%s): AnalyzeBikeFit Stack cup vs A2C diff %.12f", frame.ID, sz, s.DisplayLabel, d)
						}
						if d := (fitCup.ActiveHta - fitLonger.ActiveHta).Abs(); d > 1e-9 {
							t.Errorf("%s %s (%s): AnalyzeBikeFit HTA cup vs A2C diff %.12f", frame.ID, sz, s.DisplayLabel, d)
						}
						if d := (fitCup.ActiveSta - fitLonger.ActiveSta).Abs(); d > 1e-9 {
							t.Errorf("%s %s (%s): AnalyzeBikeFit STA cup vs A2C diff %.12f", frame.ID, sz, s.DisplayLabel, d)
						}
						if d := math.Abs(fitCup.ActiveBbDrop - fitLonger.ActiveBbDrop); d > 1e-9 {
							t.Errorf("%s %s (%s): AnalyzeBikeFit BbDrop cup vs A2C diff %.12f", frame.ID, sz, s.DisplayLabel, d)
						}
					}
				}
			}
		}
	})

	t.Run("AxlesStayLevel", func(t *testing.T) {
		for _, frame := range frames.AllFrames() {
			setups := frameSetups(t, frame)
			for _, sz := range frame.GetSizes() {
				bike, err := geometry.MaterializeBikeGeometry(frame, sz)
				if err != nil {
					t.Fatalf("MaterializeBikeGeometry(%s, %s): %v", frame.ID, sz, err)
				}
				htaRad := bike.HtaDeg.Angle()
				lChart := (bike.Stack - bike.BbDrop + bike.CadFork.RakeMm*htaRad.Cos()) / htaRad.Sin()

				for _, s := range setups {
					res, err := geometry.AnalyzeBikeFitWithSaddle(bike, model.Some(s.A2cMm), model.Some(s.RakeMm), profile, saddle)
					if err != nil {
						t.Fatalf("AnalyzeBikeFitWithSaddle(%s %s %s): %v", frame.ID, sz, s.DisplayLabel, err)
					}
					dL := (s.A2cMm + bike.HeadsetCupType.CupStackMm()) - (bike.CadFork.A2cMm + bike.GeoHeadsetSpacerMm)
					uAct := geometry.SteererTubeDir(res.ActiveHta)
					nAct := uAct.Perpendicular(geom2d.Clockwise)
					htAct := geom2d.Point2{X: res.ActiveReach, Y: res.ActiveStack}
					faAct := htAct.SubVec(uAct.Scale(lChart + dL)).Add(nAct.Scale(s.RakeMm))
					raAct := geom2d.Point2{X: -geometry.RearCenterHoriz(bike.ChainstayLength, res.ActiveBbDrop), Y: res.ActiveBbDrop}
					axleDelta := math.Abs(faAct.Y - raAct.Y)
					if axleDelta >= 1e-6 {
						t.Errorf("%s %s (%s): |FA.Y - RA.Y| = %.6f mm, want < 1e-6", frame.ID, sz, s.DisplayLabel, axleDelta)
					}
				}
			}
		}
	})
}
