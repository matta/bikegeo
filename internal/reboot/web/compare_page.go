package web

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"bikegeo/internal/reboot/assembly"
	"bikegeo/internal/reboot/catalog"
	"bikegeo/internal/reboot/compare"
	"bikegeo/internal/reboot/pose"
	"bikegeo/internal/reboot/query"
)

const redshiftMixedStateNote = "Note: default BRP (−219, 713.1) mm comes from the unsagged Redshift transfer; this post models 12 mm sagged setback with no vertical compression adjustment."

type selectOption struct {
	Value    string
	Label    string
	Selected bool
	Invalid  bool
}

type compareSelectionRowView struct {
	Index           int
	FrameID         string
	FrameLabel      string
	UnknownFrame    bool
	RawSize         string
	RawFork         string
	RawSag          string
	Sizes           []selectOption
	Forks           []selectOption
	Sags            []selectOption
	FrameInvalid    bool
	SizeInvalid     bool
	ForkInvalid     bool
	SagInvalid      bool
	RemoveURL       string
	RemoveAriaLabel string
}

type compareFormValues struct {
	PostOptions            []selectOption
	BarOptions             []selectOption
	BarOptionModes         []selectOption
	BarRoll                string
	RailPitch              string
	BrpX                   string
	BrpY                   string
	GripX                  string
	GripY                  string
	TopPerFrame            string
	Top                    string
	PostInvalid            bool
	BarInvalid             bool
	BarOptionModeInvalid   bool
	BarRollInvalid         bool
	RailPitchInvalid       bool
	BrpXInvalid            bool
	BrpYInvalid            bool
	GripXInvalid           bool
	GripYInvalid           bool
	TopPerFrameInvalid     bool
	TopInvalid             bool
	SelectionRows          []compareSelectionRowView
	AddFrames              []frameSelectOption
	ShowAllFramesLink      bool
	AllFramesURL           string
	RedshiftMixedStateNote string
	ExampleBrpX            string
	ExampleBrpY            string
	ExampleGripX           string
	ExampleGripY           string
	HandlebarCount         int
	FrameCount             int
}

type compareRowView struct {
	Rank              int
	InspectURL        string
	InspectAriaLabel  string
	FrameID           string
	FrameLabel        string
	Size              string
	ForkID            string
	ForkLabel         string
	SlidingDropouts   bool
	Tier              assembly.FitTier
	TierClass         string
	TierLabel         string
	Score             float64
	GripR             float64
	BrpMissText       string
	StemText          string
	SpacersText       string
	BarName           string
	RailOffsetText    string
	PostInsertionText string
	LeveledReachMm    float64
	LeveledStackMm    float64
	EttMm             float64
	LeveledHtaDeg     pose.Deg
	LeveledStaDeg     pose.Deg
	ChainstayMm       float64
	WheelbaseMm       float64
	RcWbRatioPct      float64
	LeveledBbDropMm   float64
	TrailMm           float64
	Attributes        []assembly.AttributeScore
}

type compareSummaryView struct {
	ModeLabel        string
	ShownCount       int
	RowCount         int
	Evaluated        int
	SeatpostName     string
	HandlebarSummary string
	BrpTargetText    string
	GripTargetText   string
	PostureNote      string
}

type compareView struct {
	PageTitle              string
	HeaderBadge            string
	HasResults             bool
	ErrorMessage           string
	Form                   compareFormValues
	Summary                compareSummaryView
	AllBarOptions          bool
	ActiveWeight           float64
	TierLegend             []fitTierLegendItem
	AttributeHeaders       []buildAttributeHeader
	Rows                   []compareRowView
	SlidingDropoutsSummary string
}

func cleanZero(v, halfStep float64) float64 {
	if math.Abs(v) < halfStep {
		return 0
	}
	return v
}

