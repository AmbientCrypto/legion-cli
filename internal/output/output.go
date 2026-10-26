// Package output renders command results as table, wide, json, yaml, name or
// jsonpath, and formats money, sizes and durations.
package output

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/ohler55/ojg/jp"
	"gopkg.in/yaml.v3"
)

// Formats is the help text for -o.
const Formats = "table|wide|json|yaml|name|jsonpath=<expr>"

// Format is a parsed -o value.
type Format struct {
	Kind string // table wide json yaml name jsonpath
	Expr string // jsonpath only
}

// Parse reads an -o value.
func Parse(s string) (Format, error) {
	s = strings.TrimSpace(s)
	if rest, ok := strings.CutPrefix(s, "jsonpath="); ok {
		if rest == "" {
			return Format{}, errors.New("jsonpath needs an expression, for example -o jsonpath='$[*].name'")
		}
		return Format{Kind: "jsonpath", Expr: rest}, nil
	}
	switch k := strings.ToLower(s); k {
	case "table", "wide", "json", "yaml", "name":
		return Format{Kind: k}, nil
	}
	return Format{}, fmt.Errorf("unknown output format %q (use %s)", s, Formats)
}

// Machine reports formats whose stdout must stay free of decoration.
func (f Format) Machine() bool { return f.Kind != "table" && f.Kind != "wide" }

// Table is a list view. Rows carry len(Cols)+len(Wide) cells.
type Table struct {
	Cols, Wide []string
	Rows       [][]string
}

// Result is what a command produced: Data for json/yaml/jsonpath, Names for
// -o name, and Table or Human for people.
type Result struct {
	Data  any
	Names []string
	Table *Table
	Human func(io.Writer)
}

// Write renders r in format f.
func Write(w io.Writer, f Format, r Result) error {
	switch f.Kind {
	case "json":
		b, err := json.MarshalIndent(r.Data, "", "  ")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(w, "%s\n", b)
		return err
	case "yaml":
		v, err := generic(r.Data)
		if err != nil {
			return err
		}
		b, err := yaml.Marshal(v)
		if err != nil {
			return err
		}
		_, err = w.Write(b)
		return err
	case "name":
		if r.Names == nil {
			return errors.New("this command has no name output; use -o json")
		}
		for _, n := range r.Names {
			fmt.Fprintln(w, n)
		}
		return nil
	case "jsonpath":
		return writePath(w, f.Expr, r.Data)
	}
	if r.Human != nil {
		r.Human(w)
		return nil
	}
	if r.Table == nil {
		return nil
	}
	return writeTable(w, r.Table, f.Kind == "wide")
}

func generic(data any) (any, error) {
	b, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	var v any
	err = json.Unmarshal(b, &v)
	return v, err
}

func writePath(w io.Writer, expr string, data any) error {
	expr = strings.TrimSpace(expr)
	expr = strings.TrimSuffix(strings.TrimPrefix(expr, "{"), "}")
	if strings.HasPrefix(expr, ".") || strings.HasPrefix(expr, "[") {
		expr = "$" + expr
	}
	expr = strings.Replace(expr, "$.[", "$[", 1)
	x, err := jp.ParseString(expr)
	if err != nil {
		return fmt.Errorf("bad jsonpath %q: %w", expr, err)
	}
	v, err := generic(data)
	if err != nil {
		return err
	}
	for _, m := range x.Get(v) {
		switch t := m.(type) {
		case string:
			fmt.Fprintln(w, t)
		case map[string]any, []any:
			b, _ := json.Marshal(t)
			fmt.Fprintln(w, string(b))
		default:
			b, _ := json.Marshal(t)
			fmt.Fprintln(w, string(b))
		}
	}
	return nil
}

func writeTable(w io.Writer, t *Table, wide bool) error {
	n := len(t.Cols)
	cols := t.Cols
	if wide {
		cols = append(append([]string{}, t.Cols...), t.Wide...)
		n = len(cols)
	}
	tw := tabwriter.NewWriter(w, 0, 8, 2, ' ', 0)
	fmt.Fprintln(tw, strings.Join(cols, "\t"))
	for _, row := range t.Rows {
		if len(row) > n {
			row = row[:n]
		}
		fmt.Fprintln(tw, strings.Join(row, "\t"))
	}
	return tw.Flush()
}

// Money formats minor units: $2.50, -$12.34; other currencies as "1.99 EUR".
func Money(minor int64, currency string) string {
	sign := ""
	if minor < 0 {
		sign, minor = "-", -minor
	}
	s := fmt.Sprintf("%d.%02d", minor/100, minor%100)
	if currency == "USD" || currency == "" {
		return sign + "$" + s
	}
	return sign + s + " " + currency
}

// ParseDollars reads a positive dollar amount ("2.50", "$3") as cents.
func ParseDollars(s string) (int64, error) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "$")
	whole, frac, _ := strings.Cut(s, ".")
	if whole == "" && frac == "" || len(frac) > 2 {
		return 0, fmt.Errorf("%q is not a dollar amount with at most 2 decimals, for example 2.50", s)
	}
	w, err := strconv.ParseInt(or0(whole), 10, 64)
	if err != nil || w < 0 {
		return 0, fmt.Errorf("%q is not a dollar amount, for example 2.50", s)
	}
	f, err := strconv.ParseInt(or0((frac+"00")[:2]), 10, 64)
	if err != nil || f < 0 {
		return 0, fmt.Errorf("%q is not a dollar amount, for example 2.50", s)
	}
	if w*100+f == 0 {
		return 0, errors.New("the amount must be above $0.00")
	}
	return w*100 + f, nil
}

func or0(s string) string {
	if s == "" {
		return "0"
	}
	return s
}

// Bytes formats a size in binary units.
func Bytes(n int64) string {
	if n < 1024 {
		return fmt.Sprintf("%d B", n)
	}
	v, u := float64(n), 0
	for ; v >= 1024 && u < 4; u++ {
		v /= 1024
	}
	return fmt.Sprintf("%.1f %s", v, []string{"B", "KiB", "MiB", "GiB", "TiB"}[u])
}

// Duration formats seconds as 59s, 1m30s, 1h2m.
func Duration(sec int64) string {
	switch {
	case sec < 60:
		return fmt.Sprintf("%ds", sec)
	case sec < 3600:
		return fmt.Sprintf("%dm%ds", sec/60, sec%60)
	}
	return fmt.Sprintf("%dh%dm", sec/3600, sec%3600/60)
}
