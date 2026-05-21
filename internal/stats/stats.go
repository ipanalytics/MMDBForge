package stats

import (
	"os"
	"sort"

	"mmdbforge/internal/mmdb"
)

type Result struct {
	DatabaseType   string             `json:"database_type"`
	IPVersion      []string           `json:"ip_version"`
	BuildEpoch     uint               `json:"build_epoch"`
	NodeCount      uint               `json:"node_count"`
	FileSizeMB     float64            `json:"file_size_mb"`
	CheckedRecords int                `json:"checked_records"`
	FieldCoverage  map[string]float64 `json:"field_coverage"`
	TopValues      map[string][][]any `json:"top_values"`
}

type FieldsResult struct {
	CheckedRecords int      `json:"checked_records"`
	Fields         []string `json:"fields"`
}

type DiffResult struct {
	OldCheckedRecords int                     `json:"old_checked_records"`
	NewCheckedRecords int                     `json:"new_checked_records"`
	CoverageChanges   map[string]CoverageDiff `json:"coverage_changes"`
	AddedFields       []string                `json:"added_fields,omitempty"`
	RemovedFields     []string                `json:"removed_fields,omitempty"`
}

type CoverageDiff struct {
	OldCoverage float64 `json:"old_coverage"`
	NewCoverage float64 `json:"new_coverage"`
	Delta       float64 `json:"delta"`
}

func Run(path string, sample, top int) (Result, error) {
	db, err := mmdb.Open(path)
	if err != nil {
		return Result{}, err
	}
	defer db.Close()
	entries, err := mmdb.SampleReader(db, sample)
	if err != nil {
		return Result{}, err
	}
	info, _ := os.Stat(path)
	fieldCounts := map[string]int{}
	valueCounts := map[string]map[string]valueCount{}
	for _, entry := range entries {
		flat := mmdb.Flatten(entry.Record)
		for field, value := range flat {
			fieldCounts[field]++
			if scalar(value) {
				if valueCounts[field] == nil {
					valueCounts[field] = map[string]valueCount{}
				}
				key := mmdb.JSONKey(value)
				item := valueCounts[field][key]
				item.Value = value
				item.Count++
				valueCounts[field][key] = item
			}
		}
	}
	res := Result{
		DatabaseType:   db.Metadata.DatabaseType,
		IPVersion:      ipVersions(db.Metadata.IPVersion),
		BuildEpoch:     db.Metadata.BuildEpoch,
		NodeCount:      db.Metadata.NodeCount,
		CheckedRecords: len(entries),
		FieldCoverage:  map[string]float64{},
		TopValues:      map[string][][]any{},
	}
	if info != nil {
		res.FileSizeMB = float64(info.Size()) / 1024 / 1024
	}
	for field, count := range fieldCounts {
		if len(entries) > 0 {
			res.FieldCoverage[field] = float64(count) * 100 / float64(len(entries))
		}
	}
	for field, counts := range valueCounts {
		values := make([]valueCount, 0, len(counts))
		for _, item := range counts {
			values = append(values, item)
		}
		sort.Slice(values, func(i, j int) bool { return values[i].Count > values[j].Count })
		if len(values) > top {
			values = values[:top]
		}
		for _, item := range values {
			res.TopValues[field] = append(res.TopValues[field], []any{item.Value, item.Count})
		}
	}
	return res, nil
}

func Fields(path string, sample int) (FieldsResult, error) {
	entries, err := mmdb.Sample(path, sample)
	if err != nil {
		return FieldsResult{}, err
	}
	seen := map[string]bool{}
	for _, entry := range entries {
		for _, field := range mmdb.Fields(entry.Record) {
			seen[field] = true
		}
	}
	fields := make([]string, 0, len(seen))
	for field := range seen {
		fields = append(fields, field)
	}
	sort.Strings(fields)
	return FieldsResult{CheckedRecords: len(entries), Fields: fields}, nil
}

func Diff(oldPath, newPath string, sample int) (DiffResult, error) {
	oldStats, err := Run(oldPath, sample, 0)
	if err != nil {
		return DiffResult{}, err
	}
	newStats, err := Run(newPath, sample, 0)
	if err != nil {
		return DiffResult{}, err
	}
	res := DiffResult{
		OldCheckedRecords: oldStats.CheckedRecords,
		NewCheckedRecords: newStats.CheckedRecords,
		CoverageChanges:   map[string]CoverageDiff{},
	}
	keys := map[string]bool{}
	for field := range oldStats.FieldCoverage {
		keys[field] = true
	}
	for field := range newStats.FieldCoverage {
		keys[field] = true
	}
	for field := range keys {
		oldCoverage, oldOK := oldStats.FieldCoverage[field]
		newCoverage, newOK := newStats.FieldCoverage[field]
		switch {
		case !oldOK && newOK:
			res.AddedFields = append(res.AddedFields, field)
		case oldOK && !newOK:
			res.RemovedFields = append(res.RemovedFields, field)
		}
		delta := newCoverage - oldCoverage
		if delta != 0 {
			res.CoverageChanges[field] = CoverageDiff{OldCoverage: oldCoverage, NewCoverage: newCoverage, Delta: delta}
		}
	}
	sort.Strings(res.AddedFields)
	sort.Strings(res.RemovedFields)
	return res, nil
}

type valueCount struct {
	Value any
	Count int
}

func scalar(v any) bool {
	switch v.(type) {
	case nil, string, bool, int64, int, uint, uint64, float64, float32:
		return true
	default:
		return false
	}
}

func ipVersions(v uint) []string {
	switch v {
	case 4:
		return []string{"ipv4"}
	case 6:
		return []string{"ipv4", "ipv6"}
	default:
		return nil
	}
}
