package cli

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/AmbientCrypto/legion-controller/api"
)

func logLine(id string, sec int, src api.RentalLogSource, lvl api.LogLevel, msg string) api.RentalLogLine {
	return api.RentalLogLine{Id: id, Time: time.Date(2026, 10, 26, 11, 0, sec, 0, time.UTC), Source: src, Level: lvl, Message: msg, RentalId: uid(1)}
}

func logPage(tail string, lines ...api.RentalLogLine) api.RentalLogPage {
	if lines == nil {
		lines = []api.RentalLogLine{}
	}
	return api.RentalLogPage{Available: true, Lines: lines, TailCursor: tail, RetentionDays: 14}
}

// fakeTimer stands in for the follow loop's timer: it records each wait and
// fires at once, and after stop waits it cancels the run (Ctrl-C).
type fakeTimer struct {
	mu     sync.Mutex
	waits  []time.Duration
	stop   int
	cancel context.CancelFunc
}

func (f *fakeTimer) after(d time.Duration) <-chan time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.waits = append(f.waits, d)
	if len(f.waits) >= f.stop {
		f.cancel()
		return nil // never fires; the cancelled context ends the loop
	}
	ch := make(chan time.Time, 1)
	ch <- time.Time{}
	return ch
}

func (h *harness) follow(stop int) *fakeTimer {
	ctx, cancel := context.WithCancel(context.Background())
	h.t.Cleanup(cancel)
	h.ctx = ctx
	ft := &fakeTimer{stop: stop, cancel: cancel}
	h.after = ft.after
	return ft
}

// logsRoute answers the logs route with one response per call, in order;
// the last one repeats.
func (h *harness) logsRoute(route string, steps ...http.HandlerFunc) {
	var mu sync.Mutex
	n := 0
	h.on(route, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		i := min(n, len(steps)-1)
		n++
		mu.Unlock()
		steps[i](w, r)
	})
}

func pageStep(p api.RentalLogPage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, p) }
}

func failStep(status int, code string, retryAfter string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if retryAfter != "" {
			w.Header().Set("Retry-After", retryAfter)
		}
		problem(w, status, code, code)
	}
}

func (h *harness) logsCalls(path string) []call {
	h.mu.Lock()
	defer h.mu.Unlock()
	var out []call
	for _, c := range h.calls {
		if c.Method == "GET" && c.Path == path {
			out = append(out, c)
		}
	}
	return out
}

const rentalLogsPath = "/v1/rentals/11000000-0000-0000-0000-000000000001/logs"

func (h *harness) rentalForLogs() {
	h.reply("GET /v1/rentals", 200, api.RentalPage{Items: []api.Rental{rentalFixture(1, "quiet-river-7k2f", api.RentalStateRunning)}})
}

func TestRentalLogsMapsFlagsToQuery(t *testing.T) {
	h := newHarness(t)
	h.rentalForLogs()
	h.reply("GET "+rentalLogsPath, 200, logPage("t1",
		logLine("a", 1, api.RentalLogSourceStdout, api.LogLevelInfo, "listening on :8000"),
		logLine("b", 2, api.RentalLogSourceLegion, api.LogLevelError, "container exited with code 137")))
	out, _ := h.mustRun("rental", "logs", "quiet-river-7k2f", "--since", "2h", "--source", "stdout,stderr", "--source", "legion",
		"--level", "warn", "--grep", "out of memory", "--limit", "50")
	q := h.last("GET", rentalLogsPath).Query
	want := h.now.Add(-2 * time.Hour).Format(time.RFC3339)
	if got := q["start"]; len(got) != 1 || !sameInstant(t, got[0], want) {
		t.Errorf("start = %v, want %s", got, want)
	}
	if got := strings.Join(q["source"], ","); got != "stdout,stderr,legion" {
		t.Errorf("source = %v (want three repeated values)", q["source"])
	}
	if qget(q, "level") != "warn" || qget(q, "q") != "out of memory" || qget(q, "limit") != "50" {
		t.Errorf("query = %v", q)
	}
	if _, ok := q["after"]; ok {
		t.Errorf("no follow, no after: %v", q)
	}
	if _, ok := q["end"]; ok {
		t.Errorf("end is left to the server: %v", q)
	}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("want 2 lines, oldest first:\n%s", out)
	}
	contains(t, "first", lines[0], "stdout", "INFO", "listening on :8000")
	contains(t, "second", lines[1], "legion", "ERROR", "container exited with code 137")
	excludes(t, "not a terminal", out, "\x1b[")
}

