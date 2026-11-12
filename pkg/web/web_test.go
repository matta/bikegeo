package web

import (
	"bufio"
	"bytes"
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"bikegeo/pkg/frames"

	"github.com/PuerkitoBio/goquery"
)

const twoBikes = "/?layout=rows&col=otso-fenrir-stainless:L@110mm@20%25&col=esker-hayduke:S3@110mm@20%25"

// do sends a loopback GET through the real handler, no network involved.
func do(t *testing.T, target string) (*httptest.ResponseRecorder, *goquery.Document) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	req.Host = "127.0.0.1:8080"
	rec := httptest.NewRecorder()
	New().ServeHTTP(rec, req)
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(rec.Body.String()))
	if err != nil {
		t.Fatalf("parse %s: %v", target, err)
	}
	return rec, doc
}

func mustStatus(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rec.Code != want {
		t.Fatalf("status = %d, want %d", rec.Code, want)
	}
}

func text(s *goquery.Selection) string {
	return strings.Join(strings.Fields(s.Text()), " ")
}

func TestCompare_DefaultRendersFullMatrix(t *testing.T) {
	rec, doc := do(t, "/")
	mustStatus(t, rec, http.StatusOK)

	if got := doc.Find(`table[data-table="fit-cockpit"] tbody tr[data-frame-id]`).Length(); got != 20 {
		t.Errorf("fit rows = %d, want 20", got)
	}
	if got := doc.Find(`table[data-table="chassis"] tbody tr[data-frame-id]`).Length(); got != 20 {
		t.Errorf("chassis rows = %d, want 20", got)
	}
	if doc.Find(`table[data-table="breakdown"]`).Length() != 1 {
		t.Error("score breakdown table missing")
	}
	if got := doc.Find(`form[method="get"]`).AttrOr("action", ""); got != "/#results" {
		t.Errorf("form action = %q", got)
	}
	if doc.Find(`button[type="submit"]`).Length() == 0 {
		t.Error("missing submit button")
	}
	if got := doc.Find(`a[href="/"]`).FilterFunction(func(_ int, s *goquery.Selection) bool {
		return strings.Contains(text(s), "Reset")
	}).Length(); got != 1 {
		t.Errorf("reset links = %d, want 1", got)
	}
	// Ideal / Acceptable reference rows live in the (sticky) thead.
	if got := doc.Find(`table[data-table="fit-cockpit"] thead tr.ref`).Length(); got != 2 {
		t.Errorf("reference rows in thead = %d, want 2", got)
	}
	if doc.Find("script").Length() != 0 {
		t.Error("page must not contain scripts")
	}
}

func TestCompare_ColumnSpecsAndFormEcho(t *testing.T) {
	rec, doc := do(t, twoBikes)
	mustStatus(t, rec, http.StatusOK)

	rows := doc.Find(`table[data-table="fit-cockpit"] tbody tr[data-frame-id]`)
	if rows.Length() != 2 {
		t.Fatalf("rows = %d, want 2", rows.Length())
	}
	ids := map[string]bool{}
	rows.Each(func(_ int, s *goquery.Selection) { ids[s.AttrOr("data-frame-id", "")] = true })
	if !ids["otso-fenrir-stainless"] || !ids["esker-hayduke"] {
		t.Errorf("frame ids = %v", ids)
	}

	// The editor echoes each resolved column as selected controls.
	editor := doc.Find(`table.editor tbody tr[data-col]`)
	if editor.Length() != 2 {
		t.Fatalf("editor rows = %d, want 2", editor.Length())
	}
	row := doc.Find(`table.editor tr[data-frame-id="esker-hayduke"]`)
	if got := row.Find(`select[name="size"] option[selected]`).AttrOr("value", ""); got != "S3" {
		t.Errorf("size echo = %q, want S3", got)
	}
	if got := row.Find(`select[name="fork"] option[selected]`).AttrOr("value", ""); got != "110mm" {
		t.Errorf("fork echo = %q, want 110mm", got)
	}
	if got := row.Find(`select[name="sag"] option[selected]`).AttrOr("value", ""); got != "20" {
		t.Errorf("sag echo = %q, want 20", got)
	}
	if got := text(doc.Find("p.cli code")); !strings.Contains(got, "bikegeo compare") || !strings.Contains(got, "esker-hayduke:S3@110mm@20%") {
		t.Errorf("CLI equivalent = %q", got)
	}
}

