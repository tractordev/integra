package integra

type emptySchema struct{}

func (s *emptySchema) Name() string {
	return ""
}

func (s *emptySchema) In() string {
	return ""
}

func (s *emptySchema) Title() string {
	return ""
}

func (s *emptySchema) Description() string {
	return ""
}

func (s *emptySchema) Type() string {
	return ""
}

func (s *emptySchema) Enum() []string {
	return nil
}

func (s *emptySchema) EnumDesc() []string {
	return nil
}

func (s *emptySchema) Items() Schema {
	return nil
}

func (s *emptySchema) ReadOnly() bool {
	return false
}

func (s *emptySchema) Required() bool {
	return false
}

func (s *emptySchema) Format() string {
	return ""
}

func (s *emptySchema) Minimum() *int {
	return nil
}

func (s *emptySchema) MinLength() *int {
	return nil
}

func (s *emptySchema) MaxLength() *int {
	return nil
}

func (s *emptySchema) Default() string {
	return ""
}

func (s *emptySchema) Nullable() bool {
	return false
}

func (s *emptySchema) Example() string {
	return ""
}

func (s *emptySchema) AnyOf() []Schema {
	return nil
}

func (s *emptySchema) OneOf() []Schema {
	return nil
}

func (s *emptySchema) Properties() []Schema {
	return nil
}

func (s *emptySchema) Property(name string) (Schema, error) {
	return nil, nil
}
