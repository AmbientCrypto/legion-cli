package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/AmbientCrypto/legion-controller/api"
	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/AmbientCrypto/legion-cli/internal/client"
	"github.com/AmbientCrypto/legion-cli/internal/ui"
)

// LGN-648: renters' logs. One page for a range, or a live tail polled with
// the page's tail_cursor.

const (
	logsPoll       = 2 * time.Second  // the live tail's interval
	logsBackoffMax = 30 * time.Second // the longest wait after a 502
	logsRetention  = 14 * 24 * time.Hour
	logsSeenMax    = 20000 // line ids remembered for dedupe
	logsUnavailMsg = "This host's agent cannot send logs yet."
)

var logSources = []string{"stdout", "stderr", "model_server", "legion_serve", "legion"}

// logFlags are the flags rental logs and services logs share.
type logFlags struct {
	follow, json bool
	since        string
	sources      []string
	level, grep  string
	limit        int
}

func (f *logFlags) register(cmd *cobra.Command) {
	fl := cmd.Flags()
	fl.BoolVarP(&f.follow, "follow", "f", false, "keep printing new lines until Ctrl-C")
	fl.StringVar(&f.since, "since", "", "how far back to read, like 30m, 2h or 3d (at most 14d), or a time (RFC 3339); default the last hour")
	fl.StringSliceVar(&f.sources, "source", nil, "only these streams: "+strings.Join(logSources, ", ")+" (comma separated or repeated; default all)")
	fl.StringVar(&f.level, "level", "", "the lowest level to show: debug, info, warn or error (default info)")
	fl.StringVar(&f.grep, "grep", "", "only lines whose message contains this text (plain text, not a pattern)")
	fl.IntVar(&f.limit, "limit", 0, "how many lines at most, 1 to 1000 (default 200)")
	fl.BoolVar(&f.json, "json", false, "print one JSON object per line (NDJSON)")
}

// logQuery is what both log operations take.
type logQuery struct {
	Start  *time.Time
	Source *[]api.RentalLogSource
	Level  *api.LogLevel
	Q      *string
	Limit  *int
	After  *string
}

// query checks the flags and turns them into the first page's query.
func (a *app) logQuery(cmd *cobra.Command, f *logFlags) (logQuery, error) {
	hint := "legion " + a.cmdPath + " --help"
	var q logQuery
	if f.since != "" {
		start, err := a.parseSince(f.since)
		if err != nil {
			return q, usagef(hint, "%v", err)
		}
		q.Start = &start
	}
	if len(f.sources) > 0 {
		var ss []api.RentalLogSource
		for _, s := range f.sources {
			s = strings.ToLower(strings.TrimSpace(s))
			if s == "" {
				continue
			}
			ok := false
			for _, v := range logSources {
				ok = ok || v == s
			}
			if !ok {
				return q, usagef(hint, "unknown --source %q: use %s", s, strings.Join(logSources, ", "))
			}
			ss = append(ss, api.RentalLogSource(s))
		}
		if len(ss) > 0 {
			q.Source = &ss
		}
	}
	if f.level != "" {
		l := api.LogLevel(strings.ToLower(strings.TrimSpace(f.level)))
		switch l {
		case api.LogLevelDebug, api.LogLevelInfo, api.LogLevelWarn, api.LogLevelError:
		default:
			return q, usagef(hint, "unknown --level %q: use debug, info, warn or error", f.level)
		}
		q.Level = &l
	}
	if f.grep != "" {
		if len([]rune(f.grep)) > 200 {
			return q, usagef(hint, "--grep is at most 200 characters")
		}
		q.Q = ptr(f.grep)
	}
	if cmd.Flags().Changed("limit") {
		if f.limit < 1 || f.limit > 1000 {
			return q, usagef(hint, "--limit must be 1 to 1000")
		}
		q.Limit = ptr(f.limit)
	}
	return q, nil
}

// parseSince reads 30m, 2h, 3d or an RFC 3339 time into a start time.
func (a *app) parseSince(s string) (time.Time, error) {
	now := a.now()
	s = strings.TrimSpace(s)
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		if now.Sub(t) > logsRetention {
			return t, fmt.Errorf("--since is at most 14 days ago (logs are kept 14 days)")
		}
		return t, nil
	}
	var d time.Duration
	if n, ok := strings.CutSuffix(s, "d"); ok {
		days, err := strconv.Atoi(n)
		if err != nil {
			return time.Time{}, fmt.Errorf("cannot read --since %q: use 30m, 2h, 3d or a time like 2026-11-19T08:00:00Z", s)
		}
		d = time.Duration(days) * 24 * time.Hour
	} else {
		var err error
		if d, err = time.ParseDuration(s); err != nil {
			return time.Time{}, fmt.Errorf("cannot read --since %q: use 30m, 2h, 3d or a time like 2026-11-19T08:00:00Z", s)
		}
	}
	if d <= 0 {
		return time.Time{}, fmt.Errorf("--since must be more than zero")
	}
	if d > logsRetention {
		return time.Time{}, fmt.Errorf("--since is at most 14d (logs are kept 14 days)")
	}
	return now.Add(-d), nil
}

