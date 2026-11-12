// Package report defines the presentation-neutral structured output of bikegeo
// commands. Both the terminal renderers (pkg/cli) and the HTML renderer
// (pkg/web) consume these types; the package performs no I/O.
package report

import "strings"

// Zone classifies how a cell value relates to an ideal profile.
const (
	ZoneNone       = ""
	ZoneIdeal      = "ideal"
	ZoneAcceptable = "acceptable"
	ZoneOutside    = "outside"
	ZoneMissing    = "missing"
)

// Row kinds.
const (
	KindData       = "data"
	KindIdeal      = "ideal"
	KindAcceptable = "acceptable"
)

// Document is the full output of one command or page.
type Document struct {
	Title  string
	Tables []Table
	Notes  []string
}

// Column describes one table column.
type Column struct {
	ID      string
	Label   string // may contain "\n" for terminal wrapping
	Numeric bool
}

// Cell is a single table cell with optional semantic metadata.
type Cell struct {
	Text     string
	Zone     string // one of the Zone constants
	MetricID string // ideal attribute ID when the cell is a metric value
	Href     string // optional link target (web only)
}

// Row is one table row.
type Row struct {
	ID    string
	Kind  string
	Attrs map[string]string // becomes data-* attributes in HTML
	Cells []Cell
}

// Footer is a titled list of lines rendered beneath a table.
type Footer struct {
	Title string
	Lines []string
}

// Table is a titled grid of cells with optional footers.
type Table struct {
	ID       string
	Title    string
	Subtitle string
	Columns  []Column
	Rows     []Row
	Footers  []Footer
}

// Headers returns the column labels.
func (t Table) Headers() []string {
	out := make([]string, len(t.Columns))
	for i, c := range t.Columns {
		out[i] = c.Label
	}
	return out
}

// StringRows flattens rows to plain cell text.
func (t Table) StringRows() [][]string {
	out := make([][]string, len(t.Rows))
	for i, r := range t.Rows {
		cells := make([]string, len(r.Cells))
		for j, c := range r.Cells {
			cells[j] = c.Text
		}
		out[i] = cells
	}
	return out
}

// FromStrings builds a Table from plain string headers and rows, as produced
// by commands that have no per-cell semantics.
func FromStrings(id, title, subtitle string, headers []string, rows [][]string, footers ...Footer) Table {
	t := Table{ID: id, Title: title, Subtitle: subtitle, Footers: footers}
	for _, h := range headers {
		t.Columns = append(t.Columns, Column{ID: Slug(h), Label: h})
	}
	for _, r := range rows {
		row := Row{Kind: KindData}
		for _, c := range r {
			row.Cells = append(row.Cells, Cell{Text: c})
		}
		t.Rows = append(t.Rows, row)
	}
	return t
}

// Slug lowercases s and collapses non-alphanumerics to single dashes.
func Slug(s string) string {
	var sb strings.Builder
	dash := false
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			sb.WriteRune(r)
			dash = false
		default:
			if !dash && sb.Len() > 0 {
				sb.WriteByte('-')
				dash = true
			}
		}
	}
	return strings.TrimSuffix(sb.String(), "-")
}
