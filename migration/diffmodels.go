package migration

var diffModels []any

// RegisterDiffModels records model instances used for diff helpers.
func RegisterDiffModels(models ...any) {
	diffModels = models
}
