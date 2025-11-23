package migration

import "reflect"

var diffModels []any

// RegisterDiffModels records model instances used for diff helpers.
func RegisterDiffModels(models ...any) {
	for _, m := range models {
		registerDiffModel(m)
	}
}

func registerDiffModel(m any) {
	if m == nil {
		return
	}
	val := reflect.ValueOf(m)
	kind := val.Kind()
	if kind == reflect.Slice || kind == reflect.Array {
		for i := 0; i < val.Len(); i++ {
			registerDiffModel(val.Index(i).Interface())
		}
		return
	}
	typ := val.Type()
	if typ.Kind() != reflect.Ptr || typ.Elem().Kind() != reflect.Struct {
		return
	}
	diffModels = append(diffModels, m)
}

func snapshotDiffModels() []any {
	if len(diffModels) == 0 {
		return nil
	}
	out := make([]any, len(diffModels))
	copy(out, diffModels)
	return out
}
