package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"

	"bikegeo/pkg/canonicalizer"
	"bikegeo/pkg/geometry"
	"bikegeo/pkg/model"
	"bikegeo/pkg/validator"
)

func (a *App) runInit(args []string) int {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(a.Err)

	var manufacturer string
	var modelName string
	var sizesSpec string
	var supportedTravelStr string
	var rigidA2c float64
	var defaultFork string
	var cupType string
	var geoSpacer float64
	var save bool

	fs.StringVar(&manufacturer, "manufacturer", "Custom", "Frame manufacturer name")
	fs.StringVar(&modelName, "model", "Hardtail", "Frame model name")
	fs.StringVar(&sizesSpec, "sizes", "s_xl", "Sizes preset (s_xl, s1_s6, cm) or comma-separated list (e.g. M,L,XL)")
	fs.StringVar(&supportedTravelStr, "supported-travel", "120,130", "Comma-separated supported suspension travel sizes in mm (or 'none' for rigid-only)")
	fs.Float64Var(&rigidA2c, "rigid-a2c", 0, "Supported rigid fork axle-to-crown in mm (0 if rigid not supported)")
	fs.StringVar(&defaultFork, "default-fork", "", "Default fork (e.g. '120mm' or 'rigid')")
	fs.StringVar(&cupType, "cup-type", "EC44", "Headset lower cup standard (EC34, EC44, ZS44, IS52, ZS56)")
	fs.Float64Var(&geoSpacer, "geo-spacer", -1, "Below-headset spacer in mm that the CAD chart was drawn with (default: matches --cup-type stack)")
	fs.BoolVar(&save, "save", false, "Save generated template directly into data/bikes/<id>.json")

	reordered := reorderFlagsForStdlib(fs, args, map[string]bool{"save": true})
	if err := fs.Parse(reordered); err != nil {
		return 2
	}

	var sizes []string
	switch strings.ToLower(strings.TrimSpace(sizesSpec)) {
	case "s_xl", "":
		sizes = []string{"S", "M", "L", "XL"}
	case "s1_s6":
		sizes = []string{"S1", "S2", "S3", "S4", "S5", "S6"}
	case "cm":
		sizes = []string{"49cm", "52cm", "54cm", "56cm", "58cm", "61cm"}
	default:
		for _, p := range strings.Split(sizesSpec, ",") {
			if s := strings.TrimSpace(p); s != "" {
				sizes = append(sizes, s)
			}
		}
	}
	if len(sizes) == 0 {
		sizes = []string{"M", "L"}
	}

	var travels []float64
	if low := strings.ToLower(strings.TrimSpace(supportedTravelStr)); low != "" && low != "none" && low != "0" {
		for _, p := range strings.Split(supportedTravelStr, ",") {
			if v, err := strconv.ParseFloat(strings.TrimSuffix(strings.TrimSpace(p), "mm"), 64); err == nil && v > 0 {
				travels = append(travels, v)
			}
		}
	}

	if len(travels) == 0 && rigidA2c <= 0 {
		rigidA2c = 483.0
	}

	var baseFork model.ForkSpec
	if len(travels) > 0 {
		baseFork = canonicalizer.GetStandardForkForTravel(travels[0])
		if defaultFork == "" {
			defaultFork = fmt.Sprintf("%.0fmm", travels[0])
		}
	} else {
		baseFork = model.ForkSpec{
			ID:       fmt.Sprintf("rigid_%.0f", rigidA2c),
			Name:     fmt.Sprintf("Rigid %.0fmm", rigidA2c),
			TravelMm: 0,
			A2c:      rigidA2c,
			Rake:     44,
		}
		if defaultFork == "" {
			defaultFork = "rigid"
		}
	}

	parsedCupType := model.HeadsetCupType(strings.ToUpper(strings.TrimSpace(cupType)))
	if !parsedCupType.IsValid() {
		fmt.Fprintf(a.Err, "Error: invalid --cup-type %q (valid: EC44, ZS44, ZS56, IS42, IS52)\n", cupType)
		return 2
	}
	if geoSpacer < 0 {
		geoSpacer = parsedCupType.CupStackMm()
	}

	// Build coherent baseline geometry using the Master Stack Equation so it validates cleanly out-of-the-box
	geos := make(map[string]model.SizeGeometry)
	htaDeg := 66.0
	staDeg := 75.0
	bbDrop := 60.0
	csLen := 430.0
	htaRad := geometry.ToRad(htaDeg)
	rearCenter := math.Sqrt(csLen*csLen - bbDrop*bbDrop)
	for i, sz := range sizes {
		reach := 425.0 + float64(i)*22.0
		htl := 100.0 + float64(i)*10.0
		structuralA2c := baseFork.A2c + geoSpacer
		// Stack = BB_drop - Rake*cos(HTA) + (A2C + geo_headset_spacer_mm + HTL)*sin(HTA)
		stack := geometry.Round(bbDrop-baseFork.Rake*math.Cos(htaRad)+(structuralA2c+htl)*math.Sin(htaRad), 1)
		fc := geometry.CalculateFrontCenter(reach, htl, htaDeg, baseFork.Rake, structuralA2c)
		wb := geometry.Round(rearCenter+fc, 1)
		geos[sz] = model.SizeGeometry{
			Reach:           reach,
			Stack:           stack,
			HtaDeg:          htaDeg,
			StaDeg:          staDeg,
			Htl:             htl,
			BbDrop:          bbDrop,
			Wheelbase:       wb,
			ChainstayLength: csLen,
			SeatTubeLength:  390.0 + float64(i)*35.0,
		}
	}

	defSize := sizes[0]
	for _, s := range sizes {
		if s == "L" || s == "S3" || s == "56cm" {
			defSize = s
			break
		}
	}

	cat := model.CategoryHardtail
	if len(travels) == 0 {
		cat = model.CategoryRigid
	}

	frame := model.FrameModel{
		Manufacturer:       manufacturer,
		Model:              modelName,
		Year:               2025,
		Category:           cat,
		WheelSize:          "29",
		HeadsetCupType:     parsedCupType,
		GeoHeadsetSpacerMm: model.Float64Ptr(geoSpacer),
		SupportedTravelMm:  travels,
		RigidA2cMm:         rigidA2c,
		DefaultFork:        defaultFork,
		Sizes:              sizes,
		DefaultSize:        defSize,
		Configurations: []model.FrameConfiguration{
			{
				ID:         "stock",
				Name:       "Stock Build",
				IsBaseline: true,
				Fork:       baseFork,
				Geometries: geos,
			},
		},
	}

	if save {
		store, err := a.openStore()
		if err != nil {
			fmt.Fprintf(a.Err, "Error: %v\n", err)
			return 1
		}
		saved, err := store.SaveFrame(frame)
		if err != nil {
			fmt.Fprintf(a.Err, "Error saving frame: %v\n", err)
			return 1
		}
		fmt.Fprintf(a.Out, "Created frame %q in %s/%s.json\n", saved.ID, a.DataDir, saved.ID)
		return 0
	}

	enc := json.NewEncoder(a.Out)
	enc.SetIndent("", "  ")
	_ = enc.Encode(frame)
	return 0
}

