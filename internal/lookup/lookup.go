package lookup

import (
	"encoding/json"
	"path/filepath"

	"mmdbforge/internal/mmdb"
)

type InspectResult struct {
	IP            string         `json:"ip"`
	Database      string         `json:"database"`
	MatchedPrefix string         `json:"matched_prefix"`
	Record        map[string]any `json:"record"`
}

type ExplainResult struct {
	IP              string         `json:"ip"`
	Database        string         `json:"database"`
	MatchedPrefix   string         `json:"matched_prefix"`
	PrefixLength    int            `json:"prefix_length"`
	RecordSizeBytes int            `json:"record_size_bytes"`
	Fields          []string       `json:"fields"`
	Warnings        []string       `json:"warnings,omitempty"`
	Record          map[string]any `json:"record"`
}

func Inspect(path, ip string) (InspectResult, error) {
	entry, err := mmdb.Lookup(path, ip)
	if err != nil {
		return InspectResult{}, err
	}
	return InspectResult{
		IP:            entry.IP,
		Database:      filepath.Base(path),
		MatchedPrefix: entry.Network,
		Record:        entry.Record,
	}, nil
}

func Explain(path, ip string) (ExplainResult, error) {
	entry, err := mmdb.Lookup(path, ip)
	if err != nil {
		return ExplainResult{}, err
	}
	body, _ := json.Marshal(entry.Record)
	fields := mmdb.Fields(entry.Record)
	res := ExplainResult{
		IP:              entry.IP,
		Database:        filepath.Base(path),
		MatchedPrefix:   entry.Network,
		PrefixLength:    prefixLength(entry.Network),
		RecordSizeBytes: len(body),
		Fields:          fields,
		Record:          entry.Record,
	}
	res.Warnings = explainWarnings(res, entry.Record)
	return res, nil
}

func prefixLength(network string) int {
	for i := len(network) - 1; i >= 0; i-- {
		if network[i] == '/' {
			var n int
			for _, ch := range network[i+1:] {
				if ch < '0' || ch > '9' {
					return 0
				}
				n = n*10 + int(ch-'0')
			}
			return n
		}
	}
	return 0
}

func explainWarnings(res ExplainResult, record map[string]any) []string {
	var warnings []string
	if res.PrefixLength == 32 || res.PrefixLength == 128 {
		warnings = append(warnings, "matched host-level record; check if this database intentionally stores host-level entries")
	}
	if score, ok := mmdb.Field(record, "risk_score"); ok {
		if n, ok := mmdb.AsFloat(score); ok && n >= 80 {
			if _, exists := mmdb.Field(record, "risk_reasons"); !exists {
				warnings = append(warnings, "risk_score is high while risk_reasons field is missing")
			}
		}
	}
	if confidence, ok := mmdb.Field(record, "confidence"); ok {
		if n, ok := mmdb.AsFloat(confidence); ok && (n < 0 || n > 100) {
			warnings = append(warnings, "confidence is outside expected 0..100 range")
		}
	}
	return warnings
}
