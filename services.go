package integra

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path"
	"strings"

	"gopkg.in/yaml.v2"
	"tractor.dev/integra/internal/jsonaccess"
)

//go:embed services
var services embed.FS

// SplitSelectorVersion takes a selector with an optional version
// at the end and returns the selector and the version if any
func SplitSelectorVersion(s string) (selector string, version string) {
	parts := strings.SplitN(s, "@", 2)
	if len(parts) > 1 {
		return parts[0], parts[1]
	}
	return parts[0], ""
}

func AvailableServices() (names []string) {
	fs.WalkDir(services, "services", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if strings.HasSuffix(path, "/meta.yaml") {
			path = strings.TrimPrefix(path, "services/")
			path = strings.TrimSuffix(path, "/meta.yaml")
			path = strings.ReplaceAll(path, "/", "-")
			names = append(names, path)
		}
		return nil
	})
	return
}

func ServiceToken(service string) string {
	service = strings.ReplaceAll(strings.ToUpper(service), "-", "_")
	return os.Getenv(fmt.Sprintf("%s_TOKEN", service))
}

func LoadServiceSchema(service, version string) (map[string]any, error) {
	return nil, nil
}

func LoadService(name, version string) (Service, error) {
	serviceDir := strings.ReplaceAll(name, "-", "/")

	b, err := fs.ReadFile(services, path.Join("services", serviceDir, "meta.yaml"))
	if err != nil {
		return nil, err
	}

	var metaRaw map[string]any
	if err := yaml.Unmarshal(b, &metaRaw); err != nil {
		return nil, err
	}
	meta := jsonaccess.New(metaRaw)

	if version == "" {
		version = jsonaccess.MustAs[string](meta.Get("latest"))
	}

	dir, err := fs.ReadDir(services, path.Join("services", serviceDir, version))
	if err != nil {
		return nil, err
	}

	for _, info := range dir {
		switch info.Name() {
		case "openapi.yaml":
			b, err = fs.ReadFile(services, path.Join("services", serviceDir, version, "openapi.yaml"))
			if err != nil {
				return nil, err
			}

			var raw map[any]any
			if err := yaml.Unmarshal(b, &raw); err != nil {
				return nil, err
			}
			data := convertYAMLToStringMap(raw)

			root := jsonaccess.New(data)
			resolver := jsonaccess.NewPointerResolver(root)
			root = root.WithResolver(resolver).WithAllOfMerge()

			return &openapiService{name: name, schema: root, meta: meta}, nil

		case "googleapi.json":
			b, err = fs.ReadFile(services, path.Join("services", serviceDir, version, "googleapi.json"))
			if err != nil {
				return nil, err
			}

			var data map[string]any
			if err := json.Unmarshal(b, &data); err != nil {
				return nil, err
			}

			root := jsonaccess.New(data)
			resolver := jsonaccess.NewDefinitionsResolver(root.Get("schemas"))
			root = root.WithResolver(resolver).WithAllOfMerge()

			return &googleService{name: name, schema: root, meta: meta}, nil
		}
	}

	return nil, fmt.Errorf("no schema found for %s@%s", name, version)
}
