package pattern

// QueryUser should match pattern "Query*"
type QueryUser[T any] interface {
	// SELECT * FROM @@table WHERE id=@id
	ByID(id int) (T, error)
}

// QueryOrder should match pattern "Query*"
type QueryOrder[T any] interface {
	// SELECT * FROM @@table WHERE number=@no
	ByNumber(no string) (T, error)
}

// Service should NOT match "Query*"
type Service[T any] interface {
	// SELECT * FROM @@table LIMIT 1
	One() (T, error)
}
