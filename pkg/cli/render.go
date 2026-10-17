package cli

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

const (
	FormatTable    = "table"
	FormatJSON     = "json"
	FormatMarkdown = "md"
	FormatCSV      = "csv"
)

// NormalizeFormat validates and normalizes an output format string.
func NormalizeFormat(f string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(f)) {
	case "", "table", "text", "ascii":
		return FormatTable, nil
	case "json":
		return FormatJSON, nil
	case "md", "markdown":
		return FormatMarkdown, nil
	case "csv":
		return FormatCSV, nil
	default:
		return "", fmt.Errorf("unsupported format %q (supported: table, json, md, csv)", f)
	}
}

// RenderOutput renders structured tabular data in the requested format (table, json, md, csv).
func RenderOutput(
	w io.Writer,
	format string,
	title string,
	subtitle string,
	headers []string,
	rows [][]string,
	jsonPayload any,
) error {
	norm, err := NormalizeFormat(format)
	if err != nil {
		return err
	}

	switch norm {
	case FormatJSON:
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(jsonPayload)

	case FormatCSV:
		cw := csv.NewWriter(w)
		if len(headers) > 0 {
			if err := cw.Write(headers); err != nil {
				return err
			}
		}
		for _, r := range rows {
			flat := make([]string, len(r))
			for i, cell := range r {
				flat[i] = strings.ReplaceAll(cell, "\n", " | ")
			}
			if err := cw.Write(flat); err != nil {
				return err
			}
		}
		cw.Flush()
		return cw.Error()

	case FormatMarkdown:
		if title != "" {
			fmt.Fprintf(w, "### %s\n", title)
		}
		if subtitle != "" {
			fmt.Fprintf(w, "%s\n\n", subtitle)
		}
		if len(headers) == 0 {
			return nil
		}
		flatHeaders := make([]string, len(headers))
		seps := make([]string, len(headers))
		for i, h := range headers {
			flatHeaders[i] = escapeMarkdownCell(h)
			seps[i] = "---"
		}
		fmt.Fprintf(w, "| %s |\n", strings.Join(flatHeaders, " | "))
		fmt.Fprintf(w, "| %s |\n", strings.Join(seps, " | "))
		for _, r := range rows {
			cells := make([]string, len(headers))
			for i := range headers {
				if i < len(r) {
					cells[i] = escapeMarkdownCell(r[i])
				}
			}
			fmt.Fprintf(w, "| %s |\n", strings.Join(cells, " | "))
		}
		return nil

	default: // FormatTable
		if title != "" {
			fmt.Fprintf(w, "%s\n", title)
		}
		if subtitle != "" {
			fmt.Fprintf(w, "%s\n", subtitle)
		}
		if len(headers) == 0 {
			return nil
		}
		RenderBoxTable(w, headers, rows)
		return nil
	}
}

func escapeMarkdownCell(s string) string {
	s = strings.ReplaceAll(s, "\n", "<br>")
	s = strings.ReplaceAll(s, "|", "\\|")
	return s
}

// RenderBoxTable prints a Unicode box-drawing table supporting multi-line cells.
func RenderBoxTable(w io.Writer, headers []string, rows [][]string) {
	numCols := len(headers)
	if numCols == 0 {
		return
	}

	colWidths := make([]int, numCols)
	for c, h := range headers {
		for _, line := range strings.Split(h, "\n") {
			if l := utf8.RuneCountInString(line); l > colWidths[c] {
				colWidths[c] = l
			}
		}
	}

	for _, r := range rows {
		for c := 0; c < numCols && c < len(r); c++ {
			for _, line := range strings.Split(r[c], "\n") {
				if l := utf8.RuneCountInString(line); l > colWidths[c] {
					colWidths[c] = l
				}
			}
		}
	}

	printBorder := func(left, mid, right, fill string) {
		var sb strings.Builder
		sb.WriteString(left)
		for c, width := range colWidths {
			sb.WriteString(strings.Repeat(fill, width+2))
			if c < numCols-1 {
				sb.WriteString(mid)
			}
		}
		sb.WriteString(right)
		fmt.Fprintln(w, sb.String())
	}

	printMultilineRow := func(cells []string) {
		splitCells := make([][]string, numCols)
		maxLines := 1
		for c := 0; c < numCols; c++ {
			val := ""
			if c < len(cells) {
				val = cells[c]
			}
			lines := strings.Split(val, "\n")
			splitCells[c] = lines
			if len(lines) > maxLines {
				maxLines = len(lines)
			}
		}

		for lineIdx := 0; lineIdx < maxLines; lineIdx++ {
			var sb strings.Builder
			sb.WriteString("│")
			for c := 0; c < numCols; c++ {
				text := ""
				if lineIdx < len(splitCells[c]) {
					text = splitCells[c][lineIdx]
				}
				pad := colWidths[c] - utf8.RuneCountInString(text)
				if pad < 0 {
					pad = 0
				}
				sb.WriteString(" ")
				sb.WriteString(text)
				sb.WriteString(strings.Repeat(" ", pad+1))
				sb.WriteString("│")
			}
			fmt.Fprintln(w, sb.String())
		}
	}

	printBorder("┌", "┬", "┐", "─")
	printMultilineRow(headers)
	printBorder("├", "┼", "┤", "─")
	for i, r := range rows {
		printMultilineRow(r)
		if i < len(rows)-1 && hasMultiline(rows) {
			printBorder("├", "┼", "┤", "─")
		}
	}
	printBorder("└", "┴", "┘", "─")
}

func hasMultiline(rows [][]string) bool {
	for _, r := range rows {
		for _, cell := range r {
			if strings.Contains(cell, "\n") {
				return true
			}
		}
	}
	return false
}
