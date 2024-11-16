package main

import (
	"fmt"
	"strings"

	"tractor.dev/integra"
)

func shortText(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if !strings.Contains(s, ".") {
		return s
	}
	sentences := strings.Split(s, ".")
	if len(sentences) == 2 && strings.TrimSpace(sentences[1]) == "" {
		return sentences[0]
	}
	return fmt.Sprintf("%s [...]", sentences[0])
}

func schemaFeatures(schema integra.Schema) (features []string) {
	// TODO: more features
	if schema.Required() {
		features = append(features, "required")
	}
	if schema.Format() != "" {
		features = append(features, fmt.Sprintf("format: %s", schema.Format()))
	}
	// if ss.MinLength != nil {
	// 	features = append(features, fmt.Sprintf("min-length: %d", *ss.MinLength))
	// }
	// if ss.MaxLength != nil {
	// 	features = append(features, fmt.Sprintf("max-length: %d", *ss.MaxLength))
	// }
	if schema.ReadOnly() {
		features = append(features, "read-only")
	}
	return
}
