package blacklist

// I1 should be included (not excluded)
type I1[T any] interface {
	// SELECT * FROM @@table WHERE id=@id
	ByID(id int) (T, error)
}

// I2 should be excluded by blacklist
type I2[T any] interface {
	// SELECT * FROM @@table WHERE name=@name
	ByName(name string) (T, error)
}
