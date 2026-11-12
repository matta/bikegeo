package cli

import (
	"fmt"
	"io"
	"strings"

	"bikegeo/pkg/report"
)

// renderMarkdown writes one table, with its footers as bullet lists, as Markdown.
func renderMarkdown(w io.Writer, t report.Table) {
	if t.Title != "" {
		fmt.Fprintf(w, "### %s\n", t.Title)
	}
	if t.Subtitle != "" {
		fmt.Fprintf(w, "%s\n\n", t.Subtitle)
	}
	if len(t.Columns) > 0 {
		heads := make([]string, len(t.Columns))
		seps := make([]string, len(t.Columns))
		for i, c := range t.Columns {
			heads[i] = escapeMarkdownCell(c.Label)
			seps[i] = "---"
		}
		fmt.Fprintf(w, "| %s |\n", strings.Join(heads, " | "))
		fmt.Fprintf(w, "| %s |\n", strings.Join(seps, " | "))
		for _, r := range t.Rows {
			cells := make([]string, len(t.Columns))
			for i := range cells {
				if i < len(r.Cells) {
					cells[i] = escapeMarkdownCell(r.Cells[i].Text)
				}
			}
			fmt.Fprintf(w, "| %s |\n", strings.Join(cells, " | "))
		}
	}
	for _, f := range t.Footers {
		if len(f.Lines) == 0 {
			continue
		}
		if f.Title != "" {
			fmt.Fprintf(w, "\n#### %s\n\n", f.Title)
		} else {
			fmt.Fprintln(w)
		}
		for _, l := range f.Lines {
			fmt.Fprintf(w, "- %s\n", l)
		}
	}
}

func escapeMarkdownCell(s string) string {
	s = strings.ReplaceAll(s, "\n", "<br>")
	s = strings.ReplaceAll(s, "|", "\\|")
	return s
}

// printDocument writes each section of doc, separated by blank lines: tables as
// Markdown, text sections verbatim.
func (a *App) printDocument(doc report.Document) {
	for i, sec := range doc.Sections {
		if i > 0 {
			fmt.Fprintln(a.Out)
		}
		if sec.Table != nil {
			renderMarkdown(a.Out, *sec.Table)
		} else {
			fmt.Fprintln(a.Out, sec.Text)
		}
	}
}
