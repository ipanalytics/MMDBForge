package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"mmdbforge/internal/audit"
	"mmdbforge/internal/diff"
	"mmdbforge/internal/lookup"
	"mmdbforge/internal/report"
	"mmdbforge/internal/schema"
	"mmdbforge/internal/smoke"
	"mmdbforge/internal/stats"
)

const usage = `MMDB Forge is a developer toolkit for inspecting, validating, diffing, and explaining custom MaxMind DB files.

Usage:
  mmdbforge inspect <db.mmdb> <ip>
  mmdbforge explain <db.mmdb> <ip>
  mmdbforge diff <old.mmdb> <new.mmdb> [--sample N] [--ips file] [--fields a,b] [--json] [--markdown [file]] [--fail-threshold changed_percent=N] [--fail-on-missing-field field] [--fail-on-drop field]
  mmdbforge validate <schema.json> <db.mmdb> [--sample N]
  mmdbforge stats <db.mmdb> [--sample N] [--top N]
  mmdbforge fields <db.mmdb> [--sample N]
  mmdbforge smoke <db.mmdb> <smoke.json>
  mmdbforge audit release --old old.mmdb --new new.mmdb [--schema schema.json] [--smoke smoke.json] [--sample N] [--markdown [file]]
`

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string, out io.Writer) error {
	if len(args) == 0 {
		fmt.Fprint(out, usage)
		return nil
	}

	switch args[0] {
	case "inspect":
		return inspectCmd(args[1:], out)
	case "explain":
		return explainCmd(args[1:], out)
	case "diff":
		return diffCmd(args[1:], out)
	case "validate":
		return validateCmd(args[1:], out)
	case "stats":
		return statsCmd(args[1:], out)
	case "fields":
		return fieldsCmd(args[1:], out)
	case "smoke":
		return smokeCmd(args[1:], out)
	case "audit":
		return auditCmd(args[1:], out)
	case "-h", "--help", "help":
		fmt.Fprint(out, usage)
		return nil
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func inspectCmd(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("inspect", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 2 {
		return errors.New("usage: mmdbforge inspect <db.mmdb> <ip>")
	}
	res, err := lookup.Inspect(fs.Arg(0), fs.Arg(1))
	if err != nil {
		return err
	}
	return report.JSON(out, res)
}

func explainCmd(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("explain", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 2 {
		return errors.New("usage: mmdbforge explain <db.mmdb> <ip>")
	}
	res, err := lookup.Explain(fs.Arg(0), fs.Arg(1))
	if err != nil {
		return err
	}
	return report.JSON(out, res)
}

func diffCmd(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("diff", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var opts diff.Options
	var fields, threshold string
	var jsonOut bool
	var markdown markdownFlag
	fs.IntVar(&opts.Sample, "sample", 10000, "sample size")
	fs.StringVar(&opts.IPsFile, "ips", "", "file with test IPs")
	fs.StringVar(&fields, "fields", "", "comma-separated fields")
	fs.BoolVar(&jsonOut, "json", false, "print JSON output")
	fs.Var(&markdown, "markdown", "write markdown to stdout or optional path")
	fs.StringVar(&threshold, "fail-threshold", "", "threshold, for example changed_percent=20")
	fs.StringVar(&opts.FailOnMissingField, "fail-on-missing-field", "", "fail if field is missing in sampled new records")
	fs.StringVar(&opts.FailOnChangeField, "fail-on-change", "", "fail if field changes")
	fs.StringVar(&opts.FailOnDropField, "fail-on-drop", "", "fail if field is removed")
	if err := fs.Parse(normalizeOptionalValue(args, "--markdown")); err != nil {
		return err
	}
	_ = jsonOut
	if fs.NArg() != 2 {
		return errors.New("usage: mmdbforge diff <old.mmdb> <new.mmdb>")
	}
	opts.Fields = splitCSV(fields)
	if threshold != "" {
		name, value, err := parseThreshold(threshold)
		if err != nil {
			return err
		}
		opts.FailThresholdName, opts.FailThresholdValue = name, value
	}
	res, err := diff.Run(fs.Arg(0), fs.Arg(1), opts)
	if err != nil {
		return err
	}
	if markdown.set {
		return writeMarkdown(out, markdown.path, report.DiffMarkdown(res))
	}
	if err := report.JSON(out, res); err != nil {
		return err
	}
	if res.Failed {
		return fmt.Errorf("diff guard failed: %s", strings.Join(res.Failures, "; "))
	}
	return nil
}

func validateCmd(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var sample int
	fs.IntVar(&sample, "sample", 10000, "sample size")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 2 {
		return errors.New("usage: mmdbforge validate <schema.json> <db.mmdb>")
	}
	res, err := schema.Validate(fs.Arg(0), fs.Arg(1), sample)
	if err != nil {
		return err
	}
	if err := report.JSON(out, res); err != nil {
		return err
	}
	if len(res.Errors) > 0 {
		return fmt.Errorf("schema validation failed with %d error(s)", len(res.Errors))
	}
	return nil
}

func statsCmd(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("stats", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var sample, top int
	fs.IntVar(&sample, "sample", 10000, "sample size")
	fs.IntVar(&top, "top", 10, "top value count")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("usage: mmdbforge stats <db.mmdb>")
	}
	res, err := stats.Run(fs.Arg(0), sample, top)
	if err != nil {
		return err
	}
	return report.JSON(out, res)
}

func fieldsCmd(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("fields", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var sample int
	fs.IntVar(&sample, "sample", 10000, "sample size")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("usage: mmdbforge fields <db.mmdb>")
	}
	res, err := stats.Fields(fs.Arg(0), sample)
	if err != nil {
		return err
	}
	return report.JSON(out, res)
}

func smokeCmd(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("smoke", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 2 {
		return errors.New("usage: mmdbforge smoke <db.mmdb> <smoke.json>")
	}
	res, err := smoke.Run(fs.Arg(0), fs.Arg(1))
	if err != nil {
		return err
	}
	if err := report.JSON(out, res); err != nil {
		return err
	}
	if res.Failed > 0 {
		return fmt.Errorf("smoke failed: %d expectation(s) failed", res.Failed)
	}
	return nil
}

func auditCmd(args []string, out io.Writer) error {
	if len(args) == 0 || args[0] != "release" {
		return errors.New("usage: mmdbforge audit release --old old.mmdb --new new.mmdb")
	}
	fs := flag.NewFlagSet("audit release", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var opts audit.Options
	var markdown markdownFlag
	fs.StringVar(&opts.OldDB, "old", "", "old database")
	fs.StringVar(&opts.NewDB, "new", "", "new database")
	fs.StringVar(&opts.SchemaPath, "schema", "", "schema file")
	fs.StringVar(&opts.SmokePath, "smoke", "", "smoke file")
	fs.IntVar(&opts.Sample, "sample", 10000, "sample size")
	fs.Var(&markdown, "markdown", "write markdown to stdout or optional path")
	if err := fs.Parse(normalizeOptionalValue(args[1:], "--markdown")); err != nil {
		return err
	}
	res, err := audit.Release(opts)
	if err != nil {
		return err
	}
	if markdown.set {
		return writeMarkdown(out, markdown.path, report.AuditMarkdown(res))
	}
	return report.JSON(out, res)
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func parseThreshold(s string) (string, float64, error) {
	parts := strings.SplitN(s, "=", 2)
	if len(parts) != 2 {
		return "", 0, fmt.Errorf("invalid threshold %q", s)
	}
	var value float64
	if _, err := fmt.Sscanf(parts[1], "%f", &value); err != nil {
		return "", 0, fmt.Errorf("invalid threshold value %q", parts[1])
	}
	return parts[0], value, nil
}

func writeMarkdown(out io.Writer, path, body string) error {
	if path == "" {
		_, err := fmt.Fprint(out, body)
		return err
	}
	return os.WriteFile(path, []byte(body), 0o644)
}

func normalizeOptionalValue(args []string, name string) []string {
	out := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == name {
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				out = append(out, name+"="+args[i+1])
				i++
				continue
			}
			out = append(out, name+"=")
			continue
		}
		out = append(out, arg)
	}
	return out
}

type markdownFlag struct {
	set  bool
	path string
}

func (f *markdownFlag) String() string { return f.path }
func (f *markdownFlag) Set(v string) error {
	f.set = true
	if v == "true" {
		v = ""
	}
	f.path = v
	return nil
}

func (f *markdownFlag) IsBoolFlag() bool { return true }
