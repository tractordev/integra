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
	DataScope() string
	Security() []string

	Resources() []Resource
	Resource(name string) (Resource, error)

	Schema() *jsonaccess.Value
}

type Resource interface {
	Service() Service
	Parent() Resource
	Name() string
	Title() string
	Description() string
	DataScope() string
	CollectionURL() string
	ItemURL() string
	Tags() []string

	Schema() Schema

	Operations() []Operation
	Operation(name string) (Operation, error)

	Debug() string
}

type Operation interface {
	Resource() Resource
	Name() string
	ID() string
	Description() string
	URL() string
	Method() string
	Tags() []string
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
