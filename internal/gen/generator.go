package gen

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"
	"text/template"
	"unicode"

	"golang.org/x/tools/imports"
	"gorm.io/cli/gorm/genconfig"
)

type (
	Generator struct {
		Typed   bool
		Files   map[string]*File
		outPath string
	}
	File struct {
		Package           string
		PackagePath       string
		Imports           []Import
		Interfaces        []Interface
		Structs           []Struct
		Config            *genconfig.Config
		applicableConfigs []*genconfig.Config
		inputPath         string
		relPath           string
		Generator         *Generator
	}
	Import struct {
		Name string
		Path string
	}
	Interface struct {
		Name      string
		IfaceName string
		Doc       string
		Methods   []*Method
	}
	Method struct {
		Name      string
		Doc       string
		SQL       ExtractedSQL
		Params    []Param
		Result    []Param
		Interface Interface
	}
	Param struct {
		Name string
		Type string
	}
	Struct struct {
		Name   string
		Doc    string
		Fields []Field
	}
	Field struct {
		Name        string
		DBName      string
		GoType      string
		NamedGoType string
		Tag         string
		file        *File
		field       *ast.Field
	}
)

// Process processes input files or directories and generates code
func (g *Generator) Process(input string) error {
	info, err := os.Stat(input)
	if err != nil {
		return err
	}

	// Store the input root for relative path calculation
	if info.IsDir() {
		inputRoot, _ := filepath.Abs(input)
		return filepath.Walk(input, func(path string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() {
				return g.processFile(path, inputRoot)
			}
			return err
		})
	}
	inputRoot, _ := filepath.Abs(filepath.Dir(input))
	return g.processFile(input, inputRoot)
}

// Gen generates code files from processed AST data
func (g *Generator) Gen() error {
	tmpl, _ := template.New("").Parse(pkgTmpl)

	// files contains config
	filesWithCfg := []string{}
	for pth, file := range g.Files {
		if file.Config != nil {
			filesWithCfg = append(filesWithCfg, pth)
		}
	}
	sort.Strings(filesWithCfg)

	for _, file := range g.Files {
		outPath := g.outPath
		for i := len(filesWithCfg) - 1; i >= 0; i-- {
			prefixPth := filesWithCfg[i]
			curFile := g.Files[filesWithCfg[i]]
			if !curFile.Config.FileLevel {
				prefixPth = filepath.Dir(filesWithCfg[i])
			}

			if strings.HasPrefix(file.inputPath, prefixPth) {
				if outPath == defaultOutPath {
					outPath = g.Files[filesWithCfg[i]].Config.OutPath
				}

				cfg := g.Files[filesWithCfg[i]].Config
				file.applicableConfigs = append(file.applicableConfigs, cfg)
				mergeImports(&file.Imports, g.Files[filesWithCfg[i]].Imports)
			}
		}

		// Apply include/exclude filters from applicable configs
		if len(file.applicableConfigs) > 0 {
			var incI, excI, incS, excS []any
			for _, cfg := range file.applicableConfigs {
				incI = append(incI, cfg.IncludeInterfaces...)
				excI = append(excI, cfg.ExcludeInterfaces...)
				incS = append(incS, cfg.IncludeStructs...)
				excS = append(excS, cfg.ExcludeStructs...)
			}

			filePkgPath := getCurrentPackagePath(file.inputPath)
			matchAnyName := func(name string, patterns []any) bool {
				name = filePkgPath + "." + stripGeneric(name)
				for _, p := range patterns {
					if stripGeneric(fmt.Sprint(p)) == name {
						return true
					}
					if ok, _ := filepath.Match("*"+stripGeneric(fmt.Sprint(p)), filepath.Base(name)); ok {
						return true
					}
				}
				return false
			}

			if len(incI) > 0 {
				for i := len(file.Interfaces) - 1; i >= 0; i-- {
					if !matchAnyName(file.Interfaces[i].Name, incI) {
						file.Interfaces = slices.Delete(file.Interfaces, i, i+1)
					}
				}
			} else if len(excI) > 0 {
				for i := len(file.Interfaces) - 1; i >= 0; i-- {
					if matchAnyName(file.Interfaces[i].Name, excI) {
						file.Interfaces = slices.Delete(file.Interfaces, i, i+1)
					}
				}
			}

			if len(incS) > 0 {
				for i := len(file.Structs) - 1; i >= 0; i-- {
					if !matchAnyName(file.Structs[i].Name, incS) {
						file.Structs = slices.Delete(file.Structs, i, i+1)
					}
				}
			} else if len(excS) > 0 {
				for i := len(file.Structs) - 1; i >= 0; i-- {
					if matchAnyName(file.Structs[i].Name, excS) {
						file.Structs = slices.Delete(file.Structs, i, i+1)
					}
				}
			}
		}

		if len(file.Interfaces) == 0 && len(file.Structs) == 0 {
			continue
		}

		outPath = filepath.Join(outPath, file.relPath)

		var results bytes.Buffer
		if err := tmpl.Execute(&results, file); err != nil {
			return fmt.Errorf("failed to render template %v, got error %v", file.inputPath, err)
		}

		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			return fmt.Errorf("failed to create directory for %v, got error %v", outPath, err)
		}

		fmt.Printf("Generating file %s from %s...\n", outPath, file.inputPath)
		if err := os.WriteFile(outPath, results.Bytes(), 0o640); err != nil {
			return fmt.Errorf("failed to write file %v, got error %v", outPath, err)
		}

		if result, err := imports.Process(outPath, results.Bytes(), nil); err == nil {
			if err := os.WriteFile(outPath, result, 0o640); err != nil {
				return fmt.Errorf("failed to write file %v, got error %v", outPath, err)
			}
		} else {
			return fmt.Errorf("failed to format generated code for %v, got error %v", outPath, err)
		}
	}
	return nil
}

