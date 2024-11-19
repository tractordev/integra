package jsonaccess

import (
	"reflect"
	"testing"
)

func TestMergeMaps(t *testing.T) {
	map1 := map[string]any{
		"key1": "value1",
		"key2": map[string]any{
			"subkey1": "subvalue1",
		},
		"key3": "value3",
	}

	map2 := map[string]any{
		"key2": map[string]any{
			"subkey2": "subvalue2",
		},
		"key4": "value4",
	}

	map3 := map[string]any{
		"key1": "newValue1",
		"key2": map[string]any{
			"subkey1": "overwrittenSubValue1",
		},
	}

	expected := map[string]any{
		"key1": "newValue1",
		"key2": map[string]any{
			"subkey1": "overwrittenSubValue1",
			"subkey2": "subvalue2",
		},
		"key3": "value3",
		"key4": "value4",
	}

	result := mergeMaps(map1, map2, map3)

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected %+v, but got %+v", expected, result)
	}
}
