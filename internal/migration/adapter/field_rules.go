package adapter

import (
	"path/filepath"
	"sort"
	"strings"

	"gorm.io/cli/gorm/internal/project"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
	"maps"
)

type tableRuleResolver struct {
	rules []TableRule
}

func newTableRuleResolver(rules []TableRule) tableRuleResolver {
	return tableRuleResolver{rules: rules}
}

func (r tableRuleResolver) ConfigForTable(table string) (TableConfig, bool) {
	include := true
	var finalCfg TableConfig
	found := false
	for _, rule := range r.rules {
		if rule.Pattern == "" {
			continue
		}
		ok, err := filepath.Match(rule.Pattern, table)
		if err != nil || !ok {
			continue
		}
		found = true
		if rule.Exclude {
			return TableConfig{}, false
		}
		if len(rule.Config.FieldRules) > 0 {
			finalCfg.FieldRules = append(finalCfg.FieldRules, cloneFieldRules(rule.Config.FieldRules)...)
		}
		if rule.Config.OutputPath != "" {
			finalCfg.OutputPath = rule.Config.OutputPath
		}
	}
	if !found {
		return TableConfig{}, true
	}
	return finalCfg, include
}

func (r tableRuleResolver) modelDirectories(defaultDir string) []string {
	dirs := make(map[string]struct{})
	if defaultDir != "" {
		dirs[project.ResolveRootPath(defaultDir)] = struct{}{}
	}
	for _, rule := range r.rules {
		if rule.Config.OutputPath == "" {
			continue
		}
		dir := project.ResolveRootPath(filepath.Dir(rule.Config.OutputPath))
		dirs[dir] = struct{}{}
	}
	result := make([]string, 0, len(dirs))
	for dir := range dirs {
		result = append(result, dir)
	}
	sort.Strings(result)
	return result
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

type fieldRuleMatcher struct {
	rules []FieldRule
}

func newFieldRuleMatcher(rules []FieldRule) fieldRuleMatcher {
	return fieldRuleMatcher{rules: rules}
}

func (m fieldRuleMatcher) match(table, column string) (FieldRule, bool) {
	full := table + "." + column
	for _, rule := range m.rules {
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

func applyFieldRulesToSchema(table *TableSchema, matcher fieldRuleMatcher) *TableSchema {
	if table == nil || table.Schema == nil {
		return nil
	}
	excluded := identifyExcludedColumns(table.Schema.Table, table.Schema.Fields, matcher)
	if len(excluded) == 0 {
		return table
	}
	newSchema := *table.Schema
	newSchema.Fields = make([]*schema.Field, 0, len(table.Schema.Fields)-len(excluded))
	newSchema.FieldsByName = make(map[string]*schema.Field, len(table.Schema.Fields)-len(excluded))
	newSchema.FieldsByDBName = make(map[string]*schema.Field, len(table.Schema.Fields)-len(excluded))
	newSchema.DBNames = make([]string, 0, len(table.Schema.DBNames))
	newSchema.PrimaryFields = nil
	newSchema.PrimaryFieldDBNames = nil
	newSchema.PrioritizedPrimaryField = nil
	var columnTypes map[string]gorm.ColumnType
	if len(table.ColumnTypes) > 0 {
		columnTypes = make(map[string]gorm.ColumnType, len(table.ColumnTypes))
	}
	for _, field := range table.Schema.Fields {
		if _, skip := excluded[field.DBName]; skip {
			continue
		}
		newSchema.Fields = append(newSchema.Fields, field)
		newSchema.FieldsByName[field.Name] = field
		newSchema.FieldsByDBName[field.DBName] = field
		newSchema.DBNames = append(newSchema.DBNames, field.DBName)
		if columnTypes != nil {
			if ct, ok := table.ColumnTypes[strings.ToLower(field.DBName)]; ok {
				columnTypes[strings.ToLower(field.DBName)] = ct
			}
		}
		if field.PrimaryKey {
			newSchema.PrimaryFields = append(newSchema.PrimaryFields, field)
			newSchema.PrimaryFieldDBNames = append(newSchema.PrimaryFieldDBNames, field.DBName)
			if newSchema.PrioritizedPrimaryField == nil {
				newSchema.PrioritizedPrimaryField = field
			}
		}
	}
	if len(newSchema.Fields) == 0 {
		return nil
	}
	filtered := &TableSchema{
		Schema:      &newSchema,
		Model:       table.Model,
		Indexes:     filterIndexesForRules(table.Indexes, excluded),
		Constraints: filterConstraintsForRules(table.Constraints, excluded),
		ColumnTypes: columnTypes,
	}
	return filtered
}

func identifyExcludedColumns(table string, fields []*schema.Field, matcher fieldRuleMatcher) map[string]struct{} {
	if len(fields) == 0 || len(matcher.rules) == 0 {
		return nil
	}
	excluded := make(map[string]struct{})
	for _, field := range fields {
		if field == nil {
			continue
		}
		if rule, ok := matcher.match(table, field.DBName); ok && rule.Exclude {
			excluded[field.DBName] = struct{}{}
		}
	}
	return excluded
}

func filterIndexesForRules(indexes []*schema.Index, ignored map[string]struct{}) []*schema.Index {
	if len(indexes) == 0 || len(ignored) == 0 {
		return indexes
	}
	filtered := make([]*schema.Index, 0, len(indexes))
	for _, idx := range indexes {
		if idx == nil {
			continue
		}
		newIdx := *idx
		newIdx.Fields = nil
		for _, option := range idx.Fields {
			if option.Field != nil {
				if _, skip := ignored[option.Field.DBName]; skip {
					continue
				}
			}
			newIdx.Fields = append(newIdx.Fields, option)
		}
		if len(idx.Fields) > 0 && len(newIdx.Fields) == 0 {
			continue
		}
		filtered = append(filtered, &newIdx)
	}
	return filtered
}

func filterConstraintsForRules(constraints []*schema.Constraint, ignored map[string]struct{}) []*schema.Constraint {
	if len(constraints) == 0 || len(ignored) == 0 {
		return constraints
	}
	filtered := make([]*schema.Constraint, 0, len(constraints))
	for _, cons := range constraints {
		if cons == nil {
			continue
		}
		if constraintReferencesExcludedField(cons, ignored) {
			continue
		}
		filtered = append(filtered, cons)
	}
	return filtered
}

func constraintReferencesExcludedField(cons *schema.Constraint, ignored map[string]struct{}) bool {
	for _, field := range cons.ForeignKeys {
		if field != nil {
			if _, skip := ignored[field.DBName]; skip {
				return true
			}
		}
	}
	for _, field := range cons.References {
		if field != nil {
			if _, skip := ignored[field.DBName]; skip {
				return true
			}
		}
	}
	return false
}
