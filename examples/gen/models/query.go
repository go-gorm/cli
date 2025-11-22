package models

type Query[T any] interface {
	// GetByID query data by id and return it as struct
	//
	// SELECT * FROM @@table WHERE id=@id
	GetByID(id int) (T, error)

	// SELECT * FROM @@table WHERE @@column=@value
	FilterWithColumn(column string, value string) (T, error)
}
