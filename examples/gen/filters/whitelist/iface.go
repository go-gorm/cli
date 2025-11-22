package whitelist

// I1 has a simple select
type I1[T any] interface {
	// SELECT * FROM @@table WHERE id=@id
	ByID(id int) (T, error)
}

// I2 should be excluded by whitelist
type I2[T any] interface {
	// SELECT * FROM @@table WHERE name=@name
	ByName(name string) (T, error)
}
