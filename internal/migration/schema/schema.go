package schema

// Table is a universal, neutral representation of a database table.
type Table struct {
	Name     string
	Fields   []*Field
	Indexes  []*Index
	Comment  string
	ModelRef string // Optional: points to its source in the code, e.g., "models.User".
}

// Field describes the universal properties of a field.
type Field struct {
	DBName        string
	DataType      string // e.g., "varchar", "int"
	IsPrimaryKey  bool
	IsNullable    bool
	IsUnique      bool
	Size          int
	Precision     int
	Scale         int
	DefaultValue  *string // Uses a pointer to distinguish between nil and an empty string "".
	AutoIncrement bool
	Comment       string
}

// Index describes an index.
type Index struct {
	Name     string
	Columns  []string
	IsUnique bool
	Option   string
}
