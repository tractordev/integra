package resource

import (
	"slices"

	"tractor.dev/integra"
)

// not yet thread safe
type Dataset struct {
	collections map[string]*Collection
}

func NewDataset() *Dataset {
	return &Dataset{
		collections: make(map[string]*Collection),
	}
}

func (d *Dataset) Collection(r integra.Resource) *Collection {
	c, exists := d.collections[r.Name()]
	if !exists {
		c = &Collection{
			resource: r,
			items:    make(map[string]Item),
		}
		d.collections[r.Name()] = c
	}
	return c
}

// not yet thread safe
type Collection struct {
	resource integra.Resource
	items    map[string]Item
}

func (c *Collection) Resource() integra.Resource {
	return c.resource
}

func (c *Collection) Keys() []string {
	var keys []string
	for k := range c.items {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

func (c *Collection) GetAll() (items []Item) {
	for _, k := range c.Keys() {
		items = append(items, c.items[k])
	}
	return
}

func (c *Collection) Get(key string) Item {
	return c.items[key]
}

func (c *Collection) Set(key string, v any) {
	c.items[key] = Item{Key: key, Value: v}
}

type Item struct {
	Key   string
	Value any
}
