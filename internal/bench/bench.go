package bench

import (
	"fmt"
	"time"

	"mmdbforge/internal/mmdb"
)

type Result struct {
	Database        string  `json:"database"`
	Lookups         int     `json:"lookups"`
	ElapsedMS       float64 `json:"elapsed_ms"`
	LookupsPerSec   float64 `json:"lookups_per_sec"`
	AvgLookupMicros float64 `json:"avg_lookup_micros"`
}

type CompareResult struct {
	Old                        Result  `json:"old"`
	New                        Result  `json:"new"`
	LookupsPerSecChangePercent float64 `json:"lookups_per_sec_change_percent"`
}

func Run(path string, sample int) (Result, error) {
	entries, err := mmdb.Sample(path, sample)
	if err != nil {
		return Result{}, err
	}
	db, err := mmdb.Open(path)
	if err != nil {
		return Result{}, err
	}
	defer db.Close()
	start := time.Now()
	for _, entry := range entries {
		if _, err := mmdb.LookupReader(db, entry.IP); err != nil {
			return Result{}, err
		}
	}
	elapsed := time.Since(start)
	res := Result{Database: path, Lookups: len(entries), ElapsedMS: float64(elapsed.Microseconds()) / 1000}
	if elapsed > 0 && len(entries) > 0 {
		res.LookupsPerSec = float64(len(entries)) / elapsed.Seconds()
		res.AvgLookupMicros = float64(elapsed.Microseconds()) / float64(len(entries))
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
	res := CompareResult{Old: oldRes, New: newRes}
	if oldRes.LookupsPerSec > 0 {
		res.LookupsPerSecChangePercent = (newRes.LookupsPerSec - oldRes.LookupsPerSec) * 100 / oldRes.LookupsPerSec
	}
	if oldRes.Lookups != newRes.Lookups {
		return res, fmt.Errorf("benchmark sample mismatch: old=%d new=%d", oldRes.Lookups, newRes.Lookups)
	}
	return res, nil
}
