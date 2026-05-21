package report

import (
	"encoding/json"
	"fmt"
	"html"
	"io"
	"sort"
	"strings"

	"mmdbforge/internal/audit"
	"mmdbforge/internal/bench"
	"mmdbforge/internal/diff"
	"mmdbforge/internal/prefixes"
	"mmdbforge/internal/stats"
	"mmdbforge/internal/testbench"
)

func JSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func AuditHTML(res audit.Result) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<!doctype html><html><head><meta charset=\"utf-8\"><title>MMDB Release Audit</title>")
	fmt.Fprintf(&b, "<style>body{font-family:-apple-system,BlinkMacSystemFont,Segoe UI,sans-serif;margin:32px;color:#17202a}h1,h2{margin-top:28px}.verdict{display:inline-block;padding:6px 10px;border-radius:6px;background:#eef2ff;font-weight:700}.fail{background:#fee2e2}.warn{background:#fef3c7}.pass{background:#dcfce7}table{border-collapse:collapse;width:100%%;margin:12px 0}th,td{border:1px solid #d8dee9;padding:8px;text-align:left}th{background:#f6f8fa}code{background:#f6f8fa;padding:2px 4px;border-radius:4px}.num{text-align:right}</style></head><body>")
	class := strings.ToLower(res.Verdict)
	fmt.Fprintf(&b, "<h1>MMDB Release Audit</h1><p>Verdict: <span class=\"verdict %s\">%s</span></p>", class, html.EscapeString(res.Verdict))
	fmt.Fprintf(&b, "<h2>Summary</h2><ul>")
	for _, item := range res.Summary {
		fmt.Fprintf(&b, "<li>%s</li>", html.EscapeString(item))
	}
	fmt.Fprintf(&b, "</ul>")
	if len(res.Failures) > 0 {
		fmt.Fprintf(&b, "<h2>Failures</h2><ul>")
		for _, failure := range res.Failures {
			fmt.Fprintf(&b, "<li>%s</li>", html.EscapeString(failure))
		}
		fmt.Fprintf(&b, "</ul>")
	}
	fmt.Fprintf(&b, "<h2>Diff</h2><table><tr><th>Metric</th><th>Value</th></tr><tr><td>Sample size</td><td class=\"num\">%d</td></tr><tr><td>Changed records</td><td class=\"num\">%d</td></tr><tr><td>Changed percent</td><td class=\"num\">%.2f%%</td></tr></table>", res.Diff.SampleSize, res.Diff.ChangedRecords, res.Diff.ChangedPercent)
	if len(res.Diff.TopChanges) > 0 {
		fmt.Fprintf(&b, "<h2>Top Changes</h2><table><tr><th>Field</th><th>Old</th><th>New</th><th>Count</th></tr>")
		for _, ch := range res.Diff.TopChanges {
			fmt.Fprintf(&b, "<tr><td><code>%s</code></td><td>%s</td><td>%s</td><td class=\"num\">%d</td></tr>", html.EscapeString(ch.Field), html.EscapeString(fmt.Sprint(ch.Old)), html.EscapeString(fmt.Sprint(ch.New)), ch.Count)
		}
		fmt.Fprintf(&b, "</table>")
	}
	fmt.Fprintf(&b, "<h2>Field Coverage Diff</h2><table><tr><th>Field</th><th>Old</th><th>New</th><th>Delta</th></tr>")
	fields := make([]string, 0, len(res.CoverageDiff.CoverageChanges))
	for field := range res.CoverageDiff.CoverageChanges {
		fields = append(fields, field)
	}
	sort.Strings(fields)
	for _, field := range fields {
		ch := res.CoverageDiff.CoverageChanges[field]
		fmt.Fprintf(&b, "<tr><td><code>%s</code></td><td class=\"num\">%.2f%%</td><td class=\"num\">%.2f%%</td><td class=\"num\">%+.2f</td></tr>", html.EscapeString(field), ch.OldCoverage, ch.NewCoverage, ch.Delta)
	}
	fmt.Fprintf(&b, "</table>")
	fmt.Fprintf(&b, "<h2>Benchmark</h2><table><tr><th>Version</th><th>Lookups</th><th>Lookups/sec</th><th>Avg lookup us</th></tr><tr><td>Old</td><td class=\"num\">%d</td><td class=\"num\">%.2f</td><td class=\"num\">%.2f</td></tr><tr><td>New</td><td class=\"num\">%d</td><td class=\"num\">%.2f</td><td class=\"num\">%.2f</td></tr></table>", res.Benchmark.Old.Lookups, res.Benchmark.Old.LookupsPerSec, res.Benchmark.Old.AvgLookupMicros, res.Benchmark.New.Lookups, res.Benchmark.New.LookupsPerSec, res.Benchmark.New.AvgLookupMicros)
	fmt.Fprintf(&b, "</body></html>")
	return b.String()
}