func qget(q map[string][]string, k string) string {
	if v := q[k]; len(v) > 0 {
		return v[0]
	}
	return ""
}

func (c call) qget(k string) string { return qget(c.Query, k) }

func sameInstant(t *testing.T, got, want string) bool {
	t.Helper()
	g, err1 := time.Parse(time.RFC3339, got)
	w, err2 := time.Parse(time.RFC3339, want)
	return err1 == nil && err2 == nil && g.Equal(w)
}

func TestRentalLogsDefaultsSendNothing(t *testing.T) {
	h := newHarness(t)
	h.rentalForLogs()
	h.reply("GET "+rentalLogsPath, 200, logPage("t1"))
	h.mustRun("rental", "logs", "quiet-river-7k2f")
	q := h.last("GET", rentalLogsPath).Query
	for _, k := range []string{"start", "end", "source", "level", "q", "limit", "after", "cursor"} {
		if _, ok := q[k]; ok {
			t.Errorf("%s sent without a flag: %v", k, q)
		}
	}
}

func TestRentalLogsAcceptsRentalID(t *testing.T) {
	h := newHarness(t)
	h.reply("GET /v1/rentals/"+uidS(1), 200, rentalFixture(1, "quiet-river-7k2f", api.RentalStateRunning))
	h.reply("GET "+rentalLogsPath, 200, logPage("t1"))
	h.mustRun("rental", "logs", uidS(1))
	if h.count("GET", rentalLogsPath) != 1 {
		t.Fatal("logs not read")
	}
}

func TestRentalLogsRejectsBadFlags(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"source", []string{"--source", "syslog"}},
		{"level", []string{"--level", "loud"}},
		{"since too long", []string{"--since", "15d"}},
		{"since garbage", []string{"--since", "yesterday"}},
		{"limit", []string{"--limit", "0"}},
		{"limit high", []string{"--limit", "1001"}},
		{"grep long", []string{"--grep", strings.Repeat("x", 201)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			h.rentalForLogs()
			h.reply("GET "+rentalLogsPath, 200, logPage("t1"))
			_, errOut, code := h.run(append([]string{"rental", "logs", "quiet-river-7k2f"}, tc.args...)...)
			if code != 2 {
				t.Fatalf("exit %d, want 2: %s", code, errOut)
			}
			if h.count("GET", rentalLogsPath) != 0 {
				t.Fatal("read logs despite a bad flag")
			}
			excludes(t, "copy", errOut, "\u2014", "\u2013")
		})
	}
}

func TestRentalLogsSinceDays(t *testing.T) {
	h := newHarness(t)
	h.rentalForLogs()
	h.reply("GET "+rentalLogsPath, 200, logPage("t1"))
	h.mustRun("rental", "logs", "quiet-river-7k2f", "--since", "3d")
	want := h.now.Add(-72 * time.Hour).Format(time.RFC3339)
	if got := h.last("GET", rentalLogsPath).qget("start"); !sameInstant(t, got, want) {
		t.Fatalf("start = %s, want %s", got, want)
	}
}

func TestRentalLogsNDJSON(t *testing.T) {
	for _, flag := range [][]string{{"--json"}, {"-o", "json"}} {
		h := newHarness(t)
		h.rentalForLogs()
		h.reply("GET "+rentalLogsPath, 200, logPage("t1",
			logLine("a", 1, api.RentalLogSourceStdout, api.LogLevelInfo, "one"),
			logLine("b", 2, api.RentalLogSourceStderr, api.LogLevelWarn, "two")))
		out, _ := h.mustRun(append([]string{"rental", "logs", "quiet-river-7k2f"}, flag...)...)
		lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
		if len(lines) != 2 {
			t.Fatalf("%v: want one object per line:\n%s", flag, out)
		}
		for i, l := range lines {
			var got api.RentalLogLine
			if err := json.Unmarshal([]byte(l), &got); err != nil {
				t.Fatalf("%v line %d not JSON: %v: %s", flag, i, err, l)
			}
			if got.Id != []string{"a", "b"}[i] || got.Message != []string{"one", "two"}[i] {
				t.Errorf("%v line %d = %+v", flag, i, got)
			}
		}
	}
}

