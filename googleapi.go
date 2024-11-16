package integra

import (
	"cmp"
	"fmt"
	"net/url"
	"slices"
	"strings"

	//lint:ignore ST1001 just for API implementations
	. "tractor.dev/integra/internal/jsonaccess"
)

type googleService struct {
	name   string
	meta   *Value
	schema *Value
}

func (s *googleService) Schema() *Value {
	return s.schema
}

func (s *googleService) Meta() *Value {
	return s.meta
}

func (s *googleService) Name() string {
	return s.name
}

func (s *googleService) DataScope() string {
	return cmp.Or(AsOrZero[string](s.meta.Get("dataScope")), "mixed")
}

func (s *googleService) Title() string {
	return AsOrZero[string](s.schema.Get("title"))
}

func (s *googleService) Provider() string {
	return AsOrZero[string](s.schema.Get("ownerDomain"))
}

func (s *googleService) Version() string {
	return AsOrZero[string](s.schema.Get("version"))
}

func (s *googleService) Categories() []string {
	c := AsOrZero[[]string](s.meta.Get("categories"))
	if c == nil {
		return nil
	}
	return c
}

func (s *googleService) BaseURL() string {
	return AsOrZero[string](s.schema.Get("baseUrl"))
}

func (s *googleService) DocsURL() string {
	return AsOrZero[string](s.schema.Get("documentationLink"))
}

func (s *googleService) Security() []string {
	auth := s.schema.Get("auth")
	if auth.IsNil() {
		return nil
	}
	return auth.Keys()
}

func (s *googleService) Resources() (res []Resource) {
	var collectResources func(schema *Value, parent *googleResource)
	collectResources = func(schema *Value, parent *googleResource) {
		resRaw := schema.Get("resources")
		if resRaw.IsNil() {
			return
		}
		for _, name := range resRaw.Keys() {
			r := &googleResource{
				name:    name,
				parent:  parent,
				service: s,
				schema:  resRaw.Get(name),
			}
			res = append(res, r)
			collectResources(resRaw.Get(name), r)
		}
	}
	collectResources(s.schema, nil)
	return
}

func (s *googleService) Resource(name string) (Resource, error) {
	for _, r := range s.Resources() {
		if r.Name() == name {
			return r, nil
		}
	}
	return nil, fmt.Errorf("resource '%s' not found", name)
}

type googleResource struct {
	name    string
	parent  *googleResource
	service *googleService
	schema  *Value
}

func (r *googleResource) Debug() string {
	return ""
}

func (r *googleResource) Service() Service {
	return r.service
}

func (r *googleResource) Parent() Resource {
	return r.parent
}

func (r *googleResource) Name() string {
	if r.parent != nil {
		return ToResourceName(strings.Join([]string{r.parent.Name(), r.name}, "_"))
	}
	return ToResourceName(r.name)
}

func (r *googleResource) Title() string {
	return strings.Title(r.name)
}

func (r *googleResource) DataScope() string {
	if r.service.DataScope() == "account" {
		return "account"
	}
	return ""
}

func (r *googleResource) Description() string {
	schema := r.schema.Get("methods", "get", "response")
	if schema.IsNil() {
		return ""
	}
	return AsOrZero[string](schema.Get("description"))

}

func (r *googleResource) CollectionURL() string {
	method := r.schema.Get("methods", "create")
	if method.IsNil() {
		method = r.schema.Get("methods", "list")
	}
	if method.IsNil() {
		return ""
	}
	path := AsOrZero[string](method.Get("flatPath"))
	if path == "" {
		path = AsOrZero[string](method.Get("path"))
	}
	if path == "" {
		return ""
	}
	u, _ := url.JoinPath(r.service.BaseURL(), path)
	u = strings.ReplaceAll(u, "%7B", "{")
	u = strings.ReplaceAll(u, "%7D", "}")
	return u
}

func (r *googleResource) ItemURL() string {
	method := r.schema.Get("methods", "get")
	if method.IsNil() {
		return ""
	}
	path := AsOrZero[string](method.Get("flatPath"))
	if path == "" {
		path = AsOrZero[string](method.Get("path"))
	}
	if path == "" {
		return ""
	}
	u, _ := url.JoinPath(r.service.BaseURL(), path)
	u = strings.ReplaceAll(u, "%7B", "{")
	u = strings.ReplaceAll(u, "%7D", "}")
	return u
}

func (r *googleResource) Tags() []string {
	return nil
}

