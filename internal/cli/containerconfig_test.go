package cli

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func testApp(t *testing.T, stdin string, tty bool, secret string) (*app, *cobra.Command) {
	a := &app{env: Env{
		Ctx: context.Background(), Stdin: strings.NewReader(stdin), Stderr: io.Discard, Stdout: io.Discard, StdinTTY: tty,
		ReadSecret: func(context.Context) (string, error) { return secret, nil },
	}}
	return a, &cobra.Command{Use: "create"}
}

func TestCheckEnvName(t *testing.T) {
	for _, ok := range []string{"A", "_x", "HF_TOKEN", "a1_b2"} {
		if err := checkEnvName(ok); err != nil {
			t.Errorf("%s: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "1A", "A-B", "LEGION_X", "PATH", "HOME", "HOSTNAME", "NVIDIA_VISIBLE_DEVICES", "CUDA_VISIBLE_DEVICES", strings.Repeat("A", 129)} {
		if checkEnvName(bad) == nil {
			t.Errorf("%q should be refused", bad)
		}
	}
}

func TestResolveConfig(t *testing.T) {
	a, cmd := testApp(t, "s3cret\n", false, "")
	cfg, err := a.resolveConfig(cmd, &configFlags{
		env: []string{"A=1", "B=x=y"}, secretEnv: []string{"TOKEN"}, setting: []string{"max_model_len=32768", "enable_prefix_caching=true", "tool_call_parser=hermes"},
		command: []string{"python", "s.py"}, arg: []string{"--port", "8000"}}, "rental create")
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Env) != 3 || cfg.Env[1].Value != "x=y" || !cfg.Env[2].Secret || cfg.Env[2].Value != "s3cret" {
		t.Fatalf("%+v", cfg.Env)
	}
	if cfg.Settings["max_model_len"] != int64(32768) || cfg.Settings["enable_prefix_caching"] != true || cfg.Settings["tool_call_parser"] != "hermes" {
		t.Fatalf("%+v", cfg.Settings)
	}
	if !cfg.hasCommand || len(cfg.Args) != 2 {
		t.Fatalf("%+v", cfg)
	}
	var b strings.Builder
	cfg.write(&b)
	if strings.Contains(b.String(), "s3cret") || !strings.Contains(b.String(), "TOKEN = (secret, hidden)") {
		t.Fatalf("preview leaks or omits: %s", b.String())
	}
}

func TestResolveConfigRefusals(t *testing.T) {
	long := strings.Repeat("x", maxEnvValue+1)
	many := make([]string, 65)
	for i := range many {
		many[i] = "V" + strings.Repeat("a", i) + "=1"
	}
	tests := map[string]*configFlags{
		"no equals":     {env: []string{"A"}},
		"reserved":      {env: []string{"LEGION_X=1"}},
		"dup":           {env: []string{"A=1", "A=2"}},
		"big":           {env: []string{"A=" + long}},
		"too many":      {env: many},
		"secret value":  {secretEnv: []string{"T=hunter2"}},
		"bad setting":   {setting: []string{"max_model_len"}},
		"dup setting":   {setting: []string{"a=1", "a=2"}},
		"long argv":     {arg: []string{strings.Repeat("a", maxArgvItemLen+1)}},
		"too many argv": {command: make([]string, 65)},
	}
	for name, f := range tests {
		t.Run(name, func(t *testing.T) {
			a, cmd := testApp(t, "v\n", false, "")
			_, err := a.resolveConfig(cmd, f, "rental create")
			if err == nil {
				t.Fatal("want a refusal")
			}
			if strings.Contains(err.Error(), "hunter2") {
				t.Fatalf("secret echoed: %v", err)
			}
		})
	}
}

func TestSecretInputs(t *testing.T) {
	a, cmd := testApp(t, "one\ntwo\n", false, "")
	cfg, err := a.resolveConfig(cmd, &configFlags{secretEnv: []string{"A", "B"}}, "rental create")
	if err != nil || cfg.Env[0].Value != "one" || cfg.Env[1].Value != "two" {
		t.Fatalf("%v %+v", err, cfg)
	}
	a, cmd = testApp(t, "one\n", false, "")
	if _, err = a.resolveConfig(cmd, &configFlags{secretEnv: []string{"A", "B"}}, "rental create"); err == nil {
		t.Fatal("line count mismatch must be refused")
	}
	a, cmd = testApp(t, "", true, "typed")
	cfg, err = a.resolveConfig(cmd, &configFlags{secretEnv: []string{"A"}}, "rental create")
	if err != nil || cfg.Env[0].Value != "typed" {
		t.Fatalf("%v %+v", err, cfg)
	}
	a, cmd = testApp(t, "", false, "")
	if _, err = a.resolveConfig(cmd, &configFlags{secretEnv: []string{"A"}}, "rental create"); err == nil {
		t.Fatal("empty secret must be refused")
	}
}
