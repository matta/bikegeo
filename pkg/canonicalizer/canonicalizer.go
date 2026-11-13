package canonicalizer

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"bikegeo/pkg/catalog"
	"bikegeo/pkg/geometry"
	"bikegeo/pkg/ideal"
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

// AllowedSagPcts defines the standard allowed suspension fork sag percentages (20%, 25%, 30%).
var AllowedSagPcts = []float64{20.0, 25.0, 30.0}

// PlausibleSagPcts defines the standard suspension fork sag percentages evaluated during combinatorial sweeps.
var PlausibleSagPcts = AllowedSagPcts

// IsAllowedSag reports whether a sag percentage is in the allowed set (20%, 25%, 30%).
func IsAllowedSag(sag float64) bool {
	for _, s := range AllowedSagPcts {
		if math.Abs(s-sag) < 0.01 {
			return true
		}
	}
	return false
}

// ExpandPlausibleForkSetups generates all candidate fork and sag setups for a frame.
// Rigid forks are included at 0% sag (unless sagOverride > 0).
// Supported suspension forks are enumerated across 10mm travel steps and plausible sag points.
func ExpandPlausibleForkSetups(
	frame model.FrameModel,
	sagOverride *float64,
) ([]model.ComparisonSetup, error) {
	var setups []model.ComparisonSetup
	seen := make(map[string]bool)

	addSetup := func(s model.ComparisonSetup) {
		key := fmt.Sprintf("%.1f|%.1f|%.1f|%s", s.A2cMm, s.RakeMm, s.SagPct, s.DisplayLabel)
		if !seen[key] {
			seen[key] = true
			setups = append(setups, s)
		}
	}

	minA2c, maxA2c, hasRange := frame.EffectiveA2cRange()

	// 1. Rigid forks (if supported and sagOverride <= 0)
	if sagOverride == nil || *sagOverride == 0 {
		var rigidForks []model.ForkRef
		for _, f := range frame.Forks {
			if f.ForkTravel() == 0 && f.A2cMm > 0 {
				rigidForks = append(rigidForks, f)
			}
		}
		if len(rigidForks) == 0 && frame.CadFork.ForkTravel() == 0 && frame.CadFork.A2cMm > 0 {
			rigidForks = append(rigidForks, frame.CadFork)
		}

		for _, rf := range rigidForks {
			rake := rf.RakeMm
			if rake <= 0 {
				rake = frame.CadFork.RakeMm
			}
			if rake <= 0 {
				rake = 44.0
			}
			if hasRange && (rf.A2cMm < minA2c-0.1 || rf.A2cMm > maxA2c+0.1) {
				continue
			}
			addSetup(model.ComparisonSetup{
				FrameID:      frame.ID,
				A2cMm:        rf.A2cMm,
				RakeMm:       rake,
				DisplayLabel: FormatRigidFork(rf.A2cMm),
				SagPct:       0.0,
				AutoExpanded: true,
			})
		}
	}

	// 2. Suspension forks (if frame supports suspension)
	if !frame.IsFixedRigidOnly() {
		var minTr, maxTr float64
		for _, f := range frame.Forks {
			tr := f.ForkTravel()
			if tr >= 60 {
				if minTr == 0 || tr < minTr {
					minTr = tr
				}
				if tr > maxTr {
					maxTr = tr
				}
			}
		}
		if minTr == 0 && frame.CadFork.ForkTravel() >= 60 {
			minTr = frame.CadFork.ForkTravel()
			maxTr = minTr
		}

		if minTr >= 60 && maxTr >= minTr {
			sags := PlausibleSagPcts
			if sagOverride != nil {
				if math.IsNaN(*sagOverride) || math.IsInf(*sagOverride, 0) || *sagOverride < 0 || math.Signbit(*sagOverride) || (*sagOverride > 0 && !IsAllowedSag(*sagOverride)) {
					return nil, fmt.Errorf("invalid sag %v%%; allowed sags are 20%%, 25%%, 30%%", *sagOverride)
				}
				sags = []float64{*sagOverride}
			}

			for tr := minTr; tr <= maxTr+0.1; tr += 10.0 {
				var stdFork model.ForkCatalogEntry
				var preferredRake float64
				found := false

				// Check frame.Forks first
				for _, fRef := range frame.Forks {
					if fRef.Fork != "" {
						if entry, ok := GetCatalogFork(fRef.Fork); ok && entry.TravelMm == tr && len(entry.Options) > 0 {
							stdFork = entry
							preferredRake = fRef.RakeMm
							found = true
							break
						}
					}
				}
				// Check frame.CadFork
				if !found && frame.CadFork.Fork != "" {
					if entry, ok := GetCatalogFork(frame.CadFork.Fork); ok && entry.TravelMm == tr && len(entry.Options) > 0 {
						stdFork = entry
						preferredRake = frame.CadFork.RakeMm
						found = true
					}
				}
				// Catalog standard fork fallback
				if !found {
					if f, ok := GetStandardForkForTravel(tr); ok && len(f.Options) > 0 {
						stdFork = f
						found = true
					}
				}

				if !found || len(stdFork.Options) == 0 {
					continue
				}

				opt := stdFork.Options[0]
				if preferredRake > 0 {
					for _, o := range stdFork.Options {
						if math.Abs(o.RakeMm-preferredRake) < 0.1 {
							opt = o
							break
						}
					}
				}

				staticA2c := opt.A2cMm
				rake := opt.RakeMm

				for _, sag := range sags {
					effA2c := geometry.Round(staticA2c-tr*(sag/100.0), 1)
					if hasRange && (effA2c < minA2c-0.1 || effA2c > maxA2c+0.1) {
						continue
					}
					addSetup(model.ComparisonSetup{
						FrameID:      frame.ID,
						A2cMm:        effA2c,
						RakeMm:       rake,
						DisplayLabel: FormatSuspensionFork(tr, sag),
						SagPct:       sag,
						AutoExpanded: true,
					})
				}
			}
		}
	}

	if len(setups) == 0 {
		return nil, fmt.Errorf("no plausible fork configurations found for frame %q", frame.ID)
	}

	return setups, nil
}

