package adapter

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"

	astutil "golang.org/x/tools/go/ast/astutil"
	importsutil "golang.org/x/tools/imports"
	"gorm.io/cli/gorm/internal/project"
	"gorm.io/cli/gorm/internal/utils"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

var errStructNotFound = errors.New("struct not found")

// GenerateModel writes placeholder structs per table.
func (a *DBAdapter) GenerateModel(opts GenerateModelOptions) error {
	ns := a.db.NamingStrategy
	modelDir := a.cfg.ModelsDir
	defaultPkg := project.DetectPackage(modelDir)
	if defaultPkg == "" {
		defaultPkg = "models"
	}
	configs, err := a.resolveTableConfigs()
	if err != nil {
		return err
	}
	if len(configs) == 0 {
		fmt.Fprintln(os.Stdout, "No tables found in database")
		return nil
	}

	for tableName := range configs {
		finalConfig := configs[tableName]
		// Determine output path
		var path string
		if finalConfig.OutputPath != "" {
			if filepath.IsAbs(finalConfig.OutputPath) {
				path = finalConfig.OutputPath
			} else {
				path = filepath.Join(project.Root(), finalConfig.OutputPath)
			}
		} else {
			path = filepath.Join(modelDir, fmt.Sprintf("%s.go", project.Slugify(tableName)))
		}
		structName := ns.SchemaName(tableName)
		outputPkg := defaultPkg
		if pkg := project.DetectPackage(filepath.Dir(path)); pkg != "" {
			outputPkg = pkg
		}

		// If file exists, attempt to merge/append before overwriting.
		if _, err := os.Stat(path); err == nil {
			err := a.mergeModelChanges(path, tableName, structName, finalConfig, opts)
			if err == nil {
				continue // Successfully merged or up-to-date
			}
			if errors.Is(err, errStructNotFound) {
				appendErr := a.appendModelDefinition(ns, path, tableName, structName, finalConfig, opts)
				if appendErr == nil {
					continue
				}
				fmt.Fprintf(os.Stderr, "Could not append model %s to %s, falling back to overwrite: %v\n", structName, path, appendErr)
			} else {
				fmt.Fprintf(os.Stderr, "Could not merge changes for %s, falling back to overwrite: %v\n", path, err)
			}
			// Fallback to overwrite logic below
		}

		// Create new file or overwrite on merge failure
		cols, err := a.db.Migrator().ColumnTypes(tableName)
		if err != nil {
			return fmt.Errorf("get column types for %s: %w", tableName, err)
		}

		fields, imports, err := a.buildStructFields(ns, tableName, cols, finalConfig)
		if err != nil {
			return err
		}
		content := renderModelFile(outputPkg, tableName, structName, fields, imports)
		formatted, err := importsutil.Process(path, []byte(content), nil)
		if err != nil {
			return fmt.Errorf("format generated model %s: %w", path, err)
		}
		contentBytes := formatted

		if opts.DryRun {
			fmt.Fprintf(os.Stdout, "--- model preview (%s) ---\n%s\n--- end ---\n", path, string(contentBytes))
			continue
		}

		ok, err := utils.ConfirmWrite(path, opts.AutoApprove)
		if err != nil {
			return err
		}
		if !ok {
			fmt.Fprintf(os.Stdout, "Skipped %s\n", path)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, contentBytes, 0o644); err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "Model written: %s\n", path)
	}
	return nil
}

func (a *DBAdapter) resolveTableConfigs() (map[string]TableConfig, error) {
	tables, err := a.db.Migrator().GetTables()
	if err != nil {
		return nil, err
	}
	resolver := newTableRuleResolver(a.cfg.TableRules)
	configs := make(map[string]TableConfig)
	for _, table := range tables {
		cfg, include := resolver.ConfigForTable(table)
		if !include {
			continue
		}
		configs[table] = cfg
	}
	return configs, nil
}

