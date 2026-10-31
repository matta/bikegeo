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
	"bikegeo/pkg/fit"
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

	var cadFork model.ForkRef
	var forks []model.ForkRef

	if len(travels) > 0 {
		for i, tr := range travels {
			stdFork, ok := canonicalizer.GetStandardForkForTravel(tr)
			optA2c := tr + 410.0
			optRake := 44.0
			forkID := fmt.Sprintf("standard_%.0f", tr)
			if ok && len(stdFork.Options) > 0 {
				forkID = stdFork.ID
				optA2c = stdFork.Options[0].A2cMm
				optRake = stdFork.Options[0].RakeMm
			}
			fRef := model.ForkRef{
				Fork:   forkID,
				A2cMm:  optA2c,
				RakeMm: optRake,
				SagPct: model.Float64Ptr(0),
			}
			forks = append(forks, fRef)
			if i == 0 {
				cadFork = fRef
			}
		}
	} else {
		cadFork = model.ForkRef{
			Fork:   fmt.Sprintf("rigid_%.0f", rigidA2c),
			A2cMm:  rigidA2c,
			RakeMm: 44,
		}
		forks = []model.ForkRef{cadFork}
	}

	if rigidA2c > 0 && len(travels) > 0 {
		rf := model.ForkRef{
			Fork:   fmt.Sprintf("rigid_%.0f", rigidA2c),
			A2cMm:  rigidA2c,
			RakeMm: 44,
		}
		forks = append([]model.ForkRef{rf}, forks...)
	}

	if defaultFork != "" {
		defClean := strings.ToLower(strings.TrimSpace(defaultFork))
		for idx, f := range forks {
			if strings.EqualFold(f.Fork, defClean) ||
				(defClean == "rigid" && f.ForkTravel() == 0) ||
				(strings.HasPrefix(defClean, fmt.Sprintf("%.0f", f.ForkTravel())) && f.ForkTravel() > 0) {
				if idx > 0 {
					forks[0], forks[idx] = forks[idx], forks[0]
				}
				break
			}
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
		structuralA2c := cadFork.A2cMm + geoSpacer
		// Stack = BB_drop - Rake*cos(HTA) + (A2C + geo_headset_spacer_mm + HTL)*sin(HTA)
		stack := geometry.Round(bbDrop-cadFork.RakeMm*math.Cos(htaRad)+(structuralA2c+htl)*math.Sin(htaRad), 1)
		fc := geometry.CalculateFrontCenter(reach, htl, htaDeg, cadFork.RakeMm, structuralA2c)
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

	canonicalStem := model.StemSpec{LengthMm: 60.0, AngleDeg: -6.0}
	stemLengths := []float64{40, 50, 60, 70, 80, 90, 100}
	if cat == model.CategoryGravel {
		canonicalStem = model.StemSpec{LengthMm: 80.0, AngleDeg: -6.0}
		stemLengths = []float64{60, 70, 80, 90, 100, 110}
	}
	var initStems []model.StemSpec
	for _, l := range stemLengths {
		for _, a := range []float64{-6, 0, 6} {
			initStems = append(initStems, model.StemSpec{LengthMm: l, AngleDeg: a})
		}
	}

	frame := model.FrameModel{
		Manufacturer:       manufacturer,
		Model:              modelName,
		Year:               2025,
		Category:           cat,
		WheelSize:          "29",
		HeadsetCupType:     parsedCupType,
		GeoHeadsetSpacerMm: model.Float64Ptr(geoSpacer),
		CadFork:            cadFork,
		Forks:              forks,
		Sizes:              sizes,
		DefaultSize:        defSize,
		Geometries:         geos,
		MaxSpacerMm:        model.Float64Ptr(40.0),
		CanonicalStem:      &canonicalStem,
		Stems:              initStems,
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

	for _, kv := range pos[1:] {
		if err := fit.ApplyAssignment(&frame, "", targetSize, kv); err != nil {
			fmt.Fprintf(a.Err, "Error: %v\n", err)
			return 2
		}
	}
	saved, err := store.SaveFrame(frame)
	if err != nil {
		fmt.Fprintf(a.Err, "Error saving frame: %v\n", err)
		return 1
	}

	matBike, _ := geometry.MaterializeBikeGeometry(*saved, targetSize)
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

	if len(frame.Geometries) == 0 {
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
		fmt.Fprintf(a.Err, "Error: frame JSON must include at least one size geometry\n")
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