func TestCompare_SettingsEcho(t *testing.T) {
	rec, doc := do(t, "/?all=1&top=5&top_per_frame=2&saddle=berthoud-aspin-open&layout=rows&score=0")
	mustStatus(t, rec, http.StatusOK)

	if got := doc.Find(`input[name="top"]`).AttrOr("value", ""); got != "5" {
		t.Errorf("top = %q", got)
	}
	if got := doc.Find(`input[name="top_per_frame"]`).AttrOr("value", ""); got != "2" {
		t.Errorf("top_per_frame = %q", got)
	}
	if _, ok := doc.Find(`input[name="all"]`).Attr("checked"); !ok {
		t.Error("all checkbox not checked")
	}
	for name, want := range map[string]string{"saddle": "berthoud-aspin-open", "layout": "rows", "score": "0"} {
		if got := doc.Find(`select[name="`+name+`"] option[selected]`).AttrOr("value", ""); got != want {
			t.Errorf("%s echo = %q, want %q", name, got, want)
		}
	}
	if got := doc.Find(`table[data-table="matrix"] tbody tr[data-frame-id], table[data-table="fit-cockpit"] tbody tr[data-frame-id]`).Length(); got != 5 {
		t.Errorf("data rows = %d, want 5 (top=5)", got)
	}
	if doc.Find(`table[data-table="breakdown"]`).Length() != 0 {
		t.Error("score=0 must omit the breakdown")
	}
}

func TestCompare_AlignedFieldsIgnoreBlankAddRow(t *testing.T) {
	q := url.Values{
		"frame": {"otso-fenrir-stainless", "esker-hayduke", ""},
		"size":  {"L", "S3", ""},
		"fork":  {"110mm", "rigid", ""},
		"sag":   {"20", "", ""},
	}
	q.Set("layout", "rows")
	rec, doc := do(t, "/?"+q.Encode())
	mustStatus(t, rec, http.StatusOK)
	if got := doc.Find(`table[data-table="fit-cockpit"] tbody tr[data-frame-id]`).Length(); got != 2 {
		t.Errorf("rows = %d, want 2", got)
	}
}

// Submitting the form built from any page must reproduce the same results:
// the URL really is the single source of truth.
func TestCompare_FormRoundTripReproducesDefaultView(t *testing.T) {
	_, first := do(t, "/")

	v := url.Values{}
	first.Find(`table.editor tbody tr[data-col]`).Each(func(_ int, s *goquery.Selection) {
		v.Add("frame", s.Find(`input[name="frame"]`).AttrOr("value", ""))
		v.Add("size", s.Find(`select[name="size"] option[selected]`).AttrOr("value", ""))
		v.Add("fork", s.Find(`select[name="fork"] option[selected]`).AttrOr("value", ""))
		v.Add("sag", s.Find(`select[name="sag"] option[selected]`).AttrOr("value", ""))
	})
	v.Add("frame", "") // the untouched "add a frame" row
	v.Add("size", "")
	v.Add("fork", "")
	v.Add("sag", "")

	rec, second := do(t, "/?"+v.Encode())
	mustStatus(t, rec, http.StatusOK)

	cells := func(d *goquery.Document) []string {
		var out []string
		d.Find(`table[data-table="fit-cockpit"] tbody tr`).Each(func(_ int, r *goquery.Selection) {
			// Skip the fork label: an exact a2c= token may phrase it differently.
			var parts []string
			r.Children().Each(func(i int, c *goquery.Selection) {
				switch i {
				case 1: // disambiguating "@ fork" suffix may be phrased differently
					parts = append(parts, strings.SplitN(text(c), " @ ", 2)[0])
				case 3:
				default:
					parts = append(parts, text(c))
				}
			})
			out = append(out, strings.Join(parts, " | "))
		})
		return out
	}
	a, b := cells(first), cells(second)
	if len(a) != len(b) {
		t.Fatalf("row counts differ: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Errorf("row %d differs:\n default: %s\n  round-trip: %s", i+1, a[i], b[i])
		}
	}
}

