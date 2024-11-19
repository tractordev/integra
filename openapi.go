package integra

import (
	"cmp"
	"fmt"
	"log"
	"net/url"
	"regexp"
	"slices"
	"strings"

	"github.com/jinzhu/inflection"

	//lint:ignore ST1001 just for API implementations
	. "tractor.dev/integra/internal/jsonaccess"
)

type openapiService struct {
	name   string
	meta   *Value
	schema *Value

	cachedRes []*openapiResource
}

func (s *openapiService) Schema() *Value {
	return s.schema
}

func (s *openapiService) Meta() *Value {
	return s.meta
}

func (s *openapiService) Name() string {
	return s.name
}

func (s *openapiService) DataScope() string {
	return cmp.Or(AsOrZero[string](s.meta.Get("dataScope")), "mixed")
}

func (s *openapiService) Title() string {
	return AsOrZero[string](s.schema.Get("info", "title"))
}

func (s *openapiService) Provider() string {
	return AsOrZero[string](s.schema.Get("info", "x-providerName"))
}

func (s *openapiService) Version() string {
	return AsOrZero[string](s.schema.Get("info", "version"))
}

func (s *openapiService) Categories() []string {
	c := AsOrZero[[]string](s.schema.Get("info", "x-apisguru-categories"))
	c = append(c, AsOrZero[[]string](s.meta.Get("categories"))...)
	return c
}

func (s *openapiService) BaseURL() string {
	return AsOrZero[string](s.schema.Get("servers", 0, "url"))
}

func (s *openapiService) DocsURL() string {
	return AsOrZero[string](s.schema.Get("externalDocs", "url"))
}

func (s *openapiService) Security() (sec []string) {
	schemes := s.schema.Get("components", "securitySchemes")
	if schemes.IsNil() {
		return nil
	}
	for _, id := range schemes.Keys() {
		name := s.securityScheme(id)
		if name != "" {
			sec = append(sec, name)
		}
	}
	return
}

// securityScheme resolves a named OpenAPI scheme to an Integra scheme string
func (s *openapiService) securityScheme(name string) string {
	t := s.schema.Get("components", "securitySchemes", name, "type")
	if t.IsNil() {
		return ""
	}
	tt := MustAs[string](t)
	if tt == "http" {
		return MustAs[string](s.schema.Get("components", "securitySchemes", name, "scheme"))
	}
	return tt
}

func (s *openapiService) Resources() []Resource {
	var out []Resource
	if len(s.cachedRes) > 0 {
		for _, r := range s.cachedRes {
			out = append(out, r)
		}
		return out
	}

	paths := s.schema.Get("paths")
	if paths.IsNil() {
		return nil
	}

	accountPatterns := AsOrZero[[]string](s.meta.Get("accountData"))
	shouldPrefix := s.DataScope() == "mixed"

	var res []*openapiResource
	resLookup := make(map[string]*openapiResource)
	for _, p := range paths.Keys() {
		name := ToResourceName(p)

		for _, pattern := range accountPatterns {
			if ok, _ := regexp.MatchString(pattern, p); ok {
				if strings.Contains(pattern, ".*") {
					name = ToResourceName(strings.Replace(p, strings.Trim(pattern, "^.*"), "", 1))
				}
				if shouldPrefix {
					name = fmt.Sprintf("~%s", name)
				}
				break
			}
		}
		if name == "" {
			continue
		}
		if _, exists := resLookup[name]; !exists {
			var dataScope string
			if s.DataScope() == "account" || strings.HasPrefix(name, "~") {
				dataScope = "account"
			}
			r := &openapiResource{
				name:      name,
				dataScope: dataScope,
				service:   s,
				paths:     make(map[string]*Value),
			}
			res = append(res, r)
			resLookup[name] = r
		}
		r := resLookup[name]
		r.paths[p] = paths.Get(p)
	}

	for _, r := range resLookup {
		determinePathTypes(r)
	}

	s.cachedRes = res
	for _, r := range res {
		out = append(out, r)
	}
	return out
}

