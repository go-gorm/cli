# GORM CLI

GORM CLI generates two complementary pieces of code for your GORM projects:

- **Interface-driven, type-safe query APIs** (from Go interfaces with SQL templates)
- **Model-driven field helpers** (from your model structs for filters, updates, and associations)

Together they give you compile-time safety and a fluent, discoverable API for all database operations.

## 🚀 Features

- **Interface-driven Query Generation**: Type-safe query APIs generated from Go interfaces with SQL templates
- **Model-driven Field Helpers**: Generate helper methods from model structs for filters, updates, ordering, and associations
- **Enhanced Type Safety**: Full compile-time safety using Go generics for all generated code (with `--typed` flag)
- **Association Operations**: Create/CreateInBatch/Update/Unlink/Delete with compile-time safety
- **Configurable Generation**: Customize output via `genconfig.Config` (OutPath, Include/Exclude, FileLevel, Custom field mapping)
- **Seamless GORM Integration**: Works smoothly with `gorm.io/gorm`

## 📦 Installation

Requires Go 1.18+ (generics).

```bash
go install gorm.io/cli/gorm@latest
```

## ⚡ Quick Start

1) Write a query interface (with SQL templates) and define your models in the same package or directory.

```go
// examples/query.go
type Query[T any] interface {
  // SELECT * FROM @@table WHERE id=@id
  GetByID(id int) (T, error)

  // where("name=@name AND age=@age")
  FilterByNameAndAge(name string, age int)
}

// examples/models/user.go
type User struct {
  gorm.Model
  Name string
  Age  int
}
```

2) Generate code

```bash
# Generate standard APIs
gorm gen -i ./examples -o ./generated

# Generate enhanced type-safe APIs
gorm gen -i ./examples -o ./generated --typed
```

3) Use the generated APIs

```go
// SELECT * FROM users WHERE id=123
u, err := generated.Query[User](db).GetByID(ctx, 123)

// SELECT * FROM users WHERE `age` > 18
users, err := gorm.G[User](db).Where(generated.User.Age.Gt(18)).Find(ctx)

// SELECT * FROM users WHERE `age` > 18
u, err := generated.Query[User](db).Where(generated.User.Age.Gt(18)).Find(ctx)

// Only with standard APIs (not --typed) can you mix string conditions with typed options
// SELECT * FROM users WHERE name = "jinzhu" AND `age` > 18
u, err := generated.Query[User](db).Where("name = ?", "jinzhu").Where(generated.User.Age.Gt(18)).Find(ctx)
```


## 🔎 Two Generators, One Workflow

1. **Query API Generation**: Write methods with SQL templates in Go interface comments to generate concrete, type-safe methods
2. **Field Helper Generation**: Generate strongly typed helpers for basic fields and associations from your model structs

### Field Helper Generation Rules:

- Basic fields include ints/floats/string/bool/time/[]byte and named types implementing Scanner/Valuer or GORM Serializer.
- Associations (has one/has many/belongs to/many2many, including polymorphic) become association helpers.


## 🧪 Working With Basic Fields

Example Model

```go
type User struct {
    ID        uint
    Name      string
    Email     string
    Age       int
    Status    string
    CreatedAt time.Time
}
```

Common predicates and setters

```go
// Predicates
generated.User.ID.Eq(1)                 // id = 1
generated.User.Name.Like("%jinzhu%")    // name LIKE '%jinzhu%'
generated.User.Age.Between(18, 65)      // age BETWEEN 18 AND 65
generated.User.Score.IsNull()           // score IS NULL (sql.NullInt64)

// Updates with zero‑values and expressions
gorm.G[User](db).
  Where(generated.User.Name.Eq("alice")).
  Set(
    generated.User.Name.Set("jinzhu"),
    generated.User.IsAdult.Set(false),
    generated.User.Score.Set(sql.NullInt64{}),
    generated.User.Age.Incr(1),
    generated.User.Age.SetExpr(clause.Expr{SQL: "GREATEST(?, ?)", Vars: []any{clause.Column{Name: "age"}, 18}}),
  ).
  Update(ctx)

// Create with Set
gorm.G[User](db).
  Set(
    generated.User.Name.Set("alice"),
    generated.User.Age.Set(0),
    generated.User.IsAdult.Set(false),
    generated.User.Role.Set("active"),
  ).
  Create(ctx)
```


## 🤝 Working With Associations

Association helpers live on generated models as `field.Struct[T]` or `field.Slice[T]`, e.g. `generated.User.Account`, `generated.User.Pets`.

Supported operations (composed into `Set(...).Update(ctx)` or `Set(...).Create(ctx)`):
- Create: create and link a related row per matched parent
- Update: update associated rows matching optional conditions
- Unlink: remove the link only (FK NULL or delete join rows), can include conditions
- Delete: delete associated rows (m2m deletes join rows only), can include conditions
- CreateInBatch: batch create/link using a slice of values

Examples