func handleCompare(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	q := r.URL.Query()
	parsed, err := query.ParseCompare(q)
	if err != nil {
		field := ""
		if inputErr, ok := errors.AsType[*query.InputError](err); ok {
			field = inputErr.Field
		}
		renderCompareError(w, q, field, err.Error())
		return
	}
	ranking, err := compare.Rank(parsed.Request)
	if err != nil {
		renderCompareError(w, q, "", err.Error())
		return
	}
	view, err := buildCompareView(parsed, ranking)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	renderCompareTemplate(w, http.StatusOK, view)
}

func buildCompareView(parsed query.Compare, ranking compare.Ranking) (compareView, error) {
	allFrames := catalog.Frames()
	framesByID := make(map[string]catalog.Frame, len(allFrames))
	addFrames := make([]frameSelectOption, len(allFrames))
	for i, f := range allFrames {
		framesByID[f.ID] = f
		addFrames[i] = frameSelectOption{
			ID:    f.ID,
			Label: f.Manufacturer + " " + f.Model,
		}
	}

	rows := make([]compareRowView, len(ranking.Rows))
	slidingFrames := make(map[string]catalog.Frame)
	for i, row := range ranking.Rows {
		f, ok := framesByID[row.FrameID]
		if !ok {
			return compareView{}, fmt.Errorf("unknown frame %q", row.FrameID)
		}
		frameLabel := f.Manufacturer + " " + f.Model
		var forkLabel string
		for _, opt := range f.ForkOptions() {
			if opt.ID() == row.ForkID {
				forkLabel = forkCellName(opt.Name(), opt.Fork.Travel.Mm(), row.SagPct)
				break
			}
		}
		if forkLabel == "" {
			forkLabel = row.ForkID
		}

		bar, err := parsed.WinningHandlebar(row)
		if err != nil {
			return compareView{}, err
		}
		inspectURL, err := parsed.RowBikeURL(row)
		if err != nil {
			return compareView{}, err
		}

		fs := row.Build.Placement.Frameset
		setup := row.Build.Placement.Setup
		sc := row.Build.Score
		st := setup.Stem
		tierClass, tierLabel := fitTierPresentation(sc.Tier)
		sliding := f.MaxChainstayMm > 0
		if sliding {
			slidingFrames[f.ID] = f
		}

		rows[i] = compareRowView{
			Rank:              i + 1,
			InspectURL:        inspectURL,
			InspectAriaLabel:  fmt.Sprintf("Inspect #%d %s %s, %s", i+1, frameLabel, row.Size, forkLabel),
			FrameID:           row.FrameID,
			FrameLabel:        frameLabel,
			Size:              row.Size,
			ForkID:            row.ForkID,
			ForkLabel:         forkLabel,
			SlidingDropouts:   sliding,
			Tier:              sc.Tier,
			TierClass:         tierClass,
			TierLabel:         tierLabel,
			Score:             sc.Total,
			GripR:             sc.GripR,
			BrpMissText:       fmt.Sprintf("(%+.1f, %+.1f) mm", cleanZero(sc.BrpMissXMm, 0.05), cleanZero(sc.BrpMissYMm, 0.05)),
			StemText:          fmt.Sprintf("%.0f mm, %+.0f°", st.LengthMm, cleanZero(float64(st.AngleDeg), 0.5)),
			SpacersText:       fmt.Sprintf("%.0f mm", st.SpacersMm),
			BarName:           bar.Name,
			RailOffsetText:    fmt.Sprintf("%+.1f mm", cleanZero(setup.Saddle.RailOffsetMm, 0.05)),
			PostInsertionText: fmt.Sprintf("%.1f mm", setup.Seatpost.InsertionMm),
			LeveledReachMm:    fs.LeveledReachMm(),
			LeveledStackMm:    fs.LeveledStackMm(),
			EttMm:             fs.EttMm(),
			LeveledHtaDeg:     fs.LeveledHeadTubeAngleDeg(),
			LeveledStaDeg:     fs.LeveledSeatTubeAngleDeg(),
			ChainstayMm:       setup.Frame.ChainstayMm,
			WheelbaseMm:       fs.WheelbaseMm(),
			RcWbRatioPct:      fs.RcWbRatioPct(),
			LeveledBbDropMm:   fs.LeveledBbDropMm(),
			TrailMm:           fs.TrailMm(),
			Attributes:        sc.Attributes,
		}
	}

	var slidingDropoutsSummary string
	if len(slidingFrames) > 0 {
		ids := make([]string, 0, len(slidingFrames))
		for id := range slidingFrames {
			ids = append(ids, id)
		}
		slices.Sort(ids)
		parts := make([]string, len(ids))
		for i, id := range ids {
			f := slidingFrames[id]
			parts[i] = fmt.Sprintf("%s %s (%.0f mm)", f.Manufacturer, f.Model, f.MaxChainstayMm)
		}
		slidingDropoutsSummary = strings.Join(parts, ", ")
	}

	var pageTitle, headerBadge, modeLabel string
	if len(parsed.Selections) == 0 {
		n := len(catalog.Frames())
		pageTitle = fmt.Sprintf("Compare %d Frames — bikegeo-reboot", n)
		headerBadge = fmt.Sprintf("All %d frames", n)
		modeLabel = headerBadge
	} else {
		n := len(parsed.Selections)
		setupNoun := "Setups"
		selNoun := "selections"
		if n == 1 {
			setupNoun = "Setup"
			selNoun = "selection"
		}
		pageTitle = fmt.Sprintf("Compare %d %s — bikegeo-reboot", n, setupNoun)
		headerBadge = fmt.Sprintf("Custom (%d %s)", n, selNoun)
		modeLabel = fmt.Sprintf("Custom (%d %s)", n, selNoun)
	}

	bars := catalog.Handlebars()
	handlebarSummary := parsed.Bar.Name
	if parsed.AllBarOptions {
		handlebarSummary = fmt.Sprintf("Best of all %d handlebars", len(bars))
	}

	var postureParts []string
	if parsed.BarRollDeg != 0 {
		postureParts = append(postureParts, fmt.Sprintf("Bar roll %+.1f°", parsed.BarRollDeg))
	}
	if parsed.RailPitchDeg != 0 {
		postureParts = append(postureParts, fmt.Sprintf("Rail pitch %+.1f°", parsed.RailPitchDeg))
	}

	selectionRows := make([]compareSelectionRowView, len(parsed.Selections))
	for i, sel := range parsed.Selections {
		f := framesByID[sel.FrameID]
		frameLabel := f.Manufacturer + " " + f.Model
		var selectedFork *catalog.Fork
		if matches, ok := f.LookupFork(sel.ForkID); ok {
			fk := matches[0].Fork
			selectedFork = &fk
		}
		rawSag := ""
		if sel.SagPct != nil {
			rawSag = query.FormatFloat(*sel.SagPct)
		}
		selectionRows[i] = compareSelectionRowView{
			Index:           i + 1,
			FrameID:         sel.FrameID,
			FrameLabel:      frameLabel,
			RawSize:         sel.Size,
			RawFork:         sel.ForkID,
			RawSag:          rawSag,
			Sizes:           buildSelectionSizeOptions(f, sel.Size, false),
			Forks:           buildSelectionForkOptions(f, sel.ForkID, false),
			Sags:            buildSelectionSagOptions(rawSag, selectedFork, true),
			RemoveURL:       parsed.WithoutSelection(i).URL(),
			RemoveAriaLabel: fmt.Sprintf("Remove selection %d: %s", i+1, frameLabel),
		}
	}

	var topPerFrameStr, topStr string
	if parsed.TopPerFrameExplicit {
		topPerFrameStr = strconv.Itoa(parsed.TopPerFrame)
	}
	if parsed.TopExplicit {
		topStr = strconv.Itoa(parsed.Top)
	}

	var mixedNote string
	if parsed.Post.ID == catalog.DefaultSeatpostID {
		mixedNote = redshiftMixedStateNote
	}

	defBrp := catalog.DefaultBrpTarget()
	defGrip := catalog.DefaultGripTarget()
	weights := catalog.DefaultScoreWeights()

	return compareView{
		PageTitle:   pageTitle,
		HeaderBadge: headerBadge,
		HasResults:  true,
		Form: compareFormValues{
			PostOptions:            buildPostSelectOptions(parsed.Post.ID, false),
			BarOptions:             buildBarSelectOptions(parsed.Bar.ID, false),
			BarOptionModes:         buildBarOptionModeOptions(parsed.AllBarOptions, "", false),
			BarRoll:                formatFormFloat(float64(parsed.BarRollDeg)),
			RailPitch:              formatFormFloat(float64(parsed.RailPitchDeg)),
			BrpX:                   formatFormFloat(parsed.BrpTarget.X),
			BrpY:                   formatFormFloat(parsed.BrpTarget.Y),
			GripX:                  formatFormFloat(parsed.GripTarget.X),
			GripY:                  formatFormFloat(parsed.GripTarget.Y),
			TopPerFrame:            topPerFrameStr,
			Top:                    topStr,
			SelectionRows:          selectionRows,
			AddFrames:              addFrames,
			ShowAllFramesLink:      len(parsed.Selections) > 0,
			AllFramesURL:           parsed.AllFrames().URL(),
			RedshiftMixedStateNote: mixedNote,
			ExampleBrpX:            formatFormFloat(defBrp.X),
			ExampleBrpY:            formatFormFloat(defBrp.Y),
			ExampleGripX:           formatFormFloat(defGrip.X),
			ExampleGripY:           formatFormFloat(defGrip.Y),
			HandlebarCount:         len(bars),
			FrameCount:             len(catalog.Frames()),
		},
		Summary: compareSummaryView{
			ModeLabel:        modeLabel,
			ShownCount:       len(rows),
			RowCount:         ranking.RowCount,
			Evaluated:        ranking.Evaluated,
			SeatpostName:     parsed.Post.Name,
			HandlebarSummary: handlebarSummary,
			BrpTargetText:    fmt.Sprintf("(%s, %s) mm", formatFormFloat(parsed.BrpTarget.X), formatFormFloat(parsed.BrpTarget.Y)),
			GripTargetText:   fmt.Sprintf("(%.1f, %.1f) mm", parsed.GripTarget.X, parsed.GripTarget.Y),
			PostureNote:      strings.Join(postureParts, " · "),
		},
		AllBarOptions:          parsed.AllBarOptions,
		ActiveWeight:           weights.ActiveTotal(true),
		TierLegend:             fitTierLegend(),
		AttributeHeaders:       buildAttributeHeaders(weights),
		Rows:                   rows,
		SlidingDropoutsSummary: slidingDropoutsSummary,
	}, nil
}

