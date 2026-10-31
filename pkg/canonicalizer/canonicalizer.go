package canonicalizer

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"bikegeo/pkg/catalog"
	"bikegeo/pkg/geometry"
	"bikegeo/pkg/model"
	"bikegeo/pkg/validator"
)

// Re-export catalog types and helpers for canonicalizer callers
type ForkSuggestion = catalog.ForkSuggestion

var CommonForkCatalog = catalog.CommonForkCatalog

func GetCatalogFork(id string) (model.ForkCatalogEntry, bool) {
	return catalog.GetCatalogFork(id)
}

func GetForkPreset(id string) (model.ForkCatalogEntry, bool) {
	return catalog.GetCatalogFork(id)
}

func GetStandardForkForTravel(travelMm float64) (model.ForkCatalogEntry, bool) {
	return catalog.GetStandardForkForTravel(travelMm)
}

func FindForkSuggestions(targetA2c, targetRake float64) []ForkSuggestion {
	return catalog.FindForkSuggestions(targetA2c, targetRake)
}

// ResolveForkToken resolves a user comparison modifier (e.g. "120mm", "rigid", "stock", "a2c=506/44")
// against the fork catalog and frame constraints.
func ResolveForkToken(
	token string,
	sagOverride *float64,
	frame model.FrameModel,
) (targetA2c float64, targetRake float64, staticA2c float64, displayLabel string, err error) {
	clean := strings.TrimSpace(token)
	lower := strings.ToLower(clean)

	checkA2cRange := func(a2c float64) error {
		if minA2c, maxA2c, ok := frame.EffectiveA2cRange(); ok {
			if a2c < minA2c-0.1 || a2c > maxA2c+0.1 {
				return fmt.Errorf(
					"frame \"%s %s\" (%s) does not support fork %q (effective A2C %.1fmm outside supported range %s)",
					frame.Manufacturer, frame.Model, frame.ID, token, a2c, strings.Join(frame.GetSupportedForkOptions(), ", "),
				)
			}
		}
		return nil
	}

	// 1. Stock / Default fork
	if lower == "" || lower == "stock" || lower == "default" {
		def := frame.GetDefaultFork()
		if def.A2cMm <= 0 {
			def = frame.CadFork
		}
		targetA2c = def.A2cMm
		targetRake = def.RakeMm
		staticA2c = def.A2cMm
		if def.Fork != "" {
			if entry, ok := GetCatalogFork(def.Fork); ok {
				if entry.TravelMm == 0 && sagOverride != nil && *sagOverride > 0 {
					return 0, 0, 0, "", fmt.Errorf("cannot apply %.0f%% sag to rigid fork on \"%s %s\"; rigid forks have 0mm travel", *sagOverride, frame.Manufacturer, frame.Model)
				}
				if entry.TravelMm > 0 && len(entry.Options) > 0 {
					staticA2c = entry.Options[0].A2cMm
					sag := def.GetSagPct()
					if sag == 0 {
						sag = 20.0
					}
					if sagOverride != nil {
						sag = *sagOverride
					}
					targetA2c = geometry.Round(staticA2c-entry.TravelMm*(sag/100.0), 1)
					displayLabel = FormatSuspensionFork(entry.TravelMm, sag)
					return targetA2c, targetRake, staticA2c, displayLabel, nil
				}
			}
		}
		if frame.IsFixedRigidOnly() && sagOverride != nil && *sagOverride > 0 {
			return 0, 0, 0, "", fmt.Errorf("cannot apply %.0f%% sag to rigid fork on \"%s %s\"; rigid forks have 0mm travel", *sagOverride, frame.Manufacturer, frame.Model)
		}
		if frame.IsFixedRigidOnly() {
			displayLabel = FormatRigidFork(targetA2c)
		} else {
			displayLabel = FormatStockFork()
		}
		return targetA2c, targetRake, staticA2c, displayLabel, nil
	}

	// 2. Rigid fork
	if lower == "rigid" || lower == "0mm" || lower == "0" || strings.HasPrefix(lower, "rigid") {
		if !frame.SupportsRigid() {
			return 0, 0, 0, "", fmt.Errorf("frame \"%s %s\" (%s) does not support a rigid fork", frame.Manufacturer, frame.Model, frame.ID)
		}
		if sagOverride != nil && *sagOverride > 0 {
			return 0, 0, 0, "", fmt.Errorf("cannot apply %.0f%% sag to rigid fork on \"%s %s\"; rigid forks have 0mm travel", *sagOverride, frame.Manufacturer, frame.Model)
		}
		targetA2c = frame.GetRigidA2c()
		targetRake = frame.GetRigidRake()
		staticA2c = targetA2c
		displayLabel = FormatRigidFork(targetA2c)
		return targetA2c, targetRake, staticA2c, displayLabel, nil
	}

	// 3. Direct A2C syntax: a2c=NNN/MM or a2c=NNN
	if strings.HasPrefix(lower, "a2c=") {
		valPart := strings.TrimPrefix(lower, "a2c=")
		if strings.Contains(valPart, "/") {
			parts := strings.SplitN(valPart, "/", 2)
			a2cVal, err := strconv.ParseFloat(parts[0], 64)
			if err != nil {
				return 0, 0, 0, "", fmt.Errorf("invalid A2C in %q: %w", token, err)
			}
			rakeVal, err := strconv.ParseFloat(parts[1], 64)
			if err != nil {
				return 0, 0, 0, "", fmt.Errorf("invalid rake in %q: %w", token, err)
			}
			targetA2c = a2cVal
			targetRake = rakeVal
		} else {
			a2cVal, err := strconv.ParseFloat(valPart, 64)
			if err != nil {
				return 0, 0, 0, "", fmt.Errorf("invalid A2C in %q: %w", token, err)
			}
			targetA2c = a2cVal
			targetRake = frame.CadFork.RakeMm
			if targetRake <= 0 {
				targetRake = 44.0
			}
		}
		staticA2c = targetA2c
		displayLabel = FormatCustomA2cFork(targetA2c, targetRake)
		if err := checkA2cRange(targetA2c); err != nil {
			return 0, 0, 0, "", err
		}
		return targetA2c, targetRake, staticA2c, displayLabel, nil
	}

	// 4. Suspension travel token (e.g. "120mm", "120")
	travelStr := strings.TrimSuffix(lower, "mm")
	if travelVal, err := strconv.ParseFloat(travelStr, 64); err == nil && travelVal >= 60 {
		if frame.IsFixedRigidOnly() {
			return 0, 0, 0, "", fmt.Errorf("frame \"%s %s\" has a fixed rigid fork (%.0fmm A2C) and does not support suspension fork %q", frame.Manufacturer, frame.Model, frame.GetRigidA2c(), token)
		}
		var stdFork model.ForkCatalogEntry
		foundFrameFork := false
		if def := frame.GetDefaultFork(); def.Fork != "" {
			if entry, ok := GetCatalogFork(def.Fork); ok && entry.TravelMm == travelVal && len(entry.Options) > 0 {
				stdFork = entry
				foundFrameFork = true
			}
		}
		if !foundFrameFork {
			if def := frame.CadFork; def.Fork != "" {
				if entry, ok := GetCatalogFork(def.Fork); ok && entry.TravelMm == travelVal && len(entry.Options) > 0 {
					stdFork = entry
					foundFrameFork = true
				}
			}
		}
		if !foundFrameFork {
			f, ok := GetStandardForkForTravel(travelVal)
			if !ok {
				return 0, 0, 0, "", fmt.Errorf("no catalog fork found for %.0fmm suspension travel", travelVal)
			}
			stdFork = f
		}
		if len(stdFork.Options) == 0 {
			return 0, 0, 0, "", fmt.Errorf("fork %s has no configured options", stdFork.ID)
		}
		opt := stdFork.Options[0]
		staticA2c = opt.A2cMm
		targetRake = opt.RakeMm
		sag := 20.0
		if sagOverride != nil {
			sag = *sagOverride
		}
		targetA2c = geometry.Round(staticA2c-stdFork.TravelMm*(sag/100.0), 1)
		displayLabel = FormatSuspensionFork(travelVal, sag)
		if err := checkA2cRange(targetA2c); err != nil {
			return 0, 0, 0, "", err
		}
		return targetA2c, targetRake, staticA2c, displayLabel, nil
	}

	// 5. Explicit catalog fork ID lookup
	if entry, ok := GetCatalogFork(token); ok {
		if len(entry.Options) == 0 {
			return 0, 0, 0, "", fmt.Errorf("fork %s has no options", entry.ID)
		}
		opt := entry.Options[0]
		staticA2c = opt.A2cMm
		targetRake = opt.RakeMm
		if entry.TravelMm == 0 {
			if sagOverride != nil && *sagOverride > 0 {
				return 0, 0, 0, "", fmt.Errorf("cannot apply %.0f%% sag to rigid fork on \"%s %s\"; rigid forks have 0mm travel", *sagOverride, frame.Manufacturer, frame.Model)
			}
			targetA2c = staticA2c
			displayLabel = FormatRigidFork(targetA2c)
		} else {
			if frame.IsFixedRigidOnly() {
				return 0, 0, 0, "", fmt.Errorf("frame \"%s %s\" has a fixed rigid fork (%.0fmm A2C) and does not support suspension fork %q", frame.Manufacturer, frame.Model, frame.GetRigidA2c(), token)
			}
			sag := 20.0
			if sagOverride != nil {
				sag = *sagOverride
			}
			targetA2c = geometry.Round(staticA2c-entry.TravelMm*(sag/100.0), 1)
			displayLabel = FormatSuspensionFork(entry.TravelMm, sag)
		}
		if err := checkA2cRange(targetA2c); err != nil {
			return 0, 0, 0, "", err
		}
		return targetA2c, targetRake, staticA2c, displayLabel, nil
	}

	return 0, 0, 0, "", fmt.Errorf("unrecognized fork specification %q for frame \"%s %s\"", token, frame.Manufacturer, frame.Model)
}

