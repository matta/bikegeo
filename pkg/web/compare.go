package web

import (
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"

	"bikegeo/pkg/canonicalizer"
	"bikegeo/pkg/catalog"
	"bikegeo/pkg/compare"
	"bikegeo/pkg/frames"
	"bikegeo/pkg/model"
	"bikegeo/pkg/report"
	"bikegeo/pkg/scoring"
)

// Option is one entry of a <select>.
type Option struct {
	Value, Label string
	Selected     bool
}

// colRow is one editable comparison column in the form. Normally it carries
// validated selects; if the submitted specs failed to resolve, Raw is true and
// Spec echoes the text instead.
type colRow struct {
	N          int
	Raw        bool
	Spec       string
	Frame      string
	FrameName  string
	Sizes      []Option
	Forks      []Option
	Sags       []Option
	BikeHref   string
	RemoveHref string
}

type compareData struct {
	base
	Q          CompareQuery
	Banner     []string
	FieldErr   map[string]string
	Rows       []colRow
	RawMode    bool
	AddFrames  []Option
	Saddles    []Option
	Handlebars []Option
	Layouts    []Option
	Scores     []Option
	Tables     []TableView
	Ledgers    []TableView
	Footers    []report.Footer
	CLI        string
	Permalink  string
	Count      int
}

func options(values []string, current string, blankLabel string) []Option {
	var out []Option
	if blankLabel != "" {
		out = append(out, Option{Value: "", Label: blankLabel, Selected: current == ""})
	}
	seen := map[string]bool{}
	found := current == ""
	for _, v := range values {
		if seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, Option{Value: v, Label: v, Selected: v == current})
		if v == current {
			found = true
		}
	}
	if !found {
		out = append([]Option{{Value: current, Label: current, Selected: true}}, out...)
	}
	return out
}

// forkToken names a row's fork the way a user would type it ("110mm",
// "rigid"). If that token would not resolve to the row's exact A2C and rake
// (the compiled default baseline carries hand-entered values that can differ
// from the frame's catalog fork by a millimetre), it falls back to the exact
// "a2c=<static>/<rake>" form so the URL reproduces what is on screen.
func forkToken(f model.FrameModel, r model.CanonicalComparisonRow) string {
	tok := "rigid"
	if !r.Setup.IsRigid {
		tok = canonicalizer.FormatForkType(r)
	}
	var sag model.Opt[float64]
	if r.Setup.SagPct > 0 {
		sag = model.Some(r.Setup.SagPct)
	}
	rf, err := canonicalizer.ResolveForkToken(tok, sag, f)
	exact := err == nil && math.Abs(rf.TargetRake-r.Setup.RakeMm) < 0.5 &&
		(math.Abs(rf.TargetA2c-r.Setup.A2cMm) < 0.5 || math.Abs(rf.StaticA2c-r.Setup.A2cMm) < 0.5)
	if !exact && tok != "rigid" {
		return fmt.Sprintf("a2c=%.0f/%.0f", r.Setup.A2cMm, r.Setup.RakeMm)
	}
	return tok
}

// forkChoices lists the fork tokens a frame supports.
func forkChoices(f model.FrameModel) []string {
	setups, err := canonicalizer.ExpandPlausibleForkSetups(f, model.Opt[float64]{})
	if err != nil {
		return nil
	}
	var out []string
	for _, s := range setups {
		switch {
		case s.IsRigid:
			out = append(out, "rigid")
		case s.TravelMm > 0:
			out = append(out, fmt.Sprintf("%.0fmm", s.TravelMm))
		default:
			out = append(out, s.DisplayLabel)
		}
	}
	return out
}

