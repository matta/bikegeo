package fit

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"bikegeo/pkg/geometry"
	"bikegeo/pkg/model"
	"bikegeo/pkg/validator"
)

// FitResult holds the CAD geometry fit analysis for a frame configuration across all sizes.
type FitResult struct {
	Frame                   model.FrameModel              `json:"frame"`
	ConfigID                string                        `json:"config_id"`
	Residuals               []validator.SizeResidual      `json:"residuals"`
	BestFitStructuralA2c    float64                       `json:"best_fit_structural_a2c"` // Minimax midpoint
	MeanStructuralA2c       float64                       `json:"mean_structural_a2c"`
	SpreadStructuralA2c     float64                       `json:"spread_structural_a2c"`
	ImpliedSpacerMm         float64                       `json:"implied_spacer_mm"`
	ImpliedStaticForkA2c    float64                       `json:"implied_static_fork_a2c"`
	DeclaredSpacerMm        float64                       `json:"declared_spacer_mm"`
	DeclaredStaticForkA2c   float64                       `json:"declared_static_fork_a2c"`
	DeclaredRake            float64                       `json:"declared_rake"`
	DeclaredSagPct          float64                       `json:"declared_sag_pct"`
	MinPassingSlopA2c       float64                       `json:"min_passing_slop_a2c"`
	MinPassingSlopFc        float64                       `json:"min_passing_slop_fc"`
	MinPassingSlopWb        float64                       `json:"min_passing_slop_wb"`
	HasPublishedFc          bool                          `json:"has_published_fc"`
	HasPublishedWb          bool                          `json:"has_published_wb"`
	IsValid                 bool                          `json:"is_valid"`
	FrameReport             model.FrameValidationReport   `json:"frame_report"`
}

