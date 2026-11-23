package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"slices"
	"strings"
	"testing"

	"bikegeo/internal/compare"
	"bikegeo/internal/pose"
	"bikegeo/internal/query"
)

// jsonPose mirrors pose.Pose's MarshalJSON output. pose.Pose has no
// UnmarshalJSON, so decoding into query.CompareResult would zero every pose.
type jsonPose struct {
	Origin     pose.Point
	HeadingDeg pose.Deg
}

// jsonResult mirrors the query.CompareResult fields the tests check.
type jsonResult struct {
	PostID        string
	BarID         string
	AllBarOptions bool
	RowCount      int
	Evaluated     int
	Rows          []struct {
		FrameID     string
		Size        string
		ForkID      string
		SagPct      float64
		HandlebarID string
		Build       struct {
			Placement struct {
				Setup struct {
					Stem struct {
						LengthMm  float64
						AngleDeg  pose.Deg
						SpacersMm float64
					}
				}
				Frameset struct {
					HeadTubeTop jsonPose
					FrontAxle   jsonPose
				}
				Saddle struct {
					BRP jsonPose
				}
				Handlebar struct {
					HandGrip jsonPose
				}
			}
			Score struct {
				Tier       string
				Total      float64
				GripR      float64
				Attributes []json.RawMessage
			}
		}
	}
}

// krampusXLFox34 returns the Surly Krampus as the only compare frame, trimmed
// to size XL on the Fox 34 534x44, so compare ranks its three sag rows instead
// of the whole catalog.
func krampusXLFox34() []compare.Frame {
	for _, f := range query.CatalogFrames() {
		if f.ID == "surly-krampus" {
			f.Sizes = slices.DeleteFunc(f.Sizes, func(sz compare.Size) bool { return sz.Name != "XL" })
			f.Forks = slices.DeleteFunc(f.Forks, func(fk compare.Fork) bool { return fk.ID != "fox34_120-534x44" })
			return []compare.Frame{f}
		}
	}
	panic("catalog has no surly-krampus")
}

// runCLI runs the CLI with compare ranking krampusXLFox34.
func runCLI(args ...string) (code int, stdout, stderr string) {
	var out, errOut bytes.Buffer
	code = runWith(krampusXLFox34, args, &out, &errOut)
	return code, out.String(), errOut.String()
}

// sag20Row returns r's 20% sag row.
func sag20Row(t *testing.T, r jsonResult) int {
	t.Helper()
	for i, row := range r.Rows {
		if row.SagPct == 20 {
			return i
		}
	}
	t.Fatalf("no 20%% sag row in %d rows", len(r.Rows))
	return -1
}

func decodeResult(t *testing.T, stdout string) jsonResult {
	t.Helper()
	var r jsonResult
	if err := json.Unmarshal([]byte(stdout), &r); err != nil {
		t.Fatalf("decode JSON: %v\n%s", err, stdout)
	}
	return r
}

func assertNear(t *testing.T, name string, got, want, tol float64) {
	t.Helper()
	if math.Abs(got-want) > tol {
		t.Errorf("%s = %.9f, want %.9f (tol %g)", name, got, want, tol)
	}
}

