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
	sagOverride model.Opt[float64],
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
	if sag, ok := sagOverride.Get(); !ok || sag == 0 {
		var rigidForks []model.ForkRef
		for _, f := range frame.Forks {
			if f.TravelMm == 0 && f.A2cMm > 0 {
				rigidForks = append(rigidForks, f)
			}
		}
		if len(rigidForks) == 0 && frame.CadFork.TravelMm == 0 && frame.CadFork.A2cMm > 0 {
			rigidForks = append(rigidForks, frame.CadFork)
		}

		for _, rf := range rigidForks {
			rake := rf.RakeMm
			if rake <= 0 {
				rake = frame.CadFork.RakeMm
			}
			if rake <= 0 {
				return nil, fmt.Errorf("frame %q rigid fork %q requires positive rake, got %v", frame.ID, rf.Fork, rake)
			}
			if hasRange && (rf.A2cMm < minA2c-0.1 || rf.A2cMm > maxA2c+0.1) {
				continue
			}
			addSetup(model.ComparisonSetup{
				FrameID:      frame.ID,
				A2cMm:        rf.A2cMm,
				RakeMm:       rake,
				IsRigid:      true,
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
			tr := f.TravelMm
			if tr >= 60 {
				if minTr == 0 || tr < minTr {
					minTr = tr
				}
				if tr > maxTr {
					maxTr = tr
				}
			}
		}
		if minTr == 0 && frame.CadFork.TravelMm >= 60 {
			minTr = frame.CadFork.TravelMm
			maxTr = minTr
		}

		if minTr >= 60 && maxTr >= minTr {
			sags := PlausibleSagPcts
			if sag, ok := sagOverride.Get(); ok {
				if math.IsNaN(sag) || math.IsInf(sag, 0) || sag < 0 || math.Signbit(sag) || (sag > 0 && !IsAllowedSag(sag)) {
					return nil, fmt.Errorf("invalid sag %v%%; allowed sags are 20%%, 25%%, 30%%", sag)
				}
				sags = []float64{sag}
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
						TravelMm:     tr,
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

// ResolvedFork holds the typed physical properties resolved from a fork modifier token.
type ResolvedFork struct {
	TargetA2c    float64
	TargetRake   float64
	StaticA2c    float64
	TravelMm     float64
	IsRigid      bool
	DisplayLabel string
}

// ResolveForkToken resolves a user comparison modifier (e.g. "120mm", "rigid", "stock", "a2c=506/44")
// against the fork catalog and frame constraints.
func ResolveForkToken(
	token string,
	sagOverride model.Opt[float64],
	frame model.FrameModel,
) (ResolvedFork, error) {
	if sag, ok := sagOverride.Get(); ok {
		if math.IsNaN(sag) || math.IsInf(sag, 0) || sag < 0 || math.Signbit(sag) {
			return ResolvedFork{}, fmt.Errorf("invalid sag %v%%; allowed sags are 20%%, 25%%, 30%%", sag)
		}
		if sag > 0 && !IsAllowedSag(sag) {
			return ResolvedFork{}, fmt.Errorf("invalid sag %.0f%%; allowed sags are 20%%, 25%%, 30%%", sag)
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

	var targetA2c, targetRake, staticA2c float64
	var displayLabel string

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
				if entry.TravelMm == 0 && sagOverride.OrZero() > 0 {
					return ResolvedFork{}, fmt.Errorf("cannot apply %.0f%% sag to rigid fork on \"%s %s\"; rigid forks have 0mm travel", sagOverride.OrZero(), frame.Manufacturer, frame.Model)
				}
				if entry.TravelMm > 0 && len(entry.Options) > 0 {
					staticA2c = entry.Options[0].A2cMm
					sag := def.GetSagPct()
					if sag == 0 {
						sag = 20.0
					}
					sag = sagOverride.Or(sag)
					targetA2c = geometry.Round(staticA2c-entry.TravelMm*(sag/100.0), 1)
					displayLabel = FormatSuspensionFork(entry.TravelMm, sag)
					return ResolvedFork{
						TargetA2c:    targetA2c,
						TargetRake:   targetRake,
						StaticA2c:    staticA2c,
						TravelMm:     entry.TravelMm,
						DisplayLabel: displayLabel,
					}, nil
				}
			}
		}
		if (frame.IsFixedRigidOnly() || def.TravelMm == 0) && sagOverride.OrZero() > 0 {
			return ResolvedFork{}, fmt.Errorf("cannot apply %.0f%% sag to rigid fork on \"%s %s\"; rigid forks have 0mm travel", sagOverride.OrZero(), frame.Manufacturer, frame.Model)
		}
		isRigid := frame.IsFixedRigidOnly() || def.TravelMm == 0
		if isRigid {
			displayLabel = FormatRigidFork(targetA2c)
		} else {
			displayLabel = FormatStockFork()
		}
		return ResolvedFork{
			TargetA2c:    targetA2c,
			TargetRake:   targetRake,
			StaticA2c:    staticA2c,
			TravelMm:     def.TravelMm,
			IsRigid:      isRigid,
			DisplayLabel: displayLabel,
		}, nil
	}

	// 2. Rigid fork
	if lower == "rigid" || lower == "0mm" || lower == "0" || strings.HasPrefix(lower, "rigid") {
		if !frame.SupportsRigid() {
			return ResolvedFork{}, fmt.Errorf("frame \"%s %s\" (%s) does not support a rigid fork", frame.Manufacturer, frame.Model, frame.ID)
		}
		if sagOverride.OrZero() > 0 {
			return ResolvedFork{}, fmt.Errorf("cannot apply %.0f%% sag to rigid fork on \"%s %s\"; rigid forks have 0mm travel", sagOverride.OrZero(), frame.Manufacturer, frame.Model)
		}
		targetA2c = frame.GetRigidA2c()
		targetRake = frame.GetRigidRake()
		staticA2c = targetA2c
		displayLabel = FormatRigidFork(targetA2c)
		return ResolvedFork{
			TargetA2c:    targetA2c,
			TargetRake:   targetRake,
			StaticA2c:    staticA2c,
			IsRigid:      true,
			DisplayLabel: displayLabel,
		}, nil
	}

	// 3. Direct A2C syntax: a2c=NNN/MM or a2c=NNN
	if after, ok := strings.CutPrefix(lower, "a2c="); ok {
		valPart := after
		if strings.Contains(valPart, "/") {
			parts := strings.SplitN(valPart, "/", 2)
			a2cVal, err := strconv.ParseFloat(parts[0], 64)
			if err != nil || math.IsNaN(a2cVal) || math.IsInf(a2cVal, 0) || a2cVal <= 0 {
				return ResolvedFork{}, fmt.Errorf("invalid A2C in %q", token)
			}
			rakeVal, err := strconv.ParseFloat(parts[1], 64)
			if err != nil || math.IsNaN(rakeVal) || math.IsInf(rakeVal, 0) || rakeVal <= 0 {
				return ResolvedFork{}, fmt.Errorf("invalid rake in %q", token)
			}
			targetA2c = a2cVal
			targetRake = rakeVal
		} else {
			a2cVal, err := strconv.ParseFloat(valPart, 64)
			if err != nil || math.IsNaN(a2cVal) || math.IsInf(a2cVal, 0) || a2cVal <= 0 {
				return ResolvedFork{}, fmt.Errorf("invalid A2C in %q", token)
			}
			targetA2c = a2cVal
			targetRake = frame.CadFork.RakeMm
			if targetRake <= 0 {
				return ResolvedFork{}, fmt.Errorf("frame %q requires positive CAD fork rake for %q, got %v", frame.ID, token, targetRake)
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
		isRigid := false
		for _, f := range rigidRefs {
			if f.TravelMm == 0 && f.A2cMm > 0 {
				rk := f.RakeMm
				if rk <= 0 {
					rk = frame.CadFork.RakeMm
				}
				if rk <= 0 {
					continue
				}
				if math.Abs(targetA2c-f.A2cMm) < 0.05 && math.Abs(targetRake-rk) < 0.05 {
					displayLabel = FormatRigidFork(targetA2c)
					isRigid = true
					break
				}
			}
		}
		if err := checkA2cRange(targetA2c); err != nil {
			return ResolvedFork{}, err
		}
		return ResolvedFork{
			TargetA2c:    targetA2c,
			TargetRake:   targetRake,
			StaticA2c:    staticA2c,
			IsRigid:      isRigid,
			DisplayLabel: displayLabel,
		}, nil
	}

	// 4. Suspension travel token (e.g. "120mm", "120")
	travelStr := strings.TrimSuffix(lower, "mm")
	if travelVal, err := strconv.ParseFloat(travelStr, 64); err == nil && !math.IsNaN(travelVal) && !math.IsInf(travelVal, 0) && travelVal >= 60 {
		if frame.IsFixedRigidOnly() {
			return ResolvedFork{}, fmt.Errorf("frame \"%s %s\" has a fixed rigid fork (%.0fmm A2C) and does not support suspension fork %q", frame.Manufacturer, frame.Model, frame.GetRigidA2c(), token)
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
				return ResolvedFork{}, fmt.Errorf("no catalog fork found for %.0fmm suspension travel", travelVal)
			}
			stdFork = f
		}
		if len(stdFork.Options) == 0 {
			return ResolvedFork{}, fmt.Errorf("fork %s has no configured options", stdFork.ID)
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
		sag := sagOverride.Or(20.0)
		targetA2c = geometry.Round(staticA2c-stdFork.TravelMm*(sag/100.0), 1)
		displayLabel = FormatSuspensionFork(travelVal, sag)
		if err := checkA2cRange(targetA2c); err != nil {
			return ResolvedFork{}, err
		}
		return ResolvedFork{
			TargetA2c:    targetA2c,
			TargetRake:   targetRake,
			StaticA2c:    staticA2c,
			TravelMm:     stdFork.TravelMm,
			DisplayLabel: displayLabel,
		}, nil
	}

	// 5. Explicit catalog fork ID lookup
	if entry, ok := GetCatalogFork(token); ok {
		if len(entry.Options) == 0 {
			return ResolvedFork{}, fmt.Errorf("fork %s has no options", entry.ID)
		}
		opt := entry.Options[0]
		staticA2c = opt.A2cMm
		targetRake = opt.RakeMm
		isRigid := entry.TravelMm == 0
		if isRigid {
			if sagOverride.OrZero() > 0 {
				return ResolvedFork{}, fmt.Errorf("cannot apply %.0f%% sag to rigid fork on \"%s %s\"; rigid forks have 0mm travel", sagOverride.OrZero(), frame.Manufacturer, frame.Model)
			}
			targetA2c = staticA2c
			displayLabel = FormatRigidFork(targetA2c)
		} else {
			if frame.IsFixedRigidOnly() {
				return ResolvedFork{}, fmt.Errorf("frame \"%s %s\" has a fixed rigid fork (%.0fmm A2C) and does not support suspension fork %q", frame.Manufacturer, frame.Model, frame.GetRigidA2c(), token)
			}
			sag := sagOverride.Or(20.0)
			targetA2c = geometry.Round(staticA2c-entry.TravelMm*(sag/100.0), 1)
			displayLabel = FormatSuspensionFork(entry.TravelMm, sag)
		}
		if err := checkA2cRange(targetA2c); err != nil {
			return ResolvedFork{}, err
		}
		return ResolvedFork{
			TargetA2c:    targetA2c,
			TargetRake:   targetRake,
			StaticA2c:    staticA2c,
			TravelMm:     entry.TravelMm,
			IsRigid:      isRigid,
			DisplayLabel: displayLabel,
		}, nil
	}

	return ResolvedFork{}, fmt.Errorf("unrecognized fork specification %q for frame \"%s %s\"", token, frame.Manufacturer, frame.Model)
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
	travelMm := setup.TravelMm
	isRigid := setup.IsRigid
	displayLabel := setup.DisplayLabel

	// If A2C is not pre-resolved, resolve default fork
	if targetA2c <= 0 {
		resolved, err := ResolveForkToken("", model.Opt[float64]{}, frame)
		if err != nil {
			return model.CanonicalComparisonRow{}, err
		}
		targetA2c = resolved.TargetA2c
		targetRake = resolved.TargetRake
		travelMm = resolved.TravelMm
		isRigid = resolved.IsRigid
		displayLabel = resolved.DisplayLabel
	}
	if targetRake <= 0 {
		targetRake = frame.CadFork.RakeMm
		if targetRake <= 0 {
			return model.CanonicalComparisonRow{}, fmt.Errorf("frame %q requires positive fork rake, got %v", frame.ID, targetRake)
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
		TravelMm:     travelMm,
		IsRigid:      isRigid,
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

		baseA2c := bike.GetCadA2c()
		if baseA2c <= 0 {
			baseA2c = valReport.GeometricA2c
		}

		activeCS := bike.ChainstayLength
		if bike.HasAdjustableChainstay() {
			activeCS = bike.GetMaxChainstay()
		}

		frame, err := geometry.FrameSizeFromBikeGeometry(bike)
		if err != nil {
			return nil, err
		}
		bar := config.RiderProfile.Handlebar
		if bar.ID == "" {
			bar = catalog.DefaultHandlebar()
		}
		cadBuild := geometry.BuildSpec{
			Fork:               frame.CadFork,
			SagPct:             frame.CadSagPct,
			PhysicalLowerCupMm: frame.CadLowerCupMm,
			WheelRadiusFrontMm: bike.WheelRadiusMm(),
			WheelRadiusRearMm:  bike.WheelRadiusMm(),
		}
		cadStem := bike.GetCanonicalStem()
		if cadStem.LengthMm <= 0 {
			if len(bike.Stems) > 0 && bike.Stems[0].LengthMm > 0 {
				cadStem = bike.Stems[0]
			} else {
				cadStem = model.StemSpec{LengthMm: 70.0}
			}
		}
		cadCockpit := geometry.CockpitSpec{
			HeadsetTopStackMm: model.DefaultHeadsetTopStackMm,
			SpacerMm:          0.0,
			Stem:              cadStem,
			StemClampHeightMm: model.StemThicknessMm,
			Handlebar:         bar,
			BarRollDeg:        0.0,
			GripInsetMm:       model.DefaultGripInsetMm,
		}
		saddle := config.Saddle
		if saddle.ID == "" {
			saddle = catalog.DefaultSaddle()
		}
		cadSeat := geometry.SeatSpec{
			PostSetbackMm: model.DefaultSeatpostSetbackMm,
			ClampLengthMm: model.DefaultSeatpostClampLengthMm,
			Saddle:        saddle,
			RailOffsetMm:  0.0,
		}
		targetRef := config.RiderProfile.TargetRef
		if targetRef == "" {
			targetRef = geometry.TargetHandGrip
		}
		cadFit := geometry.RiderFit{
			FrameSeatAxisHeightMm: seatHeight,
			BrpHeightMm:           seatHeight,
			TargetSitBonesXMm:     -model.DefaultTargetSitBoneSetbackMm,
			TargetRef:             targetRef,
			ReferenceSaddle:       saddle,
			ReferenceBar:          config.RiderProfile.ReferenceBar,
		}
		cadScene, err := geometry.BuildScene(frame, cadBuild, cadCockpit, cadSeat, cadFit)
		if err != nil {
			return nil, err
		}
		baselineSetback := cadScene.SeatAxisActualSetbackMm()
		baselineSeatExt, _ := cadScene.SeatpostExtensionMm()

		baseline := model.BaselineMetrics{
			Reach:              bike.Reach,
			Stack:              bike.Stack,
			HtaDeg:             bike.HtaDeg,
			StaDeg:             bike.StaDeg,
			BbDrop:             bike.BbDrop,
			A2c:                baseA2c,
			Aett:               cadScene.AettMm(),
			ActualSetback:      baselineSetback,
			SpareVerticalStack: cadScene.SpareVerticalStackMm(),
			SpareAxialSteerer:  cadScene.SpareAxialSteererMm(),
			SeatpostExtension:  baselineSeatExt,
			Chainstay:          activeCS,
			SeatToRearCenter:   cadScene.RearCenter() - baselineSetback,
			SaddleHbMm:         cadScene.GripDistFromBrpMm(),
			TrailMm:            cadScene.TrailMm(),
		}

		canonical, err := geometry.AnalyzeBikeFitWithSaddle(
			bike,
			model.Some(config.TargetA2c),
			model.Some(targetRake),
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

// ApplyIdealCockpitTargets selects each row's stock build against the profile's
// points-lost ledger and records it on the row's canonical SelectedBRP, SelectedHB,
// CockpitFitResult, SaddleHbMm and SaddleHbDropMm.
//
//  1. Saddle: every rail position is priced by the brp_setback and rail_offset bands
//     and the cheapest is kept. Past the rail limit the setback band absorbs the miss.
//  2. Cockpit: every stock combination (stem length × angle × spacer stack) is placed
//     relative to the chosen BRP and priced by the Oval Fits, stem_length, stem_angle
//     and spacers bands; the cheapest is kept. Past the hardware limits the oval
//     absorbs the miss.
//
// Profiles without those scored bands fall back to the legacy range selectors.
func ApplyIdealCockpitTargets(rows []model.CanonicalComparisonRow, prof *ideal.Profile) {
	if prof == nil {
		return
	}

	for i := range rows {
		row := &rows[i]
		can := &row.Canonical
		rowProf := prof.ForSaddle(can.Saddle)

		var rowBrpMin, rowBrpMax model.Opt[float64]
		var rowDistMin, rowDistMax model.Opt[float64]
		var rowDropMin, rowDropMax model.Opt[float64]
		var hasClampFit bool

		for _, attr := range rowProf.Attributes {
			if attr.ID == ideal.MetricHandlebarClampFit {
				hasClampFit = true
			} else if attr.ID == ideal.MetricBrpSetback && attr.Ideal != nil {
				rowBrpMin = attr.Ideal.Min
				rowBrpMax = attr.Ideal.Max
			} else if attr.ID == ideal.MetricSaddleHb && attr.Ideal != nil {
				rowDistMin = attr.Ideal.Min
				rowDistMax = attr.Ideal.Max
			} else if attr.ID == ideal.MetricSaddleHbDrop && attr.Ideal != nil {
				rowDropMin = attr.Ideal.Min
				rowDropMax = attr.Ideal.Max
			}
		}

		// 1. Saddle position on the rails.
		activeBRP := can.DeadCenterBRP
		if can.SelectedBRP != nil {
			activeBRP = *can.SelectedBRP
		}
		if len(can.PossibleBRPs) > 0 {
			var bestBRP model.BRPPoint
			var ok bool
			if cost := brpCost(rowProf, row); cost != nil {
				bestBRP, ok = geometry.SelectCheapestBRP(can.PossibleBRPs, cost)
			} else {
				bestBRP, ok = geometry.SelectBestBRP(can.PossibleBRPs, rowBrpMin, rowBrpMax)
			}
			if ok {
				can.SelectedBRP = &bestBRP
				can.SitBoneSetbackMm = bestBRP.SitBoneSetbackMm
				can.AchievedClampOffset = bestBRP.AchievedClampOffsetMm
				activeBRP = bestBRP
			}
			// Re-map PossibleHBs referencing the selected BRP and configured handlebar.
			bar := can.Handlebar
			if bar.ID == "" {
				bar = catalog.DefaultHandlebar()
			}
			can.PossibleHBs = geometry.MapPossibleHBsWithHandlebar(row.Bike, can.ActiveReach, can.ActiveStack, can.ActiveHta, activeBRP, bar, model.DefaultGripInsetMm, 0.0)
		}

		// 2. Cockpit hardware.
		switch {
		case hasClampFit && (rowDistMin.IsSet() || rowDistMax.IsSet()):
			if cost := cockpitCost(rowProf, row); cost != nil {
				geometry.ApplyCockpitFitByCost(can, row.Bike, rowDistMin, rowDistMax, rowDropMin, rowDropMax, prof.TargetRef, prof.ReferenceBar, cost)
			} else {
				geometry.ApplyCockpitFitForTarget(can, row.Bike, row.Bike.GetCanonicalStem(), rowDistMin, rowDistMax, rowDropMin, rowDropMax, prof.TargetRef, prof.ReferenceBar)
			}
		case len(can.PossibleHBs) > 0:
			if bestHB, ok := geometry.SelectBestHBWithDropForTarget(can.PossibleHBs, row.Bike.GetCanonicalStem(), rowDistMin, rowDistMax, rowDropMin, rowDropMax, activeBRP, prof.TargetRef, prof.ReferenceBar); ok {
				can.SelectedHB = &bestHB
				can.SaddleHbMm = bestHB.SaddleHbMm
				can.SaddleHbDropMm = bestHB.SaddleHbDropMm
			}
		}
	}
}

// brpCost prices a saddle rail position by the profile's brp_setback and
// rail_offset bands, or returns nil when neither band is scored.
func brpCost(prof *ideal.Profile, row *model.CanonicalComparisonRow) func(model.BRPPoint) float64 {
	setback, hasSetback := prof.Spec(ideal.MetricBrpSetback)
	rail, hasRail := prof.Spec(ideal.MetricRailOffset)
	hasSetback = hasSetback && setback.IsScored()
	hasRail = hasRail && rail.IsScored()
	if !hasSetback && !hasRail {
		return nil
	}
	return func(b model.BRPPoint) float64 {
		lost := 0.0
		if hasSetback {
			lost += ideal.RowPointsLost(setback, b.BrpSetbackMm, row)
		}
		if hasRail {
			lost += ideal.RowPointsLost(rail, b.RailOffsetMm, row)
		}
		return lost
	}
}

// cockpitCost prices a stock cockpit candidate by the profile's Oval Fits,
// stem_length, stem_angle and spacers bands, or returns nil when Oval Fits is
// not scored.
func cockpitCost(prof *ideal.Profile, row *model.CanonicalComparisonRow) geometry.CockpitCost {
	oval, ok := prof.Spec(ideal.MetricHandlebarClampFit)
	if !ok || !oval.IsScored() {
		return nil
	}
	type band struct {
		spec  ideal.AttributeSpec
		value func(model.HBPoint) float64
	}
	var scored []band
	for _, b := range []struct {
		id    ideal.MetricID
		value func(model.HBPoint) float64
	}{
		{ideal.MetricStemLength, func(pt model.HBPoint) float64 { return pt.Stem.LengthMm }},
		{ideal.MetricStemAngle, func(pt model.HBPoint) float64 { return float64(pt.Stem.AngleDeg) }},
		{ideal.MetricSpacers, func(pt model.HBPoint) float64 { return pt.SpacerMm }},
	} {
		if spec, ok := prof.Spec(b.id); ok && spec.IsScored() {
			scored = append(scored, band{spec: spec, value: b.value})
		}
	}
	return func(pt model.HBPoint, r float64) float64 {
		lost := ideal.OvalPointsLost(&model.CockpitFitResult{BestR: r}, oval.Cost)
		for _, b := range scored {
			lost += ideal.RowPointsLost(b.spec, b.value(pt), row)
		}
		return lost
	}
}