func TestRentalLogsUnavailable(t *testing.T) {
	h := newHarness(t)
	h.rentalForLogs()
	p := logPage("t1")
	p.Available = false
	h.reply("GET "+rentalLogsPath, 200, p)
	out, errOut := h.mustRun("rental", "logs", "quiet-river-7k2f")
	if out != "" {
		t.Errorf("stdout should be empty: %q", out)
	}
	if strings.Count(errOut, "This host's agent cannot send logs yet.") != 1 {
		t.Errorf("stderr: %q", errOut)
	}
}

func TestRentalLogsFollowPollsWithTailCursorAndDedupes(t *testing.T) {
	h := newHarness(t)
	h.rentalForLogs()
	ft := h.follow(5)
	h.logsRoute("GET "+rentalLogsPath,
		pageStep(logPage("t1", logLine("a", 1, api.RentalLogSourceStdout, api.LogLevelInfo, "one"), logLine("b", 2, api.RentalLogSourceStdout, api.LogLevelInfo, "two"))),
		pageStep(logPage("t2", logLine("b", 2, api.RentalLogSourceStdout, api.LogLevelInfo, "two"), logLine("c", 3, api.RentalLogSourceStdout, api.LogLevelInfo, "three"))),
		failStep(429, "rate_limited", "7"),
		pageStep(logPage("t3", logLine("d", 4, api.RentalLogSourceStderr, api.LogLevelWarn, "four"))),
		pageStep(logPage("t3")),
	)
	out, errOut, code := h.run("rental", "logs", "quiet-river-7k2f", "-f", "--since", "30m", "--source", "stdout", "--source", "stderr")
	if code != 0 {
		t.Fatalf("Ctrl-C ends a follow cleanly, got exit %d: %s", code, errOut)
	}
	for _, m := range []string{"one", "two", "three", "four"} {
		if n := strings.Count(out, m); n != 1 {
			t.Errorf("%q printed %d times:\n%s", m, n, out)
		}
	}
	if strings.Index(out, "one") > strings.Index(out, "four") {
		t.Errorf("order:\n%s", out)
	}
	calls := h.logsCalls(rentalLogsPath)
	var afters []string
	for _, c := range calls {
		afters = append(afters, c.qget("after"))
		if got := strings.Join(c.Query["source"], ","); got != "stdout,stderr" {
			t.Errorf("every poll keeps the filters, got source %v", c.Query["source"])
		}
	}
	if got := strings.Join(afters, "|"); got != "|t1|t2|t2|t3" {
		t.Errorf("after cursors = %q", got)
	}
	if calls[0].qget("start") == "" || calls[1].qget("start") != "" {
		t.Errorf("start only on the first page: %v / %v", calls[0].Query, calls[1].Query)
	}
	wantWaits := []time.Duration{2 * time.Second, 2 * time.Second, 7 * time.Second, 2 * time.Second, 2 * time.Second}
	if len(ft.waits) != len(wantWaits) {
		t.Fatalf("waits = %v, want %v", ft.waits, wantWaits)
	}
	for i := range wantWaits {
		if ft.waits[i] != wantWaits[i] {
			t.Fatalf("waits = %v, want %v", ft.waits, wantWaits)
		}
	}
	contains(t, "429 notice", errOut, "7s")
	excludes(t, "copy", errOut, "\u2014", "\u2013")
}

func TestRentalLogsFollowRetries502WithBackoff(t *testing.T) {
	h := newHarness(t)
	h.rentalForLogs()
	ft := h.follow(5)
	unavailable := logPage("t1")
	unavailable.Available = false
	h.logsRoute("GET "+rentalLogsPath,
		pageStep(unavailable),
		failStep(502, "logs_upstream_error", ""),
		failStep(502, "logs_upstream_error", ""),
		pageStep(unavailable),
		pageStep(logPage("t2", logLine("a", 1, api.RentalLogSourceStdout, api.LogLevelInfo, "back"))),
	)
	out, errOut, code := h.run("rental", "logs", "quiet-river-7k2f", "--follow")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	contains(t, "out", out, "back")
	want := []time.Duration{2 * time.Second, 4 * time.Second, 8 * time.Second, 2 * time.Second, 2 * time.Second}
	if len(ft.waits) != len(want) {
		t.Fatalf("waits = %v, want %v", ft.waits, want)
	}
	for i := range want {
		if ft.waits[i] != want[i] {
			t.Fatalf("waits = %v, want %v", ft.waits, want)
		}
	}
	if n := strings.Count(errOut, "This host's agent cannot send logs yet."); n != 1 {
		t.Errorf("unavailable notice printed %d times: %q", n, errOut)
	}
	contains(t, "502 notice", errOut, "Could not read the logs")
}

