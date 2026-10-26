package output

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

type item struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

func sample() Result {
	items := []item{{"alpha", 1}, {"beta", 2}}
	return Result{
		Data:  items,
		Names: []string{"alpha", "beta"},
		Table: &Table{
			Cols: []string{"NAME", "COUNT"}, Wide: []string{"EXTRA"},
			Rows: [][]string{{"alpha", "1", "x"}, {"beta", "2", "y"}},
		},
	}
}

func TestParse(t *testing.T) {
	for in, kind := range map[string]string{"table": "table", "wide": "wide", "json": "json", "yaml": "yaml", "name": "name", "JSON": "json"} {
		f, err := Parse(in)
		if err != nil || f.Kind != kind {
			t.Errorf("%s: %+v %v", in, f, err)
		}
	}
	f, err := Parse("jsonpath=$[*].name")
	if err != nil || f.Kind != "jsonpath" || f.Expr != "$[*].name" {
		t.Fatalf("%+v %v", f, err)
	}
	for _, bad := range []string{"xml", "jsonpath=", ""} {
		if _, err := Parse(bad); err == nil {
			t.Errorf("%q: want error", bad)
		}
	}
}

func render(t *testing.T, f string, r Result) string {
	t.Helper()
	ff, err := Parse(f)
	if err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	if err := Write(&b, ff, r); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

func TestFormats(t *testing.T) {
	r := sample()
	if got := render(t, "json", r); !strings.Contains(got, `"name": "alpha"`) || !strings.HasSuffix(got, "]\n") {
		t.Errorf("json: %q", got)
	}
	if got := render(t, "yaml", r); !strings.Contains(got, "- count: 1\n  name: alpha") {
		t.Errorf("yaml: %q", got)
	}
	if got := render(t, "name", r); got != "alpha\nbeta\n" {
		t.Errorf("name: %q", got)
	}
	tbl := render(t, "table", r)
	if !strings.Contains(tbl, "NAME") || strings.Contains(tbl, "EXTRA") {
		t.Errorf("table: %q", tbl)
	}
	if got := render(t, "wide", r); !strings.Contains(got, "EXTRA") || !strings.Contains(got, "y") {
		t.Errorf("wide: %q", got)
	}
}

func TestJSONPath(t *testing.T) {
	r := sample()
	if got := render(t, "jsonpath=$[*].name", r); got != "alpha\nbeta\n" {
		t.Errorf("%q", got)
	}
	if got := render(t, "jsonpath={.[1].count}", r); got != "2\n" {
		t.Errorf("braces: %q", got)
	}
	if got := render(t, "jsonpath=[0]", r); got != "{\"count\":1,\"name\":\"alpha\"}\n" {
		t.Errorf("object: %q", got)
	}
	f, _ := Parse("jsonpath=$[")
	if err := Write(&bytes.Buffer{}, f, r); err == nil {
		t.Error("want bad expression error")
	}
}

func TestNameUnsupported(t *testing.T) {
	f, _ := Parse("name")
	if err := Write(&bytes.Buffer{}, f, Result{Data: 1}); err == nil {
		t.Fatal("want error")
	}
}

func TestHumanOverridesTable(t *testing.T) {
	r := Result{Data: 1, Human: func(w io.Writer) { w.Write([]byte("detail view\n")) }}
	if got := render(t, "table", r); got != "detail view\n" {
		t.Fatal(got)
	}
}

func TestMoney(t *testing.T) {
	for _, tc := range []struct {
		minor int64
		cur   string
		want  string
	}{{250, "USD", "$2.50"}, {5, "USD", "$0.05"}, {0, "USD", "$0.00"}, {-1234, "USD", "-$12.34"}, {100000, "USD", "$1000.00"}, {199, "EUR", "1.99 EUR"}} {
		if got := Money(tc.minor, tc.cur); got != tc.want {
			t.Errorf("%d %s = %q want %q", tc.minor, tc.cur, got, tc.want)
		}
	}
}

func TestParseDollars(t *testing.T) {
	for in, want := range map[string]int64{"2.5": 250, "$2.50": 250, "3": 300, "0.05": 5, " 1.99 ": 199} {
		if got, err := ParseDollars(in); err != nil || got != want {
			t.Errorf("%q: %d %v", in, got, err)
		}
	}
	for _, bad := range []string{"", "abc", "-1", "1.234", "0"} {
		if _, err := ParseDollars(bad); err == nil {
			t.Errorf("%q: want error", bad)
		}
	}
}

func TestBytesAndAge(t *testing.T) {
	if Bytes(1536) != "1.5 KiB" || Bytes(0) != "0 B" || Bytes(5<<30) != "5.0 GiB" {
		t.Fatal(Bytes(1536), Bytes(5<<30))
	}
	if Duration(3725) != "1h2m" || Duration(59) != "59s" || Duration(90) != "1m30s" {
		t.Fatal(Duration(3725), Duration(59), Duration(90))
	}
}