func DiffMarkdown(res diff.Result) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# MMDB Diff\n\n")
	fmt.Fprintf(&b, "- Sample size: %d\n", res.SampleSize)
	fmt.Fprintf(&b, "- Changed records: %d\n", res.ChangedRecords)
	fmt.Fprintf(&b, "- Changed percent: %.2f%%\n", res.ChangedPercent)
	if len(res.Failures) > 0 {
		fmt.Fprintf(&b, "- Guard failures: %s\n", strings.Join(res.Failures, "; "))
	}
	if len(res.TopChanges) > 0 {
		fmt.Fprintf(&b, "\n## Top Changes\n\n| Field | Old | New | Count |\n|---|---:|---:|---:|\n")
		for _, ch := range res.TopChanges {
			fmt.Fprintf(&b, "| `%s` | `%v` | `%v` | %d |\n", ch.Field, ch.Old, ch.New, ch.Count)
		}
	}
	if len(res.FieldChanges) > 0 {
		fmt.Fprintf(&b, "\n## Field Changes\n\n")
		fields := make([]string, 0, len(res.FieldChanges))
		for f := range res.FieldChanges {
			fields = append(fields, f)
		}
		sort.Strings(fields)
		for _, field := range fields {
			fmt.Fprintf(&b, "- `%s`: ", field)
			stats := res.FieldChanges[field]
			fmt.Fprintf(&b, "changed=%d added=%d removed=%d true->false=%d false->true=%d\n", stats.Changed, stats.Added, stats.Removed, stats.TrueToFalse, stats.FalseToTrue)
		}
	}
	return b.String()
}

func AuditMarkdown(res audit.Result) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# MMDB Release Audit\n\n")
	fmt.Fprintf(&b, "Verdict: %s\n\n", res.Verdict)
	fmt.Fprintf(&b, "## Summary\n\n")
	for _, item := range res.Summary {
		fmt.Fprintf(&b, "- %s\n", item)
	}
	if len(res.Failures) > 0 {
		fmt.Fprintf(&b, "\n## Failures\n\n")
		for _, failure := range res.Failures {
			fmt.Fprintf(&b, "- %s\n", failure)
		}
	}
	fmt.Fprintf(&b, "\n## Diff\n\n")
	fmt.Fprintf(&b, "- Sample size: %d\n", res.Diff.SampleSize)
	fmt.Fprintf(&b, "- Changed records: %d\n", res.Diff.ChangedRecords)
	fmt.Fprintf(&b, "- Changed percent: %.2f%%\n", res.Diff.ChangedPercent)
	if len(res.Diff.TopChanges) > 0 {
		fmt.Fprintf(&b, "\n### Top Changes\n\n| Field | Old | New | Count |\n|---|---|---|---:|\n")
		for _, ch := range res.Diff.TopChanges {
			fmt.Fprintf(&b, "| `%s` | `%v` | `%v` | %d |\n", ch.Field, ch.Old, ch.New, ch.Count)
		}
	}
	fmt.Fprintf(&b, "\n## Field Coverage Diff\n\n")
	writeCoverageDiff(&b, res.CoverageDiff)
	if len(res.Prefixes.Warnings) > 0 {
		fmt.Fprintf(&b, "\n## Prefix Warnings\n\n")
		for _, warning := range res.Prefixes.Warnings {
			fmt.Fprintf(&b, "- %s\n", warning)
		}
	}
	fmt.Fprintf(&b, "\n## Benchmark\n\n")
	fmt.Fprintf(&b, "| Version | Lookups | Lookups/sec | Avg lookup us |\n|---|---:|---:|---:|\n")
	fmt.Fprintf(&b, "| Old | %d | %.2f | %.2f |\n", res.Benchmark.Old.Lookups, res.Benchmark.Old.LookupsPerSec, res.Benchmark.Old.AvgLookupMicros)
	fmt.Fprintf(&b, "| New | %d | %.2f | %.2f |\n", res.Benchmark.New.Lookups, res.Benchmark.New.LookupsPerSec, res.Benchmark.New.AvgLookupMicros)
	return b.String()
}

func StatsTable(w io.Writer, res stats.Result) error {
	_, err := fmt.Fprintf(w, "database_type\t%s\nfile_size_mb\t%.2f\nchecked_records\t%d\nnode_count\t%d\n\nFIELD\tCOVERAGE\n", res.DatabaseType, res.FileSizeMB, res.CheckedRecords, res.NodeCount)
	if err != nil {
		return err
	}
	fields := make([]string, 0, len(res.FieldCoverage))
	for field := range res.FieldCoverage {
		fields = append(fields, field)
	}
	sort.Strings(fields)
	for _, field := range fields {
		if _, err := fmt.Fprintf(w, "%s\t%.2f%%\n", field, res.FieldCoverage[field]); err != nil {
			return err
		}
	}
	return nil
}