// ResolveFrameForkSetup resolves a ComparisonSetup against a FrameModel, enforcing physical validity
// and legal fork limits, and canonicalizes to the comparison model.
func ResolveFrameForkSetup(
	frame model.FrameModel,
	setup model.ComparisonSetup,
	profile model.RiderProfile,
) (model.CanonicalComparisonRow, error) {
	if profile.SaddleHeightMm <= 0 {
		return model.CanonicalComparisonRow{}, fmt.Errorf("profile.SaddleHeightMm must be positive, got %.1f", profile.SaddleHeightMm)
	}

	// 0. Centralized frame validity gate
	frameVal := validator.ValidateFrame(frame)
	if !frameVal.IsValid {
		return model.CanonicalComparisonRow{}, fmt.Errorf("%s", frameVal.ErrorSummary())
	}

	// 1. Validate size
	size := strings.TrimSpace(setup.Size)
	if size == "" {
		size = frame.GetDefaultSize()
	} else {
		matchedSize := ""
		for _, sz := range frame.GetSizes() {
			if strings.EqualFold(sz, size) {
				matchedSize = sz
				break
			}
		}
		if matchedSize == "" {
			return model.CanonicalComparisonRow{}, fmt.Errorf(
				"frame \"%s %s\" (%s) does not have size %q (available sizes: %s)",
				frame.Manufacturer, frame.Model, frame.ID, size, strings.Join(frame.GetSizes(), ", "),
			)
		}
		size = matchedSize
	}

	targetA2c := setup.A2cMm
	targetRake := setup.RakeMm
	displayLabel := setup.DisplayLabel

	// If A2C is not pre-resolved, resolve default fork
	if targetA2c <= 0 {
		tA2c, tRake, _, dLabel, err := ResolveForkToken("", nil, frame)
		if err != nil {
			return model.CanonicalComparisonRow{}, err
		}
		targetA2c = tA2c
		targetRake = tRake
		displayLabel = dLabel
	}
	if targetRake <= 0 {
		targetRake = frame.CadFork.RakeMm
		if targetRake <= 0 {
			targetRake = 44.0
		}
	}
	if displayLabel == "" {
		displayLabel = FormatCustomA2cFork(targetA2c, targetRake)
	}

	// 2. Validate against frame effective A2C range
	if minA2c, maxA2c, ok := frame.EffectiveA2cRange(); ok {
		if targetA2c < minA2c-0.1 || targetA2c > maxA2c+0.1 {
			return model.CanonicalComparisonRow{}, fmt.Errorf(
				"effective A2C %.1fmm (%s) is outside frame \"%s %s\" legal range [%.0fmm, %.0fmm]",
				targetA2c, displayLabel, frame.Manufacturer, frame.Model, minA2c, maxA2c,
			)
		}
	}

	// 3. Materialize baseline geometry
	bike, err := geometry.MaterializeBikeGeometry(frame, size)
	if err != nil {
		return model.CanonicalComparisonRow{}, err
	}

	canonCfg := model.CanonicalConfig{
		TargetA2c:    targetA2c,
		RiderProfile: profile,
	}

	rows, err := CanonicalizeBikes([]model.BikeGeometry{bike}, canonCfg, targetRake, displayLabel)
	if err != nil {
		return model.CanonicalComparisonRow{}, err
	}
	if len(rows) == 0 {
		return model.CanonicalComparisonRow{}, fmt.Errorf("failed to canonicalize %s", frame.ID)
	}

	row := rows[0]
	row.Setup = model.ComparisonSetup{
		FrameID:      frame.ID,
		Size:         size,
		A2cMm:        targetA2c,
		RakeMm:       targetRake,
		DisplayLabel: displayLabel,
	}
	row.DisplayLabel = displayLabel
	row.EffectiveA2c = targetA2c
	row.EffectiveRake = targetRake

	return row, nil
}

