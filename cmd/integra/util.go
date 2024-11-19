package main

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"tractor.dev/integra"
)

func truncateText(s string) string {
	if len(s) > 50 {
		s = s[:50] + "..."
	}
	return s
}

func shortText(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	return truncateText(s)
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

func open(s string) error {
	// https://github.com/skratchdot/open-golang/blob/master/open
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", s).Run()
	default:
		return fmt.Errorf("todo: %s", runtime.GOOS)
	}
}