```go
// Create a new user and one pet (create + associate)
gorm.G[User](db).
  Set(
    generated.User.Name.Set("alice"),
    generated.User.Pets.Create(generated.Pet.Name.Set("fido")),
  ).
  Create(ctx)

// Create a new user and link two languages (many2many)
gorm.G[User](db).
  Set(
    generated.User.Name.Set("polyglot"),
    generated.User.Languages.CreateInBatch([]models.Language{{Code: "EN"}, {Code: "FR"}}),
  ).
  Create(ctx)

// Create one pet for each matched user (has many)
gorm.G[User](db).
  Where(generated.User.ID.Eq(1)).
  Set(generated.User.Pets.Create(generated.Pet.Name.Set("fido"))).
  Update(ctx)

// Update a user's pet where name = 'fido'
gorm.G[User](db).
  Where(generated.User.ID.Eq(1)).
  Set(generated.User.Pets.Where(generated.Pet.Name.Eq("fido")).
    Update(generated.Pet.Name.Set("rex")),
  ).
  Update(ctx)

// Unlink semantics
// - belongs to: clears parent FK; has one/has many: clears child FK; m2m: removes join rows
gorm.G[User](db).
  Where(generated.User.ID.Eq(1)).
  Set(generated.User.Pets.Unlink()).
  Update(ctx)

// Delete associated rows
gorm.G[User](db).
  Where(generated.User.ID.Eq(1)).
  Set(generated.User.Pets.Delete()).
  Update(ctx)

// Unlink/Delete with conditions (filter target side before acting)
gorm.G[User](db).
  Where(generated.User.ID.Eq(1)).
  Set(generated.User.Pets.Where(generated.Pet.Name.Eq("fido")).Unlink()).
  Update(ctx)

gorm.G[User](db).
  Where(generated.User.ID.Eq(1)).
  Set(generated.User.Pets.Where(generated.Pet.Name.Eq("old")).Delete()).
  Update(ctx)

// Batch link (has many / many2many) for an existing user
gorm.G[User](db).
  Where(generated.User.ID.Eq(1)).
  Set(generated.User.Languages.CreateInBatch([]models.Language{{Code: "EN"}, {Code: "FR"}})).
  Update(ctx)
```

Semantics by association type
- belongs to: Unlink sets parent FK to NULL; Delete removes associated rows
- has one / has many: Unlink sets child FK to NULL; Delete removes child rows
- many2many: Unlink/Delete remove join rows only; both sides remain

Parent operation semantics
- Create(ctx): inserts new parent rows using values set via Set(...), then applies association operations.
- Update(ctx): updates matched parent rows (using current Where/Select), then applies association operations.

See end‑to‑end examples in `examples/output/models_relations_test.go`.


## 🧩 Template‑based Queries

Write SQL/templating in interface method comments. Placeholders bind to parameters automatically; implementations are generated and type‑safe.

```go
type Query[T any] interface {
  // SELECT * FROM @@table WHERE id=@id
  GetByID(id int) (T, error)

  // SELECT * FROM @@table WHERE @@column=@value
  FilterWithColumn(column string, value string) (T, error)

  // SELECT * FROM @@table
  // {{where}}
  //   {{if @user.Name }} name=@user.Name {{end}}
  //   {{if @user.Age > 0}} AND age=@user.Age {{end}}
  // {{end}}
  SearchUsers(user User) ([]T, error)

  // UPDATE @@table
  // {{set}}
  //   {{if user.Name != ""}} name=@user.Name, {{end}}
  //   {{if user.Age > 0}} age=@user.Age, {{end}}
  //   {{if user.Age >= 18}} is_adult=1 {{else}} is_adult=0 {{end}}
  // {{end}}
  // WHERE id=@id
  UpdateUser(user User, id int) error
}
```

Usage notes
- Context auto‑injection: if a method doesn’t include `ctx context.Context`, the generator adds it to the implementation signature.

Example usage

```go
// SQL: SELECT * FROM users WHERE id=123
user, err := generated.Query[User](db).GetByID(ctx, 123)

// SQL: SELECT * FROM users WHERE name="jinzhu" AND age=25 (appended to current builder)
users, err := generated.Query[User](db).FilterByNameAndAge("jinzhu", 25).Find(ctx)

// SQL UPDATE users SET name="jinzhu", age=20, is_adult=1 WHERE id=1
err := generated.Query[User](db).UpdateUser(ctx, User{Name: "jinzhu", Age: 20}, 1)
```

### 📝 Template DSL

GORM CLI provides a SQL template DSL:

| Directive   | Purpose                            | Example                                  |
| ----------- | ---------------------------------- | ---------------------------------------- |
| `@@table`   | Resolves to the model’s table name | `SELECT * FROM @@table WHERE id=@id`     |
| `@@column`  | Dynamic column binding             | `@@column=@value`                        |
| `@param`    | Maps Go params to SQL params       | `WHERE name=@user.Name`                  |
| `{{where}}` | Conditional WHERE clause           | `{{where}} age > 18 {{end}}`             |
| `{{set}}`   | Conditional SET clause (UPDATE)    | `{{set}} name=@name {{end}}`             |
| `{{if}}`    | Conditional SQL fragment           | `{{if age > 0}} AND age=@age {{end}}`    |
| `{{for}}`   | Iteration over a collection        | `{{for _, t := range tags}} ... {{end}}` |