func renderCompareError(w http.ResponseWriter, q url.Values, invalidField, msg string) {
	allFrames := catalog.Frames()
	addFrames := make([]frameSelectOption, len(allFrames))
	for i, f := range allFrames {
		addFrames[i] = frameSelectOption{
			ID:    f.ID,
			Label: f.Manufacturer + " " + f.Model,
		}
	}

	postRaw := strings.TrimSpace(q.Get("post"))
	if postRaw == "" {
		postRaw = catalog.DefaultSeatpostID
	}
	_, postOK := catalog.SeatpostByID(postRaw)
	postInvalid := !postOK || invalidField == "post"

	barRaw := strings.TrimSpace(q.Get("bar"))
	if barRaw == "" {
		barRaw = catalog.DefaultHandlebarID
	}
	_, barOK := catalog.HandlebarByID(barRaw)
	barInvalid := !barOK || invalidField == "bar"

	barOptRaw := strings.TrimSpace(q.Get("bar_options"))
	barOptInvalid := invalidField == "bar_options"
	allBarOpts := false
	switch barOptRaw {
	case "", query.BarOptionsSelected:
		allBarOpts = false
	case query.BarOptionsAll:
		allBarOpts = true
	default:
		barOptInvalid = true
	}

	barRoll := "0.0"
	if q.Has("bar_roll") {
		barRoll = strings.TrimSpace(q.Get("bar_roll"))
	}
	railPitch := "0.0"
	if q.Has("rail_pitch") {
		railPitch = strings.TrimSpace(q.Get("rail_pitch"))
	}

	defBrp := catalog.DefaultBrpTarget()
	brpX := formatFormFloat(defBrp.X)
	brpY := formatFormFloat(defBrp.Y)
	if q.Has("brp_x") || q.Has("brp_y") {
		brpX = strings.TrimSpace(q.Get("brp_x"))
		brpY = strings.TrimSpace(q.Get("brp_y"))
	} else if strings.TrimSpace(q.Get("brp")) == "off" {
		brpX = ""
		brpY = ""
	}

	defGrip := catalog.DefaultGripTarget()
	gripX := formatFormFloat(defGrip.X)
	gripY := formatFormFloat(defGrip.Y)
	if q.Has("grip_x") || q.Has("grip_y") {
		gripX = strings.TrimSpace(q.Get("grip_x"))
		gripY = strings.TrimSpace(q.Get("grip_y"))
	} else if strings.TrimSpace(q.Get("grip")) == "off" {
		gripX = ""
		gripY = ""
	}

	selectionRows := buildErrorSelectionRows(q)
	hasAnySelParam := len(q["frame"]) > 0 || len(q["size"]) > 0 || len(q["fork"]) > 0 || len(q["sag"]) > 0

	cleanQ := cloneURLValues(q)
	cleanQ.Del("frame")
	cleanQ.Del("size")
	cleanQ.Del("fork")
	cleanQ.Del("sag")
	cleanQ.Del("top_per_frame")
	allFramesURL := "/"
	if len(cleanQ) > 0 {
		allFramesURL = "/?" + cleanQ.Encode()
	}

	var mixedNote string
	if postRaw == catalog.DefaultSeatpostID {
		mixedNote = redshiftMixedStateNote
	}

	view := compareView{
		PageTitle:    "Invalid Comparison — bikegeo-reboot",
		HeaderBadge:  "Invalid query",
		HasResults:   false,
		ErrorMessage: msg,
		Form: compareFormValues{
			PostOptions:            buildPostSelectOptions(postRaw, !postOK),
			BarOptions:             buildBarSelectOptions(barRaw, !barOK),
			BarOptionModes:         buildBarOptionModeOptions(allBarOpts, barOptRaw, barOptRaw != "" && barOptRaw != query.BarOptionsSelected && barOptRaw != query.BarOptionsAll),
			BarRoll:                barRoll,
			RailPitch:              railPitch,
			BrpX:                   brpX,
			BrpY:                   brpY,
			GripX:                  gripX,
			GripY:                  gripY,
			TopPerFrame:            strings.TrimSpace(q.Get("top_per_frame")),
			Top:                    strings.TrimSpace(q.Get("top")),
			PostInvalid:            postInvalid,
			BarInvalid:             barInvalid,
			BarOptionModeInvalid:   barOptInvalid,
			BarRollInvalid:         invalidField == "bar_roll",
			RailPitchInvalid:       invalidField == "rail_pitch",
			BrpXInvalid:            invalidField == "brp" || invalidField == "brp_x",
			BrpYInvalid:            invalidField == "brp" || invalidField == "brp_y",
			GripXInvalid:           invalidField == "grip" || invalidField == "grip_x",
			GripYInvalid:           invalidField == "grip" || invalidField == "grip_y",
			TopPerFrameInvalid:     invalidField == "top_per_frame",
			TopInvalid:             invalidField == "top",
			SelectionRows:          selectionRows,
			AddFrames:              addFrames,
			ShowAllFramesLink:      hasAnySelParam,
			AllFramesURL:           allFramesURL,
			RedshiftMixedStateNote: mixedNote,
			ExampleBrpX:            formatFormFloat(defBrp.X),
			ExampleBrpY:            formatFormFloat(defBrp.Y),
			ExampleGripX:           formatFormFloat(defGrip.X),
			ExampleGripY:           formatFormFloat(defGrip.Y),
			HandlebarCount:         len(catalog.Handlebars()),
			FrameCount:             len(catalog.Frames()),
		},
	}
	renderCompareTemplate(w, http.StatusBadRequest, view)
}

