package jsonaccess

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestBasicAccess(t *testing.T) {
	data := map[string]interface{}{
		"string": "hello",
		"number": 42.0,
		"bool":   true,
		"array":  []interface{}{1.0, 2.0, 3.0},
		"nested": map[string]interface{}{
			"key": "value",
		},
	}

	v := New(data)

	tests := []struct {
		name     string
		path     []interface{}
		want     interface{}
		wantType interface{}
	}{
		{
			name:     "string access",
			path:     []interface{}{"string"},
			want:     "hello",
			wantType: "",
		},
		{
			name:     "number access",
			path:     []interface{}{"number"},
			want:     42,
			wantType: 0,
		},
		{
			name:     "bool access",
			path:     []interface{}{"bool"},
			want:     true,
			wantType: false,
		},
		{
			name:     "array index access",
			path:     []interface{}{"array", 1},
			want:     2,
			wantType: 0,
		},
		{
			name:     "nested access",
			path:     []interface{}{"nested", "key"},
			want:     "value",
			wantType: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := v.Get(tt.path...)
			if result.IsNil() {
				t.Errorf("Get(%v) = nil, want %v", tt.path, tt.want)
				return
			}

			got, err := As[any](result)
			if err != nil {
				t.Errorf("As() error = %v", err)
				return
			}

			// For numbers, convert float64 to int for comparison
			if f, ok := got.(float64); ok {
				got = int(f)
			}

			if got != tt.want {
				t.Errorf("Get(%v) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestTypeConversions(t *testing.T) {
	data := map[string]interface{}{
		"number":       42.0,
		"string":       "123",
		"bool_string":  "true",
		"float_string": "3.14",
	}

	v := New(data)

	tests := []struct {
		name    string
		path    string
		conv    func(*Value) (interface{}, error)
		want    interface{}
		wantErr bool
	}{
		{
			name: "float64 to int",
			path: "number",
			conv: func(v *Value) (interface{}, error) {
				return As[int](v)
			},
			want: 42,
		},
		{
			name: "string to int",
			path: "string",
			conv: func(v *Value) (interface{}, error) {
				return As[int](v)
			},
			want: 123,
		},
		{
			name: "string to bool",
			path: "bool_string",
			conv: func(v *Value) (interface{}, error) {
				return As[bool](v)
			},
			want: true,
		},
		{
			name: "string to float64",
			path: "float_string",
			conv: func(v *Value) (interface{}, error) {
				return As[float64](v)
			},
			want: 3.14,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.conv(v.Get(tt.path))
			if (err != nil) != tt.wantErr {
				t.Errorf("conversion error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("conversion = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPointerResolver(t *testing.T) {
	jsonData := `{
		"definitions": {
			"user": {
				"name": "John",
				"friend": {"$ref": "#/definitions/friend"},
				"address": {"$ref": "#/definitions/address"}
			},
			"friend": {
				"name": "Jane",
				"back": {"$ref": "#/definitions/user"}
			},
			"address": {
				"street": "123 Main St",
				"resident": {"$ref": "#/definitions/user"}
			}
		},
		"current_user": {"$ref": "#/definitions/user"},
		"special~field": {
			"value": "special"
		},
		"slash/field": {
			"value": "slash"
		}
	}`

	var data map[string]interface{}
	if err := json.NewDecoder(strings.NewReader(jsonData)).Decode(&data); err != nil {
		t.Fatalf("Failed to parse test JSON: %v", err)
	}

	root := New(data)
	resolver := NewPointerResolver(root)
	v := root.WithResolver(resolver)

	tests := []struct {
		name    string
		path    []interface{}
		want    string
		wantErr bool
	}{
		{
			name: "direct reference",
			path: []interface{}{"current_user", "name"},
			want: "John",
		},
		{
			name: "nested reference",
			path: []interface{}{"current_user", "friend", "name"},
			want: "Jane",
		},
		{
			name: "circular reference",
			path: []interface{}{"current_user", "friend", "back", "name"},
			want: "John",
		},
		{
			name: "multiple references",
			path: []interface{}{"current_user", "address", "street"},
			want: "123 Main St",
		},
		{
			name: "escaped tilde",
			path: []interface{}{"special~field", "value"},
			want: "special",
		},
		{
			name: "escaped slash",
			path: []interface{}{"slash/field", "value"},
			want: "slash",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := As[string](v.Get(tt.path...))
			if (err != nil) != tt.wantErr {
				t.Errorf("Get(%v) error = %v, wantErr %v", tt.path, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Get(%v) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestInvalidPaths(t *testing.T) {
	data := map[string]interface{}{
		"array": []interface{}{1, 2, 3},
		"map":   map[string]interface{}{"key": "value"},
	}

	v := New(data)

	tests := []struct {
		name string
		path []interface{}
	}{
		{
			name: "array index out of bounds",
			path: []interface{}{"array", 10},
		},
		{
			name: "string key for array",
			path: []interface{}{"array", "key"},
		},
		{
			name: "int key for map",
			path: []interface{}{"map", 0},
		},
		{
			name: "missing key",
			path: []interface{}{"nonexistent"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := v.Get(tt.path...)
			if !result.IsNil() {
				t.Errorf("Get(%v) = %v, want nil", tt.path, result.Data())
			}
		})
	}
}

// Simple resolver that always returns a fixed value
type constResolver struct {
	value interface{}
}

func (r *constResolver) Resolve(ref string, parent *Value) (interface{}, error) {
	return r.value, nil
}

func TestCustomResolver(t *testing.T) {
	impl := func(value interface{}) Resolver {
		return &constResolver{value: value}
	}

	data := map[string]interface{}{
		"ref": map[string]interface{}{
			"$ref": "anything",
		},
	}

	tests := []struct {
		name     string
		resolver Resolver
		want     interface{}
	}{
		{
			name:     "string resolution",
			resolver: impl("resolved"),
			want:     "resolved",
		},
		{
			name:     "number resolution",
			resolver: impl(42.0),
			want:     42,
		},
		{
			name:     "bool resolution",
			resolver: impl(true),
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := New(data).WithResolver(tt.resolver)
			got, err := As[any](v.Get("ref"))
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			// Convert float64 to int for comparison
			if f, ok := got.(float64); ok {
				got = int(f)
			}

			if got != tt.want {
				t.Errorf("resolution = %v, want %v", got, tt.want)
			}
		})
	}
}

func ExampleValue_Get() {
	data := map[string]interface{}{
		"users": []interface{}{
			map[string]interface{}{
				"name": "John",
				"age":  30,
			},
		},
	}

	v := New(data)
	name := MustAs[string](v.Get("users", 0, "name"))
	age := MustAs[int](v.Get("users", 0, "age"))

	fmt.Printf("User: %s, Age: %d", name, age)
	// Output: User: John, Age: 30
}

func TestValue_Keys(t *testing.T) {
	tests := []struct {
		name string
		data interface{}
		want []string
	}{
		{
			name: "empty map",
			data: map[string]interface{}{},
			want: []string{},
		},
		{
			name: "simple map",
			data: map[string]interface{}{
				"c": 1,
				"a": 2,
				"b": 3,
			},
			want: []string{"a", "b", "c"},
		},
		{
			name: "mixed case",
			data: map[string]interface{}{
				"Zebra":  1,
				"apple":  2,
				"Banana": 3,
				"cherry": 4,
			},
			want: []string{"Banana", "Zebra", "apple", "cherry"},
		},
		{
			name: "numbers and special chars",
			data: map[string]interface{}{
				"3":     1,
				"1":     2,
				"2":     3,
				"$spec": 4,
			},
			want: []string{"$spec", "1", "2", "3"},
		},
		{
			name: "non-map",
			data: []interface{}{1, 2, 3},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := New(tt.data)
			got := v.Keys()
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Keys() = %v, want %v", got, tt.want)
			}
		})
	}
}
