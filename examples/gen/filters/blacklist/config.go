package blacklist

import (
	"gorm.io/cli/gorm/genconfig"
)

// Exclude I2 and S2
var _ = genconfig.Config{
	ExcludeInterfaces: []any{I2[any](nil)},
	ExcludeStructs:    []any{S2{}},
}