func TestCompare_AllFramesCheckboxFromDefaultForm(t *testing.T) {
	_, first := do(t, "/")

	// Simulate checking "All frames, best configuration each" on the default page
	// and clicking Calculate / Update, which submits all 20 default column fields
	// alongside all=1.
	v := url.Values{}
	first.Find(`table.editor tbody tr[data-col]`).Each(func(_ int, s *goquery.Selection) {
		v.Add("frame", s.Find(`input[name="frame"]`).AttrOr("value", ""))
		v.Add("size", s.Find(`select[name="size"] option[selected]`).AttrOr("value", ""))
		v.Add("fork", s.Find(`select[name="fork"] option[selected]`).AttrOr("value", ""))
		v.Add("sag", s.Find(`select[name="sag"] option[selected]`).AttrOr("value", ""))
	})
	v.Set("all", "1")

	rec, doc := do(t, "/?"+v.Encode())
	mustStatus(t, rec, http.StatusOK)

	seen := map[string]bool{}
	doc.Find(`table.editor tbody tr[data-col]`).Each(func(_ int, s *goquery.Selection) {
		if id := s.AttrOr("data-frame-id", ""); id != "" {
			seen[id] = true
		}
	})
	for _, f := range frames.AllFrames() {
		if !seen[f.ID] {
			t.Errorf("frame %q missing from all-frames comparison", f.ID)
		}
	}
	for _, id := range []string{"seven-expedition-926d", "seven-expedition-926f", "seven-custom"} {
		if !seen[id] {
			t.Errorf("Seven Cycles frame %q missing from all-frames comparison", id)
		}
	}
	if got := doc.Find(`table[data-table="fit-cockpit"] tbody tr[data-frame-id]`).Length(); got != len(frames.AllFrames()) {
		t.Errorf("fit-cockpit rows = %d, want %d", got, len(frames.AllFrames()))
	}
	// Clicking Remove on a row from the all-frames view should drop all=1 and remove that row.
	removeHref, ok := doc.Find(`table.editor tbody tr[data-col] a[href]`).First().Attr("href")
	if !ok || strings.Contains(removeHref, "all=1") {
		t.Errorf("RemoveHref = %q (ok=%v), want no all=1", removeHref, ok)
	}
}

func TestCompare_CellsCarryMetricAndZone(t *testing.T) {
	_, doc := do(t, twoBikes)

	cells := doc.Find(`table[data-table="fit-cockpit"] tbody tr[data-frame-id="otso-fenrir-stainless"] td[data-metric="brp_setback"]`)
	if cells.Length() != 1 {
		t.Fatalf("brp_setback cells = %d, want 1", cells.Length())
	}
	zone := cells.AttrOr("data-zone", "")
	if zone == "" || !cells.HasClass("zone-"+zone) {
		t.Errorf("zone attr %q does not match class %q", zone, cells.AttrOr("class", ""))
	}
	if !cells.HasClass("num") {
		t.Error("numeric metric cells need the num class for tabular alignment")
	}

	zones := map[string]int{}
	doc.Find(`table[data-table="fit-cockpit"] tbody td[data-zone]`).Each(func(_ int, s *goquery.Selection) {
		zones[s.AttrOr("data-zone", "")]++
	})
	if len(zones) < 2 {
		t.Errorf("expected several distinct zones across two bikes, got %v", zones)
	}

	// Metric headers link to the glossary entry.
	href := doc.Find(`table[data-table="fit-cockpit"] thead th[data-column="brp_setback"] a`).AttrOr("href", "")
	if href != "/help#metric-brp_setback" {
		t.Errorf("metric header link = %q", href)
	}
	// Frame cells link to validation.
	link := doc.Find(`table[data-table="fit-cockpit"] tbody tr[data-frame-id="esker-hayduke"] th a`).AttrOr("href", "")
	if link != "/validate?frame=esker-hayduke" {
		t.Errorf("frame link = %q", link)
	}
}

func TestCompare_ColumnsLayoutTagsCellsByFrame(t *testing.T) {
	rec, doc := do(t, strings.Replace(twoBikes, "layout=rows", "layout=columns", 1))
	mustStatus(t, rec, http.StatusOK)

	cell := doc.Find(`table[data-table="matrix"] td[data-frame-id="otso-fenrir-stainless"][data-metric="brp_setback"]`)
	if cell.Length() != 1 {
		t.Fatalf("matrix cells for fenrir/brp_setback = %d, want 1", cell.Length())
	}
	if cell.AttrOr("data-zone", "") == "" {
		t.Error("matrix cell lacks zone")
	}
	if got := doc.Find(`table[data-table="matrix"] thead th[data-frame-id]`).Length(); got != 2 {
		t.Errorf("bike column headers = %d, want 2", got)
	}
}