// processFile processes a single Go file and extracts AST information
func (g *Generator) processFile(inputFile, inputRoot string) error {
	inputFile, err := filepath.Abs(inputFile)
	if err != nil {
		return err
	}

	if shouldSkipFile(inputFile) {
		fmt.Printf("Skipping generated file: %s\n", inputFile)
		return nil
	}

	// Calculate relative path from input root to maintain directory structure
	relPath, err := filepath.Rel(inputRoot, inputFile)
	if err != nil {
		relPath = filepath.Base(inputFile)
	}

	fileset := token.NewFileSet()
	f, err := parser.ParseFile(fileset, inputFile, nil, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("can't parse file %q: %s", inputFile, err)
	}

	file := &File{
		Package:   f.Name.Name,
		inputPath: inputFile,
		relPath:   relPath,
		Generator: g,
	}

	// Add current package to imports for alias/path resolution and generation needs
	if pkgPath := getCurrentPackagePath(inputFile); pkgPath != "" {
		file.PackagePath = pkgPath
		file.Imports = append(file.Imports, Import{
			Name: file.Package,
			Path: pkgPath,
		})
	}

	ast.Walk(file, f)

	// Store every processed file so configs in any file are discoverable
	g.Files[file.inputPath] = file
	return nil
}

// ImportPath returns formatted import path string for template generation
func (p Import) ImportPath() string {
	if path.Base(p.Path) == p.Name {
		return fmt.Sprintf("%q", p.Path)
	}
	return fmt.Sprintf("%s %q", p.Name, p.Path)
}

// GoFullType returns the complete Go type string for a parameter
func (p Param) GoFullType() string {
	return p.Type
}

// ParamsString formats method parameters as a string for code generation
func (m Method) ParamsString() string {
	var parts []string
	hasCtx := false

	for _, p := range m.Params {
		if p.Name == "ctx" || p.Type == "context.Context" {
			hasCtx = true
			p.Name = "ctx"
		}

		parts = append(parts, fmt.Sprintf("%s %s", p.Name, p.GoFullType()))
	}

	if !hasCtx {
		parts = append([]string{"ctx context.Context"}, parts...)
	}

	return strings.Join(parts, ", ")
}

