package adapter

import (
	"path/filepath"
	"strings"

	"golang.org/x/exp/maps"
)

func buildConfigForTable(table string, rules []TableRule) (TableConfig, bool) {
	var finalCfg TableConfig
	include := true
	foundRule := false
	for _, rule := range rules {
		if rule.Pattern == "" {
			continue
		}
		ok, err := filepath.Match(rule.Pattern, table)
		if err != nil || !ok {
			continue
		}
		foundRule = true
		if rule.Exclude {
			include = false
			break
		}
		if len(rule.Config.FieldRules) > 0 {
			finalCfg.FieldRules = append(finalCfg.FieldRules, cloneFieldRules(rule.Config.FieldRules)...)
		}
		if rule.Config.OutputPath != "" {
			finalCfg.OutputPath = rule.Config.OutputPath
		}
	}
	if !foundRule {
		return TableConfig{}, true
	}
	return finalCfg, include
}

func cloneFieldRules(src []FieldRule) []FieldRule {
	dup := make([]FieldRule, len(src))
	for i, v := range src {
		dup[i] = FieldRule{
			Pattern:   v.Pattern,
			FieldName: v.FieldName,
			FieldType: v.FieldType,
			Tags:      maps.Clone(v.Tags),
			Imports:   append([]string(nil), v.Imports...),
			Exclude:   v.Exclude,
		}
	}
	return dup
}

func matchFieldRule(rules []FieldRule, table, column string) (FieldRule, bool) {
	full := table + "." + column
	for _, rule := range rules {
		pattern := strings.TrimSpace(rule.Pattern)
		if pattern == "" {
			pattern = full
		}
		if pattern == full || pattern == column {
			return rule, true
		}
		if matched, _ := filepath.Match(pattern, full); matched {
			return rule, true
		}
		if matched, _ := filepath.Match(pattern, column); matched {
			return rule, true
		}
	}
	return FieldRule{}, false
}