func (a *app) now() time.Time {
	if a.env.Now != nil {
		return a.env.Now()
	}
	return time.Now()
}

func (a *app) after(d time.Duration) <-chan time.Time {
	if a.env.After != nil {
		return a.env.After(d)
	}
	return time.After(d)
}

// logStream prints one page, then with follow polls the tail until Ctrl-C.
type logStream struct {
	a      *app
	f      *logFlags
	fetch  func(ctx context.Context, q logQuery) (*api.RentalLogPage, error)
	label  func(api.RentalLogLine) string // the replica column; nil for one rental
	jsonOn bool
	color  bool

	seen       map[string]struct{}
	order      []string
	unavailMsg bool
}

func (a *app) runLogs(ctx context.Context, cmd *cobra.Command, f *logFlags, label func(api.RentalLogLine) string,
	fetch func(ctx context.Context, q logQuery) (*api.RentalLogPage, error)) error {
	first, err := a.logQuery(cmd, f)
	if err != nil {
		return err
	}
	jsonOn := f.json
	if !jsonOn {
		if fm, err := a.format(); err == nil && fm.Kind == "json" {
			jsonOn = true
		}
	}
	s := &logStream{a: a, f: f, fetch: fetch, label: label, jsonOn: jsonOn, color: a.style().On, seen: map[string]struct{}{}}
	return s.run(ctx, first)
}

func (s *logStream) run(ctx context.Context, first logQuery) error {
	a := s.a
	if !s.f.follow {
		p, err := s.fetch(ctx, first)
		if err != nil {
			return err
		}
		s.print(p)
		if p.NextCursor != nil && *p.NextCursor != "" && len(p.Lines) > 0 {
			a.info("Showing the newest %d lines. For older ones, raise --limit or set --since.", len(p.Lines))
		}
		return nil
	}
	var tail *string
	backoff := time.Duration(0)
	for {
		q := first
		if tail != nil {
			q = logQuery{Source: first.Source, Level: first.Level, Q: first.Q, Limit: first.Limit, After: tail}
		}
		wait := logsPoll
		p, err := s.fetch(ctx, q)
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, context.Canceled) {
				return nil
			}
			var e *client.Error
			var ce *cliError
			switch {
			case errors.As(err, &e) && (e.Status == 429 || e.Code == "rate_limited"):
				wait = max(e.RetryAfter, logsPoll)
				a.info("Reading logs too often. Waiting %s before trying again.", wait)
			case errors.As(err, &e) && (e.Status == 502 || e.Code == "logs_upstream_error"),
				errors.As(err, &ce) && ce.Code == "network_error":
				backoff = min(max(backoff*2, 2*logsPoll), logsBackoffMax)
				wait = backoff
				a.info("Could not read the logs just now. Trying again in %s.", wait)
			default:
				return err
			}
		} else {
			backoff = 0
			s.print(p)
			if p.TailCursor != "" {
				tail = ptr(p.TailCursor)
			} else if tail == nil {
				tail = ptr("")
			}
		}
		select {
		case <-ctx.Done():
			return nil
		case <-a.after(wait):
		}
	}
}

func (s *logStream) print(p *api.RentalLogPage) {
	if !p.Available && !s.unavailMsg {
		s.unavailMsg = true
		fmt.Fprintln(s.a.env.Stderr, logsUnavailMsg)
	}
	enc := json.NewEncoder(s.a.env.Stdout)
	for _, l := range p.Lines {
		if _, dup := s.seen[l.Id]; dup {
			continue
		}
		s.remember(l.Id)
		if s.jsonOn {
			_ = enc.Encode(l)
			continue
		}
		lbl := ""
		if s.label != nil {
			lbl = s.label(l)
		}
		fmt.Fprintln(s.a.env.Stdout, formatLogLine(l, lbl, s.color))
	}
}

func (s *logStream) remember(id string) {
	s.seen[id] = struct{}{}
	s.order = append(s.order, id)
	if len(s.order) > logsSeenMax {
		drop := len(s.order) - logsSeenMax
		for _, old := range s.order[:drop] {
			delete(s.seen, old)
		}
		s.order = append([]string(nil), s.order[drop:]...)
	}
}

// formatLogLine is one line of text: local time, [replica], source, level,
// message. The level is colored only when color is on.
func formatLogLine(l api.RentalLogLine, replica string, color bool) string {
	st := ui.Style{On: color}
	lvl := fmt.Sprintf("%-5s", strings.ToUpper(string(l.Level)))
	switch l.Level {
	case api.LogLevelError:
		lvl = st.Red(lvl)
	case api.LogLevelWarn:
		lvl = st.Yellow(lvl)
	case api.LogLevelDebug:
		lvl = st.Dim(lvl)
	}
	var b strings.Builder
	b.WriteString(st.Dim(l.Time.Local().Format("2006-01-02 15:04:05.000")))
	b.WriteString("  ")
	if replica != "" {
		fmt.Fprintf(&b, "%-20s  ", san(replica))
	}
	fmt.Fprintf(&b, "%-12s  %s  %s", san(string(l.Source)), lvl, san(l.Message))
	if l.Truncated {
		b.WriteString(" " + st.Dim("[truncated]"))
	}
	return b.String()
}

