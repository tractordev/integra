package integra

import (
	"fmt"
	"log"
	"reflect"
	"regexp"
	"strings"

	"github.com/jinzhu/inflection"
	"github.com/pb33f/libopenapi/datamodel/high/base"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
)

var acronyms = map[string]bool{
	"lfs": true,
	"ips": true,
	"ip":  true,
	"ssh": true,
	"gpg": true,
}

var invariants = map[string]bool{
	"media": true,
}

func isAcronym(s string) bool {
	_, exists := acronyms[strings.ToLower(s)]
	return exists
}

func isInvariant(s string) bool {
	_, exists := invariants[strings.ToLower(s)]
	return exists
}

func toCamelCase(s string) string {
	// Replace dashes and underscores with spaces for easy splitting
	s = strings.ReplaceAll(s, "-", " ")
	s = strings.ReplaceAll(s, "_", " ")

	// Split the string by spaces
	words := strings.Fields(s)

	// Capitalize each word except the first, and join them together
	for i := range words {
		if isAcronym(words[i]) {
			words[i] = strings.ToUpper(words[i])
		} else {
			// Convert to lowercase for the first word
			if i == 0 {
				words[i] = strings.ToLower(words[i])
			} else {
				// Capitalize the first letter for other words
				words[i] = strings.Title(strings.ToLower(words[i]))
			}
		}
	}

	// Join all parts together without spaces
	return strings.Join(words, "")
}

func ToResourceName(path string) string {
	segments := strings.Split(path, "/")
	var nameParts []string

	// Regex to detect version segments like "v1", "v2", etc.
	versionRegex := regexp.MustCompile(`^v\d+`)

	for _, segment := range segments {
		// Skip empty segments, version segments, and placeholders
		if segment == "" || versionRegex.MatchString(segment) || strings.HasPrefix(segment, "{") {
			continue
		}

		if isAcronym(segment) {
			nameParts = append(nameParts, strings.ToUpper(segment))
		} else {
			if isInvariant(segment) {
				nameParts = append(nameParts, segment)
			} else {
				// Singularize the segment using inflection
				singularSegment := inflection.Singular(segment)
				nameParts = append(nameParts, singularSegment)
			}
		}

	}

	return toCamelCase(strings.Join(nameParts, "_"))
}

func getSchemaForPath(pathItem *v3.PathItem) *base.Schema {
	if getOp := pathItem.Get; getOp != nil {
		for statusCode, response := range getOp.Responses.Codes.FromNewest() {
			if statusCode == "200" {
				// Access the response schema for 200 status code
				if content, ok := response.Content.Get("application/json"); ok {
					s := content.Schema.Schema()
					if len(s.Type) > 0 && s.Type[0] == "array" {
						// log.Println("array")
						return s.Items.A.Schema()
					}
					if len(s.Type) == 0 || (len(s.Type) > 0 && s.Type[0] == "object") {
						if s.Properties != nil && s.Properties.Len() == 1 {
							// log.Println("collection")
							return s.Properties.First().Value().Schema()
						}
					}
					// log.Println("schema")
					return s
				}
			}
		}
	}
	return nil
}

func isCollectionPath(path string) bool {
	segments := strings.Split(path, "/")
	lastSegment := segments[len(segments)-1]

	// Check if the last segment is not a parameter (e.g., "{id}")
	return !strings.HasPrefix(lastSegment, "{")
}

func convertYAMLToStringMap(i interface{}) interface{} {
	switch x := i.(type) {
	case map[interface{}]interface{}:
		m2 := map[string]interface{}{}
		for k, v := range x {
			switch kk := k.(type) {
			case string:
				m2[kk] = convertYAMLToStringMap(v)
			case bool:
				m2[fmt.Sprintf("%v", kk)] = convertYAMLToStringMap(v)
			default:
				log.Panicf("unable to convert %#v (%s) to a string key", k, reflect.TypeOf(k))
			}

		}
		return m2
	case []interface{}:
		for i, v := range x {
			x[i] = convertYAMLToStringMap(v)
		}
	}
	return i
}