func (a *App) runSet(args []string) int {
	fs := flag.NewFlagSet("set", flag.ContinueOnError)
	fs.SetOutput(a.Err)

	var sizeFlag string
	fs.StringVar(&sizeFlag, "size", "", "Target size to modify (defaults to frame's default_size or :size suffix)")

	reordered := reorderFlagsForStdlib(fs, args, nil)
	if err := fs.Parse(reordered); err != nil {
		return 2
	}

	pos := fs.Args()
	if len(pos) < 2 {
		fmt.Fprintln(a.Err, "Usage: bikegeo set <frame[:size]> <field=value> [field=value...]")
		return 2
	}

	store, err := a.openStore()
	if err != nil {
		fmt.Fprintf(a.Err, "Error: %v\n", err)
		return 1
	}

	frames, err := store.GetAllFrames()
	if err != nil {
		fmt.Fprintf(a.Err, "Error: %v\n", err)
		return 1
	}

	framePtr, impSz, err := ResolveFrameByToken(frames, pos[0])
	if err != nil {
		fmt.Fprintf(a.Err, "Error: %v\n", err)
		return 1
	}
	frame := *framePtr

	targetSize := sizeFlag
	if targetSize == "" {
		targetSize = impSz
	}
	if targetSize == "" {
		targetSize = frame.GetDefaultSize()
	}

	matchedSz := ""
	for _, s := range frame.GetSizes() {
		if strings.EqualFold(s, targetSize) {
			matchedSz = s
			break
		}
	}
	if matchedSz == "" {
		fmt.Fprintf(a.Err, "Error: frame %q does not have size %q (available: %s)\n", frame.ID, targetSize, strings.Join(frame.GetSizes(), ", "))
		return 1
	}
	targetSize = matchedSz

	baseCfg := frame.GetBaselineConfiguration()
	if baseCfg == nil {
		fmt.Fprintf(a.Err, "Error: frame %q has no baseline configuration\n", frame.ID)
		return 1
	}
	if baseCfg.Geometries == nil {
		baseCfg.Geometries = make(map[string]model.SizeGeometry)
	}
	geo := baseCfg.Geometries[targetSize]

	for _, kv := range pos[1:] {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) != 2 {
			fmt.Fprintf(a.Err, "Error: invalid assignment %q (expected field=value)\n", kv)
			return 2
		}
		key := strings.ToLower(strings.TrimSpace(parts[0]))
		val := strings.TrimSpace(parts[1])

		parseFloat := func() (float64, error) {
			v, err := strconv.ParseFloat(val, 64)
			if err != nil {
				return 0, fmt.Errorf("invalid numeric value %q for %s", val, key)
			}
			return v, nil
		}

		switch key {
		case "reach":
			v, err := parseFloat()
			if err != nil || v <= 0 {
				fmt.Fprintf(a.Err, "Error: reach must be a positive number\n")
				return 1
			}
			geo.Reach = v
		case "stack":
			v, err := parseFloat()
			if err != nil || v <= 0 {
				fmt.Fprintf(a.Err, "Error: stack must be a positive number\n")
				return 1
			}
			geo.Stack = v
		case "hta", "hta_deg":
			v, err := parseFloat()
			if err != nil || v <= 0 {
				fmt.Fprintf(a.Err, "Error: hta_deg must be a positive number\n")
				return 1
			}
			geo.HtaDeg = v
		case "sta", "sta_deg":
			v, err := parseFloat()
			if err != nil || v <= 0 {
				fmt.Fprintf(a.Err, "Error: sta_deg must be a positive number\n")
				return 1
			}
			geo.StaDeg = v
		case "htl":
			v, err := parseFloat()
			if err != nil || v < 0 {
				fmt.Fprintf(a.Err, "Error: htl must be a non-negative number\n")
				return 1
			}
			geo.Htl = v
		case "bb_drop", "bbdrop":
			v, err := parseFloat()
			if err != nil {
				fmt.Fprintf(a.Err, "Error: %v\n", err)
				return 1
			}
			geo.BbDrop = v
		case "wheelbase":
			v, err := parseFloat()
			if err != nil || v < 0 {
				fmt.Fprintf(a.Err, "Error: wheelbase must be non-negative\n")
				return 1
			}
			geo.Wheelbase = v
		case "chainstay", "chainstay_length":
			v, err := parseFloat()
			if err != nil || v < 0 {
				fmt.Fprintf(a.Err, "Error: chainstay_length must be non-negative\n")
				return 1
			}
			geo.ChainstayLength = v
		case "front_center":
			v, err := parseFloat()
			if err != nil || v < 0 {
				fmt.Fprintf(a.Err, "Error: front_center must be non-negative\n")
				return 1
			}
			geo.FrontCenter = v
		case "seat_tube", "seat_tube_length":
			v, err := parseFloat()
			if err != nil || v < 0 {
				fmt.Fprintf(a.Err, "Error: seat_tube_length must be non-negative\n")
				return 1
			}
			geo.SeatTubeLength = v
		case "manufacturer":
			frame.Manufacturer = val
		case "model":
			frame.Model = val
		case "year":
			yr, err := strconv.Atoi(val)
			if err != nil {
				fmt.Fprintf(a.Err, "Error: invalid year %q\n", val)
				return 1
			}
			frame.Year = yr
		case "category":
			frame.Category = model.BikeCategory(val)
		case "wheel_size":
			frame.WheelSize = val
		case "headset_cup_type", "cup_type":
			ct := model.HeadsetCupType(strings.ToUpper(val))
			if !ct.IsValid() {
				fmt.Fprintf(a.Err, "Error: invalid headset_cup_type %q (valid: EC44, ZS44, ZS56, IS42, IS52)\n", val)
				return 1
			}
			frame.HeadsetCupType = ct
		case "geo_headset_spacer_mm", "geo_spacer_mm", "geo_spacer", "headset_spacer_mm":
			v, err := parseFloat()
			if err != nil || v < 0 {
				fmt.Fprintf(a.Err, "Error: geo_headset_spacer_mm must be a non-negative number\n")
				return 1
			}
			frame.GeoHeadsetSpacerMm = model.Float64Ptr(v)
		case "geo_slop_mm.a2c", "slop.a2c":
			v, err := parseFloat()
			if err != nil || v < 0 || v > 10.0 {
				fmt.Fprintf(a.Err, "Error: geo_slop_mm.a2c must be between 0 and 10\n")
				return 1
			}
			if frame.GeoSlopMm == nil {
				frame.GeoSlopMm = &model.GeoSlopMm{}
			}
			frame.GeoSlopMm.A2c = model.Float64Ptr(v)
		case "geo_slop_mm.front_center", "slop.front_center":
			v, err := parseFloat()
			if err != nil || v < 0 || v > 10.0 {
				fmt.Fprintf(a.Err, "Error: geo_slop_mm.front_center must be between 0 and 10\n")
				return 1
			}
			if frame.GeoSlopMm == nil {
				frame.GeoSlopMm = &model.GeoSlopMm{}
			}
			frame.GeoSlopMm.FrontCenter = model.Float64Ptr(v)
		case "geo_slop_mm.wheelbase", "slop.wheelbase":
			v, err := parseFloat()
			if err != nil || v < 0 || v > 10.0 {
				fmt.Fprintf(a.Err, "Error: geo_slop_mm.wheelbase must be between 0 and 10\n")
				return 1
			}
			if frame.GeoSlopMm == nil {
				frame.GeoSlopMm = &model.GeoSlopMm{}
			}
			frame.GeoSlopMm.Wheelbase = model.Float64Ptr(v)
		case "geo_slop_mm.cross_config", "slop.cross_config":
			v, err := parseFloat()
			if err != nil || v < 0 || v > 10.0 {
				fmt.Fprintf(a.Err, "Error: geo_slop_mm.cross_config must be between 0 and 10\n")
				return 1
			}
			if frame.GeoSlopMm == nil {
				frame.GeoSlopMm = &model.GeoSlopMm{}
			}
			frame.GeoSlopMm.CrossConfig = model.Float64Ptr(v)
		case "actual_sta_deg", "actual_sta":
			v, err := parseFloat()
			if err != nil || v < 60.0 || v > 85.0 {
				fmt.Fprintf(a.Err, "Error: actual_sta_deg must be between 60.0 and 85.0 degrees\n")
				return 1
			}
			frame.ActualStaDeg = v
		case "seat_tube_offset_x_mm", "seat_tube_offset":
			v, err := parseFloat()
			if err != nil || v < 0 || v > 100.0 {
				fmt.Fprintf(a.Err, "Error: seat_tube_offset_x_mm must be between 0.0 and 100.0 mm\n")
				return 1
			}
			frame.SeatTubeOffsetXMm = model.Float64Ptr(v)
		case "rigid_a2c_mm", "rigid_a2c":
			v, err := parseFloat()
			if err != nil || v < 0 {
				fmt.Fprintf(a.Err, "Error: rigid_a2c_mm must be non-negative\n")
				return 1
			}
			frame.RigidA2cMm = v
		case "supported_travel_mm", "supported_travel":
			var travels []float64
			if val != "" && val != "none" && val != "0" {
				for _, p := range strings.Split(val, ",") {
					v, err := strconv.ParseFloat(strings.TrimSuffix(strings.TrimSpace(p), "mm"), 64)
					if err != nil || v <= 0 {
						fmt.Fprintf(a.Err, "Error: invalid travel value %q\n", p)
						return 1
					}
					travels = append(travels, v)
				}
			}
			frame.SupportedTravelMm = travels
		case "default_fork":
			frame.DefaultFork = val
		case "default_size":
			frame.DefaultSize = val
		case "notes":
			frame.Notes = val
		default:
			fmt.Fprintf(a.Err, "Error: unknown field %q\n", key)
			return 2
		}
	}

	baseCfg.Geometries[targetSize] = geo
	saved, err := store.SaveFrame(frame)
	if err != nil {
		fmt.Fprintf(a.Err, "Error saving frame: %v\n", err)
		return 1
	}

	matBike, _ := geometry.MaterializeBikeGeometry(*saved, targetSize, "")
	report := validator.ValidateBikeGeometry(matBike)

	if a.Format == FormatJSON {
		_ = RenderOutput(a.Out, FormatJSON, "", "", nil, nil, saved)
		return 0
	}

	fmt.Fprintf(
		a.Out,
		"Updated %s %s (Size %s) -> Reach: %.1fmm, Stack: %.1fmm, HTA: %.1f°, STA: %.1f° [CAD Valid: %t]\n",
		saved.Manufacturer, saved.Model, targetSize,
		matBike.Reach, matBike.Stack, matBike.HtaDeg, matBike.StaDeg, report.IsValid,
	)
	return 0
}

