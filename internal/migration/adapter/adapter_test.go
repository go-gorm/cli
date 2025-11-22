package adapter

import "testing"

func TestBuildStructTag(t *testing.T) {
	got := buildStructTag(map[string]string{
		"json": "account_id",
		"gorm": "column:id;primaryKey",
		"xml":  "account",
	})
	const want = "`gorm:\"column:id;primaryKey\" json:\"account_id\" xml:\"account\"`"
	if got != want {
		t.Fatalf("buildStructTag() = %s, want %s", got, want)
	}
}

func TestBuildStructTagEmpty(t *testing.T) {
	if got := buildStructTag(nil); got != "" {
		t.Fatalf("expected empty tag for nil map, got %q", got)
	}
	if got := buildStructTag(map[string]string{"json": "   "}); got != "" {
		t.Fatalf("expected empty tag when values trim to empty, got %q", got)
	}
}