func (a *DBAdapter) mergeModelChanges(path, table, structName string, cfg TableConfig, opts GenerateModelOptions) error {
	fset := token.NewFileSet()
	original, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read existing model file %s: %w", path, err)
	}
	node, err := parser.ParseFile(fset, path, original, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("failed to parse existing model file %s: %w", path, err)
	}

	// Find the struct declaration
	var structType *ast.StructType
	ast.Inspect(node, func(n ast.Node) bool {
		if ts, ok := n.(*ast.TypeSpec); ok && ts.Name.Name == structName {
			if st, ok := ts.Type.(*ast.StructType); ok {
				structType = st
				return false
			}
		}
		return true
	})

	if structType == nil {
		return fmt.Errorf("%w: %s", errStructNotFound, structName)
	}

	// Map existing fields by DB column name
	existingFields := make(map[string]bool)
	for _, field := range structType.Fields.List {
		if len(field.Names) > 0 {
			dbName := getDBName(field, a.db.NamingStrategy)
			if dbName != "-" {
				existingFields[dbName] = true
			}
		}
	}

	// Get current DB columns
	cols, err := a.db.Migrator().ColumnTypes(table)
	if err != nil {
		return fmt.Errorf("get column types for %s: %w", table, err)
	}

	// Find and generate new fields
	var newFields []*ast.Field
	newImports := make(map[string]struct{})
	ruleMatcher := newFieldRuleMatcher(cfg.FieldRules)
	for _, col := range cols {
		if _, ok := existingFields[col.Name()]; ok {
			continue
		}
		rule, hasRule := ruleMatcher.match(table, col.Name())
		if hasRule && rule.Exclude {
			continue
		}
		fieldName, goType, tagLiteral, imports, err := a.generateField(a.db.NamingStrategy, table, col, rule, hasRule)
		if err != nil {
			return err
		}
		for _, imp := range imports {
			newImports[imp] = struct{}{}
		}
		field := &ast.Field{
			Names: []*ast.Ident{ast.NewIdent(fieldName)},
			Type:  ast.NewIdent(goType),
		}
		if tagLiteral != "" {
			field.Tag = &ast.BasicLit{Kind: token.STRING, Value: tagLiteral}
		}
		newFields = append(newFields, field)
	}

	if len(newFields) == 0 {
		fmt.Fprintf(os.Stdout, "Model %s is already up to date.\n", path)
		return nil
	}

	// Add new fields to the struct
	structType.Fields.List = append(structType.Fields.List, newFields...)

	// Add new imports
	for imp := range newImports {
		quoted := strconv.Quote(imp)
		var found bool
		for _, spec := range node.Imports {
			if spec.Path.Value == quoted {
				found = true
				break
			}
		}
		if !found {
			newImport := &ast.ImportSpec{Path: &ast.BasicLit{Kind: token.STRING, Value: quoted}}
			node.Imports = append(node.Imports, newImport)
		}
	}

	// Format the modified AST
	var buf bytes.Buffer
	if err := format.Node(&buf, fset, node); err != nil {
		return fmt.Errorf("failed to format updated model code: %w", err)
	}

	utils.PrintDiff(path, original, buf.Bytes())
	if opts.DryRun {
		return nil
	}

	ok, err := utils.ConfirmWrite(path, opts.AutoApprove)
	if err != nil {
		return err
	}
	if !ok {
		fmt.Fprintf(os.Stdout, "Skipped %s\n", path)
		return nil
	}

	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "Model updated: %s\n", path)
	return nil
}

func (a *DBAdapter) appendModelDefinition(ns schema.Namer, path, table, structName string, cfg TableConfig, opts GenerateModelOptions) error {
	cols, err := a.db.Migrator().ColumnTypes(table)
	if err != nil {
		return fmt.Errorf("get column types for %s: %w", table, err)
	}

	fields, imports, err := a.buildStructFields(ns, table, cols, cfg)
	if err != nil {
		return err
	}

	fset := token.NewFileSet()
	original, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read existing model file %s: %w", path, err)
	}
	node, err := parser.ParseFile(fset, path, original, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("failed to parse existing model file %s: %w", path, err)
	}
	for _, imp := range imports {
		if imp == "" {
			continue
		}
		astutil.AddImport(fset, node, imp)
	}
	var existing bytes.Buffer
	if err := format.Node(&existing, fset, node); err != nil {
		return fmt.Errorf("failed to format existing model code: %w", err)
	}

	snippet := renderModelStruct(structName, table, fields)
	trimmed := bytes.TrimRight(existing.Bytes(), "\n")
	combined := append(trimmed, []byte("\n\n"+snippet)...)
	formatted, err := importsutil.Process(path, combined, nil)
	if err != nil {
		return fmt.Errorf("format appended model %s: %w", path, err)
	}

	utils.PrintDiff(path, original, formatted)
	if opts.DryRun {
		return nil
	}
	ok, err := utils.ConfirmWrite(path, opts.AutoApprove)
	if err != nil {
		return err
	}
	if !ok {
		fmt.Fprintf(os.Stdout, "Skipped %s\n", path)
		return nil
	}
	if err := os.WriteFile(path, formatted, 0o644); err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "Model updated: %s\n", path)
	return nil
}

