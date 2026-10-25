package fit

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"bikegeo/pkg/catalog"
	"bikegeo/pkg/geometry"
	"bikegeo/pkg/model"
	"bikegeo/pkg/validator"
)

// FitResult holds the CAD geometry fit analysis for a frame configuration across all sizes.
type FitResult struct {
	Frame                 model.FrameModel            `json:"frame"`
	ConfigID              string                      `json:"config_id"`
	Residuals             []validator.SizeResidual    `json:"residuals"`
	BestFitStructuralA2c  float64                     `json:"best_fit_structural_a2c"` // Minimax midpoint
	MeanStructuralA2c     float64                     `json:"mean_structural_a2c"`
	SpreadStructuralA2c   float64                     `json:"spread_structural_a2c"`
	ImpliedSpacerMm       float64                     `json:"implied_spacer_mm"`
	ImpliedStaticForkA2c  float64                     `json:"implied_static_fork_a2c"`
	DeclaredSpacerMm      float64                     `json:"declared_spacer_mm"`
	DeclaredStaticForkA2c float64                     `json:"declared_static_fork_a2c"`
	DeclaredRake          float64                     `json:"declared_rake"`
	DeclaredSagPct        float64                     `json:"declared_sag_pct"`
	MinPassingSlopA2c     float64                     `json:"min_passing_slop_a2c"`
	MinPassingSlopFc      float64                     `json:"min_passing_slop_fc"`
	MinPassingSlopWb      float64                     `json:"min_passing_slop_wb"`
	HasPublishedFc        bool                        `json:"has_published_fc"`
	HasPublishedWb        bool                        `json:"has_published_wb"`
	IsValid               bool                        `json:"is_valid"`
	FrameReport           model.FrameValidationReport `json:"frame_report"`
}