// AnalyzeFit computes the minimax best-fit structural A2C, implied spacer, implied static fork A2C,
// per-size residuals, and minimum passing slop tolerances.
func AnalyzeFit(frame model.FrameModel, configID string) (*FitResult, error) {
	residuals, err := validator.SizeResiduals(frame, configID)
	if err != nil {
		return nil, err
	}
	if len(residuals) == 0 {
		return nil, fmt.Errorf("no sizes found for frame %s", frame.ID)
	}

	targetCfg := frame.GetConfiguration(configID)
	if targetCfg == nil {
		targetCfg = frame.GetBaselineConfiguration()
	}
	if targetCfg == nil {
		return nil, fmt.Errorf("configuration %q not found on frame %s", configID, frame.ID)
	}

	minStruct := math.Inf(1)
	maxStruct := math.Inf(-1)
	sumStruct := 0.0

	maxAbsDeltaA2c := 0.0
	maxAbsDeltaFc := 0.0
	maxAbsDeltaWb := 0.0
	hasFc := false
	hasWb := false

	for _, r := range residuals {
		if r.ImpliedStructuralA2c < minStruct {
			minStruct = r.ImpliedStructuralA2c
		}
		if r.ImpliedStructuralA2c > maxStruct {
			maxStruct = r.ImpliedStructuralA2c
		}
		sumStruct += r.ImpliedStructuralA2c

		absA2c := math.Abs(r.DeltaA2c)
		if absA2c > maxAbsDeltaA2c {
			maxAbsDeltaA2c = absA2c
		}

		if r.DeltaFc != nil {
			hasFc = true
			absFc := math.Abs(*r.DeltaFc)
			if absFc > maxAbsDeltaFc {
				maxAbsDeltaFc = absFc
			}
		}

		if r.DeltaWb != nil {
			hasWb = true
			absWb := math.Abs(*r.DeltaWb)
			if absWb > maxAbsDeltaWb {
				maxAbsDeltaWb = absWb
			}
		}
	}

	bestFit := geometry.Round((minStruct+maxStruct)/2.0, 2)
	spread := geometry.Round(maxStruct-minStruct, 2)
	mean := geometry.Round(sumStruct/float64(len(residuals)), 2)

	// Update DeltaVsBest for each size
	for i := range residuals {
		residuals[i].DeltaVsBest = geometry.Round(residuals[i].ImpliedStructuralA2c-bestFit, 2)
	}

	statedForkA2c := targetCfg.Fork.A2c
	if statedForkA2c <= 0 && targetCfg.Fork.TravelMm > 0 {
		statedForkA2c = targetCfg.Fork.TravelMm + 412.0
	}

	sagComp := 0.0
	if targetCfg.SagPercentagePublished > 0 && targetCfg.Fork.TravelMm > 0 {
		sagComp = targetCfg.Fork.TravelMm * (targetCfg.SagPercentagePublished / 100.0)
	}
	effectiveForkA2c := statedForkA2c - sagComp

	declaredSpacer := frame.GetGeoHeadsetSpacerMm()
	impliedSpacer := geometry.Round(bestFit-effectiveForkA2c, 2)
	impliedStaticForkA2c := geometry.Round(bestFit-declaredSpacer+sagComp, 2)

	frameReport := validator.ValidateFrame(frame)

	return &FitResult{
		Frame:                 frame,
		ConfigID:              targetCfg.ID,
		Residuals:             residuals,
		BestFitStructuralA2c:  bestFit,
		MeanStructuralA2c:     mean,
		SpreadStructuralA2c:   spread,
		ImpliedSpacerMm:       impliedSpacer,
		ImpliedStaticForkA2c:  impliedStaticForkA2c,
		DeclaredSpacerMm:      declaredSpacer,
		DeclaredStaticForkA2c: statedForkA2c,
		DeclaredRake:          targetCfg.Fork.Rake,
		DeclaredSagPct:        targetCfg.SagPercentagePublished,
		MinPassingSlopA2c:     geometry.Round(maxAbsDeltaA2c, 1),
		MinPassingSlopFc:      geometry.Round(maxAbsDeltaFc, 1),
		MinPassingSlopWb:      geometry.Round(maxAbsDeltaWb, 1),
		HasPublishedFc:        hasFc,
		HasPublishedWb:        hasWb,
		IsValid:               frameReport.IsValid,
		FrameReport:           frameReport,
	}, nil
}

// CloneFrame creates a deep copy of a FrameModel.
func CloneFrame(frame model.FrameModel) model.FrameModel {
	data, _ := json.Marshal(frame)
	var clone model.FrameModel
	_ = json.Unmarshal(data, &clone)
	return clone
}