func (r *googleResource) Schema() Schema {
	return nil
}

func (r *googleResource) Operations() (ops []Operation) {
	opsRaw := r.schema.Get("methods")
	if opsRaw.IsNil() {
		return nil
	}
	for _, name := range opsRaw.Keys() {
		ops = append(ops, &googleOperation{
			name:     name,
			resource: r,
			schema:   opsRaw.Get(name),
		})
	}
	return
}

func (r *googleResource) Operation(name string) (Operation, error) {
	for _, o := range r.Operations() {
		if o.Name() == name {
			return o, nil
		}
	}
	return nil, fmt.Errorf("operation '%s' not found", name)
}

type googleOperation struct {
	name     string
	resource *googleResource
	schema   *Value
}

func (o *googleOperation) Resource() Resource {
	return o.resource
}

func (o *googleOperation) Name() string {
	return o.name
}

func (o *googleOperation) ID() string {
	return AsOrZero[string](o.schema.Get("id"))
}

func (o *googleOperation) Description() string {
	return AsOrZero[string](o.schema.Get("description"))
}

func (o *googleOperation) URL() string {
	path := AsOrZero[string](o.schema.Get("flatPath"))
	if path == "" {
		path = AsOrZero[string](o.schema.Get("path"))
	}
	if path == "" {
		return ""
	}
	u, _ := url.JoinPath(o.resource.service.BaseURL(), path)
	u = strings.ReplaceAll(u, "%7B", "{")
	u = strings.ReplaceAll(u, "%7D", "}")
	return u
}

func (o *googleOperation) Method() string {
	return AsOrZero[string](o.schema.Get("httpMethod"))
}

func (o *googleOperation) Tags() []string {
	return nil
}

func (o *googleOperation) DocsURL() string {
	return ""
}

func (o *googleOperation) Security() []string {
	return o.resource.service.Security()
}

func (o *googleOperation) Scopes() []string {
	return AsOrZero[[]string](o.schema.Get("scopes"))
}

func (o *googleOperation) Parameters() (params []Schema) {
	globalParams := o.resource.service.schema.Get("parameters")
	paramsRaw := o.schema.Get("parameters")
	if paramsRaw.IsNil() && len(globalParams.Keys()) == 0 {
		return nil
	}
	for _, name := range paramsRaw.Keys() {
		param := &googleParameter{
			name:   name,
			schema: paramsRaw.Get(name),
		}
		if param.ReadOnly() {
			continue
		}
		params = append(params, param)
	}
	for _, name := range globalParams.Keys() {
		param := &googleParameter{
			name:   name,
			schema: globalParams.Get(name),
		}
		if param.ReadOnly() {
			continue
		}
		params = append(params, param)
	}
	return
}

func (o *googleOperation) Response() Schema {
	respRaw := o.schema.Get("response")
	if respRaw.IsNil() {
		return nil
	}
	s := &googleSchema{
		name:   "(response)",
		op:     o,
		schema: respRaw,
	}
	if o.Output().Type() == s.Type() {
		// unless output is an array and
		// this isn't, we don't need resp
		return nil
	}
	return s
}

func (o *googleOperation) Input() Schema {
	reqRaw := o.schema.Get("request")
	if reqRaw.IsNil() {
		return nil
	}
	return &googleSchema{
		name:      "(input)",
		op:        o,
		writeOnly: true,
		schema:    reqRaw,
	}
}

func (o *googleOperation) Output() Schema {
	outRaw := o.schema.Get("response")
	if outRaw.IsNil() {
		return nil
	}
	s := &googleSchema{
		name:   "(output)",
		op:     o,
		schema: outRaw,
	}
	if o.name == "list" {
		// detect array by resource name, "items", then first array prop
		if res := outRaw.Get(o.resource.name); !res.IsNil() {
			s.schema = res
			if s.Type() == "array" {
				return s
			}
		}
		if items := outRaw.Get("items"); !items.IsNil() {
			s.schema = items
			if s.Type() == "array" {
				return s
			}
		}
		for _, prop := range s.Properties() {
			if prop.Type() == "array" {
				s.schema = prop.(*googleSchema).schema
				return s
			}
		}
		// set schema back just in case
		s.schema = outRaw
	}
	return s
}

type googleParameter struct {
	name   string
	schema *Value

	emptySchema
}

func (p *googleParameter) Name() string {
	return p.name
}