func buildErrorSelectionRows(q url.Values) []compareSelectionRowView {
	framesVals := q["frame"]
	sizeVals := q["size"]
	forkVals := q["fork"]
	sagVals := q["sag"]

	if len(framesVals) == 0 {
		return nil
	}
	for _, col := range [][]string{sizeVals, forkVals, sagVals} {
		if len(col) > 0 && len(col) != len(framesVals) {
			return nil
		}
	}

	at := func(vals []string, i int) string {
		if len(vals) == 0 {
			return ""
		}
		return strings.TrimSpace(vals[i])
	}

	var rows []compareSelectionRowView
	for i := range framesVals {
		f := strings.TrimSpace(framesVals[i])
		sz := at(sizeVals, i)
		fk := at(forkVals, i)
		sg := at(sagVals, i)
		if f == "" && sz == "" && fk == "" && sg == "" {
			continue
		}

		removeURL := buildRemoveSelectionURL(q, i)
		frame, ok := catalog.FrameByID(f)
		if f == "" || !ok {
			frameLabel := "(missing frame)"
			if f != "" {
				frameLabel = fmt.Sprintf("Invalid frame: %s", f)
			}
			rows = append(rows, compareSelectionRowView{
				Index:           len(rows) + 1,
				FrameID:         f,
				FrameLabel:      frameLabel,
				UnknownFrame:    true,
				RawSize:         sz,
				RawFork:         fk,
				RawSag:          sg,
				FrameInvalid:    true,
				RemoveURL:       removeURL,
				RemoveAriaLabel: fmt.Sprintf("Remove selection %d: %s", len(rows)+1, frameLabel),
			})
			continue
		}

		frameLabel := frame.Manufacturer + " " + frame.Model
		sizeValid := sz == ""
		if sz != "" {
			_, sizeValid = frame.Size(sz)
		}
		var selectedFork *catalog.Fork
		var matchedForks []catalog.ForkOption
		forkValid := fk == ""
		if fk != "" {
			if matches, ok := frame.LookupFork(fk); ok {
				matchedForks = matches
				selected := matches[0].Fork
				selectedFork = &selected
				forkValid = true
			}
		}
		sagValid := sg == ""
		if sg != "" && forkValid && len(matchedForks) > 0 {
			if v, err := strconv.ParseFloat(sg, 64); err == nil && !math.IsNaN(v) && !math.IsInf(v, 0) {
				sagValid = true
				for _, m := range matchedForks {
					if (assembly.ForkSetup{Spec: m.Spec(), SagPct: v}).Validate() != nil {
						sagValid = false
						break
					}
				}
			}
		}

		rows = append(rows, compareSelectionRowView{
			Index:           len(rows) + 1,
			FrameID:         f,
			FrameLabel:      frameLabel,
			RawSize:         sz,
			RawFork:         fk,
			RawSag:          sg,
			Sizes:           buildSelectionSizeOptions(frame, sz, !sizeValid),
			Forks:           buildSelectionForkOptions(frame, fk, !forkValid),
			Sags:            buildSelectionSagOptions(sg, selectedFork, sagValid),
			SizeInvalid:     !sizeValid,
			ForkInvalid:     !forkValid,
			SagInvalid:      !sagValid,
			RemoveURL:       removeURL,
			RemoveAriaLabel: fmt.Sprintf("Remove selection %d: %s", len(rows)+1, frameLabel),
		})
	}
	return rows
}