// ResultString formats method return values as a string for code generation
func (m Method) ResultString() string {
	if m.SQL.Raw != "" {
		var rets []string
		for _, r := range m.Result {
			rets = append(rets, r.GoFullType())
		}

		return strings.Join(rets, ", ")
	}
	return fmt.Sprintf("%sInterface[T]", m.Interface.IfaceName)
}

// Body generates the method body code for templates
func (m Method) Body() string {
	if m.SQL.Raw != "" {
		return m.finishMethodBody()
	}
	return m.chainMethodBody()
}

// processSQL processes SQL template strings and returns formatted SQL snippet
func (m Method) processSQL(sql string) string {
	sqlSnippet, err := RenderSQLTemplate(sql)
	if err != nil {
		panic(fmt.Sprintf("Failed to parsing SQL template for %s.%s %q: %v", m.Interface.Name, m.Name, m.SQL, err))
	}

	return sqlSnippet
}

// finishMethodBody generates method body for finishing SQL operations that return data
func (m Method) finishMethodBody() string {
	sqlSnippet := m.processSQL(m.SQL.Raw)

	if len(m.Result) == 1 {
		return fmt.Sprintf(`%s
return e.Exec(ctx, sb.String(), params...)`, sqlSnippet)
	}

	return fmt.Sprintf(`%s
var result %s
err := e.Raw(sb.String(), params...).Scan(ctx, &result)
return result, err`, sqlSnippet, m.Result[0].GoFullType())
}

// chainMethodBody generates method body for chaining SQL operations that return interface
func (m Method) chainMethodBody() string {
	switch {
	case m.SQL.Select != "":
		return fmt.Sprintf(`%s

e.Select(sb.String(), params...)

return e`, m.processSQL(m.SQL.Select))
	case m.SQL.Where != "":
		return fmt.Sprintf(`%s

e.Where(clause.Expr{SQL: sb.String(), Vars: params})

return e`, m.processSQL(m.SQL.Where))
	}
	return ""
}

// parseFieldList converts AST field list to parameter slice for method signatures
func (p *File) parseFieldList(fields *ast.FieldList) []Param {
	if fields == nil {
		return nil
	}

	var params []Param
	for _, field := range fields.List {
		names := field.Names
		if len(names) == 0 {
			names = []*ast.Ident{{Name: ""}}
		}

		for _, n := range names {
			params = append(params, Param{
				Name: n.Name,
				Type: p.parseFieldType(field.Type, "", false),
			})
		}
	}

	return params
}

var typeMap = map[string]string{
	"string":    "field.String",
	"bool":      "field.Bool",
	"[]byte":    "field.Bytes",
	"time.Time": "field.Time",
}

// Type returns the field type string for template generation
func (f Field) Type() string {
	// Check FieldTypeMap and FieldNameMap from configs first
	for _, cfg := range f.file.applicableConfigs {
		if v, ok := cfg.FieldNameMap[f.NamedGoType]; ok {
			return fmt.Sprint(v)
		}

		if v, ok := cfg.FieldTypeMap[f.GoType]; ok {
			return fmt.Sprint(v)
		}
	}

	// Check if type implements allowed interfaces
	var (
		goType  = strings.TrimPrefix(f.GoType, "*")
		pkgIdx  = strings.LastIndex(goType, ".")
		pkgName = f.file.Package
		typName = goType
	)

	if pkgIdx > 0 {
		pkgName, typName = goType[:pkgIdx], goType[pkgIdx+1:]
	}

	// Handle regular field types
	if mapped, ok := typeMap[goType]; ok {
		return mapped
	}

	if strings.Contains(goType, "int") || strings.Contains(goType, "float") {
		return fmt.Sprintf("field.Number[%s]", goType)
	}

	if typ := loadNamedType(findGoModDir(f.file.inputPath), f.file.getFullImportPath(pkgName), typName); typ != nil {
		if ImplementsAllowedInterfaces(typ) { // For interface-implementing types, use generic Field
			return fmt.Sprintf("field.Field[%s]", filepath.Base(goType))
		}
	}

	// Check if this is a relation field based on its type
	if strings.HasPrefix(goType, "[]") {
		elementType := filepath.Base(strings.TrimPrefix(goType, "[]"))
		return fmt.Sprintf("field.Slice[%s]", elementType)
	} else if strings.Contains(goType, ".") {
		return fmt.Sprintf("field.Struct[%s]", filepath.Base(goType))
	}

	return fmt.Sprintf("field.Field[%s]", filepath.Base(goType))
}