func (s *server) compare(w http.ResponseWriter, r *http.Request) {
	q, errs := ParseCompareQuery(r.URL.Query())
	d := compareData{
		base:     newBase("Compare", "/"),
		Q:        q,
		FieldErr: map[string]string{},
		CLI:      q.CLI(),
	}
	status := http.StatusOK
	for _, e := range errs {
		d.Banner = append(d.Banner, e.Msg)
		d.FieldErr[e.Field] = e.Msg
		status = http.StatusBadRequest
	}

	allFrames := frames.AllFrames()
	for _, f := range allFrames {
		d.AddFrames = append(d.AddFrames, Option{Value: f.ID, Label: f.Manufacturer + " " + f.Model})
	}
	for _, sd := range catalog.AllSaddles() {
		d.Saddles = append(d.Saddles, Option{Value: sd.ID, Label: sd.Name, Selected: sd.ID == q.Saddle})
	}
	for _, hb := range catalog.AllHandlebars() {
		d.Handlebars = append(d.Handlebars, Option{Value: hb.ID, Label: hb.Name, Selected: hb.ID == q.Handlebar})
	}
	d.Layouts = []Option{
		{"auto", "Automatic", q.Layout == LayoutAuto},
		{LayoutRows, "One row per bike", q.Layout == LayoutRows},
		{LayoutColumns, "One column per bike", q.Layout == LayoutColumns},
	}
	d.Scores = []Option{{"1", "Score and rank", !q.NoScore}, {"0", "Geometry only", q.NoScore}}

	res, err := compare.Run(q.Options())
	if err != nil {
		d.Banner = append(d.Banner, err.Error())
		d.FieldErr["col"] = err.Error()
		d.RawMode = true
		for i, spec := range q.Specs {
			d.Rows = append(d.Rows, colRow{N: i + 1, Raw: true, Spec: spec})
		}
		s.render(w, http.StatusBadRequest, "compare.html", d)
		return
	}

	frameByID := map[string]model.FrameModel{}
	for _, f := range allFrames {
		frameByID[f.ID] = f
	}
	type formCol struct{ frame, size, fork, sag string }
	cols := make([]formCol, len(res.Rows))
	for i, row := range res.Rows {
		fc := formCol{frame: row.Setup.FrameID, size: row.Setup.Size, fork: forkToken(frameByID[row.Setup.FrameID], row)}
		if row.Setup.SagPct > 0 {
			fc.sag = strconv.FormatFloat(row.Setup.SagPct, 'f', -1, 64)
		}
		cols[i] = fc
	}
	for i, fc := range cols {
		f := frameByID[fc.frame]
		var rest []formCol
		rest = append(rest, cols[:i]...)
		rest = append(rest, cols[i+1:]...)
		v := q.Values()
		v.Del("all")
		for _, c := range rest {
			v.Add("frame", c.frame)
			v.Add("size", c.size)
			v.Add("fork", c.fork)
			v.Add("sag", c.sag)
		}
		bikeVals := url.Values{}
		bikeVals.Set("frame", fc.frame)
		bikeVals.Set("size", fc.size)
		if fc.fork != "" {
			bikeVals.Set("fork", fc.fork)
		}
		if fc.sag != "" {
			bikeVals.Set("sag", fc.sag)
		}
		if q.Saddle != "" {
			bikeVals.Set("saddle", q.Saddle)
		}
		if q.Handlebar != "" {
			bikeVals.Set("handlebar", q.Handlebar)
		}
		d.Rows = append(d.Rows, colRow{
			N:          i + 1,
			Frame:      fc.frame,
			FrameName:  f.Manufacturer + " " + f.Model,
			Sizes:      options(f.GetSizes(), fc.size, ""),
			Forks:      options(forkChoices(f), fc.fork, ""),
			Sags:       options([]string{"20", "25", "30"}, fc.sag, "—"),
			BikeHref:   "/bike?" + bikeVals.Encode(),
			RemoveHref: "/?" + v.Encode() + "#columns",
		})
	}
	pv := q.Values()
	if !q.All {
		for _, c := range cols {
			pv.Add("frame", c.frame)
			pv.Add("size", c.size)
			pv.Add("fork", c.fork)
			pv.Add("sag", c.sag)
		}
	}
	d.Permalink = "/?" + pv.Encode()
	d.Count = len(res.Rows)

	if status == http.StatusOK {
		doc, err := compare.BuildDocument(res, compare.DocOptions{
			Scoring:    !q.NoScore,
			Detail:     !q.NoScore,
			DocFooters: true,
		})
		if err != nil {
			d.Banner = append(d.Banner, err.Error())
			s.render(w, http.StatusInternalServerError, "compare.html", d)
			return
		}
		bikeHrefByCol := make(map[string]string, len(d.Rows))
		for _, row := range d.Rows {
			bikeHrefByCol[strconv.Itoa(row.N)] = row.BikeHref
		}
		for _, t := range doc.Tables() {
			tv := toView(t)
			attachInspectLinks(&tv, t, bikeHrefByCol)
			d.Tables = append(d.Tables, tv)
		}
		var scores []scoring.Result
		if !q.NoScore {
			scores = scoring.ScoreAllRows(res.Rows, res.Profile)
		}
		for i, row := range res.Rows {
			sc, err := buildSceneForComparisonRow(row, res.Profile)
			if err != nil {
				continue
			}
			var scorePtr *scoring.Result
			if i < len(scores) {
				scorePtr = &scores[i]
			}
			ledgerTitle := fmt.Sprintf("[%d] %s %s (%s)", i+1, row.Bike.Manufacturer, row.Bike.Model, row.Bike.Size)
			d.Ledgers = append(d.Ledgers, toView(buildComputationLedger(fmt.Sprintf("ledger-%d", i+1), ledgerTitle, sc, row.Canonical.CockpitFitResult, scorePtr)))
		}
		d.Footers = doc.Footers
	}
	s.render(w, status, "compare.html", d)
}

func attachInspectLinks(tv *TableView, t report.Table, bikeHrefByCol map[string]string) {
	if len(tv.Head) > 0 {
		for i, col := range t.Columns {
			if colID := col.Attrs["col"]; colID != "" {
				if href := bikeHrefByCol[colID]; href != "" {
					tv.Head[0].Cells[i].InspectHref = href
				}
			}
		}
	}
	frameColIdx := -1
	for i, col := range t.Columns {
		if col.ID == "frame" {
			frameColIdx = i
			break
		}
	}
	if frameColIdx < 0 {
		return
	}
	bodyIdx := 0
	for _, r := range t.Rows {
		if r.Kind == report.KindIdeal {
			continue
		}
		if bodyIdx < len(tv.Body) && frameColIdx < len(tv.Body[bodyIdx].Cells) {
			if colID := r.Attrs["col"]; colID != "" {
				if href := bikeHrefByCol[colID]; href != "" {
					tv.Body[bodyIdx].Cells[frameColIdx].InspectHref = href
				}
			}
		}
		bodyIdx++
	}
}