func TestCompare_InvalidInputGives400WithBanner(t *testing.T) {
	tests := []struct {
		name, target, field string
	}{
		{"unknown frame", "/?col=nosuchframe", "col"},
		{"sag not allowed", "/?col=otso-fenrir-stainless@17%25", "col"},
		{"unsupported travel", "/?col=otso-fenrir-stainless@300mm", "col"},
		{"unknown saddle", "/?saddle=nope", "col"},
		{"bad top", "/?top=abc", "top"},
		{"bad layout", "/?layout=zigzag", "layout"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec, doc := do(t, tc.target)
			mustStatus(t, rec, http.StatusBadRequest)
			if doc.Find(`#errors[role="alert"]`).Length() != 1 {
				t.Error("missing alert banner")
			}
			if text(doc.Find("#errors")) == "" {
				t.Error("banner is empty")
			}
			if doc.Find("form.controls").Length() != 1 {
				t.Error("form must still render so the user can correct it")
			}
		})
	}
}

func TestCompare_BadSpecIsEchoedForCorrection(t *testing.T) {
	_, doc := do(t, "/?col=nosuchframe&col=otso-fenrir-stainless")
	inputs := doc.Find(`input[name="col"]`)
	if inputs.Length() != 2 {
		t.Fatalf("raw spec inputs = %d, want 2", inputs.Length())
	}
	if got := inputs.First().AttrOr("value", ""); got != "nosuchframe" {
		t.Errorf("echoed spec = %q", got)
	}
	if doc.Find(".field-error").Length() == 0 {
		t.Error("expected an inline field error beside the specs")
	}
}

func TestCompare_FieldErrorBesideInput(t *testing.T) {
	rec, doc := do(t, "/?top=abc")
	mustStatus(t, rec, http.StatusBadRequest)
	if got := text(doc.Find(".field-error")); !strings.Contains(got, `"abc"`) {
		t.Errorf("field error = %q", got)
	}
	// Valid parts of the request still render.
	if doc.Find(`table[data-table="fit-cockpit"]`).Length() != 1 {
		t.Error("results should still render for the valid parts of the query")
	}
}

func TestPages_AllRoutesRender(t *testing.T) {
	targets := []string{
		"/", "/frames", "/frames?q=fenrir&all-sizes=1", "/forks", "/stems",
		"/ideals", "/ideals?name=idmatch", "/saddles", "/saddles?id=berthoud-aspin-open",
		"/saddle-swap", "/saddle-swap?from=sqlab-610-ergolux&to=berthoud-aspin-open",
		"/handlebars", "/handlebars?id=persuader20_800",
		"/validate", "/validate?frame=binary-havok", "/help",
	}
	for _, target := range targets {
		t.Run(target, func(t *testing.T) {
			rec, doc := do(t, target)
			mustStatus(t, rec, http.StatusOK)
			if doc.Find("table[data-table]").Length() == 0 {
				t.Error("page has no table")
			}
			if doc.Find("nav a").Length() < 10 {
				t.Error("navigation missing")
			}
			if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
				t.Errorf("content type = %q", ct)
			}
		})
	}
}

func TestPages_NavigationLinksAllResolve(t *testing.T) {
	_, home := do(t, "/")
	var paths []string
	home.Find("header.site nav a").Each(func(_ int, a *goquery.Selection) {
		paths = append(paths, a.AttrOr("href", ""))
	})
	if len(paths) != len(nav) {
		t.Fatalf("nav links = %d, want %d", len(paths), len(nav))
	}
	for _, p := range paths {
		rec, _ := do(t, p)
		if rec.Code != http.StatusOK {
			t.Errorf("nav link %s -> %d", p, rec.Code)
		}
	}
}

