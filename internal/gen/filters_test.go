package gen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readFileMust(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read %s: %v", p, err)
	}
	return string(b)
}

func readAllGeneratedGoFiles(t *testing.T, dir string) string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir %s: %v", dir, err)
	}
	var b strings.Builder
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		p := filepath.Join(dir, e.Name())
		b.WriteString(readFileMust(t, p))
		b.WriteString("\n\n")
	}
	s := b.String()
	if s == "" {
		t.Fatalf("no .go files in %s", dir)
	}
	return s
}

func TestFilters_Whitelist(t *testing.T) {
	inputDir, err := filepath.Abs("../../examples/gen/filters/whitelist")
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "whitelist")

	g := &Generator{Files: map[string]*File{}, outPath: out}
	if err := g.Process(inputDir); err != nil {
		t.Fatalf("Process: %v", err)
	}
	if err := g.Gen(); err != nil {
		t.Fatalf("Gen: %v", err)
	}

	// The generator preserves relative paths; find the generated file under out
	// Locate any generated file and assert contents
	content := readAllGeneratedGoFiles(t, out)

	// I1 must be present; I2 must not
	if !strings.Contains(content, "func I1[") {
		t.Fatalf("expected I1 to be generated")
	}
	if strings.Contains(content, "func I2[") {
		t.Fatalf("expected I2 to be filtered out by whitelist")
	}
	// S1 var present; S2 not present
	if !strings.Contains(content, "var S1 = struct") {
		t.Fatalf("expected S1 helper struct to be generated")
	}
	if strings.Contains(content, "var S2 = struct") {
		t.Fatalf("expected S2 to be filtered out by whitelist")
	}
}

func TestFilters_Blacklist(t *testing.T) {
	inputDir, err := filepath.Abs("../../examples/gen/filters/blacklist")
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "blacklist")

	g := &Generator{Files: map[string]*File{}, outPath: out}
	if err := g.Process(inputDir); err != nil {
		t.Fatalf("Process: %v", err)
	}
	if err := g.Gen(); err != nil {
		t.Fatalf("Gen: %v", err)
	}

	content := readAllGeneratedGoFiles(t, out)

	// I2 excluded; I1 included
	if strings.Contains(content, "func I2[") {
		t.Fatalf("expected I2 to be excluded by blacklist")
	}
	if !strings.Contains(content, "func I1[") {
		t.Fatalf("expected I1 to be generated")
	}
	// S2 excluded; S1 included
	if strings.Contains(content, "var S2 = struct") {
		t.Fatalf("expected S2 to be excluded by blacklist")
	}
	if !strings.Contains(content, "var S1 = struct") {
		t.Fatalf("expected S1 to be generated")
	}
}

func TestFilters_TwoLevel(t *testing.T) {
	inputDir, err := filepath.Abs("../../examples/gen/filters/twolevel")
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "twolevel")

	g := &Generator{Files: map[string]*File{}, outPath: out}
	if err := g.Process(inputDir); err != nil {
		t.Fatalf("Process: %v", err)
	}
	if err := g.Gen(); err != nil {
		t.Fatalf("Gen: %v", err)
	}

	// Root-level outputs
	rootIface := filepath.Join(out, "iface.go")
	rootModels := filepath.Join(out, "models.go")
	rIface := readFileMust(t, rootIface)
	rModels := readFileMust(t, rootModels)

	if !strings.Contains(rIface, "func I1[") || !strings.Contains(rIface, "func I2[") || strings.Contains(rIface, "func I3[") {
		t.Fatalf("root: expected I1, I2 to be generated, I3 not generated")
	}
	if !strings.Contains(rModels, "var S1 = struct") || !strings.Contains(rModels, "var S2 = struct") || strings.Contains(rModels, "var S3 = struct") {
		t.Fatalf("root: expected S1, S2 to be generated, S3 not generated")
	}

	// Nested outputs
	nestedIface := filepath.Join(out, "nested", "iface.go")
	nestedModels := filepath.Join(out, "nested", "models.go")
	nIface := readFileMust(t, nestedIface)
	nModels := readFileMust(t, nestedModels)

	// Parent excludes I2; child excludes I3. Combined: only I1 generated in nested
	if strings.Contains(nIface, "func I2[") || strings.Contains(nIface, "func I3[") {
		t.Fatalf("nested: I2 and I3 should be excluded by parent+child config")
	}
	if !strings.Contains(nIface, "func I1[") {
		t.Fatalf("nested: expected I1 to be generated")
	}
	if strings.Contains(nModels, "var S2 = struct") || strings.Contains(nModels, "var S3 = struct") {
		t.Fatalf("nested: S2 and S3 should be excluded by parent+child config")
	}
	if !strings.Contains(nModels, "var S1 = struct") {
		t.Fatalf("nested: expected S1 to be generated")
	}
}

func TestFilters_PatternInclude(t *testing.T) {
	inputDir, err := filepath.Abs("../../examples/gen/filters/pattern")
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "pattern")

	g := &Generator{Files: map[string]*File{}, outPath: out}
	if err := g.Process(inputDir); err != nil {
		t.Fatalf("Process: %v", err)
	}
	if err := g.Gen(); err != nil {
		t.Fatalf("Gen: %v", err)
	}

	// All output .go files in out
	content := readAllGeneratedGoFiles(t, out)

	// Should include only Query* interfaces (QueryUser, QueryOrder)
	if !strings.Contains(content, "func QueryUser[") || !strings.Contains(content, "func QueryOrder[") {
		t.Fatalf("expected QueryUser and QueryOrder to be generated")
	}
	if strings.Contains(content, "func Service[") {
		t.Fatalf("Service should be excluded by IncludeInterfaces pattern Query*")
	}
}
