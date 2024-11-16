/*
Package jsonaccess provides a flexible and type-safe way to access nested JSON data structures
with support for JSON References ($ref). It allows for traversing complex JSON structures
using both map keys and array indices while maintaining type safety through generics.

The package centers around the Value type, which wraps arbitrary JSON data and provides
safe access methods. It supports reference resolution through a pluggable Resolver interface,
with a built-in PointerResolver that implements JSON Pointer (RFC 6901) resolution.

Basic Usage:

	// Parse JSON data
	var data map[string]interface{}
	json.Unmarshal(someJSON, &data)

	// Create a value
	v := jsonaccess.New(data)

	// Access nested values with type safety
	name, err := jsonaccess.As[string](v.Get("user", "name"))
	age, err := jsonaccess.As[int](v.Get("user", "age"))
	firstCity, err := jsonaccess.As[string](v.Get("addresses", 0, "city"))

Reference Resolution:

The package includes built-in support for JSON References using the $ref key. References
can be resolved using the PointerResolver, which implements JSON Pointer syntax:

	// Setup reference resolution
	root := jsonaccess.New(data)
	resolver := jsonaccess.NewPointerResolver(root)
	v := root.WithResolver(resolver)

	// Data with references
	{
	    "definitions": {
	        "user": {
	            "name": "John",
	            "friend": { "$ref": "#/definitions/friend" }
	        },
	        "friend": {
	            "name": "Jane"
	        }
	    },
	    "current_user": { "$ref": "#/definitions/user" }
	}

	// Access through references
	userName := jsonaccess.MustAs[string](v.Get("current_user", "name"))
	friendName := jsonaccess.MustAs[string](v.Get("current_user", "friend", "name"))

Key Features:

  - Type-safe access to JSON data using generics
  - Support for both map key and array index access
  - Pluggable reference resolution system
  - Built-in JSON Pointer (RFC 6901) resolver
  - Circular reference support
  - Nil-safe operations
  - Common type conversions
  - Clear error messages

The package is particularly useful when working with complex JSON schemas, OpenAPI
specifications, or any JSON data that makes use of references and requires type-safe
access patterns.
*/
package jsonaccess

import (
	"fmt"
	"log"
	"sort"
	"strconv"
)

// Resolver interface for resolving references
type Resolver interface {
	Resolve(ref string, parent *Value) (interface{}, error)
}

// Value represents a JSON value that can be accessed by key or index
type Value struct {
	data       interface{}
	resolver   Resolver
	mergeAllOf bool
}

// New creates a new Value from any data
func New(data interface{}) *Value {
	return &Value{data: data}
}

// WithResolver returns a new Value with the given resolver
func (v *Value) WithResolver(resolver Resolver) *Value {
	return &Value{
		data:     v.data,
		resolver: resolver,
	}
}

// WithAllOfMerge returns a new Value that merges down allOf directives
func (v *Value) WithAllOfMerge() *Value {
	return &Value{
		data:       v.data,
		resolver:   v.resolver,
		mergeAllOf: true,
	}
}

// resolve attempts to resolve references and merge down allOf directives
func (v *Value) resolve() (interface{}, error) {
	resolved, err := v.resolveRef()
	if err != nil {
		return nil, err
	}
	tmpValue := &Value{data: resolved, resolver: v.resolver}
	return tmpValue.resolveAllOf()
}

// hasRef checks if the current value is a reference object
func (v *Value) hasRef() (string, bool) {
	if m, ok := v.data.(map[string]interface{}); ok {
		if ref, ok := m["$ref"].(string); ok {
			return ref, true
		}
	}
	return "", false
}

// resolveRef attempts to resolve a reference using the configured resolver
func (v *Value) resolveRef() (interface{}, error) {
	if ref, isRef := v.hasRef(); isRef {
		if v.resolver == nil {
			return nil, fmt.Errorf("reference %q found but no resolver configured", ref)
		}
		resolved, err := v.resolver.Resolve(ref, v)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve reference %q: %w", ref, err)
		}
		// merge with props on the $ref value if any
		resolvedMap, isMap := resolved.(map[string]interface{})
		if m, ok := v.data.(map[string]interface{}); ok && isMap {
			for key, val := range m {
				if key == "$ref" {
					continue
				}
				resolvedMap[key] = val
			}
			return resolvedMap, nil
		}
		return resolved, nil
	}
	return v.data, nil
}

// hasAllOf checks if the current value has an allOf directive
func (v *Value) hasAllOf() ([]any, bool) {
	if m, ok := v.data.(map[string]interface{}); ok {
		if allOf, ok := m["allOf"].([]any); ok {
			return allOf, true
		}
	}
	return nil, false
}

