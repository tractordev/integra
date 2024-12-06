package integra

import "tractor.dev/integra/internal/jsonaccess"

type Service interface {
	Name() string
	Title() string
	Provider() string
	Version() string
	Categories() []string
	BaseURL() string
	DocsURL() string
	Orientation() string
	Security() []string

	Resources() []Resource
	Resource(name string) (Resource, error)

	Schema() *jsonaccess.Value
	Meta() *jsonaccess.Value
}

type Resource interface {
	Service() Service
	Parent() Resource
	Superset() Resource
	Name() string
	Title() string
	Description() string
	Orientation() string
	CollectionURLs() []string
	ItemURLs() []string
	Tags() []string

	Schema() Schema

	Subresources() []Resource
	Operations() []Operation
	Operation(name string) (Operation, error)

	Debug() string
}

type Operation interface {
	Resource() Resource
	Name() string
	AbsName() string
	ID() string
	Description() string
	URL() string
	Method() string
	Tags() []string
	Orientation() string
	DocsURL() string
	Security() []string
	Scopes() []string

	Parameters() []Schema
	Response() Schema
	Input() Schema
	Output() Schema
}

type Schema interface {
	Name() string
	In() string

	Title() string
	Description() string
	Type() string

	ReadOnly() bool
	Required() bool
	Nullable() bool

	Enum() []string
	EnumDesc() []string
	Format() string
	Default() string
	Example() string

	Minimum() *int
	MinLength() *int
	MaxLength() *int
	//todo: MaxItems

	AnyOf() []Schema
	OneOf() []Schema

	Items() Schema
	Properties() []Schema
	Property(name string) (Schema, error)
}