// ResolveForkToken resolves a user comparison modifier (e.g. "120mm", "rigid", "stock", "a2c=506/44")
// against the fork catalog and frame constraints.
func ResolveForkToken(
	token string,
	sagOverride *float64,
	frame model.FrameModel,
) (targetA2c float64, targetRake float64, staticA2c float64, displayLabel string, err error) {
	if sagOverride != nil {
		if math.IsNaN(*sagOverride) || math.IsInf(*sagOverride, 0) || *sagOverride < 0 || math.Signbit(*sagOverride) {
			return 0, 0, 0, "", fmt.Errorf("invalid sag %v%%; allowed sags are 20%%, 25%%, 30%%", *sagOverride)
		}
		if *sagOverride > 0 && !IsAllowedSag(*sagOverride) {
			return 0, 0, 0, "", fmt.Errorf("invalid sag %.0f%%; allowed sags are 20%%, 25%%, 30%%", *sagOverride)
		}
	}
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
		if (frame.IsFixedRigidOnly() || def.ForkTravel() == 0) && sagOverride != nil && *sagOverride > 0 {
			return 0, 0, 0, "", fmt.Errorf("cannot apply %.0f%% sag to rigid fork on \"%s %s\"; rigid forks have 0mm travel", *sagOverride, frame.Manufacturer, frame.Model)
		}
		if frame.IsFixedRigidOnly() || def.ForkTravel() == 0 {
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
	if after, ok := strings.CutPrefix(lower, "a2c="); ok {
		valPart := after
		if strings.Contains(valPart, "/") {
			parts := strings.SplitN(valPart, "/", 2)
			a2cVal, err := strconv.ParseFloat(parts[0], 64)
			if err != nil || math.IsNaN(a2cVal) || math.IsInf(a2cVal, 0) || a2cVal <= 0 {
				return 0, 0, 0, "", fmt.Errorf("invalid A2C in %q", token)
			}
			rakeVal, err := strconv.ParseFloat(parts[1], 64)
			if err != nil || math.IsNaN(rakeVal) || math.IsInf(rakeVal, 0) || rakeVal <= 0 {
				return 0, 0, 0, "", fmt.Errorf("invalid rake in %q", token)
			}
			targetA2c = a2cVal
			targetRake = rakeVal
		} else {
			a2cVal, err := strconv.ParseFloat(valPart, 64)
			if err != nil || math.IsNaN(a2cVal) || math.IsInf(a2cVal, 0) || a2cVal <= 0 {
				return 0, 0, 0, "", fmt.Errorf("invalid A2C in %q", token)
			}
			targetA2c = a2cVal
			targetRake = frame.CadFork.RakeMm
			if targetRake <= 0 {
				targetRake = 44.0
			}
		}
		staticA2c = targetA2c
		displayLabel = FormatCustomA2cFork(targetA2c, targetRake)
		// If the explicit A2C and rake match one of the frame's declared rigid forks,
		// use the canonical rigid label so identical physical setups label identically.
		rigidRefs := frame.Forks
		if len(rigidRefs) == 0 {
			rigidRefs = []model.ForkRef{frame.CadFork}
		}
		for _, f := range rigidRefs {
			if f.ForkTravel() == 0 && f.A2cMm > 0 {
				rk := f.RakeMm
				if rk <= 0 {
					rk = frame.CadFork.RakeMm
				}
				if rk <= 0 {
					rk = 44.0
				}
				if math.Abs(targetA2c-f.A2cMm) < 0.05 && math.Abs(targetRake-rk) < 0.05 {
					displayLabel = FormatRigidFork(targetA2c)
					break
				}
			}
		}
		if err := checkA2cRange(targetA2c); err != nil {
			return 0, 0, 0, "", err
		}
		return targetA2c, targetRake, staticA2c, displayLabel, nil
	}

	// 4. Suspension travel token (e.g. "120mm", "120")
	travelStr := strings.TrimSuffix(lower, "mm")
	if travelVal, err := strconv.ParseFloat(travelStr, 64); err == nil && !math.IsNaN(travelVal) && !math.IsInf(travelVal, 0) && travelVal >= 60 {
		if frame.IsFixedRigidOnly() {
			return 0, 0, 0, "", fmt.Errorf("frame \"%s %s\" has a fixed rigid fork (%.0fmm A2C) and does not support suspension fork %q", frame.Manufacturer, frame.Model, frame.GetRigidA2c(), token)
		}
		var stdFork model.ForkCatalogEntry
		foundFrameFork := false
		var frameRake float64
		for _, fRef := range frame.Forks {
			if fRef.Fork != "" {
				if entry, ok := GetCatalogFork(fRef.Fork); ok && entry.TravelMm == travelVal && len(entry.Options) > 0 {
					stdFork = entry
					frameRake = fRef.RakeMm
					foundFrameFork = true
					break
				}
			}
		}
		if !foundFrameFork && frame.CadFork.Fork != "" {
			if entry, ok := GetCatalogFork(frame.CadFork.Fork); ok && entry.TravelMm == travelVal && len(entry.Options) > 0 {
				stdFork = entry
				frameRake = frame.CadFork.RakeMm
				foundFrameFork = true
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
		if frameRake > 0 {
			for _, o := range stdFork.Options {
				if math.Abs(o.RakeMm-frameRake) < 0.1 {
					opt = o
					break
				}
			}
		}
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
// and legal fork limits, and canonicalizes to the comparison model using the default saddle.
func ResolveFrameForkSetup(
	frame model.FrameModel,
	setup model.ComparisonSetup,
	profile model.RiderProfile,
) (model.CanonicalComparisonRow, error) {
	return ResolveFrameForkSetupWithSaddle(frame, setup, profile, catalog.DefaultSaddle())
}

// ResolveFrameForkSetupWithSaddle resolves a ComparisonSetup against a FrameModel using an explicit SaddleSpec.
func ResolveFrameForkSetupWithSaddle(
	frame model.FrameModel,
	setup model.ComparisonSetup,
	profile model.RiderProfile,
	saddle model.SaddleSpec,
) (model.CanonicalComparisonRow, error) {
	if saddle.ID == "" {
		saddle = catalog.DefaultSaddle()
	}
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
		Saddle:       saddle,
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
		SagPct:       setup.SagPct,
		AutoExpanded: setup.AutoExpanded,
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

		baseBRP := geometry.CalculateBRPPoint(
			baseSta,
			seatHeight,
			model.DefaultSeatpostSetbackMm,
			model.DefaultSaddleBrpOffsetMm,
			0.0,
		)

		bar := config.RiderProfile.Handlebar
		if bar.ID == "" {
			bar = catalog.DefaultHandlebar()
		}
		baseHB := geometry.CalculateHBPointWithHandlebar(
			bike.Reach,
			bike.Stack,
			bike.HtaDeg,
			baseBRP,
			bike.GetCanonicalStem(),
			0.0,
			bar,
			model.DefaultGripInsetMm,
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
			SeatpostExtension:  geometry.CalculateSeatpostExtension(seatHeight, bike.SeatTubeLength),
			Chainstay:          geometry.Round(activeCS, 1),
			SeatToRearCenter:   geometry.Round(baseRearCenter-baselineSetback, 1),
			SaddleHbMm:         baseHB.SaddleHbMm,
			TrailMm:            geometry.Round(geometry.CalculateTrail(bike.HtaDeg, bike.GetCadRake(), bike.WheelRadiusMm()), 1),
		}

		canonical, err := geometry.AnalyzeBikeFitWithSaddle(
			bike,
			&config.TargetA2c,
			&targetRake,
			config.RiderProfile,
			config.Saddle,
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

// ApplyIdealCockpitTargets updates each row's canonical SelectedBRP, SelectedHB, SaddleHbMm, and SaddleHbDropMm
// by first selecting the optimal BRP position along the saddle rails against the brp_setback target,
// re-mapping possible HB points from that BRP, and then selecting the optimal HB point against saddle_hb (primary)
// and saddle_hb_drop (secondary) targets.
func ApplyIdealCockpitTargets(rows []model.CanonicalComparisonRow, prof *ideal.Profile) {
	if prof == nil {
		return
	}

	var brpMin, brpMax *float64
	var distMin, distMax *float64
	var dropMin, dropMax *float64
	var hasClampFit bool

	for _, attr := range prof.Attributes {
		lowID := strings.ToLower(attr.ID)
		if lowID == "handlebar_clamp_fit" || lowID == "cockpit_fit" || lowID == "clamp_fit" {
			hasClampFit = true
		} else if (lowID == "brp_setback" || lowID == "brp" || lowID == "brpsetback") && attr.Ideal != nil {
			brpMin = attr.Ideal.Min
			brpMax = attr.Ideal.Max
		} else if (lowID == "saddle_hb" || lowID == "saddlehb") && attr.Ideal != nil {
			distMin = attr.Ideal.Min
			distMax = attr.Ideal.Max
		} else if (lowID == "saddle_hb_drop" || lowID == "hb_drop" || lowID == "saddlehbdrop") && attr.Ideal != nil {
			dropMin = attr.Ideal.Min
			dropMax = attr.Ideal.Max
		}
	}

	for i := range rows {
		can := &rows[i].Canonical

		// Derive row-specific targets anchored by reference SQlab saddle
		rowBrpMin, rowBrpMax := brpMin, brpMax
		rowDistMin, rowDistMax := distMin, distMax
		rowDropMin, rowDropMax := dropMin, dropMax
		if can.Saddle.ID != "" && can.Saddle.ID != "sqlab-610-ergolux" {
			refSQ := model.ReferenceIdmatchSaddle()
			if brpMin != nil {
				pelvicMin := refSQ.SitBoneSetbackFromBrp(*brpMin)
				trMin := can.Saddle.TargetBrpForPelvicSetback(pelvicMin)
				rowBrpMin = &trMin
			}
			if brpMax != nil {
				pelvicMax := refSQ.SitBoneSetbackFromBrp(*brpMax)
				trMax := can.Saddle.TargetBrpForPelvicSetback(pelvicMax)
				rowBrpMax = &trMax
			}
			refRise := 3.0
			if dropMin != nil && dropMax != nil {
				refRise = (*dropMin + *dropMax) / 2.0
			} else if dropMin != nil {
				refRise = *dropMin
			}
			if distMin != nil {
				tDist := can.Saddle.TranslateHbDistanceFromRef(refSQ, 746.0, 219.0, *distMin, refRise)
				rowDistMin = &tDist
			}
			if distMax != nil {
				tDist := can.Saddle.TranslateHbDistanceFromRef(refSQ, 746.0, 219.0, *distMax, refRise)
				rowDistMax = &tDist
			}
		}

		// 1. Optimize BRP on saddle rails if candidate BRPs exist
		if len(can.PossibleBRPs) > 0 {
			if bestBRP, ok := geometry.SelectBestBRP(can.PossibleBRPs, rowBrpMin, rowBrpMax); ok {
				can.SelectedBRP = &bestBRP
				can.SitBoneSetbackMm = bestBRP.SitBoneSetbackMm
				can.AchievedClampOffset = bestBRP.AchievedClampOffsetMm
			}
			activeBRP := can.DeadCenterBRP
			if can.SelectedBRP != nil {
				activeBRP = *can.SelectedBRP
			}
			// 2. Re-map PossibleHBs referencing the selected BRP and configured handlebar
			bar := can.Handlebar
			if bar.ID == "" {
				bar = catalog.DefaultHandlebar()
			}
			can.PossibleHBs = geometry.MapPossibleHBsWithHandlebar(rows[i].Bike, can.ActiveReach, can.ActiveStack, can.ActiveHta, activeBRP, bar, model.DefaultGripInsetMm, 0.0)
		}

		// 3. Evaluate Cockpit Fit
		if hasClampFit && (rowDistMin != nil || rowDistMax != nil) {
			geometry.ApplyCockpitFit(can, rows[i].Bike, rows[i].Bike.GetCanonicalStem(), rowDistMin, rowDistMax, rowDropMin, rowDropMax)
		} else if len(can.PossibleHBs) > 0 {
			if bestHB, ok := geometry.SelectBestHBWithDrop(can.PossibleHBs, rows[i].Bike.GetCanonicalStem(), rowDistMin, rowDistMax, rowDropMin, rowDropMax); ok {
				can.SelectedHB = &bestHB
				can.SaddleHbMm = bestHB.SaddleHbMm
				can.SaddleHbDropMm = bestHB.SaddleHbDropMm
			}
		}
	}
}
