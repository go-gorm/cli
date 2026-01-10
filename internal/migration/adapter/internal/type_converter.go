package internal

import (
	"database/sql"
	"database/sql/driver"
	"reflect"
	"strings"
)

// TypeConverter handles conversion between database and Go types.
type TypeConverter struct{}

// NewTypeConverter creates a new type converter.
func NewTypeConverter() *TypeConverter {
	return &TypeConverter{}
}

// ToGoType converts a database column type to a Go type string.
func (tc *TypeConverter) ToGoType(col interface{
	Name() string
	DatabaseTypeName() string
	ScanType() reflect.Type
	Nullable() (bool, bool)
	PrimaryKey() (bool, bool)
}, dialect string) (string, []string) {
	_ = dialect
	isNullable, _ := col.Nullable()
	if pk, _ := col.PrimaryKey(); pk {
		isNullable = false
	}

	scanType := col.ScanType()
	if scanType == nil {
		return "any", nil
	}

	if tc.isSQLNullType(scanType) {
		if !isNullable {
			if base, imports, ok := tc.sqlNullBaseType(scanType.Name()); ok {
				return base, imports
			}
		}
		goType, imports := tc.reflectTypeString(scanType)
		return goType, imports
	}

	baseType := scanType
	if baseType.Kind() == reflect.Pointer {
		baseType = baseType.Elem()
	}

	goType, imports := tc.reflectTypeString(baseType)
	if isNullable {
		if baseType.Kind() == reflect.Slice || tc.implementsScannerOrValuer(baseType) {
			return goType, imports
		}
		return "*" + goType, imports
	}

	return goType, imports
}

func (tc *TypeConverter) reflectTypeString(rt reflect.Type) (string, []string) {
	switch rt.Kind() {
	case reflect.Pointer:
		inner, imports := tc.reflectTypeString(rt.Elem())
		return "*" + inner, imports
	case reflect.Slice:
		inner, imports := tc.reflectTypeString(rt.Elem())
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

func (tc *TypeConverter) isSQLNullType(rt reflect.Type) bool {
	return rt.PkgPath() == "database/sql" && strings.HasPrefix(rt.Name(), "Null")
}

func (tc *TypeConverter) sqlNullBaseType(name string) (string, []string, bool) {
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

var (
	scannerIface = reflect.TypeOf((*sql.Scanner)(nil)).Elem()
	valuerIface  = reflect.TypeOf((*driver.Valuer)(nil)).Elem()
)

func (tc *TypeConverter) implementsScannerOrValuer(rt reflect.Type) bool {
	if rt.Implements(scannerIface) || rt.Implements(valuerIface) {
		return true
	}
	if rt.Kind() != reflect.Ptr {
		ptr := reflect.PointerTo(rt)
		return ptr.Implements(scannerIface) || ptr.Implements(valuerIface)
	}
	return false
}
