package prefixes

import (
	"fmt"
	"sort"

	"mmdbforge/internal/mmdb"
)

type Result struct {
	CheckedNetworks int         `json:"checked_networks"`
	ByPrefixLength  map[int]int `json:"by_prefix_length"`
	HostLevel       int         `json:"host_level"`
	Warnings        []string    `json:"warnings,omitempty"`
}

type CompareResult struct {
	Old                 Result      `json:"old"`
	New                 Result      `json:"new"`
	DeltaByPrefixLength map[int]int `json:"delta_by_prefix_length"`
	Warnings            []string    `json:"warnings,omitempty"`
}

func Run(path string, sample int) (Result, error) {
	entries, err := mmdb.Sample(path, sample)
	if err != nil {
		return Result{}, err
	}
	res := Result{ByPrefixLength: map[int]int{}}
	for _, entry := range entries {
		plen := prefixLength(entry.Network)
		res.ByPrefixLength[plen]++
		res.CheckedNetworks++
		if plen == 32 || plen == 128 {
			res.HostLevel++
		}
	}
	if res.CheckedNetworks > 0 {
		hostPct := float64(res.HostLevel) * 100 / float64(res.CheckedNetworks)
		if hostPct > 25 {
			res.Warnings = append(res.Warnings, fmt.Sprintf("host-level records are %.2f%% of sampled networks", hostPct))
		}
	}
	return res, nil
}

func Compare(oldPath, newPath string, sample int) (CompareResult, error) {
	oldRes, err := Run(oldPath, sample)
	if err != nil {
		return CompareResult{}, err
	}
	newRes, err := Run(newPath, sample)
	if err != nil {
		return CompareResult{}, err
	}
	res := CompareResult{Old: oldRes, New: newRes, DeltaByPrefixLength: map[int]int{}}
	keys := map[int]bool{}
	for k := range oldRes.ByPrefixLength {
		keys[k] = true
	}
	for k := range newRes.ByPrefixLength {
		keys[k] = true
	}
	for k := range keys {
		res.DeltaByPrefixLength[k] = newRes.ByPrefixLength[k] - oldRes.ByPrefixLength[k]
	}
	if oldRes.HostLevel > 0 {
		delta := float64(newRes.HostLevel-oldRes.HostLevel) * 100 / float64(oldRes.HostLevel)
		if delta > 50 {
			res.Warnings = append(res.Warnings, fmt.Sprintf("host-level records increased %.2f%%", delta))
		}
	}
	res.Warnings = append(res.Warnings, oldRes.Warnings...)
	res.Warnings = append(res.Warnings, newRes.Warnings...)
	return res, nil
}

func Lengths(m map[int]int) []int {
	out := make([]int, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Ints(out)
	return out
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