// ---- commands ----

const logsLong = `Lines come oldest first: the container's output (stdout, stderr), for a Legion model image also the
model server (model_server) and Legion's download and start log (legion_serve), and Legion's own events
about the rental (legion: image pulled, container started, health check passed, stopped, failed).
Without --follow it prints the newest lines of the range (the last hour by default). With --follow it
keeps printing new lines every 2 seconds until Ctrl-C. Logs are kept 14 days; values of the rental's
environment variables and secrets are shown as [redacted].`

func (a *app) rentalLogsCmd() *cobra.Command {
	f := &logFlags{}
	cmd := &cobra.Command{
		Use:   "logs <rental>",
		Short: "Show a rental's logs, or follow them live",
		Long:  logsLong,
		Example: `  legion rental logs quiet-river-7k2f
  legion rental logs quiet-river-7k2f -f --source stdout,stderr
  legion rental logs quiet-river-7k2f --since 6h --level warn --grep "out of memory"
  legion rental logs quiet-river-7k2f --json | jq -r .message`,
		Args: args(0, 1),
		RunE: func(cmd *cobra.Command, as []string) error {
			return a.withRental(cmd, as, func(ctx context.Context, c *api.ClientWithResponses, r *api.Rental) error {
				return a.runLogs(ctx, cmd, f, nil, func(ctx context.Context, q logQuery) (*api.RentalLogPage, error) {
					res, err := c.GetRentalLogsWithResponse(ctx, r.Id, &api.GetRentalLogsParams{
						Start: q.Start, Source: q.Source, Level: q.Level, Q: q.Q, Limit: q.Limit, After: q.After})
					if err := a.ok(res, err); err != nil {
						return nil, err
					}
					return need(res.JSON200)
				})
			})
		},
	}
	f.register(cmd)
	return cmd
}

func (a *app) svcLogsCmd() *cobra.Command {
	f := &logFlags{}
	var replica string
	cmd := &cobra.Command{
		Use:   "logs <service>",
		Short: "Show a service's logs from every replica, or follow them live",
		Long:  logsLong + "\nA service's lines come from all its replicas, merged by time; --replica keeps one.",
		Example: `  legion services logs chat
  legion services logs chat -f --source model_server
  legion services logs chat --replica quiet-river-7k2f --since 30m`,
		Args: args(0, 1),
		RunE: func(cmd *cobra.Command, as []string) error {
			return a.withService(cmd, as, func(ctx context.Context, c *api.ClientWithResponses, e *api.Service) error {
				reps, err := a.listReplicas(ctx, c, e.Id, true)
				if err != nil {
					return err
				}
				names := map[uuid.UUID]string{}
				for _, r := range reps {
					names[r.RentalId] = replicaName(r)
				}
				var only *api.RentalID
				if replica != "" {
					id, err := matchReplica(reps, replica, e.Name)
					if err != nil {
						return err
					}
					only = &id
				}
				label := func(l api.RentalLogLine) string {
					if n, ok := names[l.RentalId]; ok {
						return n
					}
					return l.RentalId.String()[:8]
				}
				return a.runLogs(ctx, cmd, f, label, func(ctx context.Context, q logQuery) (*api.RentalLogPage, error) {
					res, err := c.GetServiceLogsWithResponse(ctx, e.Id, &api.GetServiceLogsParams{Replica: only,
						Start: q.Start, Source: q.Source, Level: q.Level, Q: q.Q, Limit: q.Limit, After: q.After})
					if err := a.ok(res, err); err != nil {
						return nil, err
					}
					return need(res.JSON200)
				})
			})
		},
	}
	f.register(cmd)
	cmd.Flags().StringVar(&replica, "replica", "", "only this replica's lines (its rental id or name, as 'legion services replicas' lists it)")
	return cmd
}

// matchReplica finds a replica by rental id, name or unique prefix.
func matchReplica(reps []api.ServiceReplica, arg, svc string) (api.RentalID, error) {
	hint := "legion services replicas " + svc + " --all"
	arg = strings.ToLower(strings.TrimSpace(arg))
	var hits []api.ServiceReplica
	for _, r := range reps {
		if r.RentalId.String() == arg || ptrOr(r.RentalName, "") == arg {
			return r.RentalId, nil
		}
		if strings.HasPrefix(r.RentalId.String(), arg) || (r.RentalName != nil && strings.HasPrefix(*r.RentalName, arg)) {
			hits = append(hits, r)
		}
	}
	switch len(hits) {
	case 0:
		return api.RentalID{}, notFoundf(hint, "no replica %q on service %s", arg, svc)
	case 1:
		return hits[0].RentalId, nil
	}
	var names []string
	for _, r := range hits {
		names = append(names, replicaName(r))
	}
	return api.RentalID{}, usagef(hint, "%q matches several replicas: %s", arg, strings.Join(names, ", "))
}
