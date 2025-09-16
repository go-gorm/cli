# GORM CLI

GORM CLI generates two complementary layers of code for your GORM projects:

* **Type-safe, interface-driven query APIs** — from Go interfaces with powerful SQL templates
* **Model-based field helpers** — from your model structs for filters, updates, ordering, and associations

Together they deliver **compile-time safety** and a fluent, discoverable API for all database operations

## Key Features

* **Strong Type Safety (default)** — Compile-time guarantees with Go generics.
* **Interface-Driven Query Generation** — Define Go interfaces with SQL template comments to produce concrete, type-safe methods.
* **Model-Based Field Helpers** — Generate helpers from model structs for filtering, ordering, updates, and association handling.
* **Seamless GORM Integration** — Works natively with `gorm.io/gorm`—no runtime magic, just plain Go.
* **Flexible Configuration** — Customize output paths, include/exclude rules, and field mappings via `genconfig.Config`.
* **Rich Association Operations** — Strongly-typed `Create`, `CreateInBatch`, `Update`, `Unlink`, and `Delete` for associations.

## 🚀 Quick Start

### 1) Install

```bash
go install gorm.io/cli/gorm@latest
```

### 2) Define Models & a Query Interface

```go
// examples/models/user.go
type User struct {
  gorm.Model
  Name string
  Age  int
  Pets []Pet `gorm:"many2many:user_pets"`
}

type Pet struct {
  gorm.Model
  Name string
}

// examples/query.go
type Query[T any] interface {
  // SELECT * FROM @@table WHERE id=@id
  GetByID(id int) (T, error)
}
```

### 3) Generate & Use

```bash
# Default: strictly typed generics API
gorm gen -i ./examples -o ./generated

# Standard API (still generics-based, relaxed typing for more flexibility)
gorm gen -i ./examples -o ./generated --typed=false
```

```go
// Type-safe query
// SELECT * FROM users WHERE id=123
u, err := generated.Query[User](db).GetByID(ctx, 123)

// Field helpers
// SELECT * FROM users WHERE age>18
users, _ := gorm.G[User](db).Where(generated.User.Age.Gt(18)).Find(ctx)

// Association helpers: create a user with a pet
gorm.G[User](db).
  Set(
    generated.User.Name.Set("alice"),
    generated.User.Pets.Create(generated.Pet.Name.Set("fido")),
  ).
  Create(ctx)
```

---

## Two Generators, One Workflow

GORM CLI uses **two generators that work together** for full developer ergonomics:

### 1) Query API Generator

Define methods with **SQL template comments** in Go interfaces to generate concrete, type-safe methods.

### 2) Field Helper Generator

Generate strongly-typed helpers from your models to build filters, updates, ordering, and associations without raw SQL.

Supported types & associations (field helpers):
* **Basics**: integers, floats, `string`, `bool`, `time.Time`, `[]byte`
* **Named/custom types** that implement `database/sql.Scanner` / `driver.Valuer` or GORM `Serializer`
* **Associations**: `has one` \*\*(including polymorphic)`, `has many` **(including polymorphic)`, `belongs to`, `many2many`

---

## Working with Fields

Common predicates & setters:

```go
// Predicates
generated.User.ID.Eq(1)               // id = 1
generated.User.Name.Like("%jinzhu%")  // name LIKE '%jinzhu%'
generated.User.Age.Between(18, 65)    // age BETWEEN 18 AND 65
generated.User.Score.IsNull()         // score IS NULL (e.g., sql.NullInt64)

// Updates (supports expressions and zero-values)
gorm.G[User](db).
  Where(generated.User.Name.Eq("alice")).
  Set(
    generated.User.Name.Set("jinzhu"),
    generated.User.IsAdult.Set(false),
    generated.User.Score.Set(sql.NullInt64{}),
    generated.User.Count.Incr(1),
    generated.User.Age.SetExpr(clause.Expr{
      SQL:  "GREATEST(?, ?)",
      Vars: []any{clause.Column{Name: "age"}, 18},
    }),
  ).
  Update(ctx)

// Create with Set(...)
gorm.G[User](db).
  Set(
    generated.User.Name.Set("alice"),
    generated.User.Age.Set(0),
    generated.User.Status.Set("active"),
  ).
  Create(ctx)
```

> **Standard API note (`--typed=false`)**
> The default output is strictly typed. With the Standard API, you keep generics but gain flexibility to mix raw conditions with helpers:
>
> ```go
> generated.Query[User](db).
>   Where("name = ?", "jinzhu").               // raw condition
>   Where(generated.User.Age.Gt(18)).          // typed helper
>   Find(ctx)
> ```

---

## Working with Associations

