package output

import (
	"context"
	"strings"
	time "time"

	models "gorm.io/cmd/gorm/examples/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func Query[T any](db *gorm.DB, opts ...clause.Expression) QueryInterface[T] {
	return QueryImpl[T]{
		Interface: gorm.G[T](db, opts...),
	}
}

type QueryInterface[T any] interface {
	gorm.ChainInterface[T]
	GetByID(ctx context.Context, id int) (T, error)
	FilterWithColumn(ctx context.Context, column string, value string) (T, error)
	QueryWith(ctx context.Context, user models.User) (T, error)
	UpdateInfo(ctx context.Context, user models.User, id int) error
	Filter(ctx context.Context, users []models.User) ([]T, error)
	FilterByNameAndAge(ctx context.Context, name string, age int) QueryInterface[T]
	FilterWithTime(ctx context.Context, start time.Time, end time.Time) ([]T, error)
}

type QueryImpl[T any] struct {
	gorm.Interface[T]
}

func (e QueryImpl[T]) GetByID(ctx context.Context, id int) (T, error) {
	var sb strings.Builder
	params := make([]any, 0, 2)

	sb.WriteString("SELECT * FROM ? WHERE id=? AND name = \\\"@name\\\"")
	params = append(params, clause.CurrentTable, id)

	var result T
	err := e.Raw(sb.String(), params...).Scan(ctx, &result)
	return result, err
}

func (e QueryImpl[T]) FilterWithColumn(ctx context.Context, column string, value string) (T, error) {
	var sb strings.Builder
	params := make([]any, 0, 4)

	sb.WriteString("SELECT * FROM ? WHERE ?=?")
	params = append(params, clause.CurrentTable, gorm.Expr("?", column), value)

	var result T
	err := e.Raw(sb.String(), params...).Scan(ctx, &result)
	return result, err
}

func (e QueryImpl[T]) QueryWith(ctx context.Context, user models.User) (T, error) {
	var sb strings.Builder
	params := make([]any, 0, 2)

	sb.WriteString("SELECT * FROM users")
	if user.ID > 0 {
		sb.WriteString("WHERE id=?")
		params = append(params, user.ID)
	} else if user.Name != "" {
		sb.WriteString("WHERE username=?")
		params = append(params, user.Name)
	}

	var result T
	err := e.Raw(sb.String(), params...).Scan(ctx, &result)
	return result, err
}

func (e QueryImpl[T]) UpdateInfo(ctx context.Context, user models.User, id int) error {
	var sb strings.Builder
	params := make([]any, 0, 4)

	sb.WriteString("UPDATE ?")
	params = append(params, clause.CurrentTable)
	{
		var tmp strings.Builder
		if user.Name != "" {
			tmp.WriteString("username=?,")
			params = append(params, user.Name)
		}
		if user.Age > 0 {
			tmp.WriteString("age=?,")
			params = append(params, user.Age)
		}
		if user.Age >= 18 {
			tmp.WriteString("is_adult=1")
		} else {
			tmp.WriteString("is_adult=0")
		}
		c := strings.TrimSpace(tmp.String())
		if c != "" {
			if strings.HasSuffix(c, ",") {
				c = strings.TrimSpace(strings.TrimRight(c, ","))
			}
			sb.WriteString("SET ")
			sb.WriteString(c)
		}
	}
	sb.WriteString("WHERE id=?")
	params = append(params, id)

	return e.Exec(ctx, sb.String(), params...)
}

func (e QueryImpl[T]) Filter(ctx context.Context, users []models.User) ([]T, error) {
	var sb strings.Builder
	params := make([]any, 0, 13)

	sb.WriteString("SELECT * FROM ?")
	params = append(params, clause.CurrentTable)
	{
		var tmp strings.Builder
		for _, user := range users {
			if user.Name != "" && user.Age > 0 {
				tmp.WriteString("(username = ? AND age=? AND role LIKE concat(\\\"%\\\",?,\\\"%\\\")) OR")
				params = append(params, user.Name, user.Age, user.Role)
			}
		}
		c := strings.TrimSpace(tmp.String())
		if c != "" {
			sb.WriteString("WHERE ")
			if len(c) >= 3 && strings.EqualFold(c[len(c)-3:], "AND") {
				c = strings.TrimSpace(c[:len(c)-3])
			} else if len(c) >= 2 && strings.EqualFold(c[len(c)-2:], "OR") {
				c = strings.TrimSpace(c[:len(c)-2])
			}
			sb.WriteString("WHERE ")
			sb.WriteString(c)
		}
	}

	var result []T
	err := e.Raw(sb.String(), params...).Scan(ctx, &result)
	return result, err
}

func (e QueryImpl[T]) FilterByNameAndAge(ctx context.Context, name string, age int) QueryInterface[T] {
	var sb strings.Builder
	params := make([]any, 0, 2)

	sb.WriteString("name=? AND age=?")
	params = append(params, name, age)

	e.Where(sb.String(), params...)

	return e
}

func (e QueryImpl[T]) FilterWithTime(ctx context.Context, start time.Time, end time.Time) ([]T, error) {
	var sb strings.Builder
	params := make([]any, 0, 3)

	sb.WriteString("SELECT * FROM ?")
	params = append(params, clause.CurrentTable)
	{
		var tmp strings.Builder
		if !start.IsZero() {
			tmp.WriteString("created_time > ?")
			params = append(params, start)
		}
		if !end.IsZero() {
			tmp.WriteString("AND created_time < ?")
			params = append(params, end)
		}
		c := strings.TrimSpace(tmp.String())
		if c != "" {
			sb.WriteString("WHERE ")
			if len(c) >= 3 && strings.EqualFold(c[len(c)-3:], "AND") {
				c = strings.TrimSpace(c[:len(c)-3])
			} else if len(c) >= 2 && strings.EqualFold(c[len(c)-2:], "OR") {
				c = strings.TrimSpace(c[:len(c)-2])
			}
			sb.WriteString("WHERE ")
			sb.WriteString(c)
		}
	}

	var result []T
	err := e.Raw(sb.String(), params...).Scan(ctx, &result)
	return result, err
}
