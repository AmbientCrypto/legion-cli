package ui

import (
	"bufio"
	"bytes"
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