func buildRemoveSelectionURL(q url.Values, removeIdx int) string {
	nextQ := cloneURLValues(q)
	removeAt := func(vals []string, idx int) []string {
		if idx < 0 || idx >= len(vals) {
			return vals
		}
		out := make([]string, 0, len(vals)-1)
		out = append(out, vals[:idx]...)
		out = append(out, vals[idx+1:]...)
		return out
	}

	nextFrames := removeAt(nextQ["frame"], removeIdx)
	if len(nextFrames) == 0 {
		nextQ.Del("frame")
		nextQ.Del("size")
		nextQ.Del("fork")
		nextQ.Del("sag")
	} else {
		nextQ["frame"] = nextFrames
		for _, key := range []string{"size", "fork", "sag"} {
			if len(nextQ[key]) > 0 {
				nextQ[key] = removeAt(nextQ[key], removeIdx)
			}
		}
		allBlank := true
		at := func(vals []string, i int) string {
			if len(vals) == 0 {
				return ""
			}
			return strings.TrimSpace(vals[i])
		}
		for i := range nextFrames {
			if strings.TrimSpace(nextFrames[i]) != "" || at(nextQ["size"], i) != "" || at(nextQ["fork"], i) != "" || at(nextQ["sag"], i) != "" {
				allBlank = false
				break
			}
		}
		if allBlank {
			nextQ.Del("frame")
			nextQ.Del("size")
			nextQ.Del("fork")
			nextQ.Del("sag")
		}
	}
	if len(nextQ) == 0 {
		return "/"
	}
	return "/?" + nextQ.Encode()
}

