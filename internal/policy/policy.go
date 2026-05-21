package policy

import (
	"encoding/json"
	"fmt"
	"os"

	"mmdbforge/internal/audit"
)

type Policy struct {
	MaxChangedPercent         *float64 `json:"max_changed_percent"`
	MaxFileGrowthPercent      *float64 `json:"max_file_growth_percent"`
	MaxLookupSlowdownPercent  *float64 `json:"max_lookup_slowdown_percent"`
	MaxHostLevelGrowthPercent *float64 `json:"max_host_level_growth_percent"`
	RequiredFields            []string `json:"required_fields"`
	AllowedDroppedFields      []string `json:"allowed_dropped_fields"`
}

type Result struct {
	Passed   bool     `json:"passed"`
	Failures []string `json:"failures,omitempty"`
}

func Load(path string) (Policy, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return Policy{}, err
	}
	var p Policy
	if err := json.Unmarshal(body, &p); err != nil {
		return Policy{}, err
	}
	return p, nil
}

func Evaluate(path string, res audit.Result) (Result, error) {
	p, err := Load(path)
	if err != nil {
		return Result{}, err
	}
	out := Result{Passed: true}
	if p.MaxChangedPercent != nil && res.Diff.ChangedPercent > *p.MaxChangedPercent {
		out.Failures = append(out.Failures, fmt.Sprintf("changed_percent %.2f exceeds %.2f", res.Diff.ChangedPercent, *p.MaxChangedPercent))
	}
	if p.MaxFileGrowthPercent != nil && res.OldStats.FileSizeMB > 0 {
		growth := (res.NewStats.FileSizeMB - res.OldStats.FileSizeMB) * 100 / res.OldStats.FileSizeMB
		if growth > *p.MaxFileGrowthPercent {
			out.Failures = append(out.Failures, fmt.Sprintf("file growth %.2f%% exceeds %.2f%%", growth, *p.MaxFileGrowthPercent))
		}
	}
	if p.MaxLookupSlowdownPercent != nil {
		slowdown := -res.Benchmark.LookupsPerSecChangePercent
		if slowdown > *p.MaxLookupSlowdownPercent {
			out.Failures = append(out.Failures, fmt.Sprintf("lookup slowdown %.2f%% exceeds %.2f%%", slowdown, *p.MaxLookupSlowdownPercent))
		}
	}
	if p.MaxHostLevelGrowthPercent != nil && res.Prefixes.Old.HostLevel > 0 {
		growth := float64(res.Prefixes.New.HostLevel-res.Prefixes.Old.HostLevel) * 100 / float64(res.Prefixes.Old.HostLevel)
		if growth > *p.MaxHostLevelGrowthPercent {
			out.Failures = append(out.Failures, fmt.Sprintf("host-level growth %.2f%% exceeds %.2f%%", growth, *p.MaxHostLevelGrowthPercent))
		}
	}
	allowed := map[string]bool{}
	for _, field := range p.AllowedDroppedFields {
		allowed[field] = true
	}
	for _, field := range res.CoverageDiff.RemovedFields {
		if !allowed[field] {
			out.Failures = append(out.Failures, fmt.Sprintf("field %s was removed", field))
		}
	}
	for _, field := range p.RequiredFields {
		if res.NewStats.FieldCoverage[field] <= 0 {
			out.Failures = append(out.Failures, fmt.Sprintf("required field %s is missing", field))
		}
	}
	out.Passed = len(out.Failures) == 0
	return out, nil
}
