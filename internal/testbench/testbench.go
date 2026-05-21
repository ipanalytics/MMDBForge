package testbench

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"

	"gopkg.in/yaml.v3"

	"mmdbforge/internal/mmdb"
	"mmdbforge/internal/smoke"
)

type Config struct {
	Name     string       `json:"name" yaml:"name"`
	Database string       `json:"database" yaml:"database"`
	Fields   []string     `json:"fields" yaml:"fields"`
	Cases    []smoke.Case `json:"cases" yaml:"cases"`
}

type RunResult struct {
	Name        string       `json:"name"`
	Database    string       `json:"database"`
	GeneratedAt string       `json:"generated_at"`
	Checked     int          `json:"checked"`
	Passed      int          `json:"passed"`
	Failed      int          `json:"failed"`
	Results     []CaseResult `json:"results"`
}

type CaseResult struct {
	IP            string          `json:"ip"`
	MatchedPrefix string          `json:"matched_prefix"`
	Passed        bool            `json:"passed"`
	Record        map[string]any  `json:"record"`
	Observed      map[string]any  `json:"observed,omitempty"`
	Failures      []smoke.Failure `json:"failures,omitempty"`
}

type CompareResult struct {
	Baseline    string        `json:"baseline"`
	Current     string        `json:"current"`
	Checked     int           `json:"checked"`
	Changed     int           `json:"changed"`
	Regressions int           `json:"regressions"`
	Changes     []CaseChanges `json:"changes"`
}

type CaseChanges struct {
	IP           string        `json:"ip"`
	PassedBefore bool          `json:"passed_before"`
	PassedAfter  bool          `json:"passed_after"`
	FieldChanges []FieldChange `json:"field_changes,omitempty"`
}

type FieldChange struct {
	Field string `json:"field"`
	Old   any    `json:"old"`
	New   any    `json:"new"`
}

func Run(configPath string) (RunResult, error) {
	cfg, err := LoadConfig(configPath)
	if err != nil {
		return RunResult{}, err
	}
	db, err := mmdb.Open(cfg.Database)
	if err != nil {
		return RunResult{}, err
	}
	defer db.Close()
	res := RunResult{Name: cfg.Name, Database: cfg.Database, GeneratedAt: time.Now().UTC().Format(time.RFC3339)}
	for _, tc := range cfg.Cases {
		entry, err := mmdb.LookupReader(db, tc.IP)
		if err != nil {
			return RunResult{}, err
		}
		cr := CaseResult{IP: entry.IP, MatchedPrefix: entry.Network, Passed: true, Record: entry.Record, Observed: map[string]any{}}
		for _, field := range cfg.Fields {
			if value, ok := mmdb.Field(entry.Record, field); ok {
				cr.Observed[field] = value
			}
		}
		checkCase(tc, entry.Record, &cr)
		res.Checked++
		if cr.Passed {
			res.Passed++
		} else {
			res.Failed++
		}
		res.Results = append(res.Results, cr)
	}
	return res, nil
}

func Compare(baselinePath, currentPath string) (CompareResult, error) {
	base, err := LoadRun(baselinePath)
	if err != nil {
		return CompareResult{}, err
	}
	cur, err := LoadRun(currentPath)
	if err != nil {
		return CompareResult{}, err
	}
	curByIP := map[string]CaseResult{}
	for _, item := range cur.Results {
		curByIP[item.IP] = item
	}
	res := CompareResult{Baseline: baselinePath, Current: currentPath}
	for _, before := range base.Results {
		after, ok := curByIP[before.IP]
		if !ok {
			res.Changes = append(res.Changes, CaseChanges{IP: before.IP, PassedBefore: before.Passed, PassedAfter: false})
			res.Changed++
			res.Regressions++
			continue
		}
		res.Checked++
		change := CaseChanges{IP: before.IP, PassedBefore: before.Passed, PassedAfter: after.Passed}
		fields := unionFields(before.Record, after.Record)
		for _, field := range fields {
			oldValue, oldOK := mmdb.Field(before.Record, field)
			newValue, newOK := mmdb.Field(after.Record, field)
			if !oldOK {
				oldValue = nil
			}
			if !newOK {
				newValue = nil
			}
			if !mmdb.Equal(oldValue, newValue) {
				change.FieldChanges = append(change.FieldChanges, FieldChange{Field: field, Old: oldValue, New: newValue})
			}
		}
		if len(change.FieldChanges) > 0 || before.Passed != after.Passed {
			res.Changed++
			res.Changes = append(res.Changes, change)
		}
		if before.Passed && !after.Passed {
			res.Regressions++
		}
	}
	return res, nil
}

func LoadConfig(path string) (Config, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := yaml.Unmarshal(body, &cfg); err != nil {
		return Config{}, err
	}
	if cfg.Database == "" {
		return Config{}, fmt.Errorf("config database is required")
	}
	return cfg, nil
}

func LoadRun(path string) (RunResult, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return RunResult{}, err
	}
	var res RunResult
	if err := json.Unmarshal(body, &res); err != nil {
		return RunResult{}, err
	}
	return res, nil
}

func checkCase(tc smoke.Case, record map[string]any, cr *CaseResult) {
	for field, expected := range tc.Expect {
		actual, ok := mmdb.Field(record, field)
		if !ok {
			cr.Passed = false
			cr.Failures = append(cr.Failures, smoke.Failure{Field: field, Expected: expected, Message: "field missing"})
			continue
		}
		if !mmdb.Equal(actual, expected) {
			cr.Passed = false
			cr.Failures = append(cr.Failures, smoke.Failure{Field: field, Expected: expected, Actual: actual, Message: "value mismatch"})
		}
	}
	for field, allowed := range tc.Allow {
		actual, ok := mmdb.Field(record, field)
		if !ok {
			cr.Passed = false
			cr.Failures = append(cr.Failures, smoke.Failure{Field: field, Expected: allowed, Message: "field missing"})
			continue
		}
		if !contains(allowed, actual) {
			cr.Passed = false
			cr.Failures = append(cr.Failures, smoke.Failure{Field: field, Expected: allowed, Actual: actual, Message: "value not in allow list"})
		}
	}
	for field, denied := range tc.Deny {
		actual, ok := mmdb.Field(record, field)
		if ok && contains(denied, actual) {
			cr.Passed = false
			cr.Failures = append(cr.Failures, smoke.Failure{Field: field, Expected: denied, Actual: actual, Message: "value matched deny list"})
		}
	}
}

func contains(values any, actual any) bool {
	switch xs := values.(type) {
	case []any:
		for _, x := range xs {
			if mmdb.Equal(x, actual) {
				return true
			}
		}
		return false
	default:
		return mmdb.Equal(xs, actual)
	}
}

func unionFields(a, b map[string]any) []string {
	seen := map[string]bool{}
	for _, field := range mmdb.Fields(a) {
		seen[field] = true
	}
	for _, field := range mmdb.Fields(b) {
		seen[field] = true
	}
	fields := make([]string, 0, len(seen))
	for field := range seen {
		fields = append(fields, field)
	}
	sort.Strings(fields)
	return fields
}