func cloneURLValues(q url.Values) url.Values {
	out := make(url.Values, len(q))
	for k, v := range q {
		cp := make([]string, len(v))
		copy(cp, v)
		out[k] = cp
	}
	return out
}

func buildPostSelectOptions(selectedID string, invalid bool) []selectOption {
	posts := catalog.Seatposts()
	opts := make([]selectOption, 0, len(posts)+1)
	if invalid {
		opts = append(opts, selectOption{
			Value:    selectedID,
			Label:    "Invalid: " + selectedID,
			Selected: true,
			Invalid:  true,
		})
	}
	for _, p := range posts {
		opts = append(opts, selectOption{
			Value:    p.ID,
			Label:    p.Name,
			Selected: !invalid && p.ID == selectedID,
		})
	}
	return opts
}

func buildBarSelectOptions(selectedID string, invalid bool) []selectOption {
	bars := catalog.Handlebars()
	opts := make([]selectOption, 0, len(bars)+1)
	if invalid {
		opts = append(opts, selectOption{
			Value:    selectedID,
			Label:    "Invalid: " + selectedID,
			Selected: true,
			Invalid:  true,
		})
	}
	for _, b := range bars {
		opts = append(opts, selectOption{
			Value:    b.ID,
			Label:    b.Name,
			Selected: !invalid && b.ID == selectedID,
		})
	}
	return opts
}

