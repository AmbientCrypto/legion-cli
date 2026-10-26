// Package ui holds the terminal bits: prompts, the stderr spinner, colour.
package ui

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Prompter asks questions on Out (stderr) and reads answers from In. Reads
// give up when Ctx is cancelled (Ctrl-C), so a prompt never traps the user.
type Prompter struct {
	In  *bufio.Reader
	Out io.Writer
	Ctx context.Context
}

// ErrNoChoice is returned by Select after three invalid answers.
var ErrNoChoice = errors.New("no valid choice")

// Ask prints q and reads one line.
func (p *Prompter) Ask(q string) (string, error) {
	fmt.Fprintf(p.Out, "%s ", q)
	type res struct {
		line string
		err  error
	}
	ch := make(chan res, 1)
	go func() { l, err := p.In.ReadString('\n'); ch <- res{l, err} }()
	ctx := p.Ctx
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case <-ctx.Done():
		fmt.Fprintln(p.Out)
		return "", ctx.Err()
	case r := <-ch:
		if r.err != nil && r.line == "" {
			return "", fmt.Errorf("no answer: %w", r.err)
		}
		return strings.TrimSpace(r.line), nil
	}
}

// Confirm asks a yes/no question, default no. End of input is a no; an
// interrupt is an error.
func (p *Prompter) Confirm(q string) (bool, error) {
	a, err := p.Ask(q + " [y/N]:")
	if errors.Is(err, context.Canceled) {
		return false, err
	}
	a = strings.ToLower(a)
	return a == "y" || a == "yes", nil
}

// Select lists options and returns the chosen index. With one option Enter
// takes it. Three invalid answers give up.
func (p *Prompter) Select(label string, opts []string) (int, error) {
	fmt.Fprintln(p.Out, label)
	for i, o := range opts {
		fmt.Fprintf(p.Out, "  %d) %s\n", i+1, o)
	}
	q := fmt.Sprintf("Choose 1-%d:", len(opts))
	if len(opts) == 1 {
		q = "Press Enter to choose 1:"
	}
	for try := 0; try < 3; try++ {
		a, err := p.Ask(q)
		if err != nil {
			return 0, err
		}
		if a == "" && len(opts) == 1 {
			return 0, nil
		}
		if n, err := strconv.Atoi(a); err == nil && n >= 1 && n <= len(opts) {
			return n - 1, nil
		}
		fmt.Fprintln(p.Out, "Not a valid choice.")
	}
	return 0, ErrNoChoice
}

// ColorEnabled follows --no-color, NO_COLOR, TERM=dumb and whether stdout is a terminal.
func ColorEnabled(noColorFlag bool, getenv func(string) string, isTTY bool) bool {
	if noColorFlag || !isTTY || getenv("NO_COLOR") != "" || getenv("TERM") == "dumb" {
		return false
	}
	return true
}

// Style wraps text in ANSI codes when On.
type Style struct{ On bool }

func (s Style) wrap(code, t string) string {
	if !s.On {
		return t
	}
	return "\x1b[" + code + "m" + t + "\x1b[0m"
}
func (s Style) Bold(t string) string  { return s.wrap("1", t) }
func (s Style) Red(t string) string   { return s.wrap("31", t) }
func (s Style) Green(t string) string { return s.wrap("32", t) }
func (s Style) Dim(t string) string   { return s.wrap("2", t) }

// Spinner draws a one-line spinner on a terminal. Disabled, it does nothing.
type Spinner struct {
	w        io.Writer
	on       bool
	interval time.Duration
	mu       sync.Mutex
	msg      string
	stop     chan struct{}
	done     chan struct{}
}

func NewSpinner(w io.Writer, enabled bool) *Spinner {
	return &Spinner{w: w, on: enabled, interval: 100 * time.Millisecond}
}

func (s *Spinner) Start(msg string) {
	if !s.on || s.stop != nil {
		return
	}
	s.msg, s.stop, s.done = msg, make(chan struct{}), make(chan struct{})
	go func() {
		defer close(s.done)
		frames := []string{"|", "/", "-", "\\"}
		t := time.NewTicker(s.interval)
		defer t.Stop()
		for i := 0; ; i++ {
			s.mu.Lock()
			fmt.Fprintf(s.w, "\r\x1b[K%s %s", frames[i%len(frames)], s.msg)
			s.mu.Unlock()
			select {
			case <-s.stop:
				return
			case <-t.C:
			}
		}
	}()
}

func (s *Spinner) Update(msg string) {
	s.mu.Lock()
	s.msg = msg
	s.mu.Unlock()
}

func (s *Spinner) Stop() {
	if !s.on || s.stop == nil {
		return
	}
	close(s.stop)
	<-s.done
	s.stop = nil
	fmt.Fprint(s.w, "\r\x1b[K")
}