func determinePathTypes(r *openapiResource) {
	// if more than 2, what's going on??
	if len(r.paths) > 2 {
		log.Printf("!! more than 2 paths for '%s':\n", r.name)
		for p := range r.paths {
			log.Println("  ", p)
		}
		// keep going?
	}
	// if just one path, probably item path
	// if len(r.paths) == 1 {
	// 	for p := range r.paths {
	// 		r.itemPath = p
	// 	}
	// 	return
	// }

	// if last segment is parameter, probably item path
	for p := range r.paths {
		segments := strings.Split(p, "/")
		lastSegment := segments[len(segments)-1]
		if strings.HasPrefix(lastSegment, "{") {
			r.itemPath = p
		}
	}
	if r.itemPath != "" {
		for p := range r.paths {
			if p != r.itemPath && strings.HasPrefix(r.itemPath, p) {
				r.collectionPath = p
			}
		}
		return
	}

	// now if just one path, use inflection of last segment
	if len(r.paths) == 1 {
		for p := range r.paths {
			segments := strings.Split(p, "/")
			lastSegment := segments[len(segments)-1]
			// TODO: handle acronyms
			if lastSegment == inflection.Singular(lastSegment) {
				r.itemPath = p
			} else {
				r.collectionPath = p
			}
		}
		return
	}

	log.Printf("!! undetectable paths for '%s':\n", r.name)
	for p := range r.paths {
		log.Println("  ", p)
	}

}

func (s *openapiService) Resource(name string) (Resource, error) {
	for _, r := range s.Resources() {
		if r.Name() == name {
			return r, nil
		}
	}
	return nil, fmt.Errorf("resource '%s' not found", name)
}

type openapiResource struct {
	name           string
	parent         *openapiResource
	service        *openapiService
	dataScope      string
	paths          map[string]*Value
	itemPath       string
	collectionPath string
}

func (r *openapiResource) Debug() string {
	return fmt.Sprintf("%v %v", r.paths, r.Tags())
}

func (r *openapiResource) Service() Service {
	return r.service
}

func (r *openapiResource) Parent() Resource {
	return r.parent
}

func (r *openapiResource) Name() string {
	// if r.parent != nil {
	// 	return toResourceName(strings.Join([]string{r.parent.Name(), r.name}, "_"))
	// }
	return r.name
}

func (r *openapiResource) Title() string {
	return strings.Title(strings.Trim(r.name, "~"))
}

func (r *openapiResource) DataScope() string {
	return r.dataScope
}

func (r *openapiResource) Description() string {
	path, ok := r.paths[r.itemPath]
	if !ok {
		return ""
	}
	schema := path.Get("get", "responses", "200", "content", "application/json", "schema")
	if schema.IsNil() {
		return ""
	}
	return strings.TrimSpace(AsOrZero[string](schema.Get("description")))

}

func (r *openapiResource) CollectionURL() string {
	if r.collectionPath == "" {
		return ""
	}
	u, _ := url.JoinPath(r.service.BaseURL(), r.collectionPath)
	u = strings.ReplaceAll(u, "%7B", "{")
	u = strings.ReplaceAll(u, "%7D", "}")
	return u
}

func (r *openapiResource) ItemURL() string {
	if r.itemPath == "" {
		return ""
	}
	u, _ := url.JoinPath(r.service.BaseURL(), r.itemPath)
	u = strings.ReplaceAll(u, "%7B", "{")
	u = strings.ReplaceAll(u, "%7D", "}")
	return u
}

func (r *openapiResource) Tags() (tags []string) {
	tagSet := make(map[string]bool)
	for _, op := range r.Operations() {
		for _, tag := range op.Tags() {
			tagSet[tag] = true
		}
	}
	for tag := range tagSet {
		tags = append(tags, tag)
	}
	slices.Sort(tags)
	return
}