func TestPages_FormsEchoAndLink(t *testing.T) {
	_, doc := do(t, "/frames?q=fenrir&size=L")
	if got := doc.Find(`input[name="q"]`).AttrOr("value", ""); got != "fenrir" {
		t.Errorf("q echo = %q", got)
	}
	if got := doc.Find(`input[name="size"]`).AttrOr("value", ""); got != "L" {
		t.Errorf("size echo = %q", got)
	}
	if href := doc.Find(`table[data-table] tbody td a`).First().AttrOr("href", ""); !strings.HasPrefix(href, "/validate?frame=") {
		t.Errorf("frame ID link = %q", href)
	}

	_, doc = do(t, "/validate?frame=binary-havok")
	if got := doc.Find(`select[name="frame"] option[selected]`).AttrOr("value", ""); got != "binary-havok" {
		t.Errorf("frame echo = %q", got)
	}
}

func TestPages_FailingCommandIs400WithMessage(t *testing.T) {
	rec, doc := do(t, "/validate?frame=nosuchframe")
	mustStatus(t, rec, http.StatusBadRequest)
	if got := text(doc.Find(".banner")); !strings.Contains(got, "not found") {
		t.Errorf("banner = %q", got)
	}
}

func TestHelp_GlossaryAnchorsMatchHeaderLinks(t *testing.T) {
	_, help := do(t, "/help")
	if help.Find(`tr#metric-brp_setback`).Length() != 1 {
		t.Error("glossary lacks brp_setback anchor targeted by metric header links")
	}
	if help.Find(`#specs`).Length() != 1 {
		t.Error("spec grammar section missing")
	}
}

func TestServer_RejectsNonLoopbackHost(t *testing.T) {
	for host, want := range map[string]int{
		"127.0.0.1:8080": http.StatusOK,
		"localhost:9":    http.StatusOK,
		"[::1]:8080":     http.StatusOK,
		"example.com":    http.StatusForbidden,
		"evil.test:8080": http.StatusForbidden,
	} {
		req := httptest.NewRequest(http.MethodGet, "/forks", nil)
		req.Host = host
		rec := httptest.NewRecorder()
		New().ServeHTTP(rec, req)
		if rec.Code != want {
			t.Errorf("Host %q -> %d, want %d", host, rec.Code, want)
		}
	}
}

func TestServer_OnlyGETAndStaticAssets(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("x=1"))
	req.Host = "127.0.0.1"
	rec := httptest.NewRecorder()
	New().ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST / -> %d, want 405", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/static/style.css", nil)
	req.Host = "127.0.0.1"
	rec = httptest.NewRecorder()
	New().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/css") {
		t.Errorf("style.css -> %d %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	if !strings.Contains(rec.Body.String(), "tabular-nums") || !strings.Contains(rec.Body.String(), "position: sticky") {
		t.Error("stylesheet lacks tabular-nums / sticky header rules")
	}

	rec, _ = do(t, "/nope")
	if rec.Code != http.StatusNotFound {
		t.Errorf("unknown path -> %d, want 404", rec.Code)
	}
}

func TestPages_ValidateLinksFrameIDs(t *testing.T) {
	rec, doc := do(t, "/validate")
	mustStatus(t, rec, http.StatusOK)
	href := doc.Find(`table[data-table] tbody td a`).First().AttrOr("href", "")
	if !strings.HasPrefix(href, "/validate?frame=") {
		t.Errorf("/validate Frame ID link = %q, want /validate?frame=...", href)
	}
}

func TestPages_SaddleSwapFromOnlyKeepsSourceSaddle(t *testing.T) {
	rec, doc := do(t, "/saddle-swap?from=selle-italia-slr-boost-superflow-l3")
	mustStatus(t, rec, http.StatusOK)
	if got, _ := doc.Find(`select[name="from"] option[selected]`).Attr("value"); got != "selle-italia-slr-boost-superflow-l3" {
		t.Errorf("from select = %q, want the chosen source saddle", got)
	}
	if doc.Find("table[data-table]").Length() == 0 {
		t.Error("expected swap tables")
	}
}

func TestHelp_HandlebarClampFitShowsCount(t *testing.T) {
	_, help := do(t, "/help")
	row := help.Find(`tr#metric-handlebar_clamp_fit`)
	if row.Length() != 1 {
		t.Fatal("missing handlebar_clamp_fit row")
	}
	rowText := text(row)
	if !strings.Contains(rowText, "≥4") || !strings.Contains(rowText, "1–3") {
		t.Errorf("handlebar_clamp_fit row = %q, want ≥4 and 1–3", rowText)
	}
}

