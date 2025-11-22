package twolevel

type I1[T any] interface {
	// SELECT * FROM @@table WHERE id=@id
	ByID(id int) (T, error)
}

type I2[T any] interface {
	// SELECT * FROM @@table WHERE id=@id
	ByID2(id int) (T, error)
}

type I3[T any] interface {
	// SELECT * FROM @@table WHERE id=@id
	ByID3(id int) (T, error)
}