func buildBarOptionModeOptions(allBars bool, raw string, invalid bool) []selectOption {
	bars := catalog.Handlebars()
	opts := make([]selectOption, 0, 3)
	if invalid {
		opts = append(opts, selectOption{
			Value:    raw,
			Label:    "Invalid: " + raw,
			Selected: true,
			Invalid:  true,
		})
	}
	opts = append(opts,
		selectOption{
			Value:    query.BarOptionsSelected,
			Label:    "Selected bar only",
			Selected: !invalid && !allBars,
		},
		selectOption{
			Value:    query.BarOptionsAll,
			Label:    fmt.Sprintf("All %d catalog bars", len(bars)),
			Selected: !invalid && allBars,
		},
	)
	return opts
}

func buildSelectionSizeOptions(f catalog.Frame, selectedSize string, invalid bool) []selectOption {
	opts := make([]selectOption, 0, len(f.Sizes)+2)
	if invalid && selectedSize != "" {
		opts = append(opts, selectOption{
			Value:    selectedSize,
			Label:    "Invalid: " + selectedSize,
			Selected: true,
			Invalid:  true,
		})
	}
	opts = append(opts, selectOption{
		Value:    "",
		Label:    "Any size",
		Selected: !invalid && selectedSize == "",
	})
	for _, sz := range f.Sizes {
		opts = append(opts, selectOption{
			Value:    sz.Name,
			Label:    sz.Name,
			Selected: !invalid && sz.Name == selectedSize,
		})
	}
	return opts
}

