package web

import (
	"net/http"
	"net/url"
	"strings"

	"bikegeo/pkg/catalog"
	"bikegeo/pkg/cli"
	"bikegeo/pkg/compare"
	"bikegeo/pkg/frames"
	"bikegeo/pkg/ideal"
	"bikegeo/pkg/report"
)

// Control is one form input on a generic page.
type Control struct {
	Name, Label, Kind string // Kind: text | select | checkbox
	Value             string
	Options           []Option
	Hint              string
}

// page describes a generic page that wraps one CLI subcommand. The subcommand
// is run through cli.RunDocument, so the web and terminal output always agree.
type page struct {
	Path, Title, Blurb string
	Controls           func(v url.Values) []Control
	// Args maps the query to subcommand arguments; ready=false shows only the form.
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
					{Name: "category", Label: "Category", Kind: "select", Value: v.Get("category"), Options: options([]string{"hardtail", "rigid", "gravel", "full-suspension"}, v.Get("category"), "Any")},
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
			Link: map[string]string{"Frame ID": "/validate?frame=", "ID": "/validate?frame="},
		},
		{
			Path: "/forks", Title: "Forks",
			Blurb: "Standard 29er fork presets and their travel-to-A2C mappings.",
			Args:  func(url.Values) ([]string, bool) { return []string{"forks"}, true },
		},
		{
			Path: "/stems", Title: "Stems",
			Blurb: "Stem presets and their length / angle envelopes.",
			Args:  func(url.Values) ([]string, bool) { return []string{"stems"}, true },
		},
		{
			Path: "/ideals", Title: "Ideals",
			Blurb: "Ideal frame profiles: the target and acceptable ranges every setup is scored against.",
			Controls: func(v url.Values) []Control {
				var names []string
				for _, p := range ideal.ListProfiles() {
					names = append(names, p.Name)
				}
				return []Control{{Name: "name", Label: "Profile", Kind: "select", Value: v.Get("name"), Options: options(names, v.Get("name"), "All profiles")}}
			},
			Args: func(v url.Values) ([]string, bool) {
				if n := strings.TrimSpace(v.Get("name")); n != "" {
					return []string{"ideals", "show", n}, true
				}
				return []string{"ideals"}, true
			},
			Link: map[string]string{"Name": "/ideals?name="},
		},
		{
			Path: "/saddles", Title: "Saddles",
			Blurb: "Saddle catalog with pelvic-fit translation between saddles.",
			Controls: func(v url.Values) []Control {
				return []Control{{Name: "id", Label: "Saddle", Kind: "select", Value: v.Get("id"), Options: saddleOptions(v.Get("id"), "Whole catalog")}}
			},
			Args: func(v url.Values) ([]string, bool) {
				if id := strings.TrimSpace(v.Get("id")); id != "" {
					return []string{"saddles", id}, true
				}
				return []string{"saddles"}, true
			},
			Link: map[string]string{"Saddle ID": "/saddles?id="},
		},
		{
			Path: "/saddle-swap", Title: "Saddle swap",
			Blurb: "What changes when you swap saddles: naive BRP setback versus true pelvic fit.",
			Controls: func(v url.Values) []Control {
				return []Control{
					{Name: "from", Label: "From", Kind: "select", Value: v.Get("from"), Options: saddleOptions(v.Get("from"), "Reference saddle")},
					{Name: "to", Label: "To", Kind: "select", Value: v.Get("to"), Options: saddleOptions(v.Get("to"), "Default target")},
					{Name: "frame", Label: "On frame", Kind: "select", Value: v.Get("frame"), Options: frameOptions(v.Get("frame"), "No frame")},
				}
			},
			Args: func(v url.Values) ([]string, bool) {
				args := []string{"saddle-swap"}
				from, to := strings.TrimSpace(v.Get("from")), strings.TrimSpace(v.Get("to"))
				if to != "" && from == "" {
					from = catalog.DefaultSaddle().ID
				}
				if from != "" && to == "" {
					to = "berthoud-aspin-open"
				}
				if from != "" {
					args = append(args, from)
				}
				if to != "" {
					args = append(args, to)
				}
				if f := strings.TrimSpace(v.Get("frame")); f != "" {
					args = append(args, "--frame", f)
				}
				return args, true
			},
		},
		{
			Path: "/handlebars", Title: "Handlebars",
			Blurb: "Handlebar catalog, 3D kinematic model and hand-position comparison.",
			Controls: func(v url.Values) []Control {
				return []Control{
					{Name: "id", Label: "Handlebar", Kind: "select", Value: v.Get("id"), Options: handlebarOptions(v.Get("id"), "Whole catalog")},
					{Name: "compare", Label: "Compare with", Kind: "select", Value: v.Get("compare"), Options: handlebarOptions(v.Get("compare"), "Nothing")},
				}
			},
			Args: func(v url.Values) ([]string, bool) {
				args := []string{"handlebars"}
				if id := strings.TrimSpace(v.Get("id")); id != "" {
					args = append(args, id)
				}
				if c := strings.TrimSpace(v.Get("compare")); c != "" {
					args = append(args, "--compare", c)
				}
				return args, true
			},
			Link: map[string]string{"Handlebar ID": "/handlebars?id="},
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
			Link: map[string]string{"Frame ID": "/validate?frame=", "Frame": "/validate?frame="},
		},
		{
			Path: "/fit", Title: "Fit",
			Blurb: "Back-solve a frame's CAD residuals into fork A2C, spacer and sag, and sweep trial parameters.",
			Controls: func(v url.Values) []Control {
				return []Control{
					{Name: "frame", Label: "Frame", Kind: "select", Value: v.Get("frame"), Options: frameOptions(v.Get("frame"), "Choose a frame…")},
					{Name: "size", Label: "Size", Kind: "text", Value: v.Get("size"), Hint: "default size if blank"},
					{Name: "try", Label: "Trials", Kind: "text", Value: v.Get("try"), Hint: "e.g. fork.a2c=470,472 spacer=10,12"},
					{Name: "detail", Label: "Per-size detail", Kind: "checkbox", Value: v.Get("detail")},
				}
			},
			Args: func(v url.Values) ([]string, bool) {
				f := strings.TrimSpace(v.Get("frame"))
				if f == "" {
					return nil, false
				}
				if sz := strings.TrimSpace(v.Get("size")); sz != "" {
					f += ":" + sz
				}
				args := []string{"fit", f}
				for t := range strings.FieldsSeq(v.Get("try")) {
					args = append(args, "--try", t)
				}
				if isTrue(v.Get("detail")) {
					args = append(args, "--detail")
				}
				return args, true
			},
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
		args, ready := p.Args(v)
		d.Ready = ready
		if ready {
			doc, errText, code := cli.RunDocument(args)
			if code != 0 {
				status = http.StatusBadRequest
				d.Banner = errText
				if d.Banner == "" {
					d.Banner = "The request could not be completed."
				}
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
			d.CLI = "bikegeo " + strings.Join(quoteAll(args), " ")
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
	Metrics []helpMetric
}

type helpMetric struct {
	ID, Label, Unit, Ideal, Acceptable string
	Weight                             float64
	Scored                             bool
}

func (s *server) help(w http.ResponseWriter, _ *http.Request) {
	d := helpData{base: newBase("Help", "/help")}
	prof, _ := ideal.GetProfile("idmatch")
	specs := map[string]ideal.AttributeSpec{}
	if prof != nil {
		for _, a := range prof.Attributes {
			specs[a.ID] = a
		}
	}
	for _, m := range ideal.AllAttributes() {
		hm := helpMetric{ID: m.ID, Label: m.Label, Unit: m.Unit}
		if sp, ok := specs[m.ID]; ok {
			hm.Ideal, hm.Acceptable = compare.FormatAttrRanges(sp, m)
			hm.Weight = sp.Weight
			hm.Scored = sp.Weight > 0 && sp.Ideal != nil
		}
		d.Metrics = append(d.Metrics, hm)
	}
	s.render(w, http.StatusOK, "help.html", d)
}