### Examples

```sql
-- Safe parameter binding
SELECT * FROM @@table WHERE id=@id AND status=@status

-- Dynamic column binding
SELECT * FROM @@table WHERE @@column=@value

-- Conditional WHERE
SELECT * FROM @@table
{{where}}
  {{if name != ""}} name=@name {{end}}
  {{if age > 0}} AND age=@age {{end}}
{{end}}

-- Dynamic UPDATE
UPDATE @@table
{{set}}
  {{if user.Name != ""}} name=@user.Name, {{end}}
  {{if user.Email != ""}} email=@user.Email {{end}}
{{end}}
WHERE id=@id

-- Iteration
SELECT * FROM @@table
{{where}}
  {{for _, tag := range tags}}
    {{if tag != ""}} tags LIKE concat('%',@tag,'%') OR {{end}}
  {{end}}
{{end}}
```


## Generation Config (optional)

You don’t need any configuration to use the generator. For overrides, declare a package‑level `genconfig.Config` in the package being generated — the generator will pick it up automatically.

```go
package examples

import (
    "database/sql"
    "gorm.io/cli/gorm/field"
    "gorm.io/cli/gorm/genconfig"
)

var _ = genconfig.Config{
    // Override CLI -o for files in this package
    OutPath: "examples/output",

    // Map Go types to field helper types
    FieldTypeMap: map[any]any{
        sql.NullTime{}: field.Time{},
    },

    // Map `gen:"name"` names to helper types
    FieldNameMap: map[string]any{
        "date": field.Time{}, // map fields with `gen:"date"` tag to Time field helper
        "json": JSON{},       // map fields with `gen:"json"` tag to custom JSON helper
    },

    // When true, apply only to current file instead of the whole package
    FileLevel: false,

    // Optional whitelists/blacklists (shell-style patterns):
    // Whitelist takes priority: if Include* is non-empty, only those are generated,
    // and Exclude* is ignored for that kind.
    // Interfaces can be specified by pattern or by type-conversion form, e.g. models.Query(nil)
    IncludeInterfaces: []any{"Query*", models.Query(nil)},
    ExcludeInterfaces: []any{"*Deprecated*"},

    // You can also specify struct types via type literal in the config file,
    // e.g. models.User{} (treated as "models.User"), in addition to patterns.
    IncludeStructs: []any{"User", "Account*", models.User{}},
    ExcludeStructs: []any{"*DTO"},
}
```

### JSON Field Mapping Example

0) Declare Configuration

```go
package examples

import "gorm.io/cli/gorm/genconfig"

var _ = genconfig.Config{
    OutPath: "examples/output",
    FieldNameMap: map[string]any{
        "json": JSON{},        // map fields with `gen:"json"` tag to custom JSON helper
    },
}
```

1) Declare JSON on the model using struct tags

```go
package models

type User struct {
    // ... other fields ...
    // Tell the generator to use the custom JSON helper for this column
    Profile string `gen:"json"`
}
```

2) Define the JSON helper

```go
// JSON is a field helper for JSON columns that generates different SQL for different databases.
type JSON struct{ column clause.Column }

func (j JSON) WithColumn(name string) JSON {
    c := j.column
    c.Name = name
    return JSON{column: c}
}

// Equal builds an expression using database-specific JSON functions to compare
func (j JSON) Equal(path string, value any) clause.Expression {
    return jsonEqualExpr{col: j.column, path: path, val: value}
}

type jsonEqualExpr struct {
    col  clause.Column
    path string
    val  any
}

func (e jsonEqualExpr) Build(builder clause.Builder) {
    if stmt, ok := builder.(*gorm.Statement); ok {
        switch stmt.Dialector.Name() {
        case "mysql":
            v, _ := json.Marshal(e.val)
            clause.Expr{SQL: "JSON_EXTRACT(?, ?) = CAST(? AS JSON)", Vars: []any{e.col, e.path, string(v)}}.Build(builder)
        case "sqlite":
            clause.Expr{SQL: "json_valid(?) AND json_extract(?, ?) = ?", Vars: []any{e.col, e.col, e.path, e.val}}.Build(builder)
        default:
            clause.Expr{SQL: "jsonb_extract_path_text(?, ?) = ?", Vars: []any{e.col, e.path[2:], e.val}}.Build(builder)
        }
    }
}
```

3) Use it in queries

```go
// This will generate different SQL depending on the database:
// MySQL:  "JSON_EXTRACT(`profile`, "$.vip") = CAST("true" AS JSON)"
// SQLite: "json_valid(`profile`) AND json_extract(`profile`, "$.vip") = 1"
got, err := gorm.G[models.User](db).
    Where(generated.User.Profile.Equal("$.vip", true)).Take(ctx)
```