func (a *App) runImport(args []string) int {
	fs := flag.NewFlagSet("import", flag.ContinueOnError)
	fs.SetOutput(a.Err)

	var force bool
	fs.BoolVar(&force, "force", false, "Save even if CAD validation issues are detected")

	reordered := reorderFlagsForStdlib(fs, args, map[string]bool{"force": true})
	if err := fs.Parse(reordered); err != nil {
		return 2
	}

	if fs.NArg() < 1 {
		fmt.Fprintln(a.Err, "Usage: bikegeo import <file.json>")
		return 2
	}

	srcPath := fs.Arg(0)
	var data []byte
	var err error
	if srcPath == "-" {
		data, err = io.ReadAll(a.In)
	} else {
		data, err = os.ReadFile(srcPath)
	}
	if err != nil {
		fmt.Fprintf(a.Err, "Error reading %s: %v\n", srcPath, err)
		return 1
	}

	var frame model.FrameModel
	if err := json.Unmarshal(data, &frame); err != nil || frame.Manufacturer == "" || frame.Model == "" {
		fmt.Fprintf(a.Err, "Error: invalid frame JSON (manufacturer and model are required)\n")
		return 1
	}

	if len(frame.Configurations) == 0 {
		// Check if it was a single-size BikeGeometry JSON
		var bike model.BikeGeometry
		if err := json.Unmarshal(data, &bike); err == nil && bike.Reach > 0 && bike.Stack > 0 {
			store, err := a.openStore()
			if err != nil {
				fmt.Fprintf(a.Err, "Error: %v\n", err)
				return 1
			}
			report := validator.ValidateBikeGeometry(bike)
			if !report.IsValid && !force {
				fmt.Fprintf(a.Err, "Error: imported bike fails CAD validation (use --force to import anyway)\n")
				return 1
			}
			saved, err := store.SaveBike(bike)
			if err != nil {
				fmt.Fprintf(a.Err, "Error saving bike: %v\n", err)
				return 1
			}
			fmt.Fprintf(a.Out, "Imported %s %s (%s) as %s\n", saved.Manufacturer, saved.Model, saved.Size, saved.ID)
			return 0
		}
		fmt.Fprintf(a.Err, "Error: frame JSON must include at least one configuration\n")
		return 1
	}

	frameReport := validator.ValidateFrame(frame)
	if !frameReport.IsValid && !force {
		fmt.Fprintf(a.Err, "Error: frame %s %s fails CAD validation (use --force to import anyway)\n", frame.Manufacturer, frame.Model)
		fmt.Fprintf(a.Err, "  %s\n", frameReport.ErrorSummary())
		return 1
	}

	store, err := a.openStore()
	if err != nil {
		fmt.Fprintf(a.Err, "Error: %v\n", err)
		return 1
	}

	saved, err := store.SaveFrame(frame)
	if err != nil {
		fmt.Fprintf(a.Err, "Error saving frame: %v\n", err)
		return 1
	}

	fmt.Fprintf(a.Out, "Imported frame %q (%s %s, sizes: %s)\n", saved.ID, saved.Manufacturer, saved.Model, strings.Join(saved.GetSizes(), ", "))
	return 0
}