func (r *openapiResource) Schema() Schema {
	return nil
}

func (r *openapiResource) Operations() (ops []Operation) {
	addOperation := func(item bool, method, path string, schema *Value) {
		mapping := map[string]string{
			"get":    "list",
			"put":    "set",
			"patch":  "update",
			"delete": "delete",
			"post":   "create",
		}
		if item {
			mapping["get"] = "get"
			mapping["post"] = "post" // ehhh
		}
		opName := mapping[method]
		if opName == "" {
			log.Printf("!! unknown operation for '%s %s'\n", method, path)
			return
		}
		ops = append(ops, &openapiOperation{
			name:     opName,
			path:     path,
			method:   method,
			resource: r,
			schema:   schema,
		})
	}
	for p, path := range r.paths {
		if p == r.collectionPath {
			for _, method := range path.Keys() {
				if strings.HasPrefix(method, "x-") {
					continue
				}
				addOperation(false, method, p, path.Get(method))
			}
			continue
		}
		if p == r.itemPath {
			for _, method := range path.Keys() {
				if strings.HasPrefix(method, "x-") {
					continue
				}
				addOperation(true, method, p, path.Get(method))
			}
			continue
		}
		log.Printf("!! no operations for %d methods on '%s'\n", len(path.Keys()), p)
	}
	return
}

func (r *openapiResource) Operation(name string) (Operation, error) {
	for _, o := range r.Operations() {
		if o.Name() == name {
			return o, nil
		}
	}
	return nil, fmt.Errorf("operation '%s' not found", name)
}

type openapiOperation struct {
	name     string
	path     string
	method   string
	resource *openapiResource
	schema   *Value
}

func (o *openapiOperation) Resource() Resource {
	return o.resource
}

func (o *openapiOperation) Name() string {
	return o.name
}

func (o *openapiOperation) ID() string {
	return AsOrZero[string](o.schema.Get("operationId"))
}

func (o *openapiOperation) Description() string {
	summary := AsOrZero[string](o.schema.Get("summary"))
	if summary != "" {
		return strings.TrimSpace(summary)
	}
	return strings.TrimSpace(AsOrZero[string](o.schema.Get("description")))
}

func (o *openapiOperation) URL() string {
	if o.path == "" {
		return ""
	}
	u, _ := url.JoinPath(o.resource.service.BaseURL(), o.path)
	u = strings.ReplaceAll(u, "%7B", "{")
	u = strings.ReplaceAll(u, "%7D", "}")
	return u
}

func (o *openapiOperation) Method() string {
	return o.method
}

func (o *openapiOperation) Tags() []string {
	return AsOrZero[[]string](o.schema.Get("tags"))
}

func (o *openapiOperation) DocsURL() string {
	return AsOrZero[string](o.schema.Get("externalDocs", "url"))
}

func (o *openapiOperation) Security() (schemes []string) {
	security := o.schema.Get("security")
	if security.IsNil() {
		return nil
	}
	for _, el := range security.Items() {
		if len(el.Keys()) == 0 {
			continue
		}
		s := o.resource.service.securityScheme(el.Keys()[0])
		if s != "" {
			schemes = append(schemes, s)
		}
	}
	return
}

func (o *openapiOperation) Scopes() (scopes []string) {
	security := o.schema.Get("security")
	if security.IsNil() {
		return nil
	}
	for _, el := range security.Items() {
		if len(el.Keys()) == 0 {
			continue
		}
		scopes = append(scopes, AsOrZero[[]string](el.Get(el.Keys()[0]))...)
	}
	return
}

func (o *openapiOperation) Parameters() (params []Schema) {
	paramsRaw := o.schema.Get("parameters")
	if paramsRaw.IsNil() {
		return nil
	}
	for _, paramSchema := range paramsRaw.Items() {
		param := &openapiParameter{
			schema: paramSchema,
		}
		if param.ReadOnly() {
			continue
		}
		params = append(params, param)
	}
	return
}

