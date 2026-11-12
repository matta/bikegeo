package web

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"bikegeo/pkg/compare"
)

// FieldError ties a validation message to a form field (or "" for the page).
type FieldError struct {
	Field string
	Msg   string
}

// Layout values for CompareQuery.Layout.
const (
	LayoutAuto    = "auto"
	LayoutRows    = "rows"    // one row per bike (transposed)
	LayoutColumns = "columns" // one column per bike
)

// CompareQuery is the complete state of the compare page. It is parsed from
// the URL and is the only input to rendering: the page is a pure function of it.
type CompareQuery struct {
	// Specs are CLI-grammar tokens ("fenrir:L@110mm@20%"), gathered from the
	// repeated `col` parameter followed by the aligned frame/size/fork/sag
	// form fields. Empty means "use the compiled default baseline".
	Specs []string

	Saddle      string
	Handlebar   string
	Layout      string
	NoScore     bool
	All         bool
	Top         int
	TopPerFrame *int
}

// ParseCompareQuery decodes v, applying defaults for anything missing and
// collecting a FieldError for each malformed value.
func ParseCompareQuery(v url.Values) (CompareQuery, []FieldError) {
	var errs []FieldError
	q := CompareQuery{
		Saddle:    strings.TrimSpace(v.Get("saddle")),
		Handlebar: strings.TrimSpace(v.Get("handlebar")),
		Layout:    LayoutAuto,
		NoScore:   v.Get("score") == "0",
		All:       isTrue(v.Get("all")),
	}

	for _, s := range v["col"] {
		if s = strings.TrimSpace(s); s != "" {
			q.Specs = append(q.Specs, s)
		}
	}
	frames := v["frame"]
	for i, f := range frames {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		spec := f
		if sz := at(v["size"], i); sz != "" {
			spec += ":" + sz
		}
		if fk := at(v["fork"], i); fk != "" {
			spec += "@" + fk
		}
		if sg := at(v["sag"], i); sg != "" {
			spec += "@" + strings.TrimSuffix(sg, "%") + "%"
		}
		q.Specs = append(q.Specs, spec)
	}

	switch l := strings.ToLower(strings.TrimSpace(v.Get("layout"))); l {
	case "", LayoutAuto:
	case LayoutRows, LayoutColumns:
		q.Layout = l
	default:
		errs = append(errs, FieldError{"layout", fmt.Sprintf("unknown layout %q (use auto, rows or columns)", l)})
	}

	if s := strings.TrimSpace(v.Get("top")); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 0 {
			errs = append(errs, FieldError{"top", fmt.Sprintf("%q is not a non-negative whole number", s)})
		} else {
			q.Top = n
		}
	}
	if s := strings.TrimSpace(v.Get("top_per_frame")); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 0 {
			errs = append(errs, FieldError{"top_per_frame", fmt.Sprintf("%q is not a non-negative whole number", s)})
		} else {
			q.TopPerFrame = &n
		}
	}
	return q, errs
}

func isTrue(s string) bool {
	switch strings.ToLower(s) {
	case "1", "true", "on", "yes":
		return true
	}
	return false
}

func at(s []string, i int) string {
	if i < len(s) {
		return strings.TrimSpace(s[i])
	}
	return ""
}

// Options converts the query to the engine's request type.
func (q CompareQuery) Options() compare.Options {
	o := compare.Options{
		Specs:       q.Specs,
		Saddle:      q.Saddle,
		Handlebar:   q.Handlebar,
		All:         q.All,
		Top:         q.Top,
		TopPerFrame: q.TopPerFrame,
	}
	switch q.Layout {
	case LayoutRows:
		t := true
		o.Transpose = &t
	case LayoutColumns:
		t := false
		o.Transpose = &t
	}
	return o
}

// Values encodes the non-column parts of the query (columns are encoded
// separately from resolved rows so the URL always reflects what was shown).
func (q CompareQuery) Values() url.Values {
	v := url.Values{}
	if q.Saddle != "" {
		v.Set("saddle", q.Saddle)
	}
	if q.Handlebar != "" {
		v.Set("handlebar", q.Handlebar)
	}
	if q.Layout != LayoutAuto {
		v.Set("layout", q.Layout)
	}
	if q.NoScore {
		v.Set("score", "0")
	}
	if q.All {
		v.Set("all", "1")
	}
	if q.Top > 0 {
		v.Set("top", strconv.Itoa(q.Top))
	}
	if q.TopPerFrame != nil {
		v.Set("top_per_frame", strconv.Itoa(*q.TopPerFrame))
	}
	return v
}

// CLI renders the equivalent `bikegeo compare` command line.
func (q CompareQuery) CLI() string {
	parts := []string{"bikegeo", "compare"}
	if q.Saddle != "" {
		parts = append(parts, "--saddle", q.Saddle)
	}
	if q.Handlebar != "" {
		parts = append(parts, "--handlebar", q.Handlebar)
	}
	switch q.Layout {
	case LayoutRows:
		parts = append(parts, "--transpose")
	case LayoutColumns:
		parts = append(parts, "--transpose=false")
	}
	if q.NoScore {
		parts = append(parts, "--no-score")
	}
	if q.All {
		parts = append(parts, "--all")
	}
	if q.Top > 0 {
		parts = append(parts, "--top", strconv.Itoa(q.Top))
	}
	if q.TopPerFrame != nil {
		parts = append(parts, "--top-per-frame", strconv.Itoa(*q.TopPerFrame))
	}
	for _, s := range q.Specs {
		parts = append(parts, shellQuote(s))
	}
	return strings.Join(parts, " ")
}

func shellQuote(s string) string {
	if strings.ContainsAny(s, " \t%&|;<>()$`\\\"'*?[]#~=!{}") {
		return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
	}
	return s
}