// CanonicalizeBikes normalizes and canonicalizes a collection of bikes to an apples-to-apples configuration.
func CanonicalizeBikes(
	bikes []model.BikeGeometry,
	config model.CanonicalConfig,
	targetRake float64,
	displayLabel string,
) ([]model.CanonicalComparisonRow, error) {
	if config.RiderProfile.SaddleHeightMm <= 0 {
		return nil, fmt.Errorf("canonical config requires positive saddle height, got %.1f", config.RiderProfile.SaddleHeightMm)
	}
	seatHeight := config.RiderProfile.SaddleHeightMm
	var rows []model.CanonicalComparisonRow

	for _, bike := range bikes {
		valReport := validator.ValidateBikeGeometry(bike)

		baseSta := bike.StaDeg
		if bike.HasBentSeatTube() {
			baseSta = bike.CalculateEffectiveSta(seatHeight, bike.Stack, bike.StaDeg)
		}

		baselineAett := geometry.CalculateAett(
			bike.Reach,
			bike.Stack,
			bike.HtaDeg,
			baseSta,
			seatHeight,
		)
		baselineSetback := geometry.CalculateActualSetback(
			baseSta,
			seatHeight,
		)

		verticalStack, axialLength := geometry.CalculateSteererStack(
			bike.Stack,
			baseSta,
			bike.HtaDeg,
			seatHeight,
		)

		baseA2c := bike.GetCadA2c()
		if baseA2c <= 0 {
			baseA2c = valReport.GeometricA2c
		}

		activeCS := bike.ChainstayLength
		if bike.HasAdjustableChainstay() {
			activeCS = bike.GetMaxChainstay()
		}

		baseCS := math.Max(activeCS, bike.BbDrop+1.0)
		baseRearCenter := math.Sqrt(math.Max(0, baseCS*baseCS-bike.BbDrop*bike.BbDrop))

		baseHB := geometry.CalculateHBPoint(
			bike.Reach,
			bike.Stack,
			bike.HtaDeg,
			baseSta,
			seatHeight,
			bike.GetCanonicalStem(),
			0.0,
		)

		baseline := model.BaselineMetrics{
			Reach:              bike.Reach,
			Stack:              bike.Stack,
			HtaDeg:             bike.HtaDeg,
			StaDeg:             bike.StaDeg,
			BbDrop:             bike.BbDrop,
			A2c:                baseA2c,
			Aett:               geometry.Round(baselineAett, 1),
			ActualSetback:      baselineSetback,
			SpareVerticalStack: geometry.Round(verticalStack, 1),
			SpareAxialSteerer:  geometry.Round(axialLength, 1),
			Chainstay:          geometry.Round(activeCS, 1),
			SeatToRearCenter:   geometry.Round(baseRearCenter-baselineSetback, 1),
			SaddleHbMm:         baseHB.SaddleHbMm,
		}

		canonical, err := geometry.AnalyzeBikeFit(
			bike,
			&config.TargetA2c,
			&targetRake,
			config.RiderProfile,
		)
		if err != nil {
			return nil, err
		}

		rows = append(rows, model.CanonicalComparisonRow{
			DisplayLabel:  displayLabel,
			EffectiveA2c:  config.TargetA2c,
			EffectiveRake: targetRake,
			Bike:          bike,
			Validation:    valReport,
			Baseline:      baseline,
			Canonical:     canonical,
		})
	}

	return rows, nil
}

// ApplyIdealHBTarget updates each row's canonical SelectedHB and SaddleHbMm
// by selecting the optimal HB point from PossibleHBs against the target range [targetMin, targetMax].
func ApplyIdealHBTarget(rows []model.CanonicalComparisonRow, targetMin, targetMax *float64) {
	for i := range rows {
		if len(rows[i].Canonical.PossibleHBs) > 0 {
			if best, ok := geometry.SelectBestHB(rows[i].Canonical.PossibleHBs, rows[i].Bike.GetCanonicalStem(), targetMin, targetMax); ok {
				rows[i].Canonical.SelectedHB = &best
				rows[i].Canonical.SaddleHbMm = best.SaddleHbMm
			}
		}
	}
}
