package jsonaccess

import (
	"fmt"
	"strconv"
	"strings"
)

// PointerResolver implements JSON Pointer resolution from a root document
type PointerResolver struct {
	root *Value
}

// NewPointerResolver creates a new resolver that resolves references against a root document
func NewPointerResolver(root *Value) *PointerResolver {
	return &PointerResolver{root: root}
}

// Resolve implements the Resolver interface for JSON Pointer syntax
func (r *PointerResolver) Resolve(ref string, parent *Value) (interface{}, error) {
	if !strings.HasPrefix(ref, "#") {
		return nil, fmt.Errorf("only document-relative references starting with # are supported, got %q", ref)
	}

	// Handle root reference
	if ref == "#" {
		return r.root.Data(), nil
	}

	// Split path into components and remove empty strings
	parts := strings.Split(strings.TrimPrefix(ref, "#/"), "/")
	result := r.root

	for _, part := range parts {
		// Unescape JSON Pointer special sequences
		part = strings.ReplaceAll(part, "~1", "/")
		part = strings.ReplaceAll(part, "~0", "~")
		part = strings.ReplaceAll(part, "%7B", "{")
		part = strings.ReplaceAll(part, "%7D", "}")

		// Try to parse as array index first
		if idx, err := strconv.Atoi(part); err == nil {
			idxResult := result.Get(idx)
			if !idxResult.IsNil() {
				result = idxResult
			} else {
				// fallback to string lookup
				result = result.Get(part)
			}
		} else {
			result = result.Get(part)
		}

		if result.IsNil() {
			return nil, fmt.Errorf("path %q not found at segment %q", ref, part)
		}
	}

	return result.Data(), nil
}
