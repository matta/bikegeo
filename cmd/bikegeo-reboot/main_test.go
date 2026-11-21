package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"

	"bikegeo/internal/reboot/pose"
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

func runCLI(args ...string) (code int, stdout, stderr string) {
	var out, errOut bytes.Buffer
	code = run(args, &out, &errOut)
	return code, out.String(), errOut.String()
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
	const q = "frame=surly-krampus&size=XL&fork=fox34_120&sag=20"
	code, stdout, stderr := runCLI("compare", q)
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d, stderr %q; want 0, empty", code, stderr)
	}
	r := decodeResult(t, stdout)
	if r.PostID != "redshift_shockstop_pro" || r.BarID != "persuader20_800" || r.AllBarOptions {
		t.Errorf("PostID %q, BarID %q, AllBarOptions %v", r.PostID, r.BarID, r.AllBarOptions)
	}
	if r.RowCount != 1 || r.Evaluated != 749 || len(r.Rows) != 1 {
		t.Fatalf("RowCount %d, Evaluated %d, %d rows; want 1, 749, 1", r.RowCount, r.Evaluated, len(r.Rows))
	}
	row := r.Rows[0]
	if row.FrameID != "surly-krampus" || row.Size != "XL" || row.ForkID != "fox34_120" || row.SagPct != 20 {
		t.Errorf("row = %s %s %s %v%%", row.FrameID, row.Size, row.ForkID, row.SagPct)
	}
	if row.HandlebarID != "persuader20_800" {
		t.Errorf("HandlebarID = %q, want persuader20_800", row.HandlebarID)
	}
	score := row.Build.Score
	if score.Tier != "fits_all" {
		t.Errorf("Tier = %q, want fits_all", score.Tier)
	}
	assertNear(t, "Total", score.Total, 84.575011405, 1e-6)
	assertNear(t, "GripR", score.GripR, 0.759424168, 1e-6)
	if len(score.Attributes) != 12 {
		t.Errorf("got %d attributes, want 12", len(score.Attributes))
	}
	stem := row.Build.Placement.Setup.Stem
	if stem.LengthMm != 100 || stem.AngleDeg != -10 || stem.SpacersMm != 0 {
		t.Errorf("stem (%v, %v, %v), want (100, -10, 0)", stem.LengthMm, stem.AngleDeg, stem.SpacersMm)
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
	code, stdout, stderr := runCLI("compare", "frame=surly-krampus&size=XL&fork=fox34_120&sag=20&bar_options=all")
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d, stderr %q; want 0, empty", code, stderr)
	}
	r := decodeResult(t, stdout)
	if !r.AllBarOptions || r.RowCount != 1 || r.Evaluated != 11235 || len(r.Rows) != 1 {
		t.Fatalf("AllBarOptions %v, RowCount %d, Evaluated %d, %d rows; want true, 1, 11235, 1", r.AllBarOptions, r.RowCount, r.Evaluated, len(r.Rows))
	}
	if r.Rows[0].HandlebarID != "sqlab311_flx_16_30" {
		t.Errorf("HandlebarID = %q, want sqlab311_flx_16_30", r.Rows[0].HandlebarID)
	}
	assertNear(t, "Total", r.Rows[0].Build.Score.Total, 86.650135345, 1e-6)
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
	if !strings.Contains(stdout, "bikegeo-reboot compare [query]") {
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
		{"too many args", []string{"compare", "a=1", "b=2"}, 2, "Usage: bikegeo-reboot compare [query]"},
		{"bad URL escape", []string{"compare", "frame=%zz"}, 2, "invalid query"},
		{"semicolon query", []string{"compare", "frame=surly-krampus;size=XL"}, 2, "invalid query"},
		{"unknown query key", []string{"compare", "bar_option=all"}, 2, "unknown query parameter"},
		{"cleared BRP", []string{"compare", "brp=off"}, 2, "compare requires a BRP target"},
		{"cleared HandGrip", []string{"compare", "grip=off"}, 2, "compare requires a HandGrip target"},
		{"unknown frame", []string{"compare", "frame=nope"}, 2, "unknown frame"},
	}
	for _, tc := range cases {
		code, stdout, stderr := runCLI(tc.args...)
		if code != tc.code || stdout != "" || !strings.Contains(stderr, tc.wantErr) {
			t.Errorf("%s: exit %d, stdout %q, stderr %q; want %d, empty, containing %q", tc.name, code, stdout, stderr, tc.code, tc.wantErr)
		}
	}

	var errOut bytes.Buffer
	code := run([]string{"compare", "frame=surly-krampus&size=XL&fork=fox34_120&sag=20"}, failingWriter{}, &errOut)
	if code != 1 || !strings.Contains(errOut.String(), "encode JSON") {
		t.Errorf("failing stdout: exit %d, stderr %q; want 1, containing \"encode JSON\"", code, errOut.String())
	}
}
