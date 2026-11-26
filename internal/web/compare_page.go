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

	"github.com/matta/bikegeo/internal/assembly"
	"github.com/matta/bikegeo/internal/catalog"
	"github.com/matta/bikegeo/internal/compare"
	"github.com/matta/bikegeo/internal/pose"
	"github.com/matta/bikegeo/internal/query"
)

const redshiftMixedStateNote = "Note: default BRP (−219, 713.1) mm comes from the unsagged Redshift transfer; this post models 12 mm sagged setback with no vertical compression adjustment."

type selectOption struct {
	Value    string
	Label    string
	Selected bool
	Invalid  bool
}

type compareFormValues struct {
	PostOptions            []selectOption
	BarOptions             []selectOption
	BarOptionModes         []selectOption
	BrpX                   string
	BrpY                   string
	GripX                  string
	GripY                  string
	TopPerFrame            string
	Top                    string
	PostInvalid            bool
	BarInvalid             bool
	BarOptionModeInvalid   bool
	BrpXInvalid            bool
	BrpYInvalid            bool
	GripXInvalid           bool
	GripYInvalid           bool
	TopPerFrameInvalid     bool
	TopInvalid             bool
	RedshiftMixedStateNote string
	ExampleBrpX            string
	ExampleBrpY            string
	ExampleGripX           string
	ExampleGripY           string
	HandlebarCount         int
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
	GripMissText      string
	GripFitText       string
	GripFitTitle      string
	BrpMissText       string
	BrpFitText        string
	BrpFitNote        string
	BrpFitTitle       string
	StemText          string
	SpacersText       string
	BarName           string
	RailPositionText  string
	RailOffsetText    string
	RailTitle         string
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
	ShownCount       int
	RowCount         int
	Evaluated        int
	HandlebarSummary string
	BrpTargetText    string
	GripTargetText   string
}

