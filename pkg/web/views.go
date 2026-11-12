package web

import (
	"html"
	"html/template"
	"net/url"
	"sort"
	"strings"

	"bikegeo/pkg/ideal"
	"bikegeo/pkg/report"
)

// CellView is a template-ready table cell.
type CellView struct {
	Lines []string
	Class string
	Attrs template.HTMLAttr
	Href  string
	Scope string // "col" or "row" renders a <th>; empty renders a <td>
}

// RowView is a template-ready table row.
type RowView struct {
	Class string
	Attrs template.HTMLAttr
	Cells []CellView
}

// TableView is a template-ready table: header rows (labels plus Ideal /
// Acceptable reference rows) in Head, data rows in Body.
type TableView struct {
	ID       string
	Title    string
	Subtitle []string
	Head     []RowView
	Body     []RowView
	Footers  []report.Footer
}

// attrString renders m as data-* attributes with escaped values. Keys that are
// not simple lowercase words are dropped.
func attrString(maps ...map[string]string) template.HTMLAttr {
	merged := map[string]string{}
	for _, m := range maps {
		for k, v := range m {
			merged[k] = v
		}
	}
	keys := make([]string, 0, len(merged))
	for k := range merged {
		ok := k != ""
		for _, r := range k {
			if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-') {
				ok = false
			}
		}
		if ok {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	var sb strings.Builder
	for _, k := range keys {
		sb.WriteString(` data-` + k + `="` + html.EscapeString(merged[k]) + `"`)
	}
	return template.HTMLAttr(sb.String())
}

func isRowHeader(colID string) bool { return colID == "metric" || colID == "frame" }

func cellClasses(col report.Column, zone string) string {
	var cls []string
	if col.Numeric {
		cls = append(cls, "num")
	}
	if isRowHeader(col.ID) {
		cls = append(cls, "sticky")
	}
	if zone != "" {
		cls = append(cls, "zone-"+zone)
	}
	return strings.Join(cls, " ")
}

// toView converts a table for rendering. Column and row attributes become
// data-* attributes, metric columns link to the glossary, and cells that name
// a frame link to its validation page.
func toView(t report.Table) TableView {
	v := TableView{ID: t.ID, Title: t.Title, Footers: t.Footers}
	if t.Subtitle != "" {
		v.Subtitle = strings.Split(t.Subtitle, "\n")
	}

	header := RowView{Class: "labels"}
	for _, c := range t.Columns {
		cv := CellView{
			Lines: strings.Split(c.Label, "\n"),
			Class: cellClasses(c, ""),
			Attrs: attrString(map[string]string{"column": c.ID}, c.Attrs),
			Scope: "col",
		}
		if _, ok := ideal.LookupAttribute(c.ID); ok {
			cv.Href = "/help#metric-" + url.PathEscape(c.ID)
		}
		if fid := c.Attrs["frame-id"]; fid != "" {
			cv.Href = "/validate?frame=" + url.QueryEscape(fid)
		}
		header.Cells = append(header.Cells, cv)
	}
	v.Head = append(v.Head, header)

	for _, r := range t.Rows {
		rv := RowView{Attrs: attrString(map[string]string{"row": r.ID, "kind": r.Kind}, r.Attrs)}
		if r.Kind == report.KindIdeal || r.Kind == report.KindAcceptable {
			rv.Class = "ref " + r.Kind
		}
		for i, c := range r.Cells {
			var col report.Column
			if i < len(t.Columns) {
				col = t.Columns[i]
			}
			cell := CellView{
				Lines: strings.Split(c.Text, "\n"),
				Class: cellClasses(col, c.Zone),
				Href:  c.Href,
			}
			if isRowHeader(col.ID) && r.Kind == report.KindData {
				cell.Scope = "row"
			}
			attrs := map[string]string{}
			if c.MetricID != "" {
				attrs["metric"] = c.MetricID
			}
			if c.Zone != "" {
				attrs["zone"] = c.Zone
			}
			cell.Attrs = attrString(attrs, col.Attrs)
			if col.ID == "frame" && cell.Href == "" && r.Attrs["frame-id"] != "" {
				cell.Href = "/validate?frame=" + url.QueryEscape(r.Attrs["frame-id"])
			}
			rv.Cells = append(rv.Cells, cell)
		}
		if rv.Class != "" {
			v.Head = append(v.Head, rv)
		} else {
			v.Body = append(v.Body, rv)
		}
	}
	return v
}
