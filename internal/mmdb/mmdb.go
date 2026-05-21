package mmdb

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/oschwald/maxminddb-golang"
)

type Entry struct {
	IP      string         `json:"ip"`
	Network string         `json:"matched_prefix"`
	Record  map[string]any `json:"record"`
}

func Open(path string) (*maxminddb.Reader, error) {
	db, err := maxminddb.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	return db, nil
}

func Lookup(path, ipText string) (Entry, error) {
	db, err := Open(path)
	if err != nil {
		return Entry{}, err
	}
	defer db.Close()
	return LookupReader(db, ipText)
}

func LookupReader(db *maxminddb.Reader, ipText string) (Entry, error) {
	ip := net.ParseIP(ipText)
	if ip == nil {
		return Entry{}, fmt.Errorf("invalid IP %q", ipText)
	}
	var record map[string]any
	network, ok, err := db.LookupNetwork(ip, &record)
	if err != nil {
		return Entry{}, err
	}
	if !ok || record == nil {
		record = map[string]any{}
	}
	networkText := ""
	if network != nil {
		networkText = network.String()
	}
	return Entry{IP: ip.String(), Network: networkText, Record: Normalize(record).(map[string]any)}, nil
}

func Sample(path string, limit int) ([]Entry, error) {
	db, err := Open(path)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	return SampleReader(db, limit)
}

func SampleReader(db *maxminddb.Reader, limit int) ([]Entry, error) {
	if limit <= 0 {
		limit = 10000
	}
	networks := db.Networks(maxminddb.SkipAliasedNetworks)
	entries := make([]Entry, 0, limit)
	for networks.Next() {
		var record map[string]any
		network, err := networks.Network(&record)
		if err != nil {
			return nil, err
		}
		if network == nil {
			continue
		}
		ip := firstUsableIP(network)
		if record == nil {
			record = map[string]any{}
		}
		entries = append(entries, Entry{
			IP:      ip.String(),
			Network: network.String(),
			Record:  Normalize(record).(map[string]any),
		})
		if len(entries) >= limit {
			break
		}
	}
	if err := networks.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}

func LookupIPs(path string, ips []string) ([]Entry, error) {
	db, err := Open(path)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	entries := make([]Entry, 0, len(ips))
	for _, ip := range ips {
		if strings.TrimSpace(ip) == "" {
			continue
		}
		entry, err := LookupReader(db, strings.TrimSpace(ip))
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func ReadIPFile(path string) ([]string, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(body), "\n")
	ips := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		ips = append(ips, line)
	}
	return ips, nil
}

func Flatten(record map[string]any) map[string]any {
	out := make(map[string]any)
	var walk func(prefix string, v any)
	walk = func(prefix string, v any) {
		switch m := v.(type) {
		case map[string]any:
			if len(m) == 0 && prefix != "" {
				out[prefix] = m
			}
			keys := make([]string, 0, len(m))
			for k := range m {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				next := k
				if prefix != "" {
					next = prefix + "." + k
				}
				walk(next, m[k])
			}
		default:
			if prefix != "" {
				out[prefix] = v
			}
		}
	}
	walk("", record)
	return out
}

func Field(record map[string]any, path string) (any, bool) {
	var cur any = record
	for _, part := range strings.Split(path, ".") {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		cur, ok = m[part]
		if !ok {
			return nil, false
		}
	}
	return cur, true
}

func Fields(record map[string]any) []string {
	flat := Flatten(record)
	fields := make([]string, 0, len(flat))
	for f := range flat {
		fields = append(fields, f)
	}
	sort.Strings(fields)
	return fields
}

func Equal(a, b any) bool {
	return reflect.DeepEqual(Normalize(a), Normalize(b))
}

func Normalize(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, v := range x {
			out[k] = Normalize(v)
		}
		return out
	case map[any]any:
		out := make(map[string]any, len(x))
		for k, v := range x {
			out[fmt.Sprint(k)] = Normalize(v)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, v := range x {
			out[i] = Normalize(v)
		}
		return out
	case uint:
		return int64(x)
	case uint8:
		return int64(x)
	case uint16:
		return int64(x)
	case uint32:
		return int64(x)
	case uint64:
		if x <= uint64(^uint64(0)>>1) {
			return int64(x)
		}
		return float64(x)
	case int:
		return int64(x)
	case int8:
		return int64(x)
	case int16:
		return int64(x)
	case int32:
		return int64(x)
	default:
		return v
	}
}

func JSONKey(v any) string {
	b, err := json.Marshal(Normalize(v))
	if err != nil {
		return fmt.Sprint(v)
	}
	return string(b)
}

func TypeName(v any) string {
	switch v.(type) {
	case nil:
		return "null"
	case string:
		return "string"
	case bool:
		return "boolean"
	case int, int8, int16, int32, int64:
		return "integer"
	case uint, uint8, uint16, uint32, uint64:
		return "integer"
	case float32, float64:
		return "number"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	default:
		return strings.TrimPrefix(fmt.Sprintf("%T", v), "interface {}")
	}
}

func AsFloat(v any) (float64, bool) {
	switch x := v.(type) {
	case int:
		return float64(x), true
	case int8:
		return float64(x), true
	case int16:
		return float64(x), true
	case int32:
		return float64(x), true
	case int64:
		return float64(x), true
	case uint:
		return float64(x), true
	case uint8:
		return float64(x), true
	case uint16:
		return float64(x), true
	case uint32:
		return float64(x), true
	case uint64:
		return float64(x), true
	case float32:
		return float64(x), true
	case float64:
		return x, true
	case json.Number:
		n, err := strconv.ParseFloat(string(x), 64)
		return n, err == nil
	default:
		return 0, false
	}
}

func firstUsableIP(network *net.IPNet) net.IP {
	ip := append(net.IP(nil), network.IP...)
	return ip
}