// Value returns the field value string with column name for template generation
func (f Field) Value() string {
	fieldType := f.Type()
	// Check if this is a relation field based on the type
	if strings.HasPrefix(fieldType, "field.Struct[") {
		return fmt.Sprintf("%s{}.WithName(%q)", fieldType, f.Name)
	} else if strings.HasPrefix(fieldType, "field.Slice[") {
		return fmt.Sprintf("%s{}.WithName(%q)", fieldType, f.Name)
	}

	// Regular field
	return fmt.Sprintf("%s{}.WithColumn(%q)", fieldType, f.DBName)
}

// Visit implements ast.Visitor to traverse AST nodes and extract imports, interfaces, and structs
func (p *File) Visit(n ast.Node) (w ast.Visitor) {
	switch n := n.(type) {
	case *ast.ImportSpec:
		importPath, _ := strconv.Unquote(n.Path.Value)
		importName := path.Base(importPath)
		if n.Name != nil {
			importName = n.Name.Name
		}

		p.Imports = append(p.Imports, Import{
			Name: importName,
			Path: importPath,
		})
	case *ast.GenDecl:
		if n.Tok == token.VAR {
			for _, spec := range n.Specs {
				if vs, ok := spec.(*ast.ValueSpec); ok {
					if cfg := p.tryParseConfig(vs); cfg != nil {
						p.Config = cfg
					}
				}
			}
		}
	case *ast.TypeSpec:
		if data, ok := n.Type.(*ast.InterfaceType); ok {
			p.Interfaces = append(p.Interfaces, p.processInterfaceType(n, data))
		} else if data, ok := n.Type.(*ast.StructType); ok {
			if s := p.processStructType(n, data, ""); len(s.Fields) > 0 {
				p.Structs = append(p.Structs, s)
			}
		}
	}
	return p
}

// tryParseConfig attempts to parse a gorm.io/cli/gorm/genconfig.Config composite literal
// from a package-level value spec. Returns nil if not present.
func (p *File) tryParseConfig(vs *ast.ValueSpec) *genconfig.Config {
	isCmdConfigType := func(expr ast.Expr) bool {
		return p.parseFieldType(expr, "", true) == "gorm.io/cli/gorm/genconfig.Config"
	}

	for _, v := range vs.Values {
		if cl, ok := v.(*ast.CompositeLit); ok && isCmdConfigType(cl.Type) {
			if cfg := p.parseConfigLiteral(cl); cfg != nil {
				return cfg
			}
		}
	}
	return nil
}

func (p File) UsedTypedAPI() bool {
	return p.Generator.Typed
}

