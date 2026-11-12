package web

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"bikegeo/pkg/report"
)

// docb accumulates a report.Document and the first user-facing error while a
// page builder runs. Builders return 0 on success and non-zero after fail.
type docb struct {
	doc report.Document
	err string
}

func (b *docb) fail(format string, args ...any) {
	if b.err == "" {
		b.err = fmt.Sprintf(format, args...)
	}
}

func (b *docb) text(s string) {
	if s = strings.Trim(s, "\n"); strings.TrimSpace(s) != "" {
		b.doc.AddText(s)
	}
}

func (b *docb) textf(format string, args ...any) { b.text(fmt.Sprintf(format, args...)) }

func (b *docb) render(title, subtitle string, headers []string, rows [][]string) error {
	return b.renderFooters(title, subtitle, headers, rows, nil)
}

func (b *docb) renderFooters(title, subtitle string, headers []string, rows [][]string, footers []report.Footer) error {
	id := report.Slug(title)
	if id == "" {
		id = "table-" + strconv.Itoa(len(b.doc.Sections)+1)
	}
	t := report.FromStrings(id, title, subtitle, headers, rows, footers...)
	annotateZones(&t)
	b.doc.AddTable(t)
	return nil
}

// annotateZones tags cells carrying ✓ / ✗ markers or PASS / FAIL words so they
// are coloured by meaning.
func annotateZones(t *report.Table) {
	for i := range t.Rows {
		for j := range t.Rows[i].Cells {
			c := &t.Rows[i].Cells[j]
			switch {
			case strings.Contains(c.Text, "✓"), c.Text == "PASS":
				c.Zone = report.ZoneIdeal
			case strings.Contains(c.Text, "✗"), c.Text == "FAIL":
				c.Zone = report.ZoneOutside
			}
		}
	}
}

// floatParam reads a float query parameter, falling back to def when absent.
func floatParam(v url.Values, name string, def float64) (float64, error) {
	s := strings.TrimSpace(v.Get(name))
	if s == "" {
		return def, nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return def, fmt.Errorf("%s: %q is not a number", name, s)
	}
	return f, nil
}

// floats reads several float parameters, collecting the first error.
type floats struct {
	v   url.Values
	err error
}

func (f *floats) get(name string, def float64) float64 {
	x, err := floatParam(f.v, name, def)
	if err != nil && f.err == nil {
		f.err = err
	}
	return x
}
