package integra

import (
	"fmt"
	"log"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"

	"github.com/jinzhu/inflection"
)

var acronyms = map[string]bool{
	"lfs": true,
	"ips": true,
	"ip":  true,
	"ssh": true,
	"gpg": true,
	"sql": true,
	"ca":  true,
	"db":  true,
	"pdf": true,
	"csv": true,
	"cpu": true,
	"id":  true,
	"2fa": true,
}

var invariants = map[string]bool{
	"media":      true,
	"previous":   true,
	"dangerous":  true,
	"kubernetes": true,
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

// SplitWords splits a string into words based on separators and casing rules.
func SplitWords(input string) []string {
	// Replace non-alphanumeric separators with a single space
	re := regexp.MustCompile(`[\s_\-/]+`)
	cleaned := re.ReplaceAllString(input, " ")

	// Split camel case and preserve acronyms using a custom approach
	var words []string
	currentWord := strings.Builder{}

	for i, r := range cleaned {
		// Append character to the current word
		currentWord.WriteRune(r)

		// Look for word boundaries
		if i < len(cleaned)-1 {
			next := rune(cleaned[i+1])

			// Conditions for splitting:
			if (isLower(r) && isUpper(next)) || // Transition from lower to upper case
				(isUpper(r) && isUpper(next) && (i+2 < len(cleaned) && isLower(rune(cleaned[i+2])))) || // Acronyms
				(isLetter(r) && !isLetter(next)) || // Transition from letter to non-letter
				(isDigit(r) && !isDigit(next)) || // Transition from digit to non-digit
				(!isDigit(r) && isDigit(next)) { // Transition from non-digit to digit

				words = append(words, strings.TrimSpace(currentWord.String()))
				currentWord.Reset()
			}
		}
	}

	// Append the last word if any
	if currentWord.Len() > 0 {
		words = append(words, strings.TrimSpace(currentWord.String()))
	}

	// Filter out empty strings caused by consecutive spaces
	nonEmptyWords := []string{}
	for _, word := range words {
		if word != "" {
			nonEmptyWords = append(nonEmptyWords, word)
		}
	}

	return nonEmptyWords
}

func NameVariants(s string) (variants []string) {
	parts := SplitWords(s)

	// snake case: foo_bar_baz
	var lowerParts []string
	for _, p := range parts {
		lowerParts = append(lowerParts, strings.ToLower(p))
	}
	variants = append(variants, strings.Join(lowerParts, "_"))

	// slug/dash form: foo-bar-baz
	variants = append(variants, strings.Join(lowerParts, "-"))

	// pascal case: FooBarBaz
	var titleParts []string
	for _, p := range parts {
		titleParts = append(titleParts, strings.Title(p))
	}
	variants = append(variants, strings.Join(titleParts, ""))

	// pascal after lower: fooXYZBar => FooXyzBar
	var titleAfterLowerParts []string
	for _, p := range parts {
		titleAfterLowerParts = append(titleAfterLowerParts, strings.Title(strings.ToLower(p)))
	}
	variants = append(variants, strings.Join(titleAfterLowerParts, ""))

	// camel case: fooBarBaz
	var camelParts []string
	for i, p := range titleParts {
		if i == 0 {
			camelParts = append(camelParts, strings.ToLower(p))
		} else {
			camelParts = append(camelParts, p) // already title cased
		}
	}
	variants = append(variants, strings.Join(camelParts, ""))

	// camel after lower: fooXYZBar => fooXyzBar
	var camelAfterLowerParts []string
	for i, p := range parts {
		if i == 0 {
			camelAfterLowerParts = append(camelAfterLowerParts, strings.ToLower(p))
		} else {
			camelAfterLowerParts = append(camelAfterLowerParts, strings.Title(strings.ToLower(p)))
		}
	}
	variants = append(variants, strings.Join(camelAfterLowerParts, ""))

	// pluralize or singularize everything so far
	size := len(variants)
	for i := 0; i < size; i++ {
		s := inflection.Singular(variants[i])
		if variants[i] != s {
			variants = append(variants, s)
		}
		p := inflection.Plural(variants[i])
		if variants[i] != p {
			variants = append(variants, p)
		}
	}

	slices.Sort(variants)

	return slices.Compact(variants)
}

func ToResourceName(path string) string {
	ext := filepath.Ext(path)
	path = strings.TrimSuffix(path, ext)
	path = strings.ReplaceAll(path, ".", "")
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
			case int:
				m2[fmt.Sprintf("%d", kk)] = convertYAMLToStringMap(v)
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

func isLower(r rune) bool {
	return r >= 'a' && r <= 'z'
}

func isUpper(r rune) bool {
	return r >= 'A' && r <= 'Z'
}

func isLetter(r rune) bool {
	return isLower(r) || isUpper(r)
}

func isDigit(r rune) bool {
	return r >= '0' && r <= '9'
}