// parseConfigLiteral parses a cmd.Config composite literal into a Config value.
func (p *File) parseConfigLiteral(cl *ast.CompositeLit) *genconfig.Config {
	cfg := &genconfig.Config{
		FieldTypeMap: map[any]any{},
		FieldNameMap: map[string]any{},
	}

	// Helper to collect filter values from a composite literal list (e.g., []any{...})
	collect := func(val ast.Expr) (results []any) {
		if m, ok := val.(*ast.CompositeLit); ok {
			for _, el := range m.Elts {
				if s := strLit(el); s != "" {
					results = append(results, s)
				} else {
					results = append(results, p.parseFieldType(el, p.Package, true))
				}
			}
		}
		return
	}

	for _, elt := range cl.Elts {
		kv, _ := elt.(*ast.KeyValueExpr)
		keyIdent, _ := kv.Key.(*ast.Ident)

		switch keyIdent.Name {
		case "OutPath":
			cfg.OutPath = strLit(kv.Value)
		case "FileLevel":
			if ident, ok := kv.Value.(*ast.Ident); ok {
				cfg.FileLevel = ident.Name == "true"
			}
		case "FieldTypeMap", "FieldNameMap":
			if m, ok := kv.Value.(*ast.CompositeLit); ok {
				for _, me := range m.Elts {
					if pair, ok := me.(*ast.KeyValueExpr); ok {
						// Values are wrapper type instances like JSON{} or field.Time{}
						if keyIdent.Name == "FieldNameMap" {
							if key := strLit(pair.Key); key != "" {
								cfg.FieldNameMap[key] = p.parseFieldType(pair.Value, p.Package, false)
							}
						} else {
							// Keys are Go types for FieldTypeMap
							if key := p.parseFieldType(pair.Key, "", true); key != "" {
								cfg.FieldTypeMap[key] = p.parseFieldType(pair.Value, p.Package, false)
							}
						}
					}
				}
			}
		case "IncludeInterfaces":
			cfg.IncludeInterfaces = append(cfg.IncludeInterfaces, collect(kv.Value)...)
		case "ExcludeInterfaces":
			cfg.ExcludeInterfaces = append(cfg.ExcludeInterfaces, collect(kv.Value)...)
		case "IncludeStructs":
			cfg.IncludeStructs = append(cfg.IncludeStructs, collect(kv.Value)...)
		case "ExcludeStructs":
			cfg.ExcludeStructs = append(cfg.ExcludeStructs, collect(kv.Value)...)
		}
	}
	return cfg
}

// processInterfaceType processes an interface type AST node and extracts interface metadata and methods
func (p *File) processInterfaceType(n *ast.TypeSpec, data *ast.InterfaceType) Interface {
	r := Interface{
		Name:      n.Name.Name,
		IfaceName: "_" + n.Name.Name,
		Doc:       n.Doc.Text(),
	}

	methods := data.Methods.List
	for _, m := range methods {
		for _, name := range m.Names {
			method := &Method{
				Name:      name.Name,
				Doc:       m.Doc.Text(),
				SQL:       extractSQL(m.Doc.Text(), name.Name),
				Interface: r,
			}
			r.Methods = append(r.Methods, method)

			method.Params = p.parseFieldList(m.Type.(*ast.FuncType).Params)
			method.Result = p.parseFieldList(m.Type.(*ast.FuncType).Results)

			if len(method.Result) == 0 {
				if method.SQL.Where == "" && method.SQL.Select == "" || method.SQL.Raw != "" {
					panic(fmt.Sprintf("Method %s.%s: finish method must return at least one value (last return value must be error)", n.Name.Name, method.Name))
				}
			} else if len(method.Result) > 2 {
				panic(fmt.Sprintf("Method %s.%s: maximum number of return values allowed is 2 (first as data, second as error)", n.Name.Name, method.Name))
			} else if strings.ToLower(method.Result[len(method.Result)-1].Type) != "error" {
				if len(method.Result) == 1 {
					panic(fmt.Sprintf("Method %s.%s: when only one return value is defined, its type must be error", n.Name.Name, method.Name))
				}
				panic(fmt.Sprintf("Method %s.%s: when two return values are defined, the second must be error", n.Name.Name, method.Name))
			}
		}
	}
	return r
}

// processStructType processes a struct type AST node and extracts struct metadata and fields
func (p *File) processStructType(typeSpec *ast.TypeSpec, data *ast.StructType, pkgName string) Struct {
	s := Struct{
		Name: typeSpec.Name.Name,
	}

	for _, field := range data.Fields.List {
		// Handle anonymous embedding first
		if len(field.Names) == 0 {
			if p.handleAnonymousEmbedding(field, pkgName, &s) {
				continue
			}
		}

		// Add fields to struct
		for _, n := range field.Names {
			if n.IsExported() {
				var fieldTag string
				if field.Tag != nil {
					fieldTag, _ = strconv.Unquote(field.Tag.Value)
				}

				s.Fields = append(s.Fields, Field{
					Name:        n.Name,
					DBName:      generateDBName(n.Name, fieldTag),
					GoType:      p.parseFieldType(field.Type, pkgName, true),
					NamedGoType: reflect.StructTag(fieldTag).Get("gen"),
					Tag:         fieldTag,
					file:        p,
					field:       field,
				})
			}
		}
	}

	return s
}