func TestRunCompareSingleRowJSON(t *testing.T) {
	const q = "top_per_frame=0"
	code, stdout, stderr := runCLI("compare", q)
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d, stderr %q; want 0, empty", code, stderr)
	}
	r := decodeResult(t, stdout)
	if r.PostID != "redshift_shockstop_pro" || r.BarID != "persuader20_800" || r.AllBarOptions {
		t.Errorf("PostID %q, BarID %q, AllBarOptions %v", r.PostID, r.BarID, r.AllBarOptions)
	}
	if r.RowCount != 3 || r.Evaluated != 3*749 || len(r.Rows) != 3 {
		t.Fatalf("RowCount %d, Evaluated %d, %d rows; want 3, %d, 3", r.RowCount, r.Evaluated, len(r.Rows), 3*749)
	}
	row := r.Rows[sag20Row(t, r)]
	if row.FrameID != "surly-krampus" || row.Size != "XL" || row.ForkID != "fox34_120-534x44" || row.SagPct != 20 {
		t.Errorf("row = %s %s %s %v%%", row.FrameID, row.Size, row.ForkID, row.SagPct)
	}
	if row.HandlebarID != "persuader20_800" {
		t.Errorf("HandlebarID = %q, want persuader20_800", row.HandlebarID)
	}
	score := row.Build.Score
	if score.Tier != "fits_all" {
		t.Errorf("Tier = %q, want fits_all", score.Tier)
	}
	assertNear(t, "Total", score.Total, 84.690417237, 1e-6)
	assertNear(t, "GripR", score.GripR, 0.725092414, 1e-6)
	if len(score.Attributes) != 12 {
		t.Errorf("got %d attributes, want 12", len(score.Attributes))
	}
	stem := row.Build.Placement.Setup.Stem
	if stem.LengthMm != 110 || stem.AngleDeg != -7 || stem.SpacersMm != 0 {
		t.Errorf("stem (%v, %v, %v), want (110, -7, 0)", stem.LengthMm, stem.AngleDeg, stem.SpacersMm)
	}

	p := row.Build.Placement
	assertNear(t, "Saddle.BRP.Origin.Y", p.Saddle.BRP.Origin.Y, 713.1, 1e-6)
	assertNear(t, "Saddle.BRP.HeadingDeg", float64(p.Saddle.BRP.HeadingDeg), 0, 1e-9)
	assertNear(t, "Handlebar.HandGrip.HeadingDeg", float64(p.Handlebar.HandGrip.HeadingDeg), 0, 1e-9)
	if p.Frameset.HeadTubeTop.Origin.Y <= 600 {
		t.Errorf("Frameset.HeadTubeTop.Origin.Y = %v, want > 600", p.Frameset.HeadTubeTop.Origin.Y)
	}
	if p.Frameset.FrontAxle.Origin.X <= 600 {
		t.Errorf("Frameset.FrontAxle.Origin.X = %v, want > 600", p.Frameset.FrontAxle.Origin.X)
	}

	code2, stdout2, stderr2 := runCLI("compare", "?"+q)
	if code2 != 0 || stderr2 != "" || stdout2 != stdout {
		t.Errorf("leading ?: exit %d, stderr %q, identical stdout %v; want 0, empty, true", code2, stderr2, stdout2 == stdout)
	}

}

func TestRunCompareAllBarsJSON(t *testing.T) {
	code, stdout, stderr := runCLI("compare", "top_per_frame=0&bar_options=all")
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d, stderr %q; want 0, empty", code, stderr)
	}
	r := decodeResult(t, stdout)
	if !r.AllBarOptions || r.RowCount != 3 || r.Evaluated != 3*11235 || len(r.Rows) != 3 {
		t.Fatalf("AllBarOptions %v, RowCount %d, Evaluated %d, %d rows; want true, 3, %d, 3", r.AllBarOptions, r.RowCount, r.Evaluated, len(r.Rows), 3*11235)
	}
	row := r.Rows[sag20Row(t, r)]
	if row.HandlebarID != "sqlab311_20_16_15" {
		t.Errorf("HandlebarID = %q, want sqlab311_20_16_15", row.HandlebarID)
	}
	assertNear(t, "Total", row.Build.Score.Total, 87.120779384, 1e-6)
}

