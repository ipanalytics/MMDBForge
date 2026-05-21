package smoke

import (
	"encoding/json"
	"fmt"
	"os"

	"mmdbforge/internal/mmdb"
)

type Case struct {
	IP     string         `json:"ip"`
	Expect map[string]any `json:"expect"`
	Allow  map[string]any `json:"allow,omitempty"`
	Deny   map[string]any `json:"deny,omitempty"`
}

type Result struct {
	Checked int          `json:"checked"`
	Passed  int          `json:"passed"`
	Failed  int          `json:"failed"`
	Results []CaseResult `json:"results"`
}

type CaseResult struct {
	IP       string    `json:"ip"`
	Passed   bool      `json:"passed"`
	Failures []Failure `json:"failures,omitempty"`
}

type Failure struct {
	Field    string `json:"field"`
	Expected any    `json:"expected"`
	Actual   any    `json:"actual"`
	Message  string `json:"message"`
}

func Run(dbPath, smokePath string) (Result, error) {
	cases, err := load(smokePath)
	if err != nil {
		return Result{}, err
	}
	db, err := mmdb.Open(dbPath)
	if err != nil {
		return Result{}, err
	}
	defer db.Close()
	var res Result
	for _, tc := range cases {
		entry, err := mmdb.LookupReader(db, tc.IP)
		if err != nil {
			return Result{}, err
		}
		cr := CaseResult{IP: entry.IP, Passed: true}
		for field, expected := range tc.Expect {
			actual, ok := mmdb.Field(entry.Record, field)
			if !ok {
				cr.Passed = false
				cr.Failures = append(cr.Failures, Failure{Field: field, Expected: expected, Message: "field missing"})
				continue
			}
			if !mmdb.Equal(actual, expected) {
				cr.Passed = false
				cr.Failures = append(cr.Failures, Failure{Field: field, Expected: expected, Actual: actual, Message: "value mismatch"})
			}
		}
		for field, allowed := range tc.Allow {
			actual, ok := mmdb.Field(entry.Record, field)
			if !ok {
				cr.Passed = false
				cr.Failures = append(cr.Failures, Failure{Field: field, Expected: allowed, Message: "field missing"})
				continue
			}
			if !containsAllowed(allowed, actual) {
				cr.Passed = false
				cr.Failures = append(cr.Failures, Failure{Field: field, Expected: allowed, Actual: actual, Message: "value not in allow list"})
			}
		}
		for field, denied := range tc.Deny {
			actual, ok := mmdb.Field(entry.Record, field)
			if !ok {
				continue
			}
			if containsAllowed(denied, actual) {
				cr.Passed = false
				cr.Failures = append(cr.Failures, Failure{Field: field, Expected: denied, Actual: actual, Message: "value matched deny list"})
			}
		}
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

func containsAllowed(allowed any, actual any) bool {
	switch values := allowed.(type) {
	case []any:
		for _, value := range values {
			if mmdb.Equal(value, actual) {
				return true
			}
		}
		return false
	default:
		return mmdb.Equal(allowed, actual)
	}
}

func load(path string) ([]Case, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cases []Case
	if err := json.Unmarshal(body, &cases); err != nil {
		return nil, fmt.Errorf("parse smoke file: %w", err)
	}
	return cases, nil
}
