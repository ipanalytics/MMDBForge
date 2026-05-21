package cidrlint

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Result struct {
	File     string  `json:"file"`
	Checked  int     `json:"checked"`
	Errors   []Issue `json:"errors,omitempty"`
	Warnings []Issue `json:"warnings,omitempty"`
	Failed   bool    `json:"failed"`
}

type Issue struct {
	Line    int    `json:"line,omitempty"`
	CIDR    string `json:"cidr,omitempty"`
	Message string `json:"message"`
}

type item struct {
	line int
	raw  string
	pfx  netip.Prefix
}

func Run(path string) (Result, error) {
	items, issues, err := read(path)
	if err != nil {
		return Result{}, err
	}
	res := Result{File: path, Checked: len(items), Errors: issues}
	seen := map[string]int{}
	sort.Slice(items, func(i, j int) bool {
		if items[i].pfx.Addr().Compare(items[j].pfx.Addr()) == 0 {
			if items[i].pfx.Bits() == items[j].pfx.Bits() {
				return items[i].line < items[j].line
			}
			return items[i].pfx.Bits() < items[j].pfx.Bits()
		}
		return items[i].pfx.Addr().Less(items[j].pfx.Addr())
	})
	for i, it := range items {
		key := it.pfx.String()
		if first, ok := seen[key]; ok {
			res.Errors = append(res.Errors, Issue{Line: it.line, CIDR: key, Message: fmt.Sprintf("duplicate CIDR, first seen on line %d", first)})
		}
		seen[key] = it.line
		var broader *item
		for j := i - 1; j >= 0; j-- {
			prev := items[j]
			if prev.pfx.Addr().BitLen() != it.pfx.Addr().BitLen() {
				continue
			}
			if prev.pfx.Bits() >= it.pfx.Bits() {
				continue
			}
			if prev.pfx.Contains(it.pfx.Addr()) {
				if broader == nil || prev.line < broader.line {
					cp := prev
					broader = &cp
				}
			}
		}
		if broader != nil {
			res.Warnings = append(res.Warnings, Issue{Line: it.line, CIDR: key, Message: fmt.Sprintf("CIDR is shadowed by broader %s on line %d", broader.pfx, broader.line)})
		}
	}
	res.Failed = len(res.Errors) > 0
	return res, nil
}

func read(path string) ([]item, []Issue, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".csv" || ext == ".tsv" {
		return readCSV(path, ext == ".tsv")
	}
	return readLines(path)
}

func readLines(path string) ([]item, []Issue, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	var items []item
	var issues []Issue
	sc := bufio.NewScanner(f)
	line := 0
	for sc.Scan() {
		line++
		text := strings.TrimSpace(sc.Text())
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		cidr := text
		if strings.HasPrefix(text, "{") {
			cidr = cidrFromJSON(text)
		} else {
			cidr = strings.Fields(strings.Split(text, ",")[0])[0]
		}
		addItem(&items, &issues, line, cidr)
	}
	return items, issues, sc.Err()
}

func readCSV(path string, tsv bool) ([]item, []Issue, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	if tsv {
		r.Comma = '\t'
	}
	r.FieldsPerRecord = -1
	rows, err := r.ReadAll()
	if err != nil {
		return nil, nil, err
	}
	var items []item
	var issues []Issue
	for i, row := range rows {
		if len(row) == 0 || strings.HasPrefix(strings.TrimSpace(row[0]), "#") {
			continue
		}
		addItem(&items, &issues, i+1, strings.TrimSpace(row[0]))
	}
	return items, issues, nil
}

func addItem(items *[]item, issues *[]Issue, line int, cidr string) {
	pfx, err := netip.ParsePrefix(cidr)
	if err != nil {
		*issues = append(*issues, Issue{Line: line, CIDR: cidr, Message: "invalid CIDR"})
		return
	}
	*items = append(*items, item{line: line, raw: cidr, pfx: pfx.Masked()})
}

func cidrFromJSON(line string) string {
	var m map[string]any
	if err := json.Unmarshal([]byte(line), &m); err != nil {
		return line
	}
	for _, key := range []string{"cidr", "network", "prefix"} {
		if v, ok := m[key].(string); ok {
			return v
		}
	}
	return line
}
