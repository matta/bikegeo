package web

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"bikegeo/internal/catalog"
	"bikegeo/internal/compare"
	"bikegeo/internal/frames"
	"bikegeo/internal/ideal"
	"bikegeo/internal/model"
	"bikegeo/internal/report"
	"bikegeo/internal/validator"
)

// Control is one form input on a generic page.
type Control struct {
	Name, Label, Kind string // Kind: text | select | checkbox
	Value             string
	Options           []Option
	Hint              string
}

// page describes a generic page: query controls, a Build that turns the query
// into a report.Document, and optional links from cell text to other pages.
type page struct {
	Path, Title, Blurb string
	Controls           func(v url.Values) []Control
	// Build produces the page's document; a non-empty errText becomes the 400 banner.
	Build func(v url.Values) (doc report.Document, errText string)
	// Args, when set, maps the query to the equivalent `bikegeo` command line,
	// shown on the page. Only pages with a CLI counterpart set it; ready=false
	// shows only the form.
	Args func(v url.Values) (args []string, ready bool)
	// Link maps a column label to a URL prefix; cells in that column link to
	// prefix + escaped cell text.
	Link map[string]string
}

type sectionView struct {
	Text  string
	Table *TableView
}

type pageData struct {
	base
	Blurb    string
	Action   string
	Controls []Control
	Banner   string
	Sections []sectionView
	CLI      string
	Ready    bool
}

func frameOptions(current string, blank string) []Option {
	opts := []Option{{Value: "", Label: blank, Selected: current == ""}}
	for _, f := range frames.AllFrames() {
		opts = append(opts, Option{Value: f.ID, Label: f.Manufacturer + " " + f.Model + " (" + f.ID + ")", Selected: f.ID == current})
	}
	return opts
}

func saddleOptions(current, blank string) []Option {
	var opts []Option
	if blank != "" {
		opts = append(opts, Option{Value: "", Label: blank, Selected: current == ""})
	}
	for _, s := range catalog.AllSaddles() {
		opts = append(opts, Option{Value: s.ID, Label: s.Name, Selected: s.ID == current})
	}
	return opts
}

func handlebarOptions(current, blank string) []Option {
	opts := []Option{{Value: "", Label: blank, Selected: current == ""}}
	for _, h := range catalog.AllHandlebars() {
		opts = append(opts, Option{Value: h.ID, Label: h.Name, Selected: h.ID == current})
	}
	return opts
}