// ApplyAssignment parses and applies a key=value mutation to a FrameModel.
// Supports size prefixes (e.g. "XL:stack=655" or "stack=655" with defaultSize),
// fork fields ("fork.a2c=472", "fork.rake=52", "fork.travel=120", "sag=20"),
// hardware fields ("spacer=11", "headset_cup_type=EC44"), and slop ("slop.a2c=1.5").
func ApplyAssignment(frame *model.FrameModel, targetConfigID, defaultSize, assignment string) error {
	parts := strings.SplitN(assignment, "=", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid assignment %q (expected field=value)", assignment)
	}
	key := strings.TrimSpace(parts[0])
	val := strings.TrimSpace(parts[1])

	targetCfg := frame.GetConfiguration(targetConfigID)
	if targetCfg == nil {
		targetCfg = frame.GetBaselineConfiguration()
	}
	if targetCfg == nil {
		return fmt.Errorf("configuration %q not found on frame %s", targetConfigID, frame.ID)
	}
	if targetCfg.Geometries == nil {
		targetCfg.Geometries = make(map[string]model.SizeGeometry)
	}

	parseFloat := func() (float64, error) {
		v, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid numeric value %q for %s", val, key)
		}
		return v, nil
	}

	// Check for size prefix (e.g. "XL:stack", "XL.stack")
	var targetSize string
	fieldKey := key

	if colonIdx := strings.Index(key, ":"); colonIdx != -1 {
		prefix := strings.TrimSpace(key[:colonIdx])
		fieldKey = strings.TrimSpace(key[colonIdx+1:])
		matched := matchSize(frame.GetSizes(), prefix)
		if matched == "" {
			return fmt.Errorf("unknown size %q in assignment %q (available: %s)", prefix, assignment, strings.Join(frame.GetSizes(), ", "))
		}
		targetSize = matched
	} else if dotIdx := strings.Index(key, "."); dotIdx != -1 {
		prefix := strings.TrimSpace(key[:dotIdx])
		// Only treat as size prefix if prefix is NOT a known domain group (fork, slop, geo_slop_mm)
		lowerPrefix := strings.ToLower(prefix)
		if lowerPrefix != "fork" && lowerPrefix != "slop" && lowerPrefix != "geo_slop_mm" {
			matched := matchSize(frame.GetSizes(), prefix)
			if matched != "" {
				targetSize = matched
				fieldKey = strings.TrimSpace(key[dotIdx+1:])
			}
		}
	}

	if targetSize == "" {
		targetSize = defaultSize
		if targetSize == "" {
			targetSize = frame.GetDefaultSize()
		}
	}

	normField := strings.ToLower(strings.TrimSpace(fieldKey))

	// 1. Fork & Build Attributes (applies to target configuration)
	switch normField {
	case "fork.a2c", "fork_a2c", "fork.axle_to_crown":
		v, err := parseFloat()
		if err != nil || v <= 0 {
			return fmt.Errorf("fork A2C must be a positive number")
		}
		targetCfg.Fork.A2c = v
		return nil

	case "fork.rake", "fork_rake", "fork.offset":
		v, err := parseFloat()
		if err != nil || v <= 0 {
			return fmt.Errorf("fork rake must be a positive number")
		}
		targetCfg.Fork.Rake = v
		return nil

	case "fork.travel_mm", "fork.travel", "fork_travel", "travel":
		v, err := parseFloat()
		if err != nil || v < 0 {
			return fmt.Errorf("fork travel must be non-negative")
		}
		targetCfg.Fork.TravelMm = v
		return nil

	case "sag", "sag_percent", "sag_percentage_published", "fork.sag":
		v, err := parseFloat()
		if err != nil || v < 0 || v > 50 {
			return fmt.Errorf("sag percentage must be between 0 and 50")
		}
		targetCfg.SagPercentagePublished = v
		return nil
	}

	// 2. Frame-Level Hardware & Metadata Attributes
	switch normField {
	case "manufacturer":
		frame.Manufacturer = val
		return nil
	case "model":
		frame.Model = val
		return nil
	case "year":
		yr, err := strconv.Atoi(val)
		if err != nil {
			return fmt.Errorf("invalid year %q", val)
		}
		frame.Year = yr
		return nil
	case "category":
		frame.Category = model.BikeCategory(val)
		return nil
	case "wheel_size":
		frame.WheelSize = val
		return nil
	case "default_fork":
		frame.DefaultFork = val
		return nil
	case "default_size":
		frame.DefaultSize = val
		return nil
	case "notes":
		frame.Notes = val
		return nil
	case "headset_cup_type", "cup_type":
		ct := model.HeadsetCupType(strings.ToUpper(val))
		if !ct.IsValid() {
			return fmt.Errorf("invalid headset_cup_type %q (valid: EC34, EC44, ZS44, ZS56, IS42, IS52)", val)
		}
		frame.HeadsetCupType = ct
		return nil
	case "geo_headset_spacer_mm", "geo_spacer_mm", "geo_spacer", "headset_spacer_mm", "spacer":
		v, err := parseFloat()
		if err != nil || v < 0 || v > 25.0 {
			return fmt.Errorf("geo_headset_spacer_mm must be between 0 and 25 mm")
		}
		frame.GeoHeadsetSpacerMm = model.Float64Ptr(v)
		return nil
	case "geo_slop_mm.a2c", "slop.a2c":
		v, err := parseFloat()
		if err != nil || v < 0 || v > 10.0 {
			return fmt.Errorf("geo_slop_mm.a2c must be between 0 and 10 mm")
		}
		if frame.GeoSlopMm == nil {
			frame.GeoSlopMm = &model.GeoSlopMm{}
		}
		frame.GeoSlopMm.A2c = model.Float64Ptr(v)
		return nil
	case "geo_slop_mm.front_center", "slop.front_center", "slop.fc":
		v, err := parseFloat()
		if err != nil || v < 0 || v > 10.0 {
			return fmt.Errorf("geo_slop_mm.front_center must be between 0 and 10 mm")
		}
		if frame.GeoSlopMm == nil {
			frame.GeoSlopMm = &model.GeoSlopMm{}
		}
		frame.GeoSlopMm.FrontCenter = model.Float64Ptr(v)
		return nil
	case "geo_slop_mm.wheelbase", "slop.wheelbase", "slop.wb":
		v, err := parseFloat()
		if err != nil || v < 0 || v > 10.0 {
			return fmt.Errorf("geo_slop_mm.wheelbase must be between 0 and 10 mm")
		}
		if frame.GeoSlopMm == nil {
			frame.GeoSlopMm = &model.GeoSlopMm{}
		}
		frame.GeoSlopMm.Wheelbase = model.Float64Ptr(v)
		return nil
	case "geo_slop_mm.cross_config", "slop.cross_config":
		v, err := parseFloat()
		if err != nil || v < 0 || v > 10.0 {
			return fmt.Errorf("geo_slop_mm.cross_config must be between 0 and 10 mm")
		}
		if frame.GeoSlopMm == nil {
			frame.GeoSlopMm = &model.GeoSlopMm{}
		}
		frame.GeoSlopMm.CrossConfig = model.Float64Ptr(v)
		return nil
	case "actual_sta_deg", "actual_sta":
		v, err := parseFloat()
		if err != nil || v < 60.0 || v > 85.0 {
			return fmt.Errorf("actual_sta_deg must be between 60.0 and 85.0 degrees")
		}
		frame.ActualStaDeg = v
		return nil
	case "seat_tube_offset_x_mm", "seat_tube_offset":
		v, err := parseFloat()
		if err != nil || v < 0 || v > 100.0 {
			return fmt.Errorf("seat_tube_offset_x_mm must be between 0.0 and 100.0 mm")
		}
		frame.SeatTubeOffsetXMm = model.Float64Ptr(v)
		return nil
	case "rigid_a2c_mm", "rigid_a2c":
		v, err := parseFloat()
		if err != nil || v < 0 {
			return fmt.Errorf("rigid_a2c_mm must be non-negative")
		}
		frame.RigidA2cMm = v
		return nil
	case "supported_travel_mm", "supported_travel":
		var travels []float64
		if val != "" && val != "none" && val != "0" {
			for _, p := range strings.Split(val, ",") {
				v, err := strconv.ParseFloat(strings.TrimSuffix(strings.TrimSpace(p), "mm"), 64)
				if err != nil || v <= 0 {
					return fmt.Errorf("invalid travel value %q", p)
				}
				travels = append(travels, v)
			}
		}
		frame.SupportedTravelMm = travels
		return nil
	}

	// 3. Size-Specific Geometry Attributes
	matchedSz := matchSize(frame.GetSizes(), targetSize)
	if matchedSz == "" {
		return fmt.Errorf("no target size specified or found for geometry field %q (available: %s)", normField, strings.Join(frame.GetSizes(), ", "))
	}
	geo := targetCfg.Geometries[matchedSz]

	switch normField {
	case "reach":
		v, err := parseFloat()
		if err != nil || v <= 0 {
			return fmt.Errorf("reach must be a positive number")
		}
		geo.Reach = v
	case "stack":
		v, err := parseFloat()
		if err != nil || v <= 0 {
			return fmt.Errorf("stack must be a positive number")
		}
		geo.Stack = v
	case "hta", "hta_deg":
		v, err := parseFloat()
		if err != nil || v <= 0 {
			return fmt.Errorf("hta_deg must be a positive number")
		}
		geo.HtaDeg = v
	case "sta", "sta_deg":
		v, err := parseFloat()
		if err != nil || v <= 0 {
			return fmt.Errorf("sta_deg must be a positive number")
		}
		geo.StaDeg = v
	case "htl":
		v, err := parseFloat()
		if err != nil || v < 0 {
			return fmt.Errorf("htl must be a non-negative number")
		}
		geo.Htl = v
	case "bb_drop", "bbdrop":
		v, err := parseFloat()
		if err != nil {
			return err
		}
		geo.BbDrop = v
	case "wheelbase":
		v, err := parseFloat()
		if err != nil || v < 0 {
			return fmt.Errorf("wheelbase must be non-negative")
		}
		geo.Wheelbase = v
	case "chainstay", "chainstay_length":
		v, err := parseFloat()
		if err != nil || v < 0 {
			return fmt.Errorf("chainstay_length must be non-negative")
		}
		geo.ChainstayLength = v
	case "front_center":
		v, err := parseFloat()
		if err != nil || v < 0 {
			return fmt.Errorf("front_center must be non-negative")
		}
		geo.FrontCenter = v
	case "seat_tube", "seat_tube_length":
		v, err := parseFloat()
		if err != nil || v < 0 {
			return fmt.Errorf("seat_tube_length must be non-negative")
		}
		geo.SeatTubeLength = v
	case "standover", "standover_height":
		v, err := parseFloat()
		if err != nil || v < 0 {
			return fmt.Errorf("standover_height must be non-negative")
		}
		geo.StandoverHeight = v
	default:
		return fmt.Errorf("unknown field %q", normField)
	}

	targetCfg.Geometries[matchedSz] = geo
	return nil
}