func TestRunCompareHelpAndUsage(t *testing.T) {
	for _, args := range [][]string{nil, {"--help"}} {
		code, stdout, stderr := runCLI(args...)
		if code != 0 || stderr != "" {
			t.Errorf("%v: exit %d, stderr %q; want 0, empty", args, code, stderr)
		}
		if !strings.Contains(stdout, "serve") || !strings.Contains(stdout, "compare") {
			t.Errorf("%v: usage %q does not mention serve and compare", args, stdout)
		}
	}
	code, stdout, stderr := runCLI("compare", "--help")
	if code != 0 || stderr != "" {
		t.Errorf("compare --help: exit %d, stderr %q; want 0, empty", code, stderr)
	}
	if !strings.Contains(stdout, "bikegeo compare [query]") {
		t.Errorf("compare --help: usage %q does not mention the syntax", stdout)
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }

func TestRunCompareErrors(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		code    int
		wantErr string
	}{
		{"unknown subcommand", []string{"nope"}, 2, "Unknown command"},
		{"too many args", []string{"compare", "a=1", "b=2"}, 2, "Usage: bikegeo compare [query]"},
		{"bad URL escape", []string{"compare", "frame=%zz"}, 2, "invalid query"},
		{"semicolon query", []string{"compare", "frame=surly-krampus;size=XL"}, 2, "invalid query"},
		{"unknown query key", []string{"compare", "bar_option=all"}, 2, "unknown query parameter"},
		{"cleared BRP", []string{"compare", "brp=off"}, 2, "compare requires a BRP target"},
		{"cleared HandGrip", []string{"compare", "grip=off"}, 2, "compare requires a HandGrip target"},
		{"selection key", []string{"compare", "frame=surly-krampus"}, 2, `unknown query parameter "frame"`},
		{"bar roll", []string{"compare", "bar_roll=10"}, 2, `unknown query parameter "bar_roll"`},
		{"rail pitch", []string{"compare", "rail_pitch=-2"}, 2, `unknown query parameter "rail_pitch"`},
	}
	for _, tc := range cases {
		code, stdout, stderr := runCLI(tc.args...)
		if code != tc.code || stdout != "" || !strings.Contains(stderr, tc.wantErr) {
			t.Errorf("%s: exit %d, stdout %q, stderr %q; want %d, empty, containing %q", tc.name, code, stdout, stderr, tc.code, tc.wantErr)
		}
	}

	var errOut bytes.Buffer
	code := runWith(krampusXLFox34, []string{"compare"}, failingWriter{}, &errOut)
	if code != 1 || !strings.Contains(errOut.String(), "encode JSON") {
		t.Errorf("failing stdout: exit %d, stderr %q; want 1, containing \"encode JSON\"", code, errOut.String())
	}
}

// jsonBike mirrors the query.BikeResult fields the bike tests check.
type jsonBike struct {
	FrameID        string
	Size           string
	ForkID         string
	BarID          string
	PostID         string
	AllBarOptions  bool
	AllForkOptions bool
	Placement      struct {
		Setup struct {
			Seatpost struct {
				InsertionMm float64
			}
		}
		Frameset struct {
			HeadTubeTop jsonPose
		}
		Saddle struct {
			BRP jsonPose
		}
	}
	Score *struct {
		Tier  string
		Total float64
	}
	Fitting *json.RawMessage
	Builds  *struct {
		Evaluated int
		Builds    []struct {
			HandlebarID string
			ForkID      string
			Score       struct {
				Tier  string
				Total float64
			}
		}
	}
}

func decodeBike(t *testing.T, stdout string) jsonBike {
	t.Helper()
	var r jsonBike
	if err := json.Unmarshal([]byte(stdout), &r); err != nil {
		t.Fatalf("decode JSON: %v\n%s", err, stdout)
	}
	return r
}

