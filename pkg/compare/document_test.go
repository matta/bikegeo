package compare

import (
	"strings"
	"testing"

	"bikegeo/pkg/report"
)

func mustRun(t *testing.T, opts Options) *Result {
	t.Helper()
	res, err := Run(opts)
	if err != nil {
		t.Fatalf("Run(%+v): %v", opts, err)
	}
	return res
}

func TestRunDefaultBaseline(t *testing.T) {
	res := mustRun(t, Options{})
	if got, want := len(res.Rows), len(DefaultComparisonState().Columns); got != want {
		t.Errorf("default rows = %d, want %d", got, want)
	}
	if !res.State.Transpose {
		t.Error("default state with >4 columns should be transposed")
	}
}

func TestRunErrors(t *testing.T) {
	for _, opts := range []Options{
		{Specs: []string{"nosuchframe"}},
		{Specs: []string{"fenrir@17%"}},
		{Saddle: "nope"},
		{Handlebar: "nope"},
		{Specs: []string{"-all"}},
	} {
		if _, err := Run(opts); err == nil {
			t.Errorf("Run(%+v) succeeded, want error", opts)
		}
	}
}

func TestBuildDocumentSplitLayout(t *testing.T) {
	res := mustRun(t, Options{})
	doc, err := BuildDocument(res, DocOptions{Scoring: true, Detail: true, DocFooters: true})
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, tb := range doc.Tables {
		ids = append(ids, tb.ID)
	}
	if got := strings.Join(ids, ","); got != "fit-cockpit,chassis,breakdown" {
		t.Fatalf("table IDs = %s", got)
	}
	if len(doc.Footers) == 0 {
		t.Error("expected document-level footers")
	}
	for _, tb := range doc.Tables {
		if len(tb.Footers) != 0 {
			t.Errorf("table %s has footers despite DocFooters", tb.ID)
		}
	}

	fit := doc.Tables[0]
	if fit.Rows[0].Kind != report.KindIdeal || fit.Rows[1].Kind != report.KindAcceptable {
		t.Error("first two rows should be ideal/acceptable references")
	}
	zones := map[string]int{}
	for _, r := range fit.Rows[2:] {
		if r.Attrs["frame-id"] == "" || r.Attrs["rank"] == "" {
			t.Errorf("data row missing attrs: %+v", r.Attrs)
		}
		for _, c := range r.Cells {
			if c.MetricID != "" {
				zones[c.Zone]++
			}
		}
	}
	if zones[report.ZoneIdeal]+zones[report.ZoneAcceptable]+zones[report.ZoneOutside] == 0 {
		t.Errorf("no zoned metric cells found: %v", zones)
	}
	if got := len(fit.Rows) - 2; got != len(res.Rows) {
		t.Errorf("data rows = %d, want %d", got, len(res.Rows))
	}
}

func TestBuildDocumentFooterPlacement(t *testing.T) {
	res := mustRun(t, Options{Specs: []string{"fenrir:L@110mm", "hayduke:S3@110mm"}})

	withDetail, _ := BuildDocument(res, DocOptions{Scoring: true, Detail: true})
	last := withDetail.Tables[len(withDetail.Tables)-1]
	if last.ID != TableBreakdown || len(last.Footers) == 0 {
		t.Errorf("footers should follow the breakdown table, got %s with %d", last.ID, len(last.Footers))
	}

	flat, _ := BuildDocument(mustRun(t, Options{}), DocOptions{Scoring: true, Flat: true})
	if len(flat.Tables) != 1 || flat.Tables[0].ID != TableMatrix {
		t.Errorf("flat layout should be one matrix table, got %d", len(flat.Tables))
	}
}

func TestBuildDocumentColumnLayoutHasColumnAttrs(t *testing.T) {
	res := mustRun(t, Options{Specs: []string{"fenrir:L@110mm", "hayduke:S3@110mm"}})
	doc, err := BuildDocument(res, DocOptions{Scoring: true})
	if err != nil {
		t.Fatal(err)
	}
	tb := doc.Tables[0]
	if tb.ID != TableMatrix {
		t.Fatalf("table ID = %s", tb.ID)
	}
	var withAttrs int
	for _, c := range tb.Columns {
		if c.Attrs["frame-id"] != "" {
			withAttrs++
		}
	}
	if withAttrs != 2 {
		t.Errorf("columns with frame-id = %d, want 2", withAttrs)
	}
}