func pages() []page {
	return []page{
		{
			Path: "/frames", Title: "Frames",
			Blurb: "Every frame model with its published geometry and supported forks.",
			Controls: func(v url.Values) []Control {
				return []Control{
					{Name: "q", Label: "Search", Kind: "text", Value: v.Get("q"), Hint: "manufacturer, model or ID"},
					{Name: "category", Label: "Category", Kind: "select", Value: v.Get("category"), Options: options(compare.AvailableCategories(frames.AllFrames()), v.Get("category"), "Any")},
					{Name: "size", Label: "Size", Kind: "text", Value: v.Get("size"), Hint: "e.g. L"},
					{Name: "all-sizes", Label: "One row per size", Kind: "checkbox", Value: v.Get("all-sizes")},
				}
			},
			Args: func(v url.Values) ([]string, bool) {
				args := []string{"list"}
				for _, k := range []string{"q", "category", "size"} {
					if s := strings.TrimSpace(v.Get(k)); s != "" {
						args = append(args, "--"+k+"="+s)
					}
				}
				if isTrue(v.Get("all-sizes")) {
					args = append(args, "--all-sizes")
				}
				return args, true
			},
			Build: func(v url.Values) (report.Document, string) {
				doc, err := compare.BuildFrameListDocument(frames.AllFrames(), compare.ListFilter{
					Query:    v.Get("q"),
					Category: v.Get("category"),
					Size:     v.Get("size"),
					AllSizes: isTrue(v.Get("all-sizes")),
				})
				if err != nil {
					return report.Document{}, err.Error()
				}
				return doc, ""
			},
			Link: map[string]string{"Frame ID": "/validate?frame=", "ID": "/validate?frame="},
		},
		{
			Path: "/saddles", Title: "Saddles",
			Blurb: "Saddle catalog with pelvic-fit translation between saddles.",
			Controls: func(v url.Values) []Control {
				return []Control{
					{Name: "id", Label: "Saddle", Kind: "select", Value: v.Get("id"), Options: saddleOptions(v.Get("id"), "Whole catalog")},
					numberControl(v, "saddle-height", "Saddle height (mm)", "BB to BRP, default 746"),
					numberControl(v, "sit-bone-setback", "Sit-bone setback (mm)", fmt.Sprintf("behind BB, default %.0f", model.DefaultTargetSitBoneSetbackMm())),
				}
			},
			Build: run(func(db *docb, v url.Values) int { return db.saddles(v) }),
			Link:  map[string]string{"Saddle ID": "/saddles?id="},
		},
		{
			Path: "/saddle-swap", Title: "Saddle swap",
			Blurb: "What changes when you swap saddles: naive BRP setback versus true pelvic fit.",
			Controls: func(v url.Values) []Control {
				return []Control{
					{Name: "from", Label: "From", Kind: "select", Value: v.Get("from"), Options: saddleOptions(v.Get("from"), "Reference saddle")},
					{Name: "to", Label: "To", Kind: "select", Value: v.Get("to"), Options: saddleOptions(v.Get("to"), "Default target")},
					{Name: "all", Label: "Compare against every saddle", Kind: "checkbox", Value: v.Get("all")},
					{Name: "frame", Label: "On frame", Kind: "select", Value: v.Get("frame"), Options: frameOptions(v.Get("frame"), "No frame")},
					numberControl(v, "saddle-height", "Saddle height (mm)", "BB to BRP, default 746"),
					numberControl(v, "sit-bone-setback", "Sit-bone setback (mm)", fmt.Sprintf("behind BB, default %.0f", model.DefaultTargetSitBoneSetbackMm())),
					numberControl(v, "brp-setback", "BRP setback (mm)", "derives sit-bone setback"),
					numberControl(v, "seatpost", "Seatpost setback (mm)", "default 0 (zero-setback dropper)"),
					numberControl(v, "clamp-length", "Clamp length (mm)", "default 30"),
				}
			},
			Build: run(func(db *docb, v url.Values) int { return db.saddleSwap(v) }),
		},
		{
			Path: "/handlebars", Title: "Handlebars",
			Blurb: "Handlebar catalog, 3D kinematic model and hand-position comparison.",
			Controls: func(v url.Values) []Control {
				return []Control{
					{Name: "id", Label: "Handlebar", Kind: "select", Value: v.Get("id"), Options: handlebarOptions(v.Get("id"), "Whole catalog")},
					{Name: "compare", Label: "Compare with", Kind: "select", Value: v.Get("compare"), Options: handlebarOptions(v.Get("compare"), "Nothing")},
					numberControl(v, "grip-inset", "Grip inset (mm)", "tip to hand centre, default 65"),
					numberControl(v, "roll", "Roll (°)", "+ rolls top forward"),
					numberControl(v, "width", "Trimmed width (mm)", "optional"),
				}
			},
			Build: run(func(db *docb, v url.Values) int { return db.handlebars(v) }),
			Link:  map[string]string{"Handlebar ID": "/handlebars?id="},
		},
		{
			Path: "/validate", Title: "Validate",
			Blurb: "Check a frame's published numbers against the CAD stack equation and its supported fork configurations.",
			Controls: func(v url.Values) []Control {
				return []Control{
					{Name: "frame", Label: "Frame", Kind: "select", Value: v.Get("frame"), Options: frameOptions(v.Get("frame"), "All frames")},
					{Name: "size", Label: "Size", Kind: "text", Value: v.Get("size"), Hint: "default size if blank"},
				}
			},
			Args: func(v url.Values) ([]string, bool) {
				f := strings.TrimSpace(v.Get("frame"))
				if f == "" {
					return []string{"validate", "--all"}, true
				}
				args := []string{"validate", f}
				if sz := strings.TrimSpace(v.Get("size")); sz != "" {
					args = append(args, "--size", sz)
				}
				return args, true
			},
			Build: func(v url.Values) (report.Document, string) {
				var tokens []string
				if f := strings.TrimSpace(v.Get("frame")); f != "" {
					tokens = []string{f}
				}
				targets, err := compare.ResolveValidationTargets(frames.AllFrames(), tokens, strings.TrimSpace(v.Get("size")))
				if err != nil {
					return report.Document{}, err.Error()
				}
				doc, _, err := validator.BuildValidationDocument(targets, validator.Catalog{
					Forks: catalog.CommonForkCatalog,
					Stems: catalog.CommonStemCatalog,
				})
				if err != nil {
					return doc, err.Error()
				}
				return doc, ""
			},
			Link: map[string]string{"Frame ID": "/validate?frame=", "Frame": "/validate?frame="},
		},
	}
}

func (s *server) page(p page) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		v := r.URL.Query()
		d := pageData{base: newBase(p.Title, p.Path), Blurb: p.Blurb, Action: p.Path}
		if p.Controls != nil {
			d.Controls = p.Controls(v)
		}

		status := http.StatusOK
		var args []string
		ready := true
		if p.Args != nil {
			args, ready = p.Args(v)
		}
		d.Ready = ready
		if ready {
			doc, errText := p.Build(v)
			if errText != "" {
				status = http.StatusBadRequest
				d.Banner = errText
			}
			linkTables(&doc, p.Link)
			for _, sec := range doc.Sections {
				if sec.Table != nil {
					tv := toView(*sec.Table)
					d.Sections = append(d.Sections, sectionView{Table: &tv})
				} else if sec.Text != "" {
					d.Sections = append(d.Sections, sectionView{Text: sec.Text})
				}
			}
			if p.Args != nil {
				d.CLI = "bikegeo " + strings.Join(quoteAll(args), " ")
			}
		}
		s.render(w, status, "page.html", d)
	}
}

