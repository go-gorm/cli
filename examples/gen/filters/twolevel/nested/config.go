package nested

import "gorm.io/cli/gorm/genconfig"

// Child config excludes I3/S3 within nested directory
var _ = genconfig.Config{
	FileLevel:         false,
	ExcludeInterfaces: []any{"I3"},
	ExcludeStructs:    []any{"S3"},
}
