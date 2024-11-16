package jsonaccess

import "fmt"

type DefinitionsResolver struct {
	defs *Value
}

// NewDefinitionsResolver creates a new resolver that resolves references against a flat definitions object
func NewDefinitionsResolver(defs *Value) *DefinitionsResolver {
	return &DefinitionsResolver{defs: defs}
}

func (r *DefinitionsResolver) Resolve(ref string, parent *Value) (interface{}, error) {
	res := r.defs.Get(ref)
	if res.IsNil() {
		return nil, fmt.Errorf("definition for '%s' not found", ref)
	}
	return res.Data(), nil
}