Association helpers appear on generated models as `field.Struct[T]` or `field.Slice[T]` (e.g. `generated.User.Pets`, `generated.User.Account`).

Supported operations (composed into `Set(...).Update(ctx)` or `Set(...).Create(ctx)`):
* **Create** — create & link a related row per parent
* **CreateInBatch** — batch create/link from a slice
* **Update** — update related rows (with optional conditions)
* **Unlink** — remove only the relationship (clear FK or delete join rows)
* **Delete** — delete related rows (m2m: deletes join rows only)

```go
// Create a pet for each matched user
gorm.G[User](db).
  Where(generated.User.ID.Eq(1)).
  Set(generated.User.Pets.Create(generated.Pet.Name.Set("fido"))).
  Update(ctx)

// Filter on the child before acting
gorm.G[User](db).
  Where(generated.User.ID.Eq(1)).
  Set(generated.User.Pets.Where(generated.Pet.Name.Eq("old")).Delete()).
  Update(ctx)

// Batch link two pets to an existing user
gorm.G[User](db).
  Where(generated.User.ID.Eq(1)).
  Set(generated.User.Pets.CreateInBatch([]models.Pet{{Name: "rex"}, {Name: "spot"}})).
  Update(ctx)
```

Semantics by association type:

* **belongs to**: `Unlink` clears the parent FK; `Delete` removes associated rows
* **has one / has many** *(including polymorphic)*: `Unlink` clears the child FK; `Delete` removes child rows
* **many2many**: `Unlink`/`Delete` remove join rows only (both sides remain)

Parent operation semantics:

* `Create(ctx)` inserts new parent rows using your `Set(...)` values, then applies association ops
* `Update(ctx)` updates matched parent rows, then applies association ops

---

## Template-Based Queries

Write SQL and light templating in interface comments; parameters bind automatically; implementations are generated and type-safe.

```go
type Query[T any] interface {
  // SELECT * FROM @@table WHERE id=@id
  GetByID(id int) (T, error)

  // SELECT * FROM @@table WHERE @@column=@value
  FilterWithColumn(column string, value string) (T, error)

  // SELECT * FROM @@table
  // {{where}}
  //   {{if user.Name }} name=@user.Name {{end}}
  //   {{if user.Age > 0}} AND age=@user.Age {{end}}
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

> **Context auto-injection**
> If a method doesn’t include `ctx context.Context`, the generated implementation adds it.

Example usage

```go
// SQL: SELECT * FROM users WHERE id=123
user, err := generated.Query[User](db).GetByID(ctx, 123)

// SQL: SELECT * FROM users WHERE name="jinzhu" AND age=25 (appended to current builder)
users, err := generated.Query[User](db).FilterByNameAndAge("jinzhu", 25).Find(ctx)

// SQL UPDATE users SET name="jinzhu", age=20, is_adult=1 WHERE id=1
err := generated.Query[User](db).UpdateUser(ctx, User{Name: "jinzhu", Age: 20}, 1)
```

### Template DSL

| Directive   | Purpose                          | Example                                  |
| ----------- | -------------------------------- | ---------------------------------------- |
| `@@table`   | Model table name                 | `SELECT * FROM @@table WHERE id=@id`     |
| `@@column`  | Dynamic column binding           | `@@column=@value`                        |
| `@param`    | Bind Go params to SQL params     | `WHERE name=@user.Name`                  |
| `{{where}}` | Conditional WHERE wrapper        | `{{where}} age > 18 {{end}}`             |
| `{{set}}`   | Conditional SET wrapper (UPDATE) | `{{set}} name=@name {{end}}`             |
| `{{if}}`    | Conditional SQL fragment         | `{{if age > 0}} AND age=@age {{end}}`    |
| `{{for}}`   | Iterate over a collection        | `{{for _, t := range tags}} ... {{end}}` |

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

---

## ⚙️ Generation Config (optional)

You can generate without any config. For overrides, declare a package-level `genconfig.Config` in the package you generate:

```go
package examples

import (
  "database/sql"
  "gorm.io/cli/gorm/field"
  "gorm.io/cli/gorm/genconfig"
)

var _ = genconfig.Config{
  OutPath: "examples/output",

  // Map Go types to helper kinds
  FieldTypeMap: map[any]any{
    sql.NullTime{}: field.Time{},
  },

  // Map `gen:"name"` tags to helper kinds
  FieldNameMap: map[string]any{
    "json": JSON{}, // use a custom JSON helper where fields are tagged `gen:"json"`
  },

  // Narrow what gets generated (patterns or type literals)
  IncludeInterfaces: []any{"Query*", models.Query(nil)},
  IncludeStructs:    []any{"User", "Account*", models.User{}},
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