func quoteAll(args []string) []string {
	out := make([]string, len(args))
	for i, a := range args {
		out[i] = shellQuote(a)
	}
	return out
}

// linkTables sets Href on cells under any column whose label has a prefix in links.
func linkTables(doc *report.Document, links map[string]string) {
	if len(links) == 0 {
		return
	}
	for si := range doc.Sections {
		t := doc.Sections[si].Table
		if t == nil {
			continue
		}
		for ci, col := range t.Columns {
			prefix, ok := links[col.Label]
			if !ok {
				continue
			}
			for ri := range t.Rows {
				if ci < len(t.Rows[ri].Cells) {
					c := &t.Rows[ri].Cells[ci]
					if c.Text != "" {
						c.Href = prefix + url.QueryEscape(c.Text)
					}
				}
			}
		}
	}
}

// helpData backs the glossary page.
type helpData struct {
	base
	Profile  helpProfile
	Metrics  []helpMetric
	Stems    []helpStemSet
	Spacers  string
	RailNote string
}

// helpProfile summarizes the ideal profile every setup is scored against.
type helpProfile struct {
	Name, Description, SaddleHeight, TargetRef, CockpitTarget string
}

// helpStemSet is one stock stem catalog: the lengths and angles the cockpit scan tries.
type helpStemSet struct {
	ID, Lengths, Angles string
	Count               int
}

type helpMetric struct {
	ID, Label, Unit, Ideal string
	Cost                   string
	Scored                 bool
}

func (s *server) help(w http.ResponseWriter, _ *http.Request) {
	d := helpData{base: newBase("Help", "/help")}
	prof, _ := ideal.GetProfile("idmatch")
	specs := map[ideal.MetricID]ideal.AttributeSpec{}
	if prof != nil {
		for _, a := range prof.Attributes {
			specs[a.ID] = a
		}
		d.Profile = helpProfile{
			Name:          prof.Name,
			Description:   prof.Description,
			SaddleHeight:  "—",
			TargetRef:     prof.TargetRef.Label(),
			CockpitTarget: fmt.Sprintf("%.0f mm from the BRP, %+.0f mm above the sit bones", prof.CockpitTargetDistMm, prof.CockpitTargetRiseMm),
		}
		if sh, ok := prof.SaddleHeightMm.Get(); ok {
			d.Profile.SaddleHeight = fmt.Sprintf("%.0f mm", sh)
		}
	}
	for _, st := range catalog.CommonStemCatalog {
		hs := helpStemSet{ID: st.ID, Lengths: "—", Count: len(st.Expand())}
		if len(st.LengthRangeMm) == 2 {
			hs.Lengths = fmt.Sprintf("%.0f–%.0f mm in %.0f mm steps", st.LengthRangeMm[0], st.LengthRangeMm[1], model.StemLengthStepMm)
		}
		var angs []string
		for _, a := range st.AnglesDeg {
			angs = append(angs, fmt.Sprintf("%+.0f°", float64(a)))
		}
		hs.Angles = strings.Join(angs, ", ")
		d.Stems = append(d.Stems, hs)
	}
	d.Spacers = fmt.Sprintf("0 to each frame's max_spacer_mm in %.0f mm steps", model.SpacerStepMm)
	saddle := catalog.DefaultSaddle()
	railLo, railHi := saddle.ClampWindowMm(model.DefaultSeatpostClampLengthMm)
	d.RailNote = fmt.Sprintf("the saddle's clamp window in 1 mm steps (%s: %+.0f to %+.0f mm)", saddle.Name, railLo, railHi)
	for _, m := range ideal.AllAttributes() {
		hm := helpMetric{ID: string(m.ID), Label: m.Label, Unit: m.Unit}
		if sp, ok := specs[m.ID]; ok {
			hm.Ideal = compare.FormatAttrRange(sp, m)
			hm.Cost = compare.FormatCost(sp, m)
			hm.Scored = sp.IsScored()
		}
		d.Metrics = append(d.Metrics, hm)
	}
	s.render(w, http.StatusOK, "help.html", d)
}

// run adapts a docb-based builder to page.Build.
func run(fn func(db *docb, v url.Values) int) func(url.Values) (report.Document, string) {
	return func(v url.Values) (report.Document, string) {
		db := &docb{}
		fn(db, v)
		return db.doc, db.err
	}
}

func numberControl(v url.Values, name, label, hint string) Control {
	return Control{Name: name, Label: label, Kind: "text", Value: v.Get(name), Hint: hint}
}
