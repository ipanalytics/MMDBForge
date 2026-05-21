package schema

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"mmdbforge/internal/mmdb"
)

type Schema struct {
	Required   []string             `json:"required"`
	Fields     map[string]FieldRule `json:"fields"`
	Rules      []Rule               `json:"rules"`
	Properties map[string]FieldRule `json:"properties"`
}

type FieldRule struct {
	Type       any                  `json:"type"`
	Minimum    *float64             `json:"minimum"`
	Maximum    *float64             `json:"maximum"`
	Properties map[string]FieldRule `json:"properties"`
}

type Rule struct {
	If           string   `json:"if"`
	ThenRequired []string `json:"then_required"`
}

type Result struct {
	CheckedRecords int       `json:"checked_records"`
	Errors         []Message `json:"errors"`
	Warnings       []Message `json:"warnings,omitempty"`
}

type Message struct {
	IP            string `json:"ip,omitempty"`
	MatchedPrefix string `json:"matched_prefix,omitempty"`
	Field         string `json:"field,omitempty"`
	Message       string `json:"message"`
}

func Validate(schemaPath, dbPath string, sample int) (Result, error) {
	s, err := Load(schemaPath)
	if err != nil {
		return Result{}, err
	}
	entries, err := mmdb.Sample(dbPath, sample)
	if err != nil {
		return Result{}, err
	}
	res := Result{CheckedRecords: len(entries)}
	for _, entry := range entries {
		validateEntry(s, entry, &res)
	}
	return res, nil
}

func Load(path string) (Schema, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return Schema{}, err
	}
	var s Schema
	if err := json.Unmarshal(body, &s); err != nil {
		return Schema{}, err
	}
	if len(s.Fields) == 0 && len(s.Properties) > 0 {
		s.Fields = map[string]FieldRule{}
		flattenJSONSchemaProperties("", s.Properties, s.Fields)
	}
	return s, nil
}

func flattenJSONSchemaProperties(prefix string, props map[string]FieldRule, out map[string]FieldRule) {
	for name, rule := range props {
		path := name
		if prefix != "" {
			path = prefix + "." + name
		}
		if len(rule.Properties) > 0 {
			flattenJSONSchemaProperties(path, rule.Properties, out)
			continue
		}
		out[path] = FieldRule{Type: rule.Type, Minimum: rule.Minimum, Maximum: rule.Maximum}
	}
}

func validateEntry(s Schema, entry mmdb.Entry, res *Result) {
	for _, field := range s.Required {
		if _, ok := mmdb.Field(entry.Record, field); !ok {
			res.Errors = append(res.Errors, msg(entry, field, field+" is required"))
		}
	}
	for field, rule := range s.Fields {
		value, ok := mmdb.Field(entry.Record, field)
		if !ok {
			continue
		}
		if !typeAllowed(value, rule.Types()) {
			res.Errors = append(res.Errors, msg(entry, field, fmt.Sprintf("expected %s, got %s", strings.Join(rule.Types(), "|"), mmdb.TypeName(value))))
			continue
		}
		if rule.Minimum != nil || rule.Maximum != nil {
			n, ok := mmdb.AsFloat(value)
			if !ok {
				res.Errors = append(res.Errors, msg(entry, field, "expected numeric value for range check"))
				continue
			}
			if rule.Minimum != nil && n < *rule.Minimum {
				res.Errors = append(res.Errors, msg(entry, field, fmt.Sprintf("must be >= %v", *rule.Minimum)))
			}
			if rule.Maximum != nil && n > *rule.Maximum {
				res.Errors = append(res.Errors, msg(entry, field, fmt.Sprintf("must be <= %v", *rule.Maximum)))
			}
		}
	}
	for _, rule := range s.Rules {
		ok, err := evalCondition(entry.Record, rule.If)
		if err != nil {
			res.Warnings = append(res.Warnings, msg(entry, "", "rule skipped: "+err.Error()))
			continue
		}
		if !ok {
			continue
		}
		for _, field := range rule.ThenRequired {
			if _, exists := mmdb.Field(entry.Record, field); !exists {
				res.Errors = append(res.Errors, msg(entry, field, fmt.Sprintf("%s requires %s", rule.If, field)))
			}
		}
	}
}

func (r FieldRule) Types() []string {
	switch x := r.Type.(type) {
	case string:
		return []string{x}
	case []any:
		out := make([]string, 0, len(x))
		for _, v := range x {
			if s, ok := v.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func typeAllowed(value any, allowed []string) bool {
	if len(allowed) == 0 {
		return true
	}
	actual := mmdb.TypeName(value)
	for _, typ := range allowed {
		if typ == actual || (typ == "number" && actual == "integer") {
			return true
		}
	}
	return false
}

func evalCondition(record map[string]any, expr string) (bool, error) {
	parts := strings.Fields(expr)
	if len(parts) != 3 {
		return false, fmt.Errorf("unsupported condition %q", expr)
	}
	left, op, rawRight := parts[0], parts[1], parts[2]
	value, ok := mmdb.Field(record, left)
	if !ok {
		return false, nil
	}
	right := parseLiteral(rawRight)
	switch op {
	case "==":
		return mmdb.Equal(value, right), nil
	case "!=":
		return !mmdb.Equal(value, right), nil
	case ">=", ">", "<=", "<":
		a, okA := mmdb.AsFloat(value)
		b, okB := mmdb.AsFloat(right)
		if !okA || !okB {
			return false, fmt.Errorf("condition %q requires numeric operands", expr)
		}
		switch op {
		case ">=":
			return a >= b, nil
		case ">":
			return a > b, nil
		case "<=":
			return a <= b, nil
		case "<":
			return a < b, nil
		}
	}
	return false, fmt.Errorf("unsupported operator %q", op)
}

func parseLiteral(s string) any {
	switch s {
	case "true":
		return true
	case "false":
		return false
	case "null":
		return nil
	}
	var n float64
	if _, err := fmt.Sscanf(s, "%f", &n); err == nil {
		if float64(int64(n)) == n {
			return int64(n)
		}
		return n
	}
	return strings.Trim(s, `"`)
}

func msg(entry mmdb.Entry, field, message string) Message {
	return Message{IP: entry.IP, MatchedPrefix: entry.Network, Field: field, Message: message}
}
