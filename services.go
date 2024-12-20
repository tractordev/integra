package integra

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
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

func ServiceClientCredentials(service string) (string, string) {
	service = strings.ReplaceAll(strings.ToUpper(service), "-", "_")
	return os.Getenv(fmt.Sprintf("%s_CLIENT_ID", service)), os.Getenv(fmt.Sprintf("%s_CLIENT_SECRET", service))
}

func LoadService(name, version string) (Service, error) {
	serviceDir := strings.ReplaceAll(name, "-", "/")

	b, err := fs.ReadFile(services, path.Join("services", serviceDir, "meta.yaml"))
	if err != nil {
		return nil, err
	}

	var yamlData map[any]any
	if err := yaml.Unmarshal(b, &yamlData); err != nil {
		return nil, err
	}
	meta := jsonaccess.New(convertYAMLToStringMap(yamlData))

	if version == "" {
		version = jsonaccess.MustAs[string](meta.Get("latest"))
	}

	dir, err := fs.ReadDir(services, path.Join("services", serviceDir, version))
	if err != nil {
		return nil, err
	}

	for _, info := range dir {
		switch info.Name() {
		case "openapi.json":
			b, err = fs.ReadFile(services, path.Join("services", serviceDir, version, "openapi.json"))
			if err != nil {
				return nil, err
			}

			var data map[string]any
			if err := json.Unmarshal(b, &data); err != nil {
				return nil, err
			}

			root := jsonaccess.New(data)
			resolver := jsonaccess.NewPointerResolver(root)
			root = root.WithResolver(resolver).WithAllOfMerge()

			return &openapiService{name: name, schema: root, meta: meta}, nil

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

func ExpandURL(u string, params map[string]any) (string, error) {
	for k, v := range params {
		u = strings.Replace(u, fmt.Sprintf("{%s}", k), fmt.Sprint(v), 1)
	}
	if strings.Contains(u, "{") {
		return "", fmt.Errorf("parameters not sufficient to expand URL: %s", u)
	}
	return u, nil
}

func MakeRequest(op Operation, in map[string]any) (*http.Request, error) {

	var required []string
	for _, p := range op.Parameters() {
		if p.Required() {
			required = append(required, p.Name())
		}
	}

	for _, name := range required {
		_, ok := in[name]
		if !ok {
			return nil, fmt.Errorf("missing '%s' of required parameters: %v", name, required)
		}
	}

	data := make(map[string]any)
	for k, v := range in {
		data[k] = v
	}

	params := make(map[string]any)
	for _, p := range op.Parameters() {
		v, ok := data[p.Name()]
		if ok {
			params[p.Name()] = v
			delete(data, p.Name())
		}
	}
	u, err := ExpandURL(op.URL(), params)
	if err != nil {
		return nil, err
	}

	// var body io.Reader
	// if slices.Contains([]string{"create", "set", "update"}, op.Name()) {
	// 	b, err := json.Marshal(params)
	// 	if err != nil {
	// 		log.Fatal(err)
	// 	}
	// 	body = bytes.NewBuffer(b)
	// }

	// todo: need to populate url since implementing new model
	req, err := http.NewRequest(strings.ToUpper(op.Method()), u, nil)
	if err != nil {
		return nil, err
	}

	// todo: alternative schemes
	token := ServiceToken(op.Resource().Service().Name())
	if token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	}

	req.Header.Set("Content-Type", "application/json")

	return req, nil
}
