package canonicalizer

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"

	"bikegeo/pkg/geometry"
	"bikegeo/pkg/model"
	"bikegeo/pkg/validator"
)

var DefaultForkPresets = []model.ForkPreset{
	{
		ID:                 "fox34_130",
		Name:               "Fox Float 34 130mm",
		TravelMm:           130,
		StaticA2c:          541,
		Rake:               44,
		StandardSagPercent: 20,
		Description:        "Trail hardtail benchmark (static 541mm, 20% sag = 515mm, 44mm rake)",
	},
	{
		ID:                 "sid_110",
		Name:               "RockShox SID 110mm",
		TravelMm:           110,
		StaticA2c:          521,
		Rake:               44,
		StandardSagPercent: 20,
		Description:        "Downcountry / Marathon fork (static 521mm, 20% sag = 499mm, 44mm rake)",
	},
	{
		ID:                 "sid_100",
		Name:               "RockShox SID 100mm",
		TravelMm:           100,
		StaticA2c:          506,
		Rake:               44,
		StandardSagPercent: 20,
		Description:        "XC Race fork (static 506mm, 20% sag = 486mm, 44mm rake)",
	},
	{
		ID:                 "fox34_120",
		Name:               "Fox Float 34 120mm",
		TravelMm:           120,
		StaticA2c:          530,
		Rake:               44,
		StandardSagPercent: 20,
		Description:        "Downcountry standard (static 530mm, 20% sag = 506mm, 44mm rake)",
	},
	{
		ID:                 "sid_120",
		Name:               "RockShox SID 120mm",
		TravelMm:           120,
		StaticA2c:          530,
		Rake:               44,
		StandardSagPercent: 20,
		Description:        "XC race / Downcountry fork (static 530mm, 20% sag = 506mm, 44mm rake)",
	},
	{
		ID:                 "pike_140",
		Name:               "RockShox Pike 140mm",
		TravelMm:           140,
		StaticA2c:          551,
		Rake:               44,
		StandardSagPercent: 20,
		Description:        "Aggressive trail fork (static 551mm, 20% sag = 523mm, 44mm rake)",
	},
	{
		ID:                 "fox36_150",
		Name:               "Fox 36 / Lyrik 150mm",
		TravelMm:           150,
		StaticA2c:          561,
		Rake:               44,
		StandardSagPercent: 20,
		Description:        "Hardcore hardtail / Enduro (static 561mm, 20% sag = 531mm, 44mm rake)",
	},
	{
		ID:                 "rigid_483",
		Name:               "Rigid Bikepacking 483mm",
		TravelMm:           0,
		StaticA2c:          483,
		Rake:               44,
		StandardSagPercent: 0,
		Description:        "Rigid expedition fork, suspension-corrected ~100mm (483mm static, 44mm rake)",
	},
	{
		ID:                 "rigid_485",
		Name:               "Rigid Bikepacking 485mm",
		TravelMm:           0,
		StaticA2c:          485,
		Rake:               45,
		StandardSagPercent: 0,
		Description:        "Rigid adventure fork, suspension-corrected 100mm (485mm static, 45mm rake)",
	},
	{
		ID:                 "rigid_495",
		Name:               "Lithic / Rigid Bikepacking 495mm",
		TravelMm:           0,
		StaticA2c:          495,
		Rake:               51,
		StandardSagPercent: 0,
		Description:        "Rigid expedition fork, suspension-corrected 100-120mm (495mm static, 51mm rake)",
	},
}

var CommonForkPresets = DefaultForkPresets

func init() {
	candidates := []string{"data/forks.json", "../../data/forks.json"}
	for _, path := range candidates {
		if data, err := os.ReadFile(path); err == nil {
			var presets []model.ForkPreset
			if err := json.Unmarshal(data, &presets); err == nil && len(presets) > 0 {
				CommonForkPresets = presets
				break
			}
		}
	}
}

