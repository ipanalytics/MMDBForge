package diff

import (
	"sort"

	"mmdbforge/internal/mmdb"
)

type ExplainResult struct {
	IP               string         `json:"ip"`
	OldMatchedPrefix string         `json:"old_matched_prefix"`
	NewMatchedPrefix string         `json:"new_matched_prefix"`
	Changed          bool           `json:"changed"`
	ChangedFields    []FieldDelta   `json:"changed_fields"`
	OldRecord        map[string]any `json:"old_record"`
	NewRecord        map[string]any `json:"new_record"`
}

type FieldDelta struct {
	Field string `json:"field"`
	Old   any    `json:"old"`
	New   any    `json:"new"`
}

func Explain(oldPath, newPath, ip string) (ExplainResult, error) {
	oldEntry, err := mmdb.Lookup(oldPath, ip)
	if err != nil {
		return ExplainResult{}, err
	}
	newEntry, err := mmdb.Lookup(newPath, ip)
	if err != nil {
		return ExplainResult{}, err
	}
	res := ExplainResult{
		IP:               oldEntry.IP,
		OldMatchedPrefix: oldEntry.Network,
		NewMatchedPrefix: newEntry.Network,
		OldRecord:        oldEntry.Record,
		NewRecord:        newEntry.Record,
	}
	fields := unionFields(oldEntry.Record, newEntry.Record)
	for _, field := range fields {
		oldValue, oldOK := mmdb.Field(oldEntry.Record, field)
		newValue, newOK := mmdb.Field(newEntry.Record, field)
		if !oldOK {
			oldValue = nil
		}
		if !newOK {
			newValue = nil
		}
		if !mmdb.Equal(oldValue, newValue) {
			res.ChangedFields = append(res.ChangedFields, FieldDelta{Field: field, Old: oldValue, New: newValue})
		}
	}
	sort.Slice(res.ChangedFields, func(i, j int) bool { return res.ChangedFields[i].Field < res.ChangedFields[j].Field })
	res.Changed = len(res.ChangedFields) > 0 || oldEntry.Network != newEntry.Network
	return res, nil
}
