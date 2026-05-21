package report

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"mmdbforge/internal/audit"
	"mmdbforge/internal/diff"
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
	for _, item := range res.Summary {
		fmt.Fprintf(&b, "- %s\n", item)
	}
	if len(res.Failures) > 0 {
		fmt.Fprintf(&b, "\n## Failures\n\n")
		for _, failure := range res.Failures {
			fmt.Fprintf(&b, "- %s\n", failure)
		}
	}
	return b.String()
}