// GetForkPreset looks up a fork preset by ID or Name.
func GetForkPreset(id string) (model.ForkPreset, bool) {
	for _, p := range CommonForkPresets {
		if strings.EqualFold(p.ID, id) || strings.EqualFold(p.Name, id) {
			return p, true
		}
	}
	return model.ForkPreset{}, false
}

// GetStandardForkForTravel returns a standard 29er suspension ForkSpec for a given travel in mm.
func GetStandardForkForTravel(travelMm float64) model.ForkSpec {
	for _, p := range CommonForkPresets {
		if p.TravelMm > 0 && math.Abs(p.TravelMm-travelMm) < 0.1 {
			spec := p.ToForkSpec()
			spec.Name = fmt.Sprintf("%.0fmm Suspension", travelMm)
			return spec
		}
	}
	// Linear fallback for non-preset travel (e.g. 160mm -> 571mm)
	estA2c := 411.0 + travelMm
	return model.ForkSpec{
		Name:               fmt.Sprintf("%.0fmm Suspension", travelMm),
		TravelMm:           travelMm,
		A2c:                estA2c,
		Rake:               44.0,
		StandardSagPercent: 20.0,
	}
}

// ResolveForkSpec enriches a ForkSpec with registry metadata.
// If values were populated from the registry or if linked to a registry preset,
// IsInferred is set to true.
func ResolveForkSpec(spec model.ForkSpec) model.ForkSpec {
	if spec.ID == "" {
		return spec
	}
	preset, found := GetForkPreset(spec.ID)
	if !found {
		return spec
	}
	res := spec
	if res.Name == "" {
		res.Name = preset.Name
	}
	if res.TravelMm == 0 {
		res.TravelMm = preset.TravelMm
	}
	if res.A2c == 0 {
		res.A2c = preset.StaticA2c
	}
	if res.Rake == 0 {
		res.Rake = preset.Rake
	}
	if res.StandardSagPercent == 0 {
		res.StandardSagPercent = preset.StandardSagPercent
	}
	if res.Description == "" {
		res.Description = preset.Description
	}
	res.IsInferred = true
	return res
}

// CalculateSaggedA2c calculates effective A2C taking suspension sag into consideration.
func CalculateSaggedA2c(staticA2c, travelMm, sagPercent float64) float64 {
	sagMm := travelMm * (sagPercent / 100.0)
	return geometry.Round(staticA2c-sagMm, 1)
}

