package web

import (
	"net/url"
	"reflect"
	"testing"
)

func TestParseCompareQuery(t *testing.T) {
	five := 5
	tests := []struct {
		name     string
		query    string
		want     CompareQuery
		wantErrs []string // fields
	}{
		{
			name:  "empty uses defaults",
			query: "",
			want:  CompareQuery{Layout: LayoutAuto},
		},
		{
			name:  "col specs verbatim",
			query: "col=fenrir:L@110mm@20%25&col=hayduke",
			want:  CompareQuery{Layout: LayoutAuto, Specs: []string{"fenrir:L@110mm@20%", "hayduke"}},
		},
		{
			name:  "aligned form fields fold into specs and skip blank frames",
			query: "frame=fenrir&size=L&fork=110mm&sag=20&frame=&size=&fork=&sag=&frame=hayduke&size=&fork=rigid&sag=",
			want:  CompareQuery{Layout: LayoutAuto, Specs: []string{"fenrir:L@110mm@20%", "hayduke@rigid"}},
		},
		{
			name:  "settings",
			query: "saddle=berthoud-aspin-open&handlebar=x&layout=rows&score=0&all=1&top=3&top_per_frame=5",
			want: CompareQuery{
				Layout: LayoutRows, Saddle: "berthoud-aspin-open", Handlebar: "x",
				NoScore: true, All: true, Top: 3, TopPerFrame: &five,
			},
		},
		{
			name:  "all=1 ignores echoed column fields",
			query: "frame=fenrir&size=L&fork=110mm&sag=20&col=hayduke&all=1",
			want:  CompareQuery{Layout: LayoutAuto, All: true},
		},
		{
			name:     "bad numbers and layout are reported and ignored",
			query:    "top=abc&top_per_frame=-2&layout=zigzag",
			want:     CompareQuery{Layout: LayoutAuto},
			wantErrs: []string{"layout", "top", "top_per_frame"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			v, err := url.ParseQuery(tc.query)
			if err != nil {
				t.Fatal(err)
			}
			got, errs := ParseCompareQuery(v)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("query = %+v, want %+v", got, tc.want)
			}
			var fields []string
			for _, e := range errs {
				fields = append(fields, e.Field)
			}
			if !reflect.DeepEqual(fields, tc.wantErrs) {
				t.Errorf("error fields = %v, want %v", fields, tc.wantErrs)
			}
		})
	}
}

func TestCompareQueryCLI(t *testing.T) {
	q := CompareQuery{Layout: LayoutRows, NoScore: true, Specs: []string{"fenrir:L@110mm@20%", "hayduke"}}
	want := "bikegeo compare --transpose --no-score 'fenrir:L@110mm@20%' hayduke"
	if got := q.CLI(); got != want {
		t.Errorf("CLI() = %q, want %q", got, want)
	}
	if got := shellQuote(""); got != "''" {
		t.Errorf("shellQuote(\"\") = %q, want ''", got)
	}
}