type pipeListener struct {
	conns  chan net.Conn
	closed chan struct{}
}

func (l *pipeListener) Accept() (net.Conn, error) {
	select {
	case c := <-l.conns:
		return c, nil
	case <-l.closed:
		return nil, net.ErrClosed
	}
}

func (l *pipeListener) Close() error {
	select {
	case <-l.closed:
	default:
		close(l.closed)
	}
	return nil
}

func (l *pipeListener) Addr() net.Addr {
	return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8080}
}

func TestRunServe_FlagValidationAndShutdown(t *testing.T) {
	for _, args := range [][]string{
		{"--port", "-1"},
		{"--port", "70000"},
		{"--unknown-flag"},
	} {
		var out, errOut bytes.Buffer
		if code := RunServe(args, &out, &errOut); code != 2 {
			t.Errorf("RunServe(%v) = %d, want 2", args, code)
		}
	}

	ln := &pipeListener{conns: make(chan net.Conn, 1), closed: make(chan struct{})}
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- Serve(ctx, ln)
	}()

	client, serverConn := net.Pipe()
	ln.conns <- serverConn

	req, _ := http.NewRequest(http.MethodGet, "http://127.0.0.1:8080/forks", nil)
	if err := req.Write(client); err != nil {
		cancel()
		t.Fatalf("write request: %v", err)
	}
	resp, err := http.ReadResponse(bufio.NewReader(client), req)
	if err != nil {
		cancel()
		t.Fatalf("read response: %v", err)
	}
	resp.Body.Close()
	client.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /forks status = %d, want 200", resp.StatusCode)
	}

	cancel()
	if err := <-errCh; err != nil {
		t.Errorf("Serve returned %v, want nil", err)
	}
}

func TestPages_SaddleHeightParameterReachesTheEngine(t *testing.T) {
	rec, doc := do(t, "/saddles?saddle-height=760")
	mustStatus(t, rec, http.StatusOK)
	if !strings.Contains(doc.Text(), "760mm SH") {
		t.Error("saddle-height=760 should appear in the translation footers")
	}
	if got, _ := doc.Find(`input[name="saddle-height"]`).Attr("value"); got != "760" {
		t.Errorf("saddle-height control = %q, want 760", got)
	}
}

func TestPages_SaddleSwapAllComparesEverySaddle(t *testing.T) {
	rec, doc := do(t, "/saddle-swap?all=1")
	mustStatus(t, rec, http.StatusOK)
	if doc.Find("table[data-table]").Length() < 2 {
		t.Error("--all equivalent should render a summary plus per-saddle tables")
	}
}

func TestPages_HandlebarControlsChangeTheResult(t *testing.T) {
	_, base := do(t, "/handlebars?id=persuader20_800")
	rec, rolled := do(t, "/handlebars?id=persuader20_800&roll=8&grip-inset=80&width=760")
	mustStatus(t, rec, http.StatusOK)
	if base.Find("table[data-table]").Text() == rolled.Find("table[data-table]").Text() {
		t.Error("roll, grip-inset and width should change the detail tables")
	}
}

func TestPages_NonNumericParameterIs400WithBanner(t *testing.T) {
	rec, doc := do(t, "/saddles?saddle-height=tall")
	mustStatus(t, rec, http.StatusBadRequest)
	if !strings.Contains(text(doc.Find(".banner")), "saddle-height") {
		t.Errorf("banner = %q", text(doc.Find(".banner")))
	}
}

func TestPages_UnknownIDsAre400(t *testing.T) {
	for _, target := range []string{"/saddles?id=nope", "/handlebars?id=nope", "/ideals?name=nope"} {
		rec, _ := do(t, target)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status %d, want 400", target, rec.Code)
		}
	}
}

func TestPages_ForksAndStemsCatalogs(t *testing.T) {
	for _, target := range []string{"/forks", "/stems"} {
		rec, doc := do(t, target)
		mustStatus(t, rec, http.StatusOK)
		if doc.Find("table[data-table] tbody tr").Length() == 0 {
			t.Errorf("%s: no rows", target)
		}
	}
}