type compareView struct {
	PageTitle              string
	HeaderBadge            string
	FrameCount             int // frames the page ranks
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

// formatPlacedOffset describes where a placed landmark sits relative to its
// target given miss = target - placed (+X forward, +Y up).
func formatPlacedOffset(missX, missY float64) string {
	dx := cleanZero(-missX, 0.05)
	dy := cleanZero(-missY, 0.05)
	if dx == 0 && dy == 0 {
		return "On target"
	}
	var parts []string
	if dx > 0 {
		parts = append(parts, fmt.Sprintf("%.1f mm fwd", dx))
	} else if dx < 0 {
		parts = append(parts, fmt.Sprintf("%.1f mm back", -dx))
	}
	if dy > 0 {
		parts = append(parts, fmt.Sprintf("%.1f mm high", dy))
	} else if dy < 0 {
		parts = append(parts, fmt.Sprintf("%.1f mm low", -dy))
	}
	return strings.Join(parts, ", ")
}

// brpFitNote returns a concise physical explanation for a non-zero BRP miss.
func brpFitNote(setup assembly.BikeSetup, missX, missY float64) (string, error) {
	dx := cleanZero(-missX, 0.05)
	dy := cleanZero(-missY, 0.05)
	if dx == 0 && dy == 0 {
		return "", nil
	}
	railLimit, err := setup.RailOffsetLimitMm()
	if err != nil {
		return "", err
	}
	bounds, err := assembly.SeatpostInsertionBounds(setup.Frame, setup.Seatpost.Spec)
	if err != nil {
		return "", err
	}
	const tol = 1e-4
	railAtRear := math.Abs(setup.Saddle.RailOffsetMm-railLimit) <= tol
	railAtFront := math.Abs(setup.Saddle.RailOffsetMm+railLimit) <= tol
	postAtMaxExt := math.Abs(setup.Seatpost.InsertionMm-bounds.EffectiveMinInsertionMm) <= tol
	postAtBottom := math.Abs(setup.Seatpost.InsertionMm-bounds.MaxAllowedInsertionMm) <= tol

	if (railAtRear || railAtFront) && !postAtMaxExt && !postAtBottom && dy != 0 {
		if dy > 0 {
			return "raised for setback", nil
		}
		return "lowered for setback", nil
	}
	var reasons []string
	if railAtRear {
		reasons = append(reasons, "rail maxed back")
	} else if railAtFront {
		reasons = append(reasons, "rail maxed fwd")
	}
	if postAtMaxExt {
		reasons = append(reasons, "post maxed out")
	} else if postAtBottom {
		reasons = append(reasons, "post bottomed out")
	}
	return strings.Join(reasons, ", "), nil
}

// formatRailPosition describes where the saddle sits along its rails relative
// to the centered ±5 mm scoring band and the hardware travel limits.
func formatRailPosition(offsetMm, limitMm float64) string {
	const (
		tol              = 1e-4
		railIdealLimitMm = 5.0
	)
	if offsetMm >= limitMm-tol {
		return "Max back"
	}
	if offsetMm <= -limitMm+tol {
		return "Max fwd"
	}
	if math.Abs(offsetMm) <= railIdealLimitMm+tol {
		return "Centered"
	}
	if offsetMm > 0 {
		return "Shifted back"
	}
	return "Shifted fwd"
}

// handleCompare serves / over the frames compareFrames returns:
// query.CatalogFrames in production, a few trimmed frames in tests.
func handleCompare(w http.ResponseWriter, r *http.Request, compareFrames func() []compare.Frame) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	frames := compareFrames()
	q := r.URL.Query()
	parsed, err := query.ParseCompare(q)
	if err != nil {
		field := ""
		if inputErr, ok := errors.AsType[*query.InputError](err); ok {
			field = inputErr.Field
		}
		renderCompareError(w, q, len(frames), field, err.Error())
		return
	}
	parsed.Request.Frames = frames
	ranking, err := compare.Rank(parsed.Request)
	if err != nil {
		renderCompareError(w, q, len(frames), "", err.Error())
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
	for _, f := range allFrames {
		framesByID[f.ID] = f
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
		gripMiss := row.Build.GripMiss()
		gripMissText := fmt.Sprintf("(%+.1f, %+.1f) mm", cleanZero(gripMiss.X, 0.05), cleanZero(gripMiss.Y, 0.05))
		brpMissText := fmt.Sprintf("(%+.1f, %+.1f) mm", cleanZero(sc.BrpMissXMm, 0.05), cleanZero(sc.BrpMissYMm, 0.05))
		brpNote, err := brpFitNote(setup, sc.BrpMissXMm, sc.BrpMissYMm)
		if err != nil {
			return compareView{}, err
		}
		railLimit, err := setup.RailOffsetLimitMm()
		if err != nil {
			return compareView{}, err
		}
		railOffsetText := fmt.Sprintf("%+.1f mm", cleanZero(setup.Saddle.RailOffsetMm, 0.05))

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
			GripMissText:      gripMissText,
			GripFitText:       formatPlacedOffset(gripMiss.X, gripMiss.Y),
			GripFitTitle:      fmt.Sprintf("Oval R %.2f · target − placed %s (%.1f mm)", sc.GripR, gripMissText, gripMiss.Norm()),
			BrpMissText:       brpMissText,
			BrpFitText:        formatPlacedOffset(sc.BrpMissXMm, sc.BrpMissYMm),
			BrpFitNote:        brpNote,
			BrpFitTitle:       fmt.Sprintf("target − placed %s", brpMissText),
			StemText:          fmt.Sprintf("%.0f mm, %+.0f°", st.LengthMm, cleanZero(float64(st.AngleDeg), 0.5)),
			SpacersText:       fmt.Sprintf("%.0f mm", st.SpacersMm),
			BarName:           bar.Name,
			RailPositionText:  formatRailPosition(setup.Saddle.RailOffsetMm, railLimit),
			RailOffsetText:    railOffsetText,
			RailTitle:         fmt.Sprintf("Rail offset %s (range ±%.1f mm)", railOffsetText, railLimit),
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

	frameCount := len(parsed.Request.Frames)
	pageTitle, headerBadge := compareTitle(frameCount)

	bars := catalog.Handlebars()
	handlebarSummary := parsed.Bar.Name
	if parsed.AllBarOptions {
		handlebarSummary = fmt.Sprintf("Best of all %d handlebars", len(bars))
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
		FrameCount:  frameCount,
		HasResults:  true,
		Form: compareFormValues{
			PostOptions:            buildPostSelectOptions(parsed.Post.ID, false),
			BarOptions:             buildBarSelectOptions(parsed.Bar.ID, false),
			BarOptionModes:         buildBarOptionModeOptions(parsed.AllBarOptions, "", false),
			BrpX:                   formatFormFloat(parsed.BrpTarget.X),
			BrpY:                   formatFormFloat(parsed.BrpTarget.Y),
			GripX:                  formatFormFloat(parsed.GripTarget.X),
			GripY:                  formatFormFloat(parsed.GripTarget.Y),
			TopPerFrame:            topPerFrameStr,
			Top:                    topStr,
			RedshiftMixedStateNote: mixedNote,
			ExampleBrpX:            formatFormFloat(defBrp.X),
			ExampleBrpY:            formatFormFloat(defBrp.Y),
			ExampleGripX:           formatFormFloat(defGrip.X),
			ExampleGripY:           formatFormFloat(defGrip.Y),
			HandlebarCount:         len(bars),
		},
		Summary: compareSummaryView{
			ShownCount:       len(rows),
			RowCount:         ranking.RowCount,
			Evaluated:        ranking.Evaluated,
			HandlebarSummary: handlebarSummary,
			BrpTargetText:    fmt.Sprintf("(%s, %s) mm", formatFormFloat(parsed.BrpTarget.X), formatFormFloat(parsed.BrpTarget.Y)),
			GripTargetText:   fmt.Sprintf("(%.1f, %.1f) mm", parsed.GripTarget.X, parsed.GripTarget.Y),
		},
		AllBarOptions:          parsed.AllBarOptions,
		ActiveWeight:           weights.ActiveTotal(true),
		TierLegend:             fitTierLegend(),
		AttributeHeaders:       buildAttributeHeaders(weights),
		Rows:                   rows,
		SlidingDropoutsSummary: slidingDropoutsSummary,
	}, nil
}

// compareTitle returns the page title and header badge for ranking frameCount frames.
func compareTitle(frameCount int) (pageTitle, headerBadge string) {
	return fmt.Sprintf("Compare %d Frames — bikegeo", frameCount), fmt.Sprintf("All %d frames", frameCount)
}

func renderCompareError(w http.ResponseWriter, q url.Values, frameCount int, invalidField, msg string) {
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

	var mixedNote string
	if postRaw == catalog.DefaultSeatpostID {
		mixedNote = redshiftMixedStateNote
	}

	view := compareView{
		PageTitle:    "Invalid Comparison — bikegeo",
		HeaderBadge:  "Invalid query",
		FrameCount:   frameCount,
		HasResults:   false,
		ErrorMessage: msg,
		Form: compareFormValues{
			PostOptions:            buildPostSelectOptions(postRaw, !postOK),
			BarOptions:             buildBarSelectOptions(barRaw, !barOK),
			BarOptionModes:         buildBarOptionModeOptions(allBarOpts, barOptRaw, barOptRaw != "" && barOptRaw != query.BarOptionsSelected && barOptRaw != query.BarOptionsAll),
			BrpX:                   brpX,
			BrpY:                   brpY,
			GripX:                  gripX,
			GripY:                  gripY,
			TopPerFrame:            strings.TrimSpace(q.Get("top_per_frame")),
			Top:                    strings.TrimSpace(q.Get("top")),
			PostInvalid:            postInvalid,
			BarInvalid:             barInvalid,
			BarOptionModeInvalid:   barOptInvalid,
			BrpXInvalid:            invalidField == "brp" || invalidField == "brp_x",
			BrpYInvalid:            invalidField == "brp" || invalidField == "brp_y",
			GripXInvalid:           invalidField == "grip" || invalidField == "grip_x",
			GripYInvalid:           invalidField == "grip" || invalidField == "grip_y",
			TopPerFrameInvalid:     invalidField == "top_per_frame",
			TopInvalid:             invalidField == "top",
			RedshiftMixedStateNote: mixedNote,
			ExampleBrpX:            formatFormFloat(defBrp.X),
			ExampleBrpY:            formatFormFloat(defBrp.Y),
			ExampleGripX:           formatFormFloat(defGrip.X),
			ExampleGripY:           formatFormFloat(defGrip.Y),
			HandlebarCount:         len(catalog.Handlebars()),
		},
	}
	renderCompareTemplate(w, http.StatusBadRequest, view)
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
