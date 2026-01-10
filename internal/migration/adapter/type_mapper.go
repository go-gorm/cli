package adapter

import (
	"database/sql"
	"database/sql/driver"
	"reflect"
	"strings"

	"gorm.io/gorm"
)

var (
	scannerIface = reflect.TypeOf((*sql.Scanner)(nil)).Elem()
	valuerIface  = reflect.TypeOf((*driver.Valuer)(nil)).Elem()
)

func toGoType(col gorm.ColumnType, dialect string) (string, []string) {
	_ = dialect
	isNullable, _ := col.Nullable()
	if pk, _ := col.PrimaryKey(); pk {
		isNullable = false
	}
	scanType := col.ScanType()
	if scanType == nil {
		return "any", nil
	}
	if isSQLNullType(scanType) {
		if !isNullable {
			if base, imports, ok := sqlNullBaseType(scanType.Name()); ok {
				return base, imports
			}
		}
		goType, imports := reflectTypeString(scanType)
		return goType, imports
	}
	baseType := scanType
	if baseType.Kind() == reflect.Pointer {
		baseType = baseType.Elem()
	}
	goType, imports := reflectTypeString(baseType)
	if isNullable {
		if baseType.Kind() == reflect.Slice || implementsScannerOrValuer(baseType) {
			return goType, imports
		}
		return "*" + goType, imports
	}
	return goType, imports
}

func reflectTypeString(rt reflect.Type) (string, []string) {
	switch rt.Kind() {
	case reflect.Pointer:
		inner, imports := reflectTypeString(rt.Elem())
		return "*" + inner, imports
	case reflect.Slice:
		inner, imports := reflectTypeString(rt.Elem())
		if inner == "uint8" {
			return "[]byte", imports
		}
		return "[]" + inner, imports
	default:
		pkg := rt.PkgPath()
		name := rt.String()
		if pkg == "" {
			return name, nil
		}
		return name, []string{pkg}
	}
}

func isSQLNullType(rt reflect.Type) bool {
	return rt.PkgPath() == "database/sql" && strings.HasPrefix(rt.Name(), "Null")
}

func sqlNullBaseType(name string) (string, []string, bool) {
	switch name {
	case "NullString":
		return "string", nil, true
	case "NullBool":
		return "bool", nil, true
	case "NullInt16":
		return "int16", nil, true
	case "NullInt32":
		return "int32", nil, true
	case "NullInt64":
		return "int64", nil, true
	case "NullFloat64":
		return "float64", nil, true
	case "NullTime":
		return "time.Time", []string{"time"}, true
	default:
		return "", nil, false
	}
}

func implementsScannerOrValuer(rt reflect.Type) bool {
	if rt.Implements(scannerIface) || rt.Implements(valuerIface) {
		return true
	}
	if rt.Kind() != reflect.Ptr {
		ptr := reflect.PointerTo(rt)
		return ptr.Implements(scannerIface) || ptr.Implements(valuerIface)
	}
	return false
}
