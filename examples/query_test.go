package examples

import (
	"testing"
	"time"

	"gorm.io/cli/gorm/examples/models"
)

// TestQueryInterface 检查 Query 接口是否正确定义
func TestQueryInterface(t *testing.T) {
	// 这是一个编译时测试，确保接口定义没有语法错误
	var _ Query[any] = (*testQueryImpl[any])(nil)
}

// testQueryImpl 是 Query 接口的一个空实现，用于测试
type testQueryImpl[T any] struct{}

func (t *testQueryImpl[T]) GetByID(id int) (T, error) {
	var result T
	return result, nil
}

func (t *testQueryImpl[T]) FilterWithColumn(column string, value string) (T, error) {
	var result T
	return result, nil
}

func (t *testQueryImpl[T]) QueryWith(user models.User) (T, error) {
	var result T
	return result, nil
}

func (t *testQueryImpl[T]) UpdateInfo(user models.User, id int) error {
	return nil
}

func (t *testQueryImpl[T]) Filter(users []models.User) ([]T, error) {
	var result []T
	return result, nil
}

func (t *testQueryImpl[T]) FilterByNameAndAge(params Params) {
}

func (t *testQueryImpl[T]) FilterWithTime(start, end time.Time) ([]T, error) {
	var result []T
	return result, nil
}

// TestParamsStruct 检查 Params 结构体是否正确定义
func TestParamsStruct(t *testing.T) {
	params := Params{
		Name: "test",
		Age:  25,
	}

	if params.Name != "test" {
		t.Errorf("Expected Name to be 'test', got %s", params.Name)
	}

	if params.Age != 25 {
		t.Errorf("Expected Age to be 25, got %d", params.Age)
	}
}

// TestJSONType 检查 JSON 类型是否正确定义
func TestJSONType(t *testing.T) {
	// 这是一个编译时测试，确保 JSON 类型定义没有语法错误
	jsonField := JSON{}
	
	// 确保可以调用 WithColumn 方法
	jsonWithColumn := jsonField.WithColumn("test_column")
	
	// 确保返回的是正确的类型
	_ = jsonWithColumn
}