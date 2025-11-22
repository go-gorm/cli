package models

type UserQuery interface {
	// SELECT * FROM @@table
	// {{where}}
	//   {{if user.ID > 0}} id=@user.ID  {{end}}
	//   {{if user.Name != ""}} name=@user.Name {{end}}
	// {{end}}
	QueryWith(user User) (User, error)

	// UPDATE @@table
	//{{set}}
	//  {{if user.Name != ""}} name=@user.Name, {{end}}
	//  {{if user.Age > 0}} age=@user.Age {{end}}
	//{{end}}
	//WHERE id=@user.ID
	UpdateWith(user User) error
}