// AnalyzeFit computes the minimax best-fit structural A2C, implied spacer, implied static fork A2C,
// per-size residuals, and minimum passing slop tolerances.
func AnalyzeFit(frame model.FrameModel, configID string) (*FitResult, error) {
	residuals, err := validator.SizeResiduals(frame)
	if err != nil {
		return nil, err
	}
	if len(residuals) == 0 {
		return nil, fmt.Errorf("no sizes found for frame %s", frame.ID)
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

	effectiveForkA2c := frame.CadFork.A2cMm
	statedForkA2c := frame.CadFork.StaticA2c()
	if statedForkA2c <= 0 {
		statedForkA2c = effectiveForkA2c
	}
	sagPct := 0.0
	if frame.CadFork.SagPct != nil {
		sagPct = *frame.CadFork.SagPct
	}
	sagComp := statedForkA2c - effectiveForkA2c

	declaredSpacer := frame.GetGeoHeadsetSpacerMm()
	impliedSpacer := geometry.Round(bestFit-effectiveForkA2c, 2)
	impliedStaticForkA2c := geometry.Round(bestFit-declaredSpacer+sagComp, 2)

	frameReport := validator.ValidateFrame(frame)

	return &FitResult{
		Frame:                 frame,
		ConfigID:              configID,
		Residuals:             residuals,
		BestFitStructuralA2c:  bestFit,
		MeanStructuralA2c:     mean,
		SpreadStructuralA2c:   spread,
		ImpliedSpacerMm:       impliedSpacer,
		ImpliedStaticForkA2c:  impliedStaticForkA2c,
		DeclaredSpacerMm:      declaredSpacer,
		DeclaredStaticForkA2c: statedForkA2c,
		DeclaredRake:          frame.CadFork.RakeMm,
		DeclaredSagPct:        sagPct,
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
		// Only treat as size prefix if prefix is NOT a known domain group (fork, cad_fork, default_fork, slop, geo_slop_mm)
		lowerPrefix := strings.ToLower(prefix)
		if lowerPrefix != "fork" && lowerPrefix != "cad_fork" && lowerPrefix != "default_fork" && lowerPrefix != "slop" && lowerPrefix != "geo_slop_mm" {
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

	// 1. Fork & Build Attributes
	switch normField {
	case "cad_fork.a2c", "cad_fork.a2c_mm", "fork.a2c", "fork_a2c", "fork.axle_to_crown", "a2c":
		v, err := parseFloat()
		if err != nil || v <= 0 {
			return fmt.Errorf("fork A2C must be a positive number")
		}
		frame.CadFork.A2cMm = v
		return nil

	case "cad_fork.rake", "cad_fork.rake_mm", "fork.rake", "fork_rake", "fork.offset", "rake":
		v, err := parseFloat()
		if err != nil || v <= 0 {
			return fmt.Errorf("fork rake must be a positive number")
		}
		frame.CadFork.RakeMm = v
		return nil

	case "cad_fork.sag", "sag", "sag_percent", "sag_percentage_published", "fork.sag":
		v, err := parseFloat()
		if err != nil || v < 0 || v > 50 {
			return fmt.Errorf("sag percentage must be between 0 and 50")
		}
		frame.CadFork.SagPct = model.Float64Ptr(v)
		return nil

	case "cad_fork.fork", "fork":
		frame.CadFork.Fork = val
		return nil

	case "default_fork.a2c", "default_fork.a2c_mm":
		v, err := parseFloat()
		if err != nil || v <= 0 {
			return fmt.Errorf("default_fork A2C must be a positive number")
		}
		frame.DefaultFork.A2cMm = v
		return nil

	case "default_fork.rake", "default_fork.rake_mm":
		v, err := parseFloat()
		if err != nil || v <= 0 {
			return fmt.Errorf("default_fork rake must be a positive number")
		}
		frame.DefaultFork.RakeMm = v
		return nil

	case "default_fork.sag":
		v, err := parseFloat()
		if err != nil || v < 0 || v > 50 {
			return fmt.Errorf("default_fork sag must be between 0 and 50")
		}
		frame.DefaultFork.SagPct = model.Float64Ptr(v)
		return nil

	case "default_fork.fork", "default_fork":
		frame.DefaultFork.Fork = val
		return nil

	// 2. Frame-Level Hardware & Metadata Attributes
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
	case "geo_slop_mm.reason", "slop.reason":
		if frame.GeoSlopMm == nil {
			frame.GeoSlopMm = &model.GeoSlopMm{}
		}
		frame.GeoSlopMm.Reason = val
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
		if frame.RigidFork == nil {
			frame.RigidFork = &model.ForkRef{}
		}
		frame.RigidFork.A2cMm = v
		return nil
	}

	// 3. Size-Specific Geometry Attributes
	if frame.Geometries == nil {
		frame.Geometries = make(map[string]model.SizeGeometry)
	}
	matchedSz := matchSize(frame.GetSizes(), targetSize)
	if matchedSz == "" {
		return fmt.Errorf("no target size specified or found for geometry field %q (available: %s)", normField, strings.Join(frame.GetSizes(), ", "))
	}
	geo := frame.Geometries[matchedSz]

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

	frame.Geometries[matchedSz] = geo
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

// SolveFit automatically explores candidate physical explanations for CAD geometry
// (plausible fork travels, standard sag points, static A2C values, rakes including 44, 51, 55mm, and lower spacers).
func SolveFit(frame model.FrameModel, configID, defaultSize string) ([]ComboResult, error) {
	baseFit, err := AnalyzeFit(frame, configID)
	if err != nil {
		return nil, err
	}
	targetA2c := baseFit.BestFitStructuralA2c

	// 1. Candidate Travels
	travelSet := make(map[float64]bool)
	for _, t := range []float64{0, 100, 110, 120, 130, 140, 150} {
		travelSet[t] = true
	}
	if tr := frame.CadFork.ForkTravel(); tr > 0 {
		travelSet[tr] = true
	}

	// 2. Candidate Rakes: declared rake, 44, 51, 55 mm
	rakeSet := make(map[float64]bool)
	rakeSet[44.0] = true
	rakeSet[51.0] = true
	rakeSet[55.0] = true
	if frame.CadFork.RakeMm > 0 {
		rakeSet[frame.CadFork.RakeMm] = true
	}
	var rakes []float64
	for r := range rakeSet {
		rakes = append(rakes, r)
	}
	sort.Float64s(rakes)

	// 3. Candidate Spacers
	spacerSet := make(map[float64]bool)
	spacerSet[frame.GetGeoHeadsetSpacerMm()] = true
	// Probe standard physical headset spacers: 0 (bare / integrated), 2 (ZS), and 10, 12, 14 (EC)
	for _, s := range []float64{0.0, 2.0, 10.0, 12.0, 14.0} {
		spacerSet[s] = true
	}
	var spacers []float64
	for s := range spacerSet {
		spacers = append(spacers, s)
	}
	sort.Float64s(spacers)

	// 4. Generate candidate (travel, sag, staticA2c, spacer)
	type forkCandidate struct {
		travel    float64
		sag       float64
		staticA2c float64
		spacer    float64
		effA2c    float64
		structA2c float64
	}

	var forkCombos []forkCandidate

	for travel := range travelSet {
		var sags []float64
		if travel == 0 {
			sags = []float64{0}
		} else {
			sags = []float64{0, 15, 20, 25, 30}
		}

		for _, sag := range sags {
			var staticA2cs []float64
			if travel == 0 {
				if frame.RigidFork != nil {
					staticA2cs = append(staticA2cs, frame.RigidFork.A2cMm)
				}
				if frame.CadFork.A2cMm > 0 {
					staticA2cs = append(staticA2cs, frame.CadFork.A2cMm)
				}
				for _, a := range []float64{440, 470, 480, 483, 485, 490, 495, 500, 510} {
					staticA2cs = append(staticA2cs, a)
				}
			} else {
				staticA2cs = []float64{travel + 406, travel + 410, travel + 412, travel + 420}
				if frame.CadFork.StaticA2c() > 0 {
					staticA2cs = append(staticA2cs, frame.CadFork.StaticA2c())
				}
			}

			a2cSeen := make(map[float64]bool)
			for _, a2c := range staticA2cs {
				if a2cSeen[a2c] {
					continue
				}
				a2cSeen[a2c] = true

				effA2c := a2c - travel*(sag/100.0)

				for _, sp := range spacers {
					structA2c := effA2c + sp
					if math.Abs(structA2c-targetA2c) <= 6.0 {
						forkCombos = append(forkCombos, forkCandidate{
							travel:    travel,
							sag:       sag,
							staticA2c: a2c,
							spacer:    sp,
							effA2c:    effA2c,
							structA2c: structA2c,
						})
					}
				}
			}
		}
	}

	var results []ComboResult
	seenCombos := make(map[string]bool)

	for _, fc := range forkCombos {
		for _, rake := range rakes {
			var assigns []string
			assigns = append(assigns, fmt.Sprintf("cad_fork.a2c=%.0f", fc.effA2c))
			assigns = append(assigns, fmt.Sprintf("cad_fork.rake=%.0f", rake))
			if fc.travel > 0 {
				assigns = append(assigns, fmt.Sprintf("cad_fork.sag=%.0f", fc.sag))
			}
			assigns = append(assigns, fmt.Sprintf("spacer=%.0f", fc.spacer))

			suggs := catalog.FindForkSuggestions(fc.effA2c, rake)
			if len(suggs) > 0 {
				assigns = append(assigns, fmt.Sprintf("cad_fork.fork=%s", suggs[0].ForkID))
			}

			desc := strings.Join(assigns, " ")
			if seenCombos[desc] {
				continue
			}
			seenCombos[desc] = true

			clone := CloneFrame(frame)
			for _, a := range assigns {
				_ = ApplyAssignment(&clone, configID, defaultSize, a)
			}

			fitRes, err := AnalyzeFit(clone, configID)
			if err != nil {
				continue
			}

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
	}

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

	if len(results) > 10 {
		results = results[:10]
	}

	return results, nil
}