func (a *App) runDelete(args []string) int {
	fs := flag.NewFlagSet("delete", flag.ContinueOnError)
	fs.SetOutput(a.Err)
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if fs.NArg() < 1 {
		fmt.Fprintln(a.Err, "Usage: bikegeo delete <frame[:size]>")
		return 2
	}

	target := fs.Arg(0)
	store, err := a.openStore()
	if err != nil {
		fmt.Fprintf(a.Err, "Error: %v\n", err)
		return 1
	}

	frames, err := store.GetAllFrames()
	if err != nil {
		fmt.Fprintf(a.Err, "Error: %v\n", err)
		return 1
	}

	frame, impSz, err := ResolveFrameByToken(frames, target)
	if err != nil {
		fmt.Fprintf(a.Err, "Error: %v\n", err)
		return 1
	}

	deleteID := frame.ID
	if impSz != "" {
		deleteID = fmt.Sprintf("%s-%s", frame.ID, strings.ToLower(impSz))
	}

	deleted, err := store.DeleteBike(deleteID)
	if err != nil {
		fmt.Fprintf(a.Err, "Error deleting %s: %v\n", deleteID, err)
		return 1
	}
	if !deleted {
		fmt.Fprintf(a.Err, "Error: %s not found\n", deleteID)
		return 1
	}

	if impSz != "" {
		fmt.Fprintf(a.Out, "Deleted size %s from frame %s\n", impSz, frame.ID)
	} else {
		fmt.Fprintf(a.Out, "Deleted frame %s (%s %s)\n", frame.ID, frame.Manufacturer, frame.Model)
	}
	return 0
}