func getDBName(field *ast.Field, ns schema.Namer) string {
	if field.Tag == nil {
		return ns.TableName(field.Names[0].Name)
	}
	tag := reflect.StructTag(strings.Trim(field.Tag.Value, "`"))
	gormTag := tag.Get("gorm")
	settings := schema.ParseTagSetting(gormTag, ";")
	if _, disabled := settings["-"]; disabled {
		return "-"
	}
	if column, ok := settings["COLUMN"]; ok && column != "" {
		return column
	}
	return ns.TableName(field.Names[0].Name)
}

func (a *DBAdapter) buildStructFields(ns schema.Namer, table string, cols []gorm.ColumnType, cfg TableConfig) (string, []string, error) {
	imports := make(map[string]struct{})
	fieldStrings := make([]string, 0, len(cols))
	matcher := newFieldRuleMatcher(cfg.FieldRules)

	for _, col := range cols {
		colName := col.Name()
		rule, hasRule := matcher.match(table, colName)
		if hasRule && rule.Exclude {
			continue
		}
		fieldName, goType, tagLiteral, newImports, err := a.generateField(ns, table, col, rule, hasRule)
		if err != nil {
			return "", nil, err
		}
		for _, imp := range newImports {
			imports[imp] = struct{}{}
		}
		fieldDef := fmt.Sprintf("\t%s %s", fieldName, goType)
		if tagLiteral != "" {
			fieldDef += " " + tagLiteral
		}
		fieldStrings = append(fieldStrings, fieldDef)
	}

	importList := make([]string, 0, len(imports))
	for imp := range imports {
		importList = append(importList, imp)
	}

	return strings.Join(fieldStrings, "\n"), importList, nil
}

func (a *DBAdapter) generateField(ns schema.Namer, table string, col gorm.ColumnType, rule FieldRule, hasRule bool) (fieldName, goType, structTag string, imports []string, err error) {
	colName := col.Name()

	fieldName = ns.SchemaName(colName)
	if hasRule && rule.FieldName != "" {
		fieldName = rule.FieldName
	}

	if hasRule && rule.FieldType != "" {
		goType = rule.FieldType
		imports = rule.Imports
	} else {
		goType, imports = toGoType(col, a.db.Dialector.Name())
	}

	tags := make(map[string]string)
	if gormVal := buildGormTag(ns, col, fieldName); gormVal != "" {
		tags["gorm"] = gormVal
	}
	if hasRule && len(rule.Tags) > 0 {
		ctx := tagContext{Table: table, Name: fieldName, DBName: colName, DBType: col.DatabaseTypeName()}
		for key, tpl := range rule.Tags {
			val, tplErr := renderTagValue(tpl, ctx)
			if tplErr != nil {
				err = tplErr
				return
			}
			if strings.TrimSpace(val) == "" {
				delete(tags, key)
				continue
			}
			tags[key] = val
		}
	}
	structTag = buildStructTag(tags)
	return
}

func buildGormTag(ns schema.Namer, col gorm.ColumnType, fieldName string) string {
	tags := []string{}
	if pk, _ := col.PrimaryKey(); pk {
		tags = append(tags, "primaryKey")
	}

	if col.Name() != ns.TableName(fieldName) {
		tags = append(tags, "column:"+col.Name())
	}

	if val, ok := col.DefaultValue(); ok {
		tags = append(tags, "default:"+val)
	}

	if len(tags) == 0 {
		return ""
	}
	return strings.Join(tags, ";")
}

func buildStructTag(tags map[string]string) string {
	if len(tags) == 0 {
		return ""
	}
	keys := make([]string, 0, len(tags))
	for k, v := range tags {
		if strings.TrimSpace(v) == "" {
			continue
		}
		keys = append(keys, k)
	}
	if len(keys) == 0 {
		return ""
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s:%s", k, strconv.Quote(tags[k])))
	}
	return "`" + strings.Join(parts, " ") + "`"
}
