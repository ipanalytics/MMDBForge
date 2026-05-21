package audit

import (
	"fmt"

	"mmdbforge/internal/diff"
	"mmdbforge/internal/schema"
	"mmdbforge/internal/smoke"
	"mmdbforge/internal/stats"
)

type Options struct {
	OldDB      string
	NewDB      string
	SchemaPath string
	SmokePath  string
	Sample     int
}

type Result struct {
	Verdict  string         `json:"verdict"`
	Summary  []string       `json:"summary"`
	Failures []string       `json:"failures,omitempty"`
	Diff     diff.Result    `json:"diff"`
	Schema   *schema.Result `json:"schema,omitempty"`
	Smoke    *smoke.Result  `json:"smoke,omitempty"`
	OldStats stats.Result   `json:"old_stats"`
	NewStats stats.Result   `json:"new_stats"`
}

func Release(opts Options) (Result, error) {
	if opts.OldDB == "" || opts.NewDB == "" {
		return Result{}, fmt.Errorf("--old and --new are required")
	}
	d, err := diff.Run(opts.OldDB, opts.NewDB, diff.Options{Sample: opts.Sample})
	if err != nil {
		return Result{}, err
	}
	oldStats, err := stats.Run(opts.OldDB, opts.Sample, 5)
	if err != nil {
		return Result{}, err
	}
	newStats, err := stats.Run(opts.NewDB, opts.Sample, 5)
	if err != nil {
		return Result{}, err
	}
	res := Result{Verdict: "PASS", Diff: d, OldStats: oldStats, NewStats: newStats}
	res.Summary = append(res.Summary, fmt.Sprintf("%.2f%% sampled records changed", d.ChangedPercent))
	if oldStats.FileSizeMB > 0 {
		delta := (newStats.FileSizeMB - oldStats.FileSizeMB) * 100 / oldStats.FileSizeMB
		res.Summary = append(res.Summary, fmt.Sprintf("file size changed %.2f%%", delta))
		if delta > 50 {
			res.Verdict = "WARN"
			res.Failures = append(res.Failures, fmt.Sprintf("file size increased %.2f%%", delta))
		}
	}
	if opts.SchemaPath != "" {
		sr, err := schema.Validate(opts.SchemaPath, opts.NewDB, opts.Sample)
		if err != nil {
			return Result{}, err
		}
		res.Schema = &sr
		res.Summary = append(res.Summary, fmt.Sprintf("schema validation errors: %d", len(sr.Errors)))
		if len(sr.Errors) > 0 {
			res.Verdict = "FAIL"
			res.Failures = append(res.Failures, fmt.Sprintf("schema validation failed with %d error(s)", len(sr.Errors)))
		}
	}
	if opts.SmokePath != "" {
		sm, err := smoke.Run(opts.NewDB, opts.SmokePath)
		if err != nil {
			return Result{}, err
		}
		res.Smoke = &sm
		res.Summary = append(res.Summary, fmt.Sprintf("smoke tests failed: %d", sm.Failed))
		if sm.Failed > 0 {
			res.Verdict = "FAIL"
			res.Failures = append(res.Failures, fmt.Sprintf("smoke failed with %d failed case(s)", sm.Failed))
		}
	}
	return res, nil
}
