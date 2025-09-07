package examples

import (
	"database/sql"
	"time"

	"gorm.io/cmd/gorm/examples/models"
	"gorm.io/cmd/gorm/field"
	"gorm.io/cmd/gorm/genconfig"
)

var _ = genconfig.Config{
	OutPath: "examples/output",
	Package: true,
	FieldTypeMap: map[any]any{
		sql.NullTime{}: field.Time{},
	},
	FieldNameMap: map[string]any{
		"date": field.Time{},
		"json": JSON{},
	},
}

type Query[T any] interface {
	// GetByID query data by id and return it as struct
	//
	// SELECT * FROM @@table WHERE id=@id AND name = "\@name"
	GetByID(id int) (T, error)

	// SELECT * FROM @@table WHERE @@column=@value
	FilterWithColumn(column string, value string) (T, error)

	// SELECT * FROM users
	//   {{if user.ID > 0}}
	//       WHERE id=@user.ID
	//   {{else if user.Name != ""}}
	//       WHERE name=@user.Name
	//   {{end}}
	QueryWith(user models.User) (T, error)

	// UPDATE @@table
	//  {{set}}
	//    {{if user.Name != ""}} name=@user.Name, {{end}}
	//    {{if user.Age > 0}} age=@user.Age, {{end}}
	//    {{if user.Age >= 18}} is_adult=1 {{else}} is_adult=0 {{end}}
	//  {{end}}
	// WHERE id=@id
	UpdateInfo(user models.User, id int) error

	// SELECT * FROM @@table
	// {{where}}
	//   {{for _, user := range users}}
	//     {{if user.Name != "" && user.Age > 0}}
	//       (name = @user.Name AND age=@user.Age AND role LIKE concat("%",@user.Role,"%")) OR
	//     {{end}}
	//   {{end}}
	// {{end}}
	Filter(users []models.User) ([]T, error)

	// where("name=@name AND age=@age")
	FilterByNameAndAge(name string, age int)

	// SELECT * FROM @@table
	//  {{where}}
	//    {{if !start.IsZero()}}
	//      created_at > @start
	//    {{end}}
	//    {{if !end.IsZero()}}
	//      AND created_at < @end
	//    {{end}}
	//  {{end}}
	FilterWithTime(start, end time.Time) ([]T, error)
}
