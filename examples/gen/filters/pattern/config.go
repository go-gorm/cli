package pattern

import "gorm.io/cli/gorm/genconfig"

// Include only interfaces whose names start with "Query"
var _ = genconfig.Config{
	IncludeInterfaces: []any{"Query*"},
}
