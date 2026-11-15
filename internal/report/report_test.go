package report

import (
	"reflect"
	"testing"
)

func tableHeaders(t Table) []string {
	out := make([]string, len(t.Columns))
	for i, c := range t.Columns {
		out[i] = c.Label
	}
	return out
}

func tableStringRows(t Table) [][]string {
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

func TestFromStringsRoundTrip(t *testing.T) {
	headers := []string{"Frame (Size)", "Reach"}
	rows := [][]string{{"A (L)", "480"}, {"B (XL)", "500"}}
	tbl := FromStrings("t", "Title", "Sub", headers, rows)

	if !reflect.DeepEqual(tableHeaders(tbl), headers) {
		t.Errorf("tableHeaders() = %v, want %v", tableHeaders(tbl), headers)
	}
	if !reflect.DeepEqual(tableStringRows(tbl), rows) {
		t.Errorf("tableStringRows() = %v, want %v", tableStringRows(tbl), rows)
	}
	if tbl.Columns[0].ID != "frame-size" {
		t.Errorf("column ID = %q, want frame-size", tbl.Columns[0].ID)
	}
}

func TestSlug(t *testing.T) {
	cases := map[string]string{
		"Seat Axis–HT Drop": "seat-axis-ht-drop",
		"  Fork & Eff A2C":  "fork-eff-a2c",
		"BB\nDrop":          "bb-drop",
		"":                  "",
	}
	for in, want := range cases {
		if got := Slug(in); got != want {
			t.Errorf("Slug(%q) = %q, want %q", in, got, want)
		}
	}
}
