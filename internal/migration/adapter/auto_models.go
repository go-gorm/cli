package adapter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/types"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
	"gorm.io/cli/gorm/internal/project"
	"gorm.io/gorm/schema"
)

type modelStruct struct {
	PackagePath string
	TypeName    string
}

type helperImport struct {
	Alias string
	Path  string
}

type helperTarget struct {
	Alias       string
	PackagePath string
	TypeName    string
}

type helperTemplateData struct {
	Imports        []helperImport
	Targets        []helperTarget
	NamingStrategy schema.NamingStrategy
}

func (a *DBAdapter) collectModelSchemas() (map[string]*TableSchema, error) {
	root := project.Root()
	if root == "" {
		return nil, fmt.Errorf("unable to determine project root")
	}
	if _, err := os.Stat(a.cfg.ModelsDir); err != nil {
		if os.IsNotExist(err) {
			return map[string]*TableSchema{}, nil
		}
		return nil, fmt.Errorf("stat models dir: %w", err)
	}
	structs, err := discoverModelStructs(root, a.cfg.ModelsDir)
	if err != nil {
		return nil, err
	}
	if len(structs) == 0 {
		return map[string]*TableSchema{}, nil
	}
	data := buildHelperTemplateData(structs, a.db.NamingStrategy)
	source, err := renderModelHelper(data)
	if err != nil {
		return nil, err
	}
	output, err := runModelHelper(root, source)
	if err != nil {
		return nil, err
	}
	var snapshots []helperModelSnapshot
	if err := json.Unmarshal(output, &snapshots); err != nil {
		return nil, fmt.Errorf("decode model metadata: %w", err)
	}
	return parseHelperSnapshots(snapshots)
}

func discoverModelStructs(root, modelsDir string) ([]modelStruct, error) {
	rel, err := filepath.Rel(root, modelsDir)
	if err != nil {
		return nil, fmt.Errorf("models directory must reside within the module root: %w", err)
	}
	rel = filepath.ToSlash(rel)
	pattern := "./" + rel
	if rel == "." {
		pattern = "./..."
	} else {
		pattern += "/..."
	}
	cfg := &packages.Config{
		Mode: packages.NeedTypes | packages.NeedName,
		Dir:  root,
	}
	pkgs, err := packages.Load(cfg, pattern)
	if err != nil {
		return nil, fmt.Errorf("load model packages: %w", err)
	}
	var structs []modelStruct
	seen := make(map[string]struct{})
	for _, pkg := range pkgs {
		if pkg == nil || pkg.Types == nil || pkg.Types.Scope() == nil {
			continue
		}
		if strings.HasSuffix(pkg.PkgPath, ".test") {
			continue
		}
		if len(pkg.Errors) > 0 {
			return nil, fmt.Errorf("load package %s: %v", pkg.PkgPath, pkg.Errors[0])
		}
		scope := pkg.Types.Scope()
		for _, name := range scope.Names() {
			obj := scope.Lookup(name)
			typeName, ok := obj.(*types.TypeName)
			if !ok || !typeName.Exported() {
				continue
			}
			if _, ok := typeName.Type().Underlying().(*types.Struct); !ok {
				continue
			}
			key := pkg.PkgPath + "." + name
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			structs = append(structs, modelStruct{PackagePath: pkg.PkgPath, TypeName: name})
		}
	}
	sort.Slice(structs, func(i, j int) bool {
		if structs[i].PackagePath == structs[j].PackagePath {
			return structs[i].TypeName < structs[j].TypeName
		}
		return structs[i].PackagePath < structs[j].PackagePath
	})
	return structs, nil
}

func buildHelperTemplateData(structs []modelStruct, ns schema.Namer) helperTemplateData {
	imports := make([]helperImport, 0)
	targets := make([]helperTarget, 0, len(structs))
	aliasMap := make(map[string]string)
	for _, st := range structs {
		alias, ok := aliasMap[st.PackagePath]
		if !ok {
			alias = fmt.Sprintf("pkg%d", len(aliasMap))
			aliasMap[st.PackagePath] = alias
			imports = append(imports, helperImport{Alias: alias, Path: st.PackagePath})
		}
		targets = append(targets, helperTarget{Alias: alias, PackagePath: st.PackagePath, TypeName: st.TypeName})
	}
	return helperTemplateData{Imports: imports, Targets: targets, NamingStrategy: ns}
}

func renderModelHelper(data helperTemplateData) (string, error) {
	var buf bytes.Buffer
	if err := modelHelperTpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("render helper: %w", err)
	}
	return buf.String(), nil
}

func runModelHelper(root, source string) ([]byte, error) {
	tmpDir, err := os.MkdirTemp(root, "gorm-migrate-models-*")
	if err != nil {
		return nil, fmt.Errorf("create helper dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)
	helperFile := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(helperFile, []byte(source), 0o644); err != nil {
		return nil, fmt.Errorf("write helper: %w", err)
	}
	cmd := exec.Command("go", "run", helperFile)
	cmd.Dir = root
	cmd.Env = os.Environ()
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("run helper: %w\n%s", err, stderr.String())
	}
	return stdout.Bytes(), nil
}