// ResolveFrameForkSetup validates and resolves a ComparisonSetup (size, fork travel or rigid, and sag %)
// against a FrameModel's physical constraints and RiderProfile, returning the canonicalized comparison row.
func ResolveFrameForkSetup(
	frame model.FrameModel,
	setup model.ComparisonSetup,
	profile model.RiderProfile,
) (model.CanonicalComparisonRow, error) {
	if profile.SaddleHeightMm <= 0 {
		return model.CanonicalComparisonRow{}, fmt.Errorf("profile.SaddleHeightMm must be positive, got %.1f", profile.SaddleHeightMm)
	}

	// 0. Centralized frame validity gate: a frame in an error state may not be used.
	frameVal := validator.ValidateFrame(frame)
	if !frameVal.IsValid {
		return model.CanonicalComparisonRow{}, fmt.Errorf("%s", frameVal.ErrorSummary())
	}

	// 1. Validate & normalize size
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

	// 2. Resolve Fork (suspension travel mm vs rigid)
	forkToken := strings.TrimSpace(setup.Fork)
	ExplicitlyProvidedFork := forkToken != ""
	if forkToken == "" {
		forkToken = frame.GetDefaultForkSpecString()
	}
	lowerFork := strings.ToLower(forkToken)

	var resolvedFork model.ForkSpec
	var canonicalForkLabel string
	isRigid := false

	// Check if token represents a rigid fork ("rigid", "0mm", or a rigid preset/config)
	if lowerFork == "rigid" || lowerFork == "0mm" || lowerFork == "0" || strings.HasPrefix(lowerFork, "rigid") {
		isRigid = true
	} else if preset, ok := GetForkPreset(forkToken); ok && preset.TravelMm == 0 {
		isRigid = true
	} else {
		for _, cfg := range frame.Configurations {
			if strings.EqualFold(cfg.ID, forkToken) && cfg.Fork.TravelMm == 0 {
				isRigid = true
				break
			}
		}
	}

	supportedDesc := strings.Join(frame.GetSupportedForkOptions(), ", ")

	if isRigid {
		if !frame.SupportsRigid() {
			return model.CanonicalComparisonRow{}, fmt.Errorf(
				"frame \"%s %s\" (%s) does not support a rigid fork (supported: %s)",
				frame.Manufacturer, frame.Model, frame.ID, supportedDesc,
			)
		}
		if setup.SagPercent != nil && *setup.SagPercent > 0 {
			return model.CanonicalComparisonRow{}, fmt.Errorf(
				"cannot apply %.0f%% sag to rigid fork on \"%s %s\" (%s); rigid forks have 0mm travel",
				*setup.SagPercent, frame.Manufacturer, frame.Model, frame.ID,
			)
		}
		rigidA2c := frame.GetRigidA2c()
		rake := 44.0
		// Check if frame has a rigid config for rake
		for _, cfg := range frame.Configurations {
			if cfg.Fork.TravelMm == 0 && cfg.Fork.Rake > 0 {
				rake = cfg.Fork.Rake
				break
			}
		}
		resolvedFork = model.ForkSpec{
			ID:       fmt.Sprintf("rigid_%.0f", rigidA2c),
			Name:     fmt.Sprintf("Rigid (%.0fmm A2C)", rigidA2c),
			TravelMm: 0,
			A2c:      rigidA2c,
			Rake:     rake,
		}
		canonicalForkLabel = "rigid"
	} else {
		// Suspension fork requested
		if frame.IsFixedRigidOnly() && ExplicitlyProvidedFork {
			return model.CanonicalComparisonRow{}, fmt.Errorf(
				"frame \"%s %s\" (%s) has a fixed rigid fork (%.0fmm A2C) and does not support suspension fork %q",
				frame.Manufacturer, frame.Model, frame.ID, frame.GetRigidA2c(), forkToken,
			)
		}

		// Parse travel in mm from token (e.g. "120mm", "120", or preset/config ID)
		var travelMm float64
		var explicitPreset *model.ForkPreset

		cleaned := strings.TrimSuffix(lowerFork, "mm")
		if val, err := strconv.ParseFloat(cleaned, 64); err == nil && val > 0 {
			travelMm = val
		} else if preset, ok := GetForkPreset(forkToken); ok && preset.TravelMm > 0 {
			travelMm = preset.TravelMm
			explicitPreset = &preset
		} else {
			for _, cfg := range frame.Configurations {
				if strings.EqualFold(cfg.ID, forkToken) && cfg.Fork.TravelMm > 0 {
					travelMm = cfg.Fork.TravelMm
					break
				}
			}
		}

		if travelMm <= 0 {
			return model.CanonicalComparisonRow{}, fmt.Errorf(
				"invalid fork specification %q for frame \"%s %s\" (supported: %s)",
				forkToken, frame.Manufacturer, frame.Model, supportedDesc,
			)
		}

		if !frame.SupportsTravel(travelMm) {
			return model.CanonicalComparisonRow{}, fmt.Errorf(
				"frame \"%s %s\" (%s) does not support a %.0fmm suspension fork (supported: %s)",
				frame.Manufacturer, frame.Model, frame.ID, travelMm, supportedDesc,
			)
		}

		// Resolve A2C and Rake for this travel size:
		// 1. Explicit preset if specified
		// 2. Frame's own configuration matching this travel (if its A2C is static suspension A2C)
		// 3. Standard fork for this travel from forks.json
		if explicitPreset != nil {
			resolvedFork = explicitPreset.ToForkSpec()
		} else {
			foundInFrame := false
			for _, cfg := range frame.Configurations {
				if math.Abs(cfg.Fork.TravelMm-travelMm) < 0.1 && cfg.Fork.A2c > 0 {
					resolvedFork = cfg.Fork
					resolvedFork.Name = fmt.Sprintf("%.0fmm Suspension (%.0fmm A2C)", travelMm, cfg.Fork.A2c)
					foundInFrame = true
					break
				}
			}
			if !foundInFrame {
				std := GetStandardForkForTravel(travelMm)
				std.Name = fmt.Sprintf("%.0fmm Suspension (%.0fmm A2C)", travelMm, std.A2c)
				resolvedFork = std
			}
		}
		canonicalForkLabel = fmt.Sprintf("%.0fmm", travelMm)
	}

	// 3. Resolve & validate Sag %
	sagPct := 0.0
	if !isRigid {
		if setup.SagPercent != nil {
			if *setup.SagPercent < 0 || *setup.SagPercent > 50 {
				return model.CanonicalComparisonRow{}, fmt.Errorf(
					"invalid sag percentage %.1f%% for \"%s %s\" (must be between 0%% and 50%%)",
					*setup.SagPercent, frame.Manufacturer, frame.Model,
				)
			}
			sagPct = *setup.SagPercent
		} else if resolvedFork.StandardSagPercent > 0 {
			sagPct = resolvedFork.StandardSagPercent
		} else {
			sagPct = 20.0
		}
	}

	// 4. Materialize baseline BikeGeometry for this frame & size
	baseCfg := frame.GetBaselineConfiguration()
	baseCfgID := ""
	if baseCfg != nil {
		baseCfgID = baseCfg.ID
	}
	bike, err := geometry.MaterializeBikeGeometry(frame, size, baseCfgID)
	if err != nil {
		return model.CanonicalComparisonRow{}, err
	}

	// 5. Compute Effective A2C
	effectiveA2c := CalculateSaggedA2c(resolvedFork.A2c, resolvedFork.TravelMm, sagPct)

	canonCfg := model.CanonicalConfig{
		TargetA2c:    effectiveA2c,
		SagPercent:   sagPct,
		RiderProfile: profile,
	}

	rows, err := CanonicalizeBikes([]model.BikeGeometry{bike}, canonCfg)
	if err != nil {
		return model.CanonicalComparisonRow{}, err
	}
	if len(rows) == 0 {
		return model.CanonicalComparisonRow{}, fmt.Errorf("failed to canonicalize %s", frame.ID)
	}

	row := rows[0]
	sagCopy := sagPct
	row.Setup = model.ComparisonSetup{
		FrameID:    frame.ID,
		Size:       size,
		Fork:       canonicalForkLabel,
		SagPercent: &sagCopy,
	}
	row.SetupFork = resolvedFork
	row.SagPercent = sagPct
	row.EffectiveA2c = effectiveA2c

	return row, nil
}

// CanonicalizeBikes normalizes and canonicalizes a collection of bikes to an apples-to-apples configuration.
func CanonicalizeBikes(bikes []model.BikeGeometry, config model.CanonicalConfig) ([]model.CanonicalComparisonRow, error) {
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

		baseA2c := bike.ForkA2c
		if baseA2c <= 0 {
			baseA2c = valReport.GeometricA2c
		}

		activeCS := bike.ChainstayLength
		if bike.HasAdjustableChainstay() {
			activeCS = bike.GetMaxChainstay()
		}

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
		}

		canonical, err := geometry.AnalyzeBikeFit(
			bike,
			&config.TargetA2c,
			config.RiderProfile,
		)
		if err != nil {
			return nil, err
		}

		rows = append(rows, model.CanonicalComparisonRow{
			SagPercent:   config.SagPercent,
			EffectiveA2c: config.TargetA2c,
			Bike:         bike,
			Validation:   valReport,
			Baseline:     baseline,
			Canonical:    canonical,
		})
	}

	return rows, nil
}