func (o *openapiOperation) Input() Schema {
	reqRaw := o.schema.Get("requestBody", "content", "application/json", "schema")
	if reqRaw.IsNil() {
		return nil
	}
	return &openapiSchema{
		name:      "(input)",
		op:        o,
		writeOnly: true,
		schema:    reqRaw,
	}
}

func (o *openapiOperation) Response() Schema {
	resp := o.responseSchema()
	if resp == nil {
		return nil
	}
	// if o.Output().Name() == resp.Name() {
	// 	// if output is the response,
	// 	// we don't need response
	// 	return nil
	// }
	return resp
}

func (o *openapiOperation) responseSchema() *openapiSchema {
	s := o.schema.Get("responses", "200", "content", "application/json", "schema")
	if s.IsNil() {
		return nil
	}
	return &openapiSchema{
		name:   "(response)",
		op:     o,
		schema: s,
	}
}

func (o *openapiOperation) listingResponse() (*openapiSchema, bool) {
	resp := o.responseSchema()
	if resp == nil {
		return nil, false
	}
	if resp.Type() == "array" {
		// response is an array
		return resp, true
	}
	for _, name := range NameVariants(o.resource.name) {
		if s := resp.schema.Get("properties", name); !s.IsNil() && AsOrZero[string](s.Get("type")) == "array" {
			// response has array under resource name or variant
			return &openapiSchema{
				name:   name,
				op:     o,
				schema: s,
			}, true
		}
	}
	if s := resp.schema.Get("properties", "items"); !s.IsNil() && AsOrZero[string](s.Get("type")) == "array" {
		// response has array under "items" key
		return &openapiSchema{
			name:   "items",
			op:     o,
			schema: s,
		}, true
	}
	// TODO: more strategies
	return nil, false
}

func (o *openapiOperation) Output() Schema {
	s, isListing := o.listingResponse()
	if isListing {
		return s
	}
	// TODO: detect other envelopes
	resp := o.responseSchema()
	if resp == nil {
		return nil
	}
	return resp
}

type openapiParameter struct {
	schema *Value

	emptySchema
}

func (p *openapiParameter) Name() string {
	return AsOrZero[string](p.schema.Get("name"))
}

func (p *openapiParameter) In() string {
	return AsOrZero[string](p.schema.Get("in"))
}

func (p *openapiParameter) Description() string {
	return AsOrZero[string](p.schema.Get("description"))
}

func (p *openapiParameter) Type() string {
	return AsOrZero[string](p.schema.Get("schema", "type"))
}

func (p *openapiParameter) Enum() []string {
	return AsOrZero[[]string](p.schema.Get("schema", "enum"))
}

func (p *openapiParameter) EnumDesc() []string {
	return AsOrZero[[]string](p.schema.Get("schema", "enumDescriptions"))
}

func (p *openapiParameter) ReadOnly() bool {
	return false
}

func (p *openapiParameter) Required() bool {
	return AsOrZero[bool](p.schema.Get("required"))
}

func (p *openapiParameter) Format() string {
	return AsOrZero[string](p.schema.Get("schema", "format"))
}

func (p *openapiParameter) Default() string {
	return AsOrZero[string](p.schema.Get("schema", "default"))
}

func (p *openapiParameter) Nullable() bool {
	return AsOrZero[bool](p.schema.Get("schema", "nullable"))
}

func (p *openapiParameter) Example() string {
	return AsOrZero[string](p.schema.Get("example"))
}

type openapiSchema struct {
	name         string
	schema       *Value
	op           *openapiOperation
	writeOnly    bool
	requiredProp bool

	emptySchema
}

func (s *openapiSchema) Name() string {
	return s.name
}

func (s *openapiSchema) Title() string {
	return AsOrZero[string](s.schema.Get("title"))
}

