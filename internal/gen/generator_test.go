package gen

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"text/template"
)

func TestParseTemplate(t *testing.T) {
	if _, err := template.New("").Parse(pkgTmpl); err != nil {
		t.Errorf("failed to parse template, got %v", err)
	}
}

func TestLoadNamedTypes(t *testing.T) {
	for _, i := range allowedInterfaces {
		if i == nil {
			t.Fatalf("failed to load named type, got nil")
		}
	}
}

func TestGeneratorWithQueryInterface(t *testing.T) {
	inputPath, err := filepath.Abs("../../examples/query.go")
	if err != nil {
		t.Fatalf("failed to get absolute path: %v", err)
	}

	goldenPath, err := filepath.Abs("../../examples/output/query.go")
	if err != nil {
		t.Fatalf("failed to get absolute output path: %v", err)
	}

	outputDir := filepath.Join(t.TempDir(), "output")

	g := &Generator{Files: map[string]*File{}, outPath: outputDir}

	if err := g.Process(inputPath); err != nil {
		t.Fatalf("Process error: %v", err)
	}
	if err := g.Gen(); err != nil {
		t.Fatalf("Gen error: %v", err)
	}

	files, err := os.ReadDir(outputDir)
	if err != nil {
		t.Fatalf("failed to read output dir: %v", err)
	}
	if len(files) == 0 {
		t.Fatalf("no files were generated in %s", outputDir)
	}

	goldenBytes, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("failed to read golden file %s: %v", goldenPath, err)
	}
	goldenStr := string(goldenBytes)

	generatedFile := filepath.Join(outputDir, files[0].Name())
	genBytes, err := os.ReadFile(generatedFile)
	if err != nil {
		t.Fatalf("failed to read generated file %s: %v", generatedFile, err)
	}
	generatedStr := string(genBytes)

	if _, err := parser.ParseFile(token.NewFileSet(), generatedFile, genBytes, parser.AllErrors); err != nil {
		t.Errorf("generated code %s has invalid Go syntax: %v", generatedFile, err)
	}

	if goldenStr != generatedStr {
		t.Errorf("generated file differs from golden file\nGOLDEN: %s\nGENERATED: %s\n%s",
			goldenPath, generatedFile, generatedStr)
	}
}

func TestProcessStructType(t *testing.T) {
	fileset := token.NewFileSet()
	file, err := parser.ParseFile(fileset, "../../examples/models/user.go", nil, parser.AllErrors)
	if err != nil {
		t.Fatalf("failed to parse file: %v", err)
	}

	var structType *ast.StructType

	ast.Inspect(file, func(n ast.Node) bool {
		typeSpec, ok := n.(*ast.TypeSpec)
		if ok && typeSpec.Name.Name == "User" {
			structType = typeSpec.Type.(*ast.StructType)
			return false
		}
		return true
	})

	if structType == nil {
		t.Fatalf("failed to find User struct")
	}

	expected := Struct{
		Name: "User",
		Fields: []Field{
			{Name: "ID", DBName: "id", GoType: "uint"},
			{Name: "CreatedAt", DBName: "created_at", GoType: "time.Time"},
			{Name: "UpdatedAt", DBName: "updated_at", GoType: "time.Time"},
			{Name: "DeletedAt", DBName: "deleted_at", GoType: "gorm.io/gorm.DeletedAt"},
			{Name: "Name", DBName: "name", GoType: "string"},
			{Name: "Age", DBName: "age", GoType: "int"},
			{Name: "Birthday", DBName: "birthday", GoType: "*time.Time"},
			{Name: "Score", DBName: "score", GoType: "sql.NullInt64"},
			{Name: "LastLogin", DBName: "last_login", GoType: "sql.NullTime"},
			{Name: "Account", DBName: "account", GoType: "Account"},
			{Name: "Pets", DBName: "pets", GoType: "[]*Pet"},
			{Name: "Toys", DBName: "toys", GoType: "[]Toy"},
			{Name: "CompanyID", DBName: "company_id", GoType: "*int"},
			{Name: "Company", DBName: "company", GoType: "Company"},
			{Name: "ManagerID", DBName: "manager_id", GoType: "*uint"},
			{Name: "Manager", DBName: "manager", GoType: "*User"},
			{Name: "Team", DBName: "team", GoType: "[]User"},
			{Name: "Languages", DBName: "languages", GoType: "[]Language"},
			{Name: "Friends", DBName: "friends", GoType: "[]*User"},
			{Name: "Role", DBName: "role", GoType: "string"},
			{Name: "IsAdult", DBName: "is_adult", GoType: "bool"},
			{Name: "Profile", DBName: "profile", GoType: "string", NamedGoType: "json"},
		},
	}

	p := File{
		Imports: []Import{
			{Name: "gorm", Path: "gorm.io/gorm"},
		},
	}

	result := p.processStructType(&ast.TypeSpec{Name: &ast.Ident{Name: "User"}}, structType, "")
	// Only compare stable fields (Name, DBName, GoType); ignore tags/alias and internal pointers.
	trimmed := Struct{Name: result.Name}
	for _, f := range result.Fields {
		trimmed.Fields = append(trimmed.Fields, Field{Name: f.Name, DBName: f.DBName, GoType: f.GoType, NamedGoType: f.NamedGoType})
	}
	if !reflect.DeepEqual(trimmed, expected) {
		t.Errorf("Expected %+v, \n got %+v", expected, trimmed)
	}
}
