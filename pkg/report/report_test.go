package report

import (
	"reflect"
	"testing"
)

func TestFromStringsRoundTrip(t *testing.T) {
	headers := []string{"Frame (Size)", "Reach"}
	rows := [][]string{{"A (L)", "480"}, {"B (XL)", "500"}}
	tbl := FromStrings("t", "Title", "Sub", headers, rows)

	if !reflect.DeepEqual(tbl.Headers(), headers) {
		t.Errorf("Headers() = %v, want %v", tbl.Headers(), headers)
	}
	if !reflect.DeepEqual(tbl.StringRows(), rows) {
		t.Errorf("StringRows() = %v, want %v", tbl.StringRows(), rows)
	}
	if tbl.Columns[0].ID != "frame-size" {
		t.Errorf("column ID = %q, want frame-size", tbl.Columns[0].ID)
	}
}

func TestSlug(t *testing.T) {
	cases := map[string]string{
		"Saddle-HT Drop":   "saddle-ht-drop",
		"  Fork & Eff A2C": "fork-eff-a2c",
		"BB\nDrop":         "bb-drop",
		"":                 "",
	}
	for in, want := range cases {
		if got := Slug(in); got != want {
			t.Errorf("Slug(%q) = %q, want %q", in, got, want)
		}
	}
}
