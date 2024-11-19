package integra

import (
	"reflect"
	"strings"
	"testing"
)

func TestSplitWords(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"FooBar", []string{"Foo", "Bar"}},
		{"fooBar", []string{"foo", "Bar"}},
		{"FooXYZBar", []string{"Foo", "XYZ", "Bar"}},
		{"foo-bar_baz/123", []string{"foo", "bar", "baz", "123"}},
		{"HTMLParser-Example_test", []string{"HTML", "Parser", "Example", "test"}},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			output := SplitWords(test.input)
			if !reflect.DeepEqual(output, test.expected) {
				t.Errorf("SplitWords(%q) = %v; want %v", test.input, output, test.expected)
			}
		})
	}
}

func TestNameVariants(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"fooBar", strings.Split("FooBar FooBars foo-bar foo-bars fooBar fooBars foo_bar foo_bars", " ")},
		{"FooXYZBar", strings.Split("FooXYZBar FooXYZBars FooXyzBar FooXyzBars foo-xyz-bar foo-xyz-bars fooXYZBar fooXYZBars fooXyzBar fooXyzBars foo_xyz_bar foo_xyz_bars", " ")},
		{"foo-bar_baz/123", strings.Split("FooBarBaz123 foo-bar-baz-123 fooBarBaz123 foo_bar_baz_123", " ")},
		{"HTMLParser-Example_test", strings.Split("HTMLParserExampleTest HTMLParserExampleTests HtmlParserExampleTest HtmlParserExampleTests html-parser-example-test html-parser-example-tests htmlParserExampleTest htmlParserExampleTests html_parser_example_test html_parser_example_tests", " ")},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			output := NameVariants(test.input)
			if !reflect.DeepEqual(output, test.expected) {
				t.Errorf("NameVariants(%q) = %v; want %v", test.input, output, test.expected)
			}
		})
	}
}
