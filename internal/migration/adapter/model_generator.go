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
	"strings"

	"golang.org/x/tools/go/ast/astutil"
	importsutil "golang.org/x/tools/imports"
	"gorm.io/cli/gorm/internal/project"
	"gorm.io/cli/gorm/internal/utils"
	"gorm.io/gorm"
	gormSchema "gorm.io/gorm/schema"
)

// ModelGenerator handles generating Go model files from database schema.
type ModelGenerator struct {
	db  *gorm.DB
	cfg Config
}

// NewModelGenerator creates a new ModelGenerator.
func NewModelGenerator(db *gorm.DB, cfg Config) *ModelGenerator {
	return &ModelGenerator{db: db, cfg: cfg}
}

// Generate reverse-engineers the database schema to Go structs.
func (g *ModelGenerator) Generate(opts GenerateModelOptions) error {
	ns := g.db.NamingStrategy
	modelDir := g.cfg.ModelsDir
	configs, err := g.resolveTableConfigs()
	if err != nil {
		return err
	}

	if len(configs) == 0 {
		fmt.Fprintln(os.Stdout, "No tables found in database")
		return nil
	}

	for tableName := range configs {
		finalConfig := configs[tableName]
		path := g.resolveOutputPath(tableName, finalConfig, modelDir)
		structName := ns.SchemaName(tableName)
		outputPkg := project.DetectPackage(filepath.Dir(path))

		originalContent, err := os.ReadFile(path)
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to read existing model file %s: %w", path, err)
		}

		if len(originalContent) > 0 {
			if err := g.tryMergeOrAppend(path, tableName, structName, finalConfig, originalContent, opts, ns); err == nil {
				continue
			}
		}

		// Generate fresh model file
		cols, err := g.db.Migrator().ColumnTypes(tableName)
		if err != nil {
			return fmt.Errorf("get column types for %s: %w", tableName, err)
		}

		fields, imports, err := g.buildStructFields(ns, tableName, cols, finalConfig)
		if err != nil {
			return err
		}

		content := renderModelFile(outputPkg, tableName, structName, fields, imports)
		formatted, err := importsutil.Process(path, []byte(content), nil)
		if err != nil {
			return fmt.Errorf("format generated model %s: %w", path, err)
		}

		if err := utils.WriteFileWithConfirmation(path, originalContent, formatted, opts.DryRun, opts.AutoApprove, false); err != nil {
			return err
		}
	}

	return nil
}

func (g *ModelGenerator) resolveOutputPath(tableName string, cfg TableConfig, modelDir string) string {
	if cfg.OutputPath != "" {
		if filepath.IsAbs(cfg.OutputPath) {
			return cfg.OutputPath
		}
		return filepath.Join(project.Root(), cfg.OutputPath)
	}
	return filepath.Join(modelDir, fmt.Sprintf("%s.go", project.Slugify(tableName)))
}

func (g *ModelGenerator) tryMergeOrAppend(path, tableName, structName string, cfg TableConfig, original []byte, opts GenerateModelOptions, ns gormSchema.Namer) error {
	// Try to merge changes first
	err := g.mergeModelChanges(path, tableName, structName, cfg, original, opts)
	if err == nil {
		return nil
	}

	// If struct not found, try to append
	if errors.Is(err, errStructNotFound) {
		appendErr := g.appendModelDefinition(ns, path, tableName, structName, cfg, original, opts)
		if appendErr == nil {
			return nil
		}
		fmt.Fprintf(os.Stderr, "Could not append model %s to %s, falling back to overwrite: %v\n", structName, path, appendErr)
	} else {
		fmt.Fprintf(os.Stderr, "Could not merge changes for %s, falling back to overwrite: %v\n", path, err)
	}

	return err
}

func (g *ModelGenerator) resolveTableConfigs() (map[string]TableConfig, error) {
	tables, err := g.db.Migrator().GetTables()
	if err != nil {
		return nil, err
	}

	configs := make(map[string]TableConfig)
	for _, table := range tables {
		cfg, include := buildConfigForTable(table, g.cfg.TableRules)
		if !include {
			continue
		}
		configs[table] = cfg
	}

	return configs, nil
}

func (g *ModelGenerator) mergeModelChanges(path, table, structName string, cfg TableConfig, original []byte, opts GenerateModelOptions) error {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, path, original, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("failed to parse existing model file %s: %w", path, err)
	}

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

	existingFields := make(map[string]bool)
	for _, field := range structType.Fields.List {
		if len(field.Names) > 0 {
			dbName := getDBName(field, g.db.NamingStrategy)
			if dbName != "-" {
				existingFields[dbName] = true
			}
		}
	}

	cols, err := g.db.Migrator().ColumnTypes(table)
	if err != nil {
		return fmt.Errorf("get column types for %s: %w", table, err)
	}

	var newFields []*ast.Field
	newImports := make(map[string]struct{})

	for _, col := range cols {
		if _, ok := existingFields[col.Name()]; ok {
			continue
		}

		rule, hasRule := matchFieldRule(cfg.FieldRules, table, col.Name())
		if hasRule && rule.Exclude {
			continue
		}

		fieldName, goType, tagLiteral, imports, err := g.generateField(g.db.NamingStrategy, table, col, rule, hasRule)
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

	structType.Fields.List = append(structType.Fields.List, newFields...)

	for imp := range newImports {
		astutil.AddImport(fset, node, imp)
	}

	var buf bytes.Buffer
	if err := format.Node(&buf, fset, node); err != nil {
		return fmt.Errorf("failed to format updated model code: %w", err)
	}

	final, err := importsutil.Process(path, buf.Bytes(), nil)
	if err != nil {
		return fmt.Errorf("format merged model %s: %w", path, err)
	}

	return utils.WriteFileWithConfirmation(path, original, final, opts.DryRun, opts.AutoApprove, false)
}

func (g *ModelGenerator) appendModelDefinition(ns gormSchema.Namer, path, table, structName string, cfg TableConfig, original []byte, opts GenerateModelOptions) error {
	cols, err := g.db.Migrator().ColumnTypes(table)
	if err != nil {
		return fmt.Errorf("get column types for %s: %w", table, err)
	}

	fields, imports, err := g.buildStructFields(ns, table, cols, cfg)
	if err != nil {
		return err
	}

	fset := token.NewFileSet()
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

	return utils.WriteFileWithConfirmation(path, original, formatted, opts.DryRun, opts.AutoApprove, false)
}

func (g *ModelGenerator) buildStructFields(ns gormSchema.Namer, table string, cols []gorm.ColumnType, cfg TableConfig) (string, []string, error) {
	imports := make(map[string]struct{})
	fieldStrings := make([]string, 0, len(cols))

	for _, col := range cols {
		colName := col.Name()
		rule, hasRule := matchFieldRule(cfg.FieldRules, table, colName)
		if hasRule && rule.Exclude {
			continue
		}

		fieldName, goType, tagLiteral, newImports, err := g.generateField(ns, table, col, rule, hasRule)
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

func (g *ModelGenerator) generateField(ns gormSchema.Namer, table string, col gorm.ColumnType, rule FieldRule, hasRule bool) (fieldName, goType, structTag string, imports []string, err error) {
	colName := col.Name()
	fieldName = ns.SchemaName(colName)

	if hasRule && rule.FieldName != "" {
		fieldName = rule.FieldName
	}

	if hasRule && rule.FieldType != "" {
		goType = rule.FieldType
		imports = rule.Imports
	} else {
		goType, imports = toGoType(col, g.db.Dialector.Name())
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