// parseFieldType extracts the type string from an AST field type expression
func (p *File) parseFieldType(expr ast.Expr, pkgName string, fullMode bool) string {
	switch t := expr.(type) {
	case *ast.Ident:
		// Check if it's defined locally (has an Obj and is in current file)
		if t.Obj != nil {
			// Don't add package prefix to generic type parameters
			// Generic type parameters have Obj.Decl as *ast.Field (from type parameter list)
			// Regular types have Obj.Decl as *ast.TypeSpec (from type declarations)
			if _, isField := t.Obj.Decl.(*ast.Field); isField {
				return t.Name
			}
			if fullMode && p.PackagePath != "" {
				return p.PackagePath + "." + t.Name
			}
			if p.Package != "" {
				return p.Package + "." + t.Name
			}
		}

		if pkgName != "" && !unicode.IsLower(rune(t.Name[0])) {
			if fullMode {
				return p.getFullImportPath(pkgName) + "." + t.Name
			}
			return pkgName + "." + t.Name
		}

		return t.Name
	case *ast.SelectorExpr:
		if pkgIdent, ok := t.X.(*ast.Ident); ok {
			if fullMode {
				return p.getFullImportPath(pkgIdent.Name) + "." + t.Sel.Name
			}

			return pkgIdent.Name + "." + t.Sel.Name
		}
	case *ast.IndexExpr:
		base := p.parseFieldType(t.X, pkgName, fullMode)
		idx := p.parseFieldType(t.Index, pkgName, fullMode)
		if base == "" || idx == "" {
			return ""
		}
		return base + "[" + idx + "]"
	case *ast.StarExpr:
		innerType := p.parseFieldType(t.X, pkgName, fullMode)
		return "*" + innerType
	case *ast.ArrayType:
		elementType := p.parseFieldType(t.Elt, pkgName, fullMode)
		return "[]" + elementType
	case *ast.UnaryExpr:
		// Dereference address-of composite literals: &Type{}
		if t.Op == token.AND {
			if cl, ok := t.X.(*ast.CompositeLit); ok {
				return p.parseFieldType(cl.Type, pkgName, fullMode)
			}
		}
		return p.parseFieldType(t.X, pkgName, fullMode)
	case *ast.CompositeLit:
		return p.parseFieldType(t.Type, pkgName, fullMode)
	case *ast.CallExpr:
		return p.parseFieldType(t.Fun, pkgName, fullMode)
	}
	return "any"
}

func (p *File) getFullImportPath(shortName string) string {
	for _, i := range p.Imports {
		if i.Name == shortName {
			return i.Path
		}
	}
	return shortName
}

// handleAnonymousEmbedding processes anonymous embedded fields and returns true if handled
func (p *File) handleAnonymousEmbedding(field *ast.Field, pkgName string, s *Struct) bool {
	switch t := field.Type.(type) {
	case *ast.Ident:
		// Local type embedding
		if t.Obj != nil {
			if ts, ok := t.Obj.Decl.(*ast.TypeSpec); ok {
				if st, ok := ts.Type.(*ast.StructType); ok {
					sub := p.processStructType(ts, st, pkgName)
					s.Fields = append(s.Fields, sub.Fields...)
					return true
				}
			}
		}
	case *ast.SelectorExpr:
		// External package type embedding
		if pkgIdent, ok := t.X.(*ast.Ident); ok {
			st, err := loadNamedStructType(findGoModDir(p.inputPath), p.getFullImportPath(pkgIdent.Name), t.Sel.Name)
			if err == nil && st != nil {
				sub := p.processStructType(&ast.TypeSpec{Name: &ast.Ident{Name: t.Sel.Name}}, st, pkgIdent.Name)
				s.Fields = append(s.Fields, sub.Fields...)
				return true
			}
		}
	case *ast.StructType:
		// Anonymous inline struct embedding
		sub := p.processStructType(&ast.TypeSpec{Name: &ast.Ident{Name: "AnonymousStruct"}}, t, pkgName)
		s.Fields = append(s.Fields, sub.Fields...)
		return true
	}

	return false
}