func (s *openapiSchema) Description() string {
	return strings.TrimSpace(AsOrZero[string](s.schema.Get("description")))
}

func (s *openapiSchema) Type() string {
	return AsOrZero[string](s.schema.Get("type"))
}

func (s *openapiSchema) Enum() []string {
	return AsOrZero[[]string](s.schema.Get("enum"))
}

func (s *openapiSchema) EnumDesc() []string {
	return AsOrZero[[]string](s.schema.Get("enumDescriptions"))
}

func (s *openapiSchema) ReadOnly() bool {
	return AsOrZero[bool](s.schema.Get("readOnly"))
}

func (s *openapiSchema) Format() string {
	return AsOrZero[string](s.schema.Get("format"))
}

func (s *openapiSchema) Default() string {
	return AsOrZero[string](s.schema.Get("default"))
}

func (s *openapiSchema) Nullable() bool {
	return AsOrZero[bool](s.schema.Get("nullable"))
}

func (s *openapiSchema) Example() string {
	return AsOrZero[string](s.schema.Get("example"))
}

func (s *openapiSchema) Minimum() *int {
	min := s.schema.Get("minimum")
	if min.IsNil() {
		return nil
	}
	v := AsOrZero[int](min)
	return &v
}

func (s *openapiSchema) MinLength() *int {
	min := s.schema.Get("minLength")
	if min.IsNil() {
		return nil
	}
	v := AsOrZero[int](min)
	return &v
}

func (s *openapiSchema) MaxLength() *int {
	max := s.schema.Get("maxLength")
	if max.IsNil() {
		return nil
	}
	v := AsOrZero[int](max)
	return &v
}

func (s *openapiSchema) Required() bool {
	if s.requiredProp {
		return true
	}
	local := s.schema.Get("required")
	if !local.IsNil() {
		return AsOrZero[bool](local)
	}
	return false
}

func (s *openapiSchema) Properties() (props []Schema) {
	propsRaw := s.schema.Get("properties")
	if propsRaw.IsNil() {
		return nil
	}
	requiredProps := AsOrZero[[]string](s.schema.Get("required"))
	for _, name := range propsRaw.Keys() {
		prop := &openapiSchema{
			name:         name,
			op:           s.op,
			schema:       propsRaw.Get(name),
			requiredProp: slices.Contains(requiredProps, name),
		}
		if s.writeOnly && prop.ReadOnly() {
			continue
		}
		props = append(props, prop)
	}
	return
}

func (s *openapiSchema) Property(name string) (Schema, error) {
	for _, p := range s.Properties() {
		if p.Name() == name {
			return p, nil
		}
	}
	return nil, fmt.Errorf("property '%s' not found", name)
}

func (s *openapiSchema) Items() Schema {
	items := s.schema.Get("items")
	if items.IsNil() {
		return nil
	}
	return &openapiSchema{
		name:      "(item)",
		op:        s.op,
		writeOnly: s.writeOnly,
		schema:    items,
	}
}

func (s *openapiSchema) AnyOf() (schemas []Schema) {
	anyOf := s.schema.Get("anyOf")
	if anyOf.IsNil() {
		return
	}
	for idx, schemaRaw := range anyOf.Items() {
		schemas = append(schemas, &openapiSchema{
			name:      fmt.Sprintf("%s/%d", s.name, idx),
			op:        s.op,
			writeOnly: s.writeOnly,
			schema:    schemaRaw,
		})
	}
	return
}

func (s *openapiSchema) OneOf() (schemas []Schema) {
	oneOf := s.schema.Get("oneOf")
	if oneOf.IsNil() {
		return
	}
	for idx, schemaRaw := range oneOf.Items() {
		schemas = append(schemas, &openapiSchema{
			name:      fmt.Sprintf("%s/%d", s.name, idx),
			op:        s.op,
			writeOnly: s.writeOnly,
			schema:    schemaRaw,
		})
	}
	return
}
