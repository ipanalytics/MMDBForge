package diff

import (
	"fmt"
	"sort"

	"mmdbforge/internal/mmdb"
)

type Options struct {
	Sample             int
	IPsFile            string
	Fields             []string
	FailThresholdName  string
	FailThresholdValue float64
	FailOnMissingField string
	FailOnChangeField  string
	FailOnDropField    string
}

type Result struct {
	SampleSize     int                    `json:"sample_size"`
	ChangedRecords int                    `json:"changed_records"`
	ChangedPercent float64                `json:"changed_percent"`
	FieldChanges   map[string]FieldChange `json:"field_changes"`
	TopChanges     []TopChange            `json:"top_changes"`
	Failures       []string               `json:"failures,omitempty"`
	Failed         bool                   `json:"failed"`
}

type FieldChange struct {
	Changed     int `json:"changed,omitempty"`
	Added       int `json:"added,omitempty"`
	Removed     int `json:"removed,omitempty"`
	FalseToTrue int `json:"false_to_true,omitempty"`
	TrueToFalse int `json:"true_to_false,omitempty"`
	Missing     int `json:"missing,omitempty"`
}

type TopChange struct {
	Field string `json:"field"`
	Old   any    `json:"old"`
	New   any    `json:"new"`
	Count int    `json:"count"`
}

type changeKey struct {
	field string
	old   string
	new   string
}

func Run(oldPath, newPath string, opts Options) (Result, error) {
	oldEntries, err := baseEntries(oldPath, opts)
	if err != nil {
		return Result{}, err
	}
	oldDB, err := mmdb.Open(oldPath)
	if err != nil {
		return Result{}, err
	}
	defer oldDB.Close()
	newDB, err := mmdb.Open(newPath)
	if err != nil {
		return Result{}, err
	}
	defer newDB.Close()

	res := Result{FieldChanges: map[string]FieldChange{}}
	topCounts := map[changeKey]TopChange{}
	for _, base := range oldEntries {
		oldEntry, err := mmdb.LookupReader(oldDB, base.IP)
		if err != nil {
			return Result{}, err
		}
		newEntry, err := mmdb.LookupReader(newDB, base.IP)
		if err != nil {
			return Result{}, err
		}
		res.SampleSize++
		if !mmdb.Equal(oldEntry.Record, newEntry.Record) {
			res.ChangedRecords++
		}
		fields := opts.Fields
		if len(fields) == 0 {
			fields = unionFields(oldEntry.Record, newEntry.Record)
		}
		for _, field := range fields {
			oldValue, oldOK := mmdb.Field(oldEntry.Record, field)
			newValue, newOK := mmdb.Field(newEntry.Record, field)
			if !oldOK && !newOK {
				continue
			}
			stat := res.FieldChanges[field]
			switch {
			case !oldOK && newOK:
				stat.Added++
			case oldOK && !newOK:
				stat.Removed++
			case !mmdb.Equal(oldValue, newValue):
				stat.Changed++
			}
			if opts.FailOnMissingField == field && !newOK {
				stat.Missing++
			}
			if oldBool, ok := oldValue.(bool); ok {
				if newBool, ok := newValue.(bool); ok && oldBool != newBool {
					if !oldBool && newBool {
						stat.FalseToTrue++
					} else {
						stat.TrueToFalse++
					}
				}
			}
			if !mmdb.Equal(oldValue, newValue) {
				key := changeKey{field: field, old: mmdb.JSONKey(oldValue), new: mmdb.JSONKey(newValue)}
				ch := topCounts[key]
				ch.Field = field
				ch.Old = oldValue
				ch.New = newValue
				ch.Count++
				topCounts[key] = ch
				if opts.FailOnChangeField == field {
					res.Failures = appendOnce(res.Failures, fmt.Sprintf("%s changed", field))
				}
				if opts.FailOnDropField == field && oldOK && !newOK {
					res.Failures = appendOnce(res.Failures, fmt.Sprintf("%s dropped", field))
				}
			}
			res.FieldChanges[field] = stat
		}
	}
	if res.SampleSize > 0 {
		res.ChangedPercent = float64(res.ChangedRecords) * 100 / float64(res.SampleSize)
	}
	for _, ch := range topCounts {
		res.TopChanges = append(res.TopChanges, ch)
	}
	sort.Slice(res.TopChanges, func(i, j int) bool { return res.TopChanges[i].Count > res.TopChanges[j].Count })
	if len(res.TopChanges) > 20 {
		res.TopChanges = res.TopChanges[:20]
	}
	if opts.FailThresholdName != "" {
		switch opts.FailThresholdName {
		case "changed_percent":
			if res.ChangedPercent > opts.FailThresholdValue {
				res.Failures = append(res.Failures, fmt.Sprintf("changed_percent %.2f exceeds %.2f", res.ChangedPercent, opts.FailThresholdValue))
			}
		default:
			res.Failures = append(res.Failures, fmt.Sprintf("unknown threshold %s", opts.FailThresholdName))
		}
	}
	if opts.FailOnMissingField != "" {
		missing := 0
		for _, entry := range oldEntries {
			newEntry, err := mmdb.LookupReader(newDB, entry.IP)
			if err != nil {
				return Result{}, err
			}
			if _, ok := mmdb.Field(newEntry.Record, opts.FailOnMissingField); !ok {
				missing++
			}
		}
		if missing > 0 {
			res.Failures = append(res.Failures, fmt.Sprintf("%s missing in %d sampled record(s)", opts.FailOnMissingField, missing))
		}
	}
	res.Failed = len(res.Failures) > 0
	return res, nil
}

func baseEntries(path string, opts Options) ([]mmdb.Entry, error) {
	if opts.IPsFile != "" {
		ips, err := mmdb.ReadIPFile(opts.IPsFile)
		if err != nil {
			return nil, err
		}
		return mmdb.LookupIPs(path, ips)
	}
	return mmdb.Sample(path, opts.Sample)
}

func unionFields(a, b map[string]any) []string {
	seen := map[string]bool{}
	for _, f := range mmdb.Fields(a) {
		seen[f] = true
	}
	for _, f := range mmdb.Fields(b) {
		seen[f] = true
	}
	fields := make([]string, 0, len(seen))
	for f := range seen {
		fields = append(fields, f)
	}
	sort.Strings(fields)
	return fields
}

func appendOnce(items []string, value string) []string {
	for _, item := range items {
		if item == value {
			return items
		}
	}
	return append(items, value)
}