func buildSelectionForkOptions(f catalog.Frame, selectedFork string, invalid bool) []selectOption {
	opts := make([]selectOption, 0, len(f.ForkOptions())+len(f.Forks)+2)
	if invalid && selectedFork != "" {
		opts = append(opts, selectOption{
			Value:    selectedFork,
			Label:    "Invalid: " + selectedFork,
			Selected: true,
			Invalid:  true,
		})
	}
	opts = append(opts, selectOption{
		Value:    "",
		Label:    "Any fork",
		Selected: !invalid && selectedFork == "",
	})
	for _, fk := range f.Forks {
		if !fk.IsMultiVariant() {
			opt := catalog.ForkOption{Fork: fk, Variant: fk.Variants[0]}
			opts = append(opts, selectOption{
				Value:    opt.ID(),
				Label:    makeForkOption(opt).Label,
				Selected: !invalid && opt.ID() == selectedFork,
			})
			continue
		}
		opts = append(opts, selectOption{
			Value:    fk.ID,
			Label:    fk.Name + " (all variants)",
			Selected: !invalid && fk.ID == selectedFork,
		})
		for _, v := range fk.Variants {
			opt := catalog.ForkOption{Fork: fk, Variant: v}
			opts = append(opts, selectOption{
				Value:    opt.ID(),
				Label:    makeForkOption(opt).Label,
				Selected: !invalid && opt.ID() == selectedFork,
			})
		}
	}
	return opts
}

func buildSelectionSagOptions(rawSag string, selectedFork *catalog.Fork, sagValid bool) []selectOption {
	type sagStep struct {
		val   string
		num   float64
		label string
	}
	var steps []sagStep
	switch {
	case selectedFork == nil:
		steps = []sagStep{
			{"0", 0, "0% (rigid)"},
			{"15", 15, "15%"},
			{"20", 20, "20%"},
			{"25", 25, "25%"},
		}
	case !selectedFork.Travel.IsSuspension():
		steps = []sagStep{{"0", 0, "0% (rigid)"}}
	default:
		for _, pct := range selectedFork.Travel.SagPcts() {
			val := query.FormatFloat(pct)
			steps = append(steps, sagStep{val: val, num: pct, label: val + "%"})
		}
	}

	var parsedSag float64
	var hasParsed bool
	if rawSag != "" {
		if v, err := strconv.ParseFloat(rawSag, 64); err == nil && !math.IsNaN(v) && !math.IsInf(v, 0) {
			parsedSag = v
			hasParsed = true
		}
	}

	matchedStep := false
	opts := make([]selectOption, 0, len(steps)+2)
	opts = append(opts, selectOption{
		Value:    "",
		Label:    "Any sag",
		Selected: rawSag == "",
	})
	for _, st := range steps {
		sel := false
		inv := false
		if rawSag != "" {
			if (hasParsed && parsedSag == st.num) || rawSag == st.val {
				sel = true
				inv = !sagValid
				matchedStep = true
			}
		}
		opts = append(opts, selectOption{
			Value:    st.val,
			Label:    st.label,
			Selected: sel,
			Invalid:  inv,
		})
	}

	if rawSag != "" && !matchedStep {
		if sagValid && selectedFork != nil && hasParsed {
			formatted := query.FormatFloat(parsedSag)
			opts = append(opts, selectOption{
				Value:    formatted,
				Label:    formatted + "%",
				Selected: true,
			})
		} else {
			opts = slices.Insert(opts, 0, selectOption{
				Value:    rawSag,
				Label:    "Invalid: " + rawSag,
				Selected: true,
				Invalid:  true,
			})
		}
	}
	return opts
}
