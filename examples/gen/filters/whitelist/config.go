package whitelist

import (
	"gorm.io/cli/gorm/genconfig"
)

// Only generate I1 and S1
var _ = genconfig.Config{
	IncludeInterfaces: []any{"I1"},
	IncludeStructs:    []any{"S1"},
}