func CoverageDiffTable(w io.Writer, res stats.DiffResult) error {
	_, err := fmt.Fprintln(w, "FIELD\tOLD\tNEW\tDELTA")
	if err != nil {
		return err
	}
	fields := make([]string, 0, len(res.CoverageChanges))
	for field := range res.CoverageChanges {
		fields = append(fields, field)
	}
	sort.Strings(fields)
	for _, field := range fields {
		ch := res.CoverageChanges[field]
		if _, err := fmt.Fprintf(w, "%s\t%.2f%%\t%.2f%%\t%+.2f\n", field, ch.OldCoverage, ch.NewCoverage, ch.Delta); err != nil {
			return err
		}
	}
	return nil
}

func PrefixTable(w io.Writer, res prefixes.Result) error {
	if _, err := fmt.Fprintf(w, "checked_networks\t%d\nhost_level\t%d\n\nPREFIX_LENGTH\tCOUNT\n", res.CheckedNetworks, res.HostLevel); err != nil {
		return err
	}
	for _, length := range prefixes.Lengths(res.ByPrefixLength) {
		if _, err := fmt.Fprintf(w, "/%d\t%d\n", length, res.ByPrefixLength[length]); err != nil {
			return err
		}
	}
	return nil
}

func BenchTable(w io.Writer, res bench.Result) error {
	_, err := fmt.Fprintf(w, "database\t%s\nlookups\t%d\nelapsed_ms\t%.2f\nlookups_per_sec\t%.2f\navg_lookup_micros\t%.2f\n", res.Database, res.Lookups, res.ElapsedMS, res.LookupsPerSec, res.AvgLookupMicros)
	return err
}

func TestbenchMarkdown(res testbench.CompareResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# MMDB Testbench Compare\n\n")
	fmt.Fprintf(&b, "- Checked: %d\n", res.Checked)
	fmt.Fprintf(&b, "- Changed: %d\n", res.Changed)
	fmt.Fprintf(&b, "- Regressions: %d\n", res.Regressions)
	if len(res.Changes) > 0 {
		fmt.Fprintf(&b, "\n## Changes\n\n| IP | Passed before | Passed after | Changed fields |\n|---|---:|---:|---:|\n")
		for _, ch := range res.Changes {
			fmt.Fprintf(&b, "| `%s` | %v | %v | %d |\n", ch.IP, ch.PassedBefore, ch.PassedAfter, len(ch.FieldChanges))
		}
	}
	return b.String()
}

func TestbenchHTML(res testbench.CompareResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<!doctype html><html><head><meta charset=\"utf-8\"><title>MMDB Testbench</title>")
	fmt.Fprintf(&b, "<style>body{font-family:-apple-system,BlinkMacSystemFont,Segoe UI,sans-serif;margin:32px;color:#17202a}table{border-collapse:collapse;width:100%%}th,td{border:1px solid #d8dee9;padding:8px}th{background:#f6f8fa}.bad{background:#fee2e2}.ok{background:#dcfce7}.num{text-align:right}code{background:#f6f8fa;padding:2px 4px;border-radius:4px}</style></head><body>")
	fmt.Fprintf(&b, "<h1>MMDB Testbench Compare</h1><ul><li>Checked: %d</li><li>Changed: %d</li><li>Regressions: %d</li></ul>", res.Checked, res.Changed, res.Regressions)
	fmt.Fprintf(&b, "<table><tr><th>IP</th><th>Passed before</th><th>Passed after</th><th>Changed fields</th></tr>")
	for _, ch := range res.Changes {
		class := "ok"
		if ch.PassedBefore && !ch.PassedAfter {
			class = "bad"
		}
		fmt.Fprintf(&b, "<tr class=\"%s\"><td><code>%s</code></td><td>%v</td><td>%v</td><td class=\"num\">%d</td></tr>", class, html.EscapeString(ch.IP), ch.PassedBefore, ch.PassedAfter, len(ch.FieldChanges))
	}
	fmt.Fprintf(&b, "</table></body></html>")
	return b.String()
}

func writeCoverageDiff(b *strings.Builder, res stats.DiffResult) {
	if len(res.AddedFields) > 0 {
		fmt.Fprintf(b, "Added fields: `%s`\n\n", strings.Join(res.AddedFields, "`, `"))
	}
	if len(res.RemovedFields) > 0 {
		fmt.Fprintf(b, "Removed fields: `%s`\n\n", strings.Join(res.RemovedFields, "`, `"))
	}
	if len(res.CoverageChanges) == 0 {
		fmt.Fprintf(b, "No field coverage changes in sampled records.\n")
		return
	}
	fmt.Fprintf(b, "| Field | Old | New | Delta |\n|---|---:|---:|---:|\n")
	fields := make([]string, 0, len(res.CoverageChanges))
	for field := range res.CoverageChanges {
		fields = append(fields, field)
	}
	sort.Strings(fields)
	for _, field := range fields {
		ch := res.CoverageChanges[field]
		fmt.Fprintf(b, "| `%s` | %.2f%% | %.2f%% | %+.2f |\n", field, ch.OldCoverage, ch.NewCoverage, ch.Delta)
	}
}