func matchSize(available []string, target string) string {
	target = strings.TrimSpace(target)
	for _, s := range available {
		if strings.EqualFold(s, target) {
			return s
		}
	}
	return ""
}

// TrySpec represents one --try specification, e.g. Key="fork.a2c", Values=["470", "472"].
type TrySpec struct {
	Key    string
	Values []string
}

// ParseTryFlag parses a flag value like "fork.a2c=470,472" or "XL:stack=655,660".
func ParseTryFlag(flagVal string) (TrySpec, error) {
	parts := strings.SplitN(flagVal, "=", 2)
	if len(parts) != 2 {
		return TrySpec{}, fmt.Errorf("invalid --try format %q (expected key=v1,v2,...)", flagVal)
	}
	key := strings.TrimSpace(parts[0])
	valStr := strings.TrimSpace(parts[1])
	if key == "" || valStr == "" {
		return TrySpec{}, fmt.Errorf("empty key or value in --try %q", flagVal)
	}

	valTokens := strings.Split(valStr, ",")
	var values []string
	for _, tok := range valTokens {
		tok = strings.TrimSpace(tok)
		if tok != "" {
			values = append(values, tok)
		}
	}
	if len(values) == 0 {
		return TrySpec{}, fmt.Errorf("no values provided in --try %q", flagVal)
	}

	return TrySpec{Key: key, Values: values}, nil
}