func TestRentalLogsFollowStopsOn503(t *testing.T) {
	h := newHarness(t)
	h.rentalForLogs()
	h.follow(10)
	h.logsRoute("GET "+rentalLogsPath, pageStep(logPage("t1")), failStep(503, "logs_unavailable", ""))
	_, errOut, code := h.run("rental", "logs", "quiet-river-7k2f", "-f")
	if code == 0 {
		t.Fatalf("503 should end the command: %s", errOut)
	}
	contains(t, "503", errOut, "Logs are not available right now")
	if n := h.count("GET", rentalLogsPath); n != 2 {
		t.Errorf("reads = %d, want 2", n)
	}
}

func TestRentalLogsErrorWording(t *testing.T) {
	h := newHarness(t)
	h.rentalForLogs()
	h.fail("GET "+rentalLogsPath, 502, "logs_upstream_error", "upstream")
	_, errOut, code := h.run("rental", "logs", "quiet-river-7k2f")
	if code != 1 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	contains(t, "502", errOut, "Could not read the logs", "Try again")
}

func TestServiceLogsReplicaAndLabels(t *testing.T) {
	h := newHarness(t)
	h.svcFixtures(svcFixture(1, "chat", modeAuto))
	h.reply("GET /v1/services/"+uidS(1)+"/replicas", 200, api.ServiceReplicaPage{Items: []api.ServiceReplica{
		{RentalId: uid(40), RentalName: ptr("quiet-river-7k2f")},
		{RentalId: uid(41), RentalName: ptr("amber-lake-9x1q")},
	}})
	path := "/v1/services/" + uidS(1) + "/logs"
	l := logLine("a", 1, api.RentalLogSourceModelServer, api.LogLevelInfo, "model loaded")
	l.RentalId = uid(41)
	h.reply("GET "+path, 200, logPage("t1", l))
	out, _ := h.mustRun("svc", "logs", "chat", "--replica", "amber-lake", "--level", "debug")
	q := h.last("GET", path).Query
	if qget(q, "replica") != uidS(41) || qget(q, "level") != "debug" {
		t.Errorf("query = %v", q)
	}
	if !strings.Contains(h.last("GET", "/v1/services/"+uidS(1)+"/replicas").qget("include_retired"), "true") {
		t.Errorf("retired replicas have logs too; list them")
	}
	contains(t, "out", out, "amber-lake-9x1q", "model_server", "model loaded")
}

func TestServiceLogsUnknownReplica(t *testing.T) {
	h := newHarness(t)
	h.svcFixtures(svcFixture(1, "chat", modeAuto))
	h.reply("GET /v1/services/"+uidS(1)+"/replicas", 200, api.ServiceReplicaPage{Items: []api.ServiceReplica{{RentalId: uid(40), RentalName: ptr("quiet-river-7k2f")}}})
	_, errOut, code := h.run("services", "logs", "chat", "--replica", "nope")
	if code != 4 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	contains(t, "hint", errOut, "legion services replicas chat --all")
}

func TestLogLineFormatColor(t *testing.T) {
	l := logLine("a", 1, api.RentalLogSourceStderr, api.LogLevelError, "boom\x1b[2Jbad")
	l.Truncated = true
	plain := formatLogLine(l, "", false)
	if strings.Contains(plain, "\x1b") {
		t.Fatalf("escape left in plain output: %q", plain)
	}
	contains(t, "plain", plain, "stderr", "ERROR", "boom[2Jbad", "[truncated]")
	colored := formatLogLine(l, "", true)
	contains(t, "colored", colored, "\x1b[31mERROR")
	if w := formatLogLine(logLine("b", 1, api.RentalLogSourceStdout, api.LogLevelWarn, "x"), "replica-1", false); !strings.Contains(w, "replica-1") {
		t.Errorf("replica label missing: %q", w)
	}
}