func TestRunBikeDefaultAndWinningBuildJSON(t *testing.T) {
	code, stdout, stderr := runCLI("bike")
	if code != 0 || stderr != "" {
		t.Fatalf("bike: exit %d, stderr %q; want 0, empty", code, stderr)
	}
	r := decodeBike(t, stdout)
	if r.FrameID != "otso-fenrir-stainless" || r.Size != "L" || r.ForkID != "lithic_carbon_mountain-495x44" ||
		r.PostID != "redshift_shockstop_pro" || r.BarID != "persuader20_800" {
		t.Errorf("IDs = %s %s %s %s %s", r.FrameID, r.Size, r.ForkID, r.PostID, r.BarID)
	}
	if r.Placement.Setup.Seatpost.InsertionMm != 175 {
		t.Errorf("InsertionMm = %v, want 175", r.Placement.Setup.Seatpost.InsertionMm)
	}
	if r.Score == nil || r.Score.Tier != "fits_neither" {
		t.Fatalf("Score = %+v, want fits_neither", r.Score)
	}
	assertNear(t, "Total", r.Score.Total, 53.545302, 1e-5)
	if r.Builds == nil || r.Builds.Evaluated != 749 || len(r.Builds.Builds) != 5 {
		t.Fatalf("Builds = %+v, want 749 evaluated, 5 builds", r.Builds)
	}
	if r.Placement.Frameset.HeadTubeTop.Origin == (pose.Point{}) || r.Placement.Saddle.BRP.Origin == (pose.Point{}) {
		t.Errorf("HeadTubeTop %+v / BRP %+v must be non-zero", r.Placement.Frameset.HeadTubeTop, r.Placement.Saddle.BRP)
	}

	const winning = "frame=surly-krampus&size=XL&fork=fox34_120-534x44&sag=20&post=redshift_shockstop_pro&post_insert=163.13014276565696&rail_offset=15&rail_pitch=0&bar=persuader20_800&bar_roll=0&stem_length=110&stem_angle=-7&spacers=0"
	code, stdout, stderr = runCLI("bike", "?"+winning)
	if code != 0 || stderr != "" {
		t.Fatalf("winning: exit %d, stderr %q; want 0, empty", code, stderr)
	}
	r = decodeBike(t, stdout)
	if r.Score == nil || r.Score.Tier != "fits_all" || r.Builds == nil || len(r.Builds.Builds) == 0 {
		t.Fatalf("winning = %+v", r)
	}
	assertNear(t, "winning Total", r.Score.Total, 84.690417237, 1e-6)
	assertNear(t, "winning top build Total", r.Builds.Builds[0].Score.Total, 84.690417237, 1e-6)
	code2, stdout2, stderr2 := runCLI("bike", winning)
	if code2 != 0 || stderr2 != "" || stdout2 != stdout {
		t.Errorf("without ?: exit %d, stderr %q, identical stdout %v; want 0, empty, true", code2, stderr2, stdout2 == stdout)
	}

	code, stdout, stderr = runCLI("bike", "brp=off&grip=off")
	if code != 0 || stderr != "" {
		t.Fatalf("targets off: exit %d, stderr %q", code, stderr)
	}
	r = decodeBike(t, stdout)
	if r.Score != nil || r.Fitting != nil || r.Builds != nil {
		t.Errorf("targets off: Score %v, Fitting %v, Builds %v; want all nil", r.Score, r.Fitting, r.Builds)
	}

	code, stdout, stderr = runCLI("bike", "stem_length=65&stem_angle=-6")
	if code != 0 || stderr != "" {
		t.Fatalf("off-catalog stem: exit %d, stderr %q", code, stderr)
	}
	r = decodeBike(t, stdout)
	if r.Score != nil || r.Builds == nil {
		t.Errorf("off-catalog stem: Score %v, Builds %v; want nil, non-nil", r.Score, r.Builds)
	}
}

func TestRunBikeHelpAndErrors(t *testing.T) {
	code, stdout, stderr := runCLI("--help")
	if code != 0 || stderr != "" || !strings.Contains(stdout, "serve") || !strings.Contains(stdout, "compare") || !strings.Contains(stdout, "bike") {
		t.Errorf("--help: exit %d, stderr %q, usage %q", code, stderr, stdout)
	}
	code, stdout, stderr = runCLI("bike", "--help")
	if code != 0 || stderr != "" || !strings.Contains(stdout, "bikegeo bike [query]") {
		t.Errorf("bike --help: exit %d, stderr %q, usage %q", code, stderr, stdout)
	}

	cases := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{"too many args", []string{"bike", "a=1", "b=2"}, "Usage: bikegeo bike [query]"},
		{"bad URL escape", []string{"bike", "frame=%zz"}, "invalid query"},
		{"semicolon query", []string{"bike", "frame=surly-krampus;size=XL"}, "invalid query"},
		{"unknown query key", []string{"bike", "fram=surly-krampus"}, "unknown query parameter"},
		{"retired key", []string{"bike", "post_ext=200"}, "unknown query parameter"},
		{"unknown frame", []string{"bike", "frame=nope"}, "unknown frame"},
		{"bare multi-variant fork", []string{"bike", "fork=fox34_120"}, "has multiple variants"},
		{"out-of-bounds insertion", []string{"bike", "post=redshift_shockstop_pro&post_insert=275"}, "insertion"},
		{"half target", []string{"bike", "brp_x=-219"}, "requires both"},
	}
	for _, tc := range cases {
		code, stdout, stderr := runCLI(tc.args...)
		if code != 2 || stdout != "" || !strings.Contains(stderr, tc.wantErr) {
			t.Errorf("%s: exit %d, stdout %q, stderr %q; want 2, empty, containing %q", tc.name, code, stdout, stderr, tc.wantErr)
		}
	}

	var errOut bytes.Buffer
	code = run([]string{"bike"}, failingWriter{}, &errOut)
	if code != 1 || !strings.Contains(errOut.String(), "encode JSON") {
		t.Errorf("failing stdout: exit %d, stderr %q; want 1, containing \"encode JSON\"", code, errOut.String())
	}
}