// ComboResult holds the result of testing a specific combination of trial values.
type ComboResult struct {
	Description       string     `json:"description"`
	Assignments       []string   `json:"assignments"`
	MaxAbsDeltaA2c    float64    `json:"max_abs_delta_a2c"`
	MaxAbsDeltaFc     float64    `json:"max_abs_delta_fc"`
	MaxAbsDeltaWb     float64    `json:"max_abs_delta_wb"`
	MinPassingSlopA2c float64    `json:"min_passing_slop_a2c"`
	MinPassingSlopFc  float64    `json:"min_passing_slop_fc"`
	MinPassingSlopWb  float64    `json:"min_passing_slop_wb"`
	IsValid           bool       `json:"is_valid"`
	FitResult         *FitResult `json:"fit_result,omitempty"`
}

const MaxTryCombinations = 200

// RunSweeps executes a combinatorial sweep across all provided --try specifications.
// Combinations are capped at MaxTryCombinations (200).
// Results are returned sorted with valid results first, then lowest max |ΔA2C|, then lowest |ΔWB| and |ΔFC|.
func RunSweeps(frame model.FrameModel, configID, defaultSize string, tryFlags []string) ([]ComboResult, error) {
	if len(tryFlags) == 0 {
		return nil, nil
	}

	var specs []TrySpec
	totalCombos := 1
	for _, flagVal := range tryFlags {
		spec, err := ParseTryFlag(flagVal)
		if err != nil {
			return nil, err
		}
		specs = append(specs, spec)
		totalCombos *= len(spec.Values)
	}

	if totalCombos > MaxTryCombinations {
		return nil, fmt.Errorf("total combinations %d exceeds limit of %d; please narrow your --try options", totalCombos, MaxTryCombinations)
	}

	// Generate Cartesian product of assignments
	var comboAssignments [][]string
	var generateCombos func(specIdx int, current []string)
	generateCombos = func(specIdx int, current []string) {
		if specIdx == len(specs) {
			copied := make([]string, len(current))
			copy(copied, current)
			comboAssignments = append(comboAssignments, copied)
			return
		}
		spec := specs[specIdx]
		for _, val := range spec.Values {
			assign := fmt.Sprintf("%s=%s", spec.Key, val)
			generateCombos(specIdx+1, append(current, assign))
		}
	}
	generateCombos(0, nil)

	var results []ComboResult

	for _, assigns := range comboAssignments {
		clone := CloneFrame(frame)
		var descParts []string
		for _, a := range assigns {
			if err := ApplyAssignment(&clone, configID, defaultSize, a); err != nil {
				return nil, fmt.Errorf("failed to apply %q: %w", a, err)
			}
			descParts = append(descParts, a)
		}

		fitRes, err := AnalyzeFit(clone, configID)
		if err != nil {
			return nil, err
		}

		desc := strings.Join(descParts, " ")
		results = append(results, ComboResult{
			Description:       desc,
			Assignments:       assigns,
			MaxAbsDeltaA2c:    fitRes.MinPassingSlopA2c,
			MaxAbsDeltaFc:     fitRes.MinPassingSlopFc,
			MaxAbsDeltaWb:     fitRes.MinPassingSlopWb,
			MinPassingSlopA2c: fitRes.MinPassingSlopA2c,
			MinPassingSlopFc:  fitRes.MinPassingSlopFc,
			MinPassingSlopWb:  fitRes.MinPassingSlopWb,
			IsValid:           fitRes.IsValid,
			FitResult:         fitRes,
		})
	}

	// Sort combinations:
	// 1. IsValid (true before false)
	// 2. Lowest MaxAbsDeltaA2c
	// 3. Lowest MaxAbsDeltaWb
	// 4. Lowest MaxAbsDeltaFc
	sort.Slice(results, func(i, j int) bool {
		if results[i].IsValid != results[j].IsValid {
			return results[i].IsValid
		}
		if math.Abs(results[i].MaxAbsDeltaA2c-results[j].MaxAbsDeltaA2c) > 0.001 {
			return results[i].MaxAbsDeltaA2c < results[j].MaxAbsDeltaA2c
		}
		if math.Abs(results[i].MaxAbsDeltaWb-results[j].MaxAbsDeltaWb) > 0.001 {
			return results[i].MaxAbsDeltaWb < results[j].MaxAbsDeltaWb
		}
		return results[i].MaxAbsDeltaFc < results[j].MaxAbsDeltaFc
	})

	return results, nil
}