// resolveAllOf attempts to merge down any objects under an allOf directive
func (v *Value) resolveAllOf() (interface{}, error) {
	if vals, hasAllOf := v.hasAllOf(); hasAllOf && v.mergeAllOf {
		resolved := make(map[string]any)
		for _, val := range vals {
			if m, ok := val.(map[string]any); ok {
				for k, v := range m {
					resolved[k] = v
				}
			}
		}
		if m, ok := v.data.(map[string]any); ok {
			for k, v := range m {
				resolved[k] = v
			}
		}
		return resolved, nil
	}
	return v.data, nil
}

// Keys returns all keys of a map value in alphanumeric order.
// Returns nil if the value is not a map.
func (v *Value) Keys() (keys []string) {
	m, ok := v.data.(map[string]any)
	if !ok {
		return nil
	}
	keys = make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return
}

// Items returns a Value for each element in a slice value.
// Returns nil if the value is not a slice.
func (v *Value) Items() (items []*Value) {
	s, ok := v.data.([]any)
	if !ok {
		return nil
	}
	for i := 0; i < len(s); i++ {
		items = append(items, v.Get(i))
	}
	return
}

// Get returns a new Value for the given key path
// Keys can be string map keys or integer array indices
func (v *Value) Get(keys ...interface{}) *Value {
	current := v.data

	// Try to resolve any reference at the start
	if resolved, err := v.resolve(); err == nil {
		current = resolved
	} else {
		log.Println(err)
	}

	for _, key := range keys {
		switch k := key.(type) {
		case string:
			if m, ok := current.(map[string]interface{}); ok {
				current = m[k]
			} else {
				return &Value{data: nil, resolver: v.resolver}
			}
		case int:
			if arr, ok := current.([]interface{}); ok {
				if k < 0 || k >= len(arr) {
					return &Value{data: nil, resolver: v.resolver}
				}
				current = arr[k]
			} else {
				return &Value{data: nil, resolver: v.resolver}
			}
		default:
			return &Value{data: nil, resolver: v.resolver}
		}

		// Try to resolve reference at each step
		newValue := &Value{data: current, resolver: v.resolver}
		if resolved, err := newValue.resolve(); err == nil {
			current = resolved
		} else {
			log.Println(err)
		}
	}

	return &Value{data: current, resolver: v.resolver}
}

// Data returns the raw underlying data
func (v *Value) Data() interface{} {
	return v.data
}

// As attempts to convert the value to type T
func As[T any](v *Value) (T, error) {
	var zero T

	// Try to resolve any reference before type conversion
	resolved, err := v.resolve()
	if err != nil {
		return zero, err
	}

	if resolved == nil {
		return zero, fmt.Errorf("value is nil")
	}

	// Try direct type assertion first
	if typed, ok := resolved.(T); ok {
		return typed, nil
	}

	// Handle common type conversions
	switch any(zero).(type) {
	case []string:
		switch val := resolved.(type) {
		case []any:
			var strs []string
			for _, v := range val {
				// todo: check to make sure v is string
				strs = append(strs, v.(string))
			}
			return any(strs).(T), nil
		}
	case string:
		str := fmt.Sprintf("%v", resolved)
		return any(str).(T), nil

	case int:
		switch val := resolved.(type) {
		case float64:
			return any(int(val)).(T), nil
		case string:
			if i, err := strconv.Atoi(val); err == nil {
				return any(i).(T), nil
			}
		}

	case float64:
		switch val := resolved.(type) {
		case int:
			return any(float64(val)).(T), nil
		case string:
			if f, err := strconv.ParseFloat(val, 64); err == nil {
				return any(f).(T), nil
			}
		}

	case bool:
		switch val := resolved.(type) {
		case string:
			if b, err := strconv.ParseBool(val); err == nil {
				return any(b).(T), nil
			}
		}
	}

	return zero, fmt.Errorf("cannot convert %T to %T", resolved, zero)
}

// IsNil checks if the value is nil
func (v *Value) IsNil() bool {
	resolved, err := v.resolve()
	if err != nil {
		return true
	}
	return resolved == nil
}

// MustAs is like As but panics on error
func MustAs[T any](v *Value) T {
	val, err := As[T](v)
	if err != nil {
		panic(err)
	}
	return val
}

func AsOrZero[T any](v *Value) T {
	var zero T
	val, err := As[T](v)
	if err != nil {
		return zero
	}
	return val
}
