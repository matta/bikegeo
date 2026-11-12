package cli

import (
	"bytes"
	"strconv"
	"strings"

	"bikegeo/pkg/report"
)

// collector records command output as ordered report sections. Text written to
// App.Out becomes a text section; tables passed to App.render become table
// sections, preserving the order a terminal user would see.
type collector struct {
	doc report.Document
	buf bytes.Buffer
}

func (c *collector) Write(p []byte) (int, error) { return c.buf.Write(p) }

func (c *collector) flushText() {
	text := strings.Trim(c.buf.String(), "\n")
	c.buf.Reset()
	if strings.TrimSpace(text) == "" {
		return
	}
	c.doc.Sections = append(c.doc.Sections, report.Section{Text: text})
}

func (c *collector) addTable(t report.Table) {
	c.flushText()
	if t.ID == "" {
		t.ID = "table-" + strconv.Itoa(len(c.doc.Sections)+1)
	}
	c.doc.Sections = append(c.doc.Sections, report.Section{Table: &t})
}

// RunDocument executes a subcommand (without global flags) and returns its
// output as a structured document instead of printing it. errText is whatever
// the command wrote to stderr; code is the exit code.
func RunDocument(args []string) (doc report.Document, errText string, code int) {
	var errBuf bytes.Buffer
	col := &collector{}
	app := &App{Format: FormatTable, In: strings.NewReader(""), Out: col, Err: &errBuf, collect: col}
	code = app.dispatch(args)
	col.flushText()
	return col.doc, strings.TrimSpace(errBuf.String()), code
}

// render prints a table in a.Format, or records it when collecting.
func (a *App) render(title, subtitle string, headers []string, rows [][]string) error {
	return a.renderFooters(title, subtitle, headers, rows, nil)
}

// renderFooters is render with footer sections.
func (a *App) renderFooters(title, subtitle string, headers []string, rows [][]string, footers []report.Footer) error {
	if a.collect != nil {
		t := report.FromStrings(report.Slug(title), title, subtitle, headers, rows, footers...)
		annotateZones(&t)
		a.collect.addTable(t)
		return nil
	}
	return RenderOutputWithFooter(a.Out, a.Format, title, subtitle, headers, rows, footers)
}

// annotateZones tags cells carrying ✓ / ✗ markers or PASS / FAIL words so HTML
// output can colour them.
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
