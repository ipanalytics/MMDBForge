package report

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"mmdbforge/internal/audit"
	"mmdbforge/internal/bench"
	"mmdbforge/internal/diff"
	"mmdbforge/internal/prefixes"
	"mmdbforge/internal/stats"
)

func JSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
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
