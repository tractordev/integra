package integra

import (
	"cmp"
	"fmt"
	"log"
	"net/url"
	"path"
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

func (s *openapiService) Orientation() string {
	return cmp.Or(AsOrZero[string](s.meta.Get("contentOrientation")), "mixed")
}

func (s *openapiService) Title() string {
	return AsOrZero[string](s.schema.Get("info", "title"))
}

func (s *openapiService) Provider() string {
	u, _ := url.Parse(s.BaseURL())
	return cmp.Or(
		AsOrZero[string](s.schema.Get("info", "x-providerName")),
		u.Hostname(),
	)
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
	baseURL := AsOrZero[string](s.schema.Get("servers", 0, "url"))
	baseExtension := s.meta.Get("extendBaseTo")
	if !baseExtension.IsNil() {
		baseURL, _ = url.JoinPath(baseURL, MustAs[string](baseExtension))
	}
	return baseURL
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

func (s *openapiService) Resource(name string) (Resource, error) {
	for _, r := range s.Resources() {
		if r.Name() == name {
			return r, nil
		}
	}
	return nil, fmt.Errorf("resource '%s' not found", name)
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
	baseExtension := AsOrZero[string](s.meta.Get("extendBaseTo"))
	relativePatterns := AsOrZero[[]string](s.meta.Get("relativeContentPaths"))

	var res []*openapiResource
	resLookup := make(map[string]*openapiResource)

	for _, pathKey := range paths.Keys() {
		fixedPath := strings.TrimPrefix(pathKey, baseExtension)
		resName := ToResourceName(fixedPath)
		if resName == "" {
			continue
		}

		if _, exists := resLookup[resName]; !exists {
			r := &openapiResource{
				name:    resName,
				service: s,
			}
			res = append(res, r)
			resLookup[resName] = r
		}
		r := resLookup[resName]

		orientation := s.Orientation()
		if orientation == "mixed" {
			orientation = "absolute"
		}
		for _, pattern := range relativePatterns {
			if ok, _ := regexp.MatchString(pattern, fixedPath); ok {
				orientation = "relative"
			}
		}
		p := &openapiPath{
			parts:       strings.Split(fixedPath, "/"),
			schema:      paths.Get(pathKey),
			orientation: orientation,
			resource:    r,
		}
		r.paths = append(r.paths, p)
	}

	s.cachedRes = res
	for _, r := range res {
		out = append(out, r)
	}
	return out
}

type openapiResource struct {
	name    string
	service *openapiService
	paths   []*openapiPath

	cachedParent    Resource
	useCachedParent bool
}

func (r *openapiResource) Debug() string {
	return fmt.Sprintf("%v %v", r.paths, r.Tags())
}

func (r *openapiResource) Service() Service {
	return r.service
}

func (r *openapiResource) Subresources() (res []Resource) {
	for _, rr := range r.service.Resources() {
		if rr.Parent() == r {
			res = append(res, rr)
		}
	}
	return
}

func (r *openapiResource) Parent() Resource {
	if r.useCachedParent {
		return r.cachedParent
	}

	defer func() {
		r.useCachedParent = true
	}()

	reparent := r.service.meta.Get("forceParent", r.Name())
	if !reparent.IsNil() {
		parentName := AsOrZero[string](reparent)
		if parentName == "" {
			return nil
		}
		rr, err := r.service.Resource(parentName)
		if err != nil {
			log.Printf("!! unable to reparent '%s': %w\n", r.Name(), err)
		} else {
			r.cachedParent = rr
			return rr
		}
	}

	var shortestPath *openapiPath
	if r.Orientation() == "relative" {
		for _, p := range r.relativePaths() {
			shortestPath = p
			break
		}
	} else {
		for _, p := range r.absolutePaths() {
			shortestPath = p
			break
		}
	}

	pathname := shortestPath.parentPathname()
	for pathname != "/" && pathname != "." && pathname != "" {
		parentURL := denamePathParams(r.expandToURL(pathname))
		for _, rr := range r.service.Resources() {
			if rr.Name() == r.Name() {
				continue
			}
			for _, u := range rr.ItemURLs() {
				if denamePathParams(u) == parentURL {
					r.cachedParent = rr
					return rr
				}
			}
			for _, u := range rr.CollectionURLs() {
				if denamePathParams(u) == parentURL {
					r.cachedParent = rr
					return rr
				}
			}
			if len(rr.CollectionURLs()) == 0 {
				// even if collection endpoint doesn't exist, could be
				// under what it might be if it did exist. though we
				// need to compare paths not URLs to use path.Dir
				for _, u := range rr.ItemURLs() {
					iu, _ := url.Parse(u)
					pu, _ := url.Parse(parentURL)
					iu.Path = path.Dir(iu.Path)
					if denamePathParams(iu.Path) == pu.Path {
						r.cachedParent = rr
						return rr
					}
				}
			}
		}
		pathname = path.Dir(pathname)
	}

	return nil
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

func (r *openapiResource) relativePaths() (paths []*openapiPath) {
	for _, p := range r.paths {
		if p.orientation == "relative" {
			paths = append(paths, p)
		}
	}
	return
}

func (r *openapiResource) absolutePaths() (paths []*openapiPath) {
	for _, p := range r.paths {
		if p.orientation == "absolute" {
			paths = append(paths, p)
		}
	}
	return
}

func (r *openapiResource) Orientation() string {
	hasRelative := len(r.relativePaths()) > 0
	hasAbsolute := len(r.absolutePaths()) > 0
	if hasRelative && !hasAbsolute {
		return "relative"
	}
	if hasAbsolute && !hasRelative {
		return "absolute"
	}
	return "mixed"
}

func (r *openapiResource) primaryPath() *openapiPath {
	// todo: improve way to find primary item
	lastPath := r.paths[len(r.paths)-1]
	return lastPath
}

func (r *openapiResource) Description() string {
	path := r.primaryPath()
	schema := path.schema.Get("get", "responses", "200", "content", "application/json", "schema")
	if schema.IsNil() {
		return ""
	}
	return strings.TrimSpace(AsOrZero[string](schema.Get("description")))

}

func (r *openapiResource) expandToURL(path string) string {
	u, _ := url.JoinPath(r.service.BaseURL(), path)
	u = strings.ReplaceAll(u, "%7B", "{")
	u = strings.ReplaceAll(u, "%7D", "}")
	return u
}

func (r *openapiResource) CollectionURLs() (urls []string) {
	for _, p := range r.paths {
		if !p.isItemPath() {
			urls = append(urls, p.resource.expandToURL(p.name()))
		}
	}
	return
}

func (r *openapiResource) ItemURLs() (urls []string) {
	for _, p := range r.paths {
		if p.isItemPath() {
			urls = append(urls, p.resource.expandToURL(p.name()))
		}
	}
	return
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

func (r *openapiResource) Operation(name string) (Operation, error) {
	for _, o := range r.Operations() {
		if o.Name() == name {
			return o, nil
		}
	}
	return nil, fmt.Errorf("operation '%s' not found", name)
}

func (r *openapiResource) Operations() (ops []Operation) {
	names := make(map[string]*openapiOperation)
	for _, p := range r.paths {
		for _, o := range p.operations() {
			ops = append(ops, o)
			names[o.Name()] = o
		}
	}

	names = make(map[string]*openapiOperation)
	for _, op := range ops {
		_, exists := names[op.Name()]
		if exists {
			log.Printf("!! multiple operations named '%s' on %s\n", op.Name(), r.name)
		}
		names[op.Name()] = op.(*openapiOperation)
	}
	return
}

type openapiPath struct {
	parts       []string
	schema      *Value
	orientation string

	resource       *openapiResource
	cachedItemPath *bool
}

func (p *openapiPath) name() string {
	return strings.Join(p.parts, "/")
}

func (p *openapiPath) parentPathname() string {
	return strings.Join(p.parts[:len(p.parts)-1], "/")
}

func (p *openapiPath) hasParamBase() bool {
	lastSegment := p.parts[len(p.parts)-1]
	return strings.HasPrefix(lastSegment, "{")
}

func (p *openapiPath) baseParamName() string {
	lastSegment := p.parts[len(p.parts)-1]
	if !strings.HasPrefix(lastSegment, "{") {
		return ""
	}
	return strings.Trim(lastSegment, "{}")
}

func (p *openapiPath) hasDoubleParamBase() bool {
	if !p.hasParamBase() {
		return false
	}
	if len(p.parts) < 2 {
		return false
	}
	lastSegment := p.parts[len(p.parts)-2]
	return strings.HasPrefix(lastSegment, "{")
}

func (p *openapiPath) isItemPath() (b bool) {
	if p.cachedItemPath != nil {
		return *p.cachedItemPath
	}

	defer func() {
		p.cachedItemPath = &b
	}()

	forcedItem := p.resource.service.meta.Get("forceItemPaths", p.name())
	if !forcedItem.IsNil() {
		b = MustAs[bool](forcedItem)
		return
	}

	// /path/{} or /path/{}/{} is prob item
	if p.hasParamBase() || p.hasDoubleParamBase() {
		b = true
		return
	}

	// if /path when /path/{} exists, prob not item
	if p.isDirOfItemPath() {
		b = false
		return
	}

	// otherwise, if base segment is plural, prob not item
	b = !p.isPluralBase()
	return
}

func (p *openapiPath) isDirOfItemPath() bool {
	for _, pp := range p.siblingPaths() {
		if pp.parentPathname() == p.name() {
			return true
		}
	}
	return false
}

func (p *openapiPath) isPluralBase() bool {
	if p.hasParamBase() {
		return false
	}
	lastSegment := p.parts[len(p.parts)-1]
	// TODO: handle acronyms
	return lastSegment == inflection.Plural(lastSegment)
}

func (p *openapiPath) sharedParams() (params []Schema) {
	paramsRaw := p.schema.Get("parameters")
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

func (p *openapiPath) siblingPaths() (paths []*openapiPath) {
	for _, pp := range p.resource.paths {
		if pp != p {
			paths = append(paths, pp)
		}
	}
	return
}

func (p *openapiPath) operations() (ops []*openapiOperation) {
	for _, method := range p.schema.Keys() {
		if strings.HasPrefix(method, "x-") || method == "parameters" {
			continue
		}
		ops = append(ops, &openapiOperation{
			path:   p,
			method: method,
			schema: p.schema.Get(method),
		})
	}
	return
}

type openapiOperation struct {
	path         *openapiPath
	method       string
	schema       *Value
	nameOverride string
}

func (o *openapiOperation) Resource() Resource {
	return o.path.resource
}

func (o *openapiOperation) Name() string {
	name := o.AbsName()
	if o.path.resource.service.Orientation() == "mixed" && o.path.orientation == "relative" {
		return name + "~"
	}
	return name
}

func (o *openapiOperation) AbsName() string {
	forceNames := o.path.resource.service.meta.Get("forceMethodOpName")
	if !forceNames.IsNil() {
		name := forceNames.Get(o.path.name(), o.method)
		if !name.IsNil() {
			return MustAs[string](name)
		}
	}

	if o.nameOverride != "" {
		return o.nameOverride
	}
	mapping := map[string]string{
		"get":    "list",
		"put":    "replace",
		"patch":  "patch",
		"delete": "purge",
		"post":   "create",
		"head":   "scan",
	}
	if o.path.isItemPath() {
		mapping = map[string]string{
			"get":    "get",
			"put":    "set",
			"patch":  "update",
			"delete": "delete",
			"post":   "apply",
			"head":   "check",
		}
		if o.path.hasDoubleParamBase() {
			// if /path/{}/{} then /path/{} prob exists and we need to
			// differentiate, so we use the final path param name
			mapping["get"] = toCamelCase(fmt.Sprintf("get_with_%s", o.path.baseParamName()))
		}
	}
	return cmp.Or(mapping[o.method], "??"+o.method)
}

func (o *openapiOperation) ID() string {
	return AsOrZero[string](o.schema.Get("operationId"))
}

func (o *openapiOperation) Orientation() string {
	// todo: per operation orientation?
	return o.path.orientation
}

func (o *openapiOperation) Description() string {
	summary := AsOrZero[string](o.schema.Get("summary"))
	if summary != "" {
		return strings.TrimSpace(summary)
	}
	return strings.TrimSpace(AsOrZero[string](o.schema.Get("description")))
}

func (o *openapiOperation) URL() string {
	return o.path.resource.expandToURL(o.path.name())
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
		s := o.path.resource.service.securityScheme(el.Keys()[0])
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
	if !paramsRaw.IsNil() {
		for _, paramSchema := range paramsRaw.Items() {
			param := &openapiParameter{
				schema: paramSchema,
			}
			if param.ReadOnly() {
				continue
			}
			params = append(params, param)
		}
	}
	params = append(params, o.path.sharedParams()...)
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
	for _, name := range NameVariants(o.path.resource.name) {
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
