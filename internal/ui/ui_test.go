package ui

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

func prompter(in string) (*Prompter, *bytes.Buffer) {
	var out bytes.Buffer
	return &Prompter{In: bufio.NewReader(strings.NewReader(in)), Out: &out}, &out
}

func TestConfirm(t *testing.T) {
	for in, want := range map[string]bool{"y\n": true, "YES\n": true, "n\n": false, "\n": false, "": false, "maybe\n": false} {
		p, _ := prompter(in)
		got, err := p.Confirm("Go?")
		if err != nil || got != want {
			t.Errorf("%q: %v %v", in, got, err)
		}
	}
}

func TestSelect(t *testing.T) {
	p, out := prompter("x\n9\n2\n")
	i, err := p.Select("Pick one", []string{"a", "b", "c"})
	if err != nil || i != 1 {
		t.Fatalf("%d %v", i, err)
	}
	if !strings.Contains(out.String(), "1) a") || !strings.Contains(out.String(), "Pick one") {
		t.Fatal(out.String())
	}
	p, _ = prompter("")
	if _, err := p.Select("Pick", []string{"a"}); err == nil {
		t.Fatal("EOF should error")
	}
	p, _ = prompter("\n")
	if i, err := p.Select("Pick", []string{"a"}); err != nil || i != 0 {
		t.Fatalf("Enter on one choice: %d %v", i, err)
	}
	p, _ = prompter("x\ny\nz\n1\n")
	if _, err := p.Select("Pick", []string{"a", "b"}); !errors.Is(err, ErrNoChoice) {
		t.Fatalf("want ErrNoChoice, got %v", err)
	}
}

func TestAskReturnsOnCancel(t *testing.T) {
	pr, _ := io.Pipe() // never written: a blocked read
	ctx, cancel := context.WithCancel(context.Background())
	p := &Prompter{In: bufio.NewReader(pr), Out: io.Discard, Ctx: ctx}
	go func() { time.Sleep(20 * time.Millisecond); cancel() }()
	if _, err := p.Ask("?"); !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
	if ok, err := p.Confirm("?"); ok || !errors.Is(err, context.Canceled) {
		t.Fatalf("confirm on cancelled ctx: %v %v", ok, err)
	}
}

func TestAskTypedConfirmation(t *testing.T) {
	p, _ := prompter("quiet-river\n")
	got, err := p.Ask("Type the name")
	if err != nil || got != "quiet-river" {
		t.Fatal(got, err)
	}
}

func TestColorEnabled(t *testing.T) {
	env := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }
	if !ColorEnabled(false, env(nil), true) {
		t.Error("tty should color")
	}
	if ColorEnabled(true, env(nil), true) {
		t.Error("--no-color")
	}
	if ColorEnabled(false, env(map[string]string{"NO_COLOR": "1"}), true) {
		t.Error("NO_COLOR")
	}
	if ColorEnabled(false, env(nil), false) {
		t.Error("non-tty")
	}
	if ColorEnabled(false, env(map[string]string{"TERM": "dumb"}), true) {
		t.Error("dumb")
	}
}

func TestSpinnerDisabledWritesNothing(t *testing.T) {
	var b bytes.Buffer
	s := NewSpinner(&b, false)
	s.Start("waiting")
	s.Update("still")
	s.Stop()
	if b.Len() != 0 {
		t.Fatalf("%q", b.String())
	}
}

func TestSpinnerEnabledClearsLine(t *testing.T) {
	var b syncBuf
	s := NewSpinner(&b, true)
	s.interval = time.Millisecond
	s.Start("waiting")
	time.Sleep(20 * time.Millisecond)
	s.Stop()
	out := b.String()
	if !strings.Contains(out, "waiting") || !strings.HasSuffix(out, "\r\x1b[K") {
		t.Fatalf("%q", out)
	}
}