func (p *googleParameter) In() string {
	return AsOrZero[string](p.schema.Get("location"))
}

func (p *googleParameter) Description() string {
	return AsOrZero[string](p.schema.Get("description"))
}

func (p *googleParameter) Type() string {
	return AsOrZero[string](p.schema.Get("type"))
}

func (p *googleParameter) Enum() []string {
	return AsOrZero[[]string](p.schema.Get("enum"))
}

func (p *googleParameter) EnumDesc() []string {
	return AsOrZero[[]string](p.schema.Get("enumDescriptions"))
}

func (p *googleParameter) ReadOnly() bool {
	return AsOrZero[bool](p.schema.Get("readOnly"))
}

func (p *googleParameter) Required() bool {
	return AsOrZero[bool](p.schema.Get("required"))
}

func (p *googleParameter) Format() string {
	return AsOrZero[string](p.schema.Get("format"))
}

func (p *googleParameter) Default() string {
	return AsOrZero[string](p.schema.Get("default"))
}

func (p *googleParameter) Nullable() bool {
	return AsOrZero[bool](p.schema.Get("nullable"))
}

func (p *googleParameter) Example() string {
	return AsOrZero[string](p.schema.Get("example"))
}

type googleSchema struct {
	name      string
	schema    *Value
	op        *googleOperation
	writeOnly bool

	emptySchema
}

func (s *googleSchema) Name() string {
	return s.name
}

func (s *googleSchema) Title() string {
	return AsOrZero[string](s.schema.Get("title"))
}

func (s *googleSchema) Description() string {
	return AsOrZero[string](s.schema.Get("description"))
}

func (s *googleSchema) Type() string {
	return AsOrZero[string](s.schema.Get("type"))
}

func (s *googleSchema) Enum() []string {
	return AsOrZero[[]string](s.schema.Get("enum"))
}

func (s *googleSchema) EnumDesc() []string {
	return AsOrZero[[]string](s.schema.Get("enumDescriptions"))
}

func (s *googleSchema) ReadOnly() bool {
	return AsOrZero[bool](s.schema.Get("readOnly"))
}

func (s *googleSchema) Format() string {
	return AsOrZero[string](s.schema.Get("format"))
}

func (s *googleSchema) Default() string {
	return AsOrZero[string](s.schema.Get("default"))
}

func (s *googleSchema) Nullable() bool {
	return AsOrZero[bool](s.schema.Get("nullable"))
}

func (s *googleSchema) Example() string {
	return AsOrZero[string](s.schema.Get("example"))
}

func (s *googleSchema) Minimum() *int {
	min := s.schema.Get("minimum")
	if min.IsNil() {
		return nil
	}
	v := AsOrZero[int](min)
	return &v
}

func (s *googleSchema) MinLength() *int {
	min := s.schema.Get("minLength")
	if min.IsNil() {
		return nil
	}
	v := AsOrZero[int](min)
	return &v
}

func (s *googleSchema) MaxLength() *int {
	max := s.schema.Get("maxLength")
	if max.IsNil() {
		return nil
	}
	v := AsOrZero[int](max)
	return &v
}

func (s *googleSchema) Required() bool {
	local := s.schema.Get("required")
	if !local.IsNil() {
		return MustAs[bool](local)
	}
	annot := s.schema.Get("annotations", "required")
	if annot.IsNil() || s.op == nil {
		return false
	}
	if slices.Contains(AsOrZero[[]string](annot), s.op.ID()) {
		return true
	}
	return false
}

func (s *googleSchema) Properties() (props []Schema) {
	propsRaw := s.schema.Get("properties")
	if propsRaw.IsNil() {
		return nil
	}
	for _, name := range propsRaw.Keys() {
		prop := &googleSchema{
			name:   name,
			op:     s.op,
			schema: propsRaw.Get(name),
		}
		if s.writeOnly && prop.ReadOnly() {
			continue
		}
		props = append(props, prop)
	}
	return
}

func (s *googleSchema) Property(name string) (Schema, error) {
	for _, p := range s.Properties() {
		if p.Name() == name {
			return p, nil
		}
	}
	return nil, fmt.Errorf("property '%s' not found", name)
}

func (s *googleSchema) Items() Schema {
	items := s.schema.Get("items")
	if items.IsNil() {
		return nil
	}
	return &googleSchema{
		name:      "(item)",
		op:        s.op,
		writeOnly: s.writeOnly,
		schema:    items,
	}
}
