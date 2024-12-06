package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/jinzhu/inflection"
	"tractor.dev/integra"
	"tractor.dev/integra/internal/jsonaccess"
	"tractor.dev/integra/internal/resource"
	"tractor.dev/toolkit-go/engine/cli"
)

func fetchCmd() *cli.Command {
	cmd := &cli.Command{
		Usage: "fetch <service> <dir>",
		Short: "",
		Args:  cli.MinArgs(2),
		Run: func(ctx *cli.Context, args []string) {
			selector, version := integra.SplitSelectorVersion(args[0])
			sel := strings.Split(selector, ".")

			s, err := integra.LoadService(sel[0], version)
			if err != nil {
				log.Fatal(err)
			}

			targetDir := filepath.Join(args[1], sel[0])
			os.MkdirAll(targetDir, 0755)

			w := describeTabWriter()
			defer w.Flush()

			dataset := resource.NewDataset()

			// first get top level singleton resources
			integra.WalkResources(s, func(r integra.Resource) {
				for _, op := range r.Operations() {
					if op.Orientation() != "relative" {
						continue
					}

					if op.AbsName() != "get" {
						continue
					}

					// top level singletons should have no params
					if len(integra.RequiredParameters(op)) > 0 {
						continue
					}

					col := dataset.Collection(r)
					fmt.Fprintf(w, "%s.%s\n", r.Name(), op.Name())
					w.Flush()

					data, err := fetch(op, nil)
					if err != nil {
						if warn, ok := isWarning(err); ok {
							fmt.Fprintf(w, "  WARN: %s\n", warn)
						} else {
							fmt.Fprintf(w, "  ERROR: %s\n", err)
						}
						w.Flush()
						continue
					}

					// singleton resources use empty keys
					col.Set("", data.Data())
				}

			})

			// now get resource listings
			integra.WalkResources(s, func(r integra.Resource) {
				for _, op := range r.Operations() {
					if op.Orientation() != "relative" {
						continue
					}

					if op.AbsName() != "list" {
						continue
					}

					parent := r.Parent()
					if parent != nil {
						col := dataset.Collection(parent)
						for _, item := range col.GetAll() {
							fetchList(w, dataset, r, op, item.Key)
						}
						continue
					}

					fetchList(w, dataset, r, op, "")
				}

			})
		},
	}
	return cmd
}

func fetchList(w *tabwriter.Writer, dataset *resource.Dataset, r integra.Resource, op integra.Operation, parentID string) {
	defer w.Flush()

	reqParams := integra.RequiredParameters(op)
	if len(reqParams) > 1 {
		var names []string
		for _, p := range reqParams {
			names = append(names, p.Name())
		}
		fmt.Fprintf(w, "%s.%s\n", r.Name(), op.Name())
		fmt.Fprintf(w, "  SKIP: needs params: %s\n", strings.Join(names, ","))
		return
	}

	var params map[string]any
	if parentID != "" {
		parents := integra.ResourceParents(r)
		if len(parents) == 0 {
			fmt.Fprintf(w, "%s.%s\n", r.Name(), op.Name())
			fmt.Fprintf(w, "  SKIP: param but no parent: %s\n", reqParams[0].Name())
			return
		}
		// we're guessing the single param is the parent id, regardless of name
		params = map[string]any{
			reqParams[0].Name(): parentID,
		}
		fmt.Fprintf(w, "%s.%s [%s:%s]\n", r.Name(), op.Name(), parents[0].Name(), parentID)
		w.Flush()
	} else {
		fmt.Fprintf(w, "%s.%s\n", r.Name(), op.Name())
		w.Flush()
	}

	resp, err := fetch(op, params)
	if err != nil {
		if warn, ok := isWarning(err); ok {
			fmt.Fprintf(w, "  WARN: %s\n", warn)
		} else {
			fmt.Fprintf(w, "  ERROR: %s\n", err)
		}
		return
	}

	getOp := integra.ResourceGetter(op.Resource())
	if getOp == nil {
		fmt.Fprintf(w, "  TODO: handle listable resources with no getter\n")
		return
	}

	schema, items, err := parseListing(op, resp)
	if err != nil {
		fmt.Fprintf(w, "  ERROR: %s\n", err)
		return
	}

	ok, keyProps := getterKeyProps(getOp, op, schema)
	if !ok {
		var names []string
		for k, v := range keyProps {
			if v == nil {
				names = append(names, k)
			}
		}
		fmt.Fprintf(w, "  SKIP: unable to determine key props for getter: %s\n", strings.Join(names, ","))
		return
	}

	col := dataset.Collection(r)
	for _, item := range items {
		params := buildItemParams(item, keyProps)
		resp, err := fetch(getOp, params)
		if err != nil {
			fmt.Fprintf(w, "  ERROR: %s\n", err)
			w.Flush()
			break
		}
		_, item := parseItem(getOp, resp)
		key := getItemKey(item, schema, params)
		fmt.Fprintf(w, "  %s.%s: %v (%s)\n", getOp.Resource().Name(), getOp.Name(), params, key)
		w.Flush()
		col.Set(key, resp.Data())
	}
}

func propString(item *jsonaccess.Value, prop integra.Schema) string {
	v := item.Get(prop.Name())
	if v.IsNil() {
		return ""
	}
	switch prop.Type() {
	case "string":
		return jsonaccess.MustAs[string](v)
	case "integer":
		i := jsonaccess.MustAs[int](v)
		return strconv.Itoa(i)
	default:
		log.Panicf("unsupported prop type as string: %s", prop.Type())
	}
	return ""
}

func getItemKey(item *jsonaccess.Value, listSchema integra.Schema, params map[string]any) string {
	// todo: more robust system for determining key

	// look for first property ending with "id"
	for _, p := range listSchema.Items().Properties() {
		if strings.HasSuffix(p.Name(), "id") {
			return propString(item, p)
		}
	}

	// if single param, its probably the key
	if len(params) == 1 {
		for _, v := range params {
			return v.(string)
		}
	}

	// otherwise use the first listed property
	return propString(item, listSchema.Items().Properties()[0])

}

func parseItem(op integra.Operation, resp *jsonaccess.Value) (schema integra.Schema, item *jsonaccess.Value) {
	if op.Response().Name() == op.Output().Name() {
		return op.Response(), resp
	}
	schema = op.Output()
	item = resp.Get(schema.Name())
	return
}

func parseListing(op integra.Operation, resp *jsonaccess.Value) (schema integra.Schema, items []*jsonaccess.Value, err error) {
	if op.Response().Name() == op.Output().Name() && op.Response().Type() != "array" {
		return nil, nil, fmt.Errorf("no listing found in response")
	}
	schema = op.Response()
	if schema.Type() == "array" {
		items = resp.Items()
		return
	}
	schema = op.Output()
	items = resp.Get(schema.Name()).Items()
	return
}

func getterKeyProps(getOp, listOp integra.Operation, listSchema integra.Schema) (ok bool, keyProps map[string]integra.Schema) {
	// keyProps are required params for the getOp.
	// first making keys for their names...
	keyProps = make(map[string]integra.Schema)
	for _, p := range getOp.Parameters() {
		if p.Required() {
			keyProps[p.Name()] = nil
		}
	}

	listURLParts := strings.Split(listOp.URL(), "/")
	resShortName := inflection.Singular(listURLParts[len(listURLParts)-1])

	for _, p := range listSchema.Items().Properties() {
		if p.Name() == "id" {
			if len(keyProps) == 1 {
				// we are guessing the 1 keyProp param is the id
				// so we set the schema to the id field schema
				for k := range keyProps {
					keyProps[k] = p
				}
			} else {
				// otherwise we see if the resource name with _id suffix
				// is in keyProps and if so we set to the id field schema
				_, ok := keyProps[fmt.Sprintf("%s_id", resShortName)]
				if ok {
					keyProps[fmt.Sprintf("%s_id", resShortName)] = p
				}
			}
		} else {
			// otherwise lets check for an exact matching property
			// TODO: could be an object and need the key from it!
			//		see github starred.list: map[owner:owner repo:] (owner is object)
			for k := range keyProps {
				if p.Name() == k {
					keyProps[k] = p
					break
				}
			}
		}
	}

	ok = true
	// now check for nils in keyProps
	// to determine we have all needed keys
	for k, prop := range keyProps {
		if prop == nil {
			ok = false
			break
		}
		if !slices.Contains([]string{"string", "integer"}, prop.Type()) {
			log.Println("!! needs prop type:", prop.Type(), k)
			ok = false
			break
		}
	}

	return
}

func buildItemParams(item *jsonaccess.Value, keyProps map[string]integra.Schema) map[string]any {
	params := make(map[string]any)
	// use keyProps to build params
	for k, prop := range keyProps {
		v := item.Get(prop.Name())
		if v.IsNil() {
			log.Panicf("key prop not found in item")
		}
		switch prop.Type() {
		case "string":
			params[k] = jsonaccess.MustAs[string](v)
		case "integer":
			i := jsonaccess.MustAs[int](v)
			params[k] = strconv.Itoa(i)
		}
	}
	return params
}

func isWarning(err error) (string, bool) {
	if err.Error() == "401" {
		return "unauthorized", true
	}
	if err.Error() == "403" {
		return "forbidden", true
	}
	if err.Error() == "404" {
		return "not found", true
	}
	return "", false
}

func fetch(op integra.Operation, params map[string]any) (*jsonaccess.Value, error) {
	req, err := integra.MakeRequest(op, params)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if slices.Contains([]int{http.StatusNotFound, http.StatusUnauthorized, http.StatusForbidden}, resp.StatusCode) {
		return nil, fmt.Errorf("%d", resp.StatusCode)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected response status: %s", resp.Status)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var data any
	if err := json.Unmarshal(b, &data); err != nil {
		return nil, err
	}

	return jsonaccess.New(data), nil
}

func writeResponse(targetDir string, s integra.Service, req *http.Request, data []byte) error {
	outPath := filepath.Join(targetDir, strings.TrimPrefix(req.URL.String(), s.BaseURL()))
	outPath = fmt.Sprintf("%s.json", strings.TrimSuffix(outPath, ".json"))
	os.MkdirAll(filepath.Dir(outPath), 0755)
	if err := os.WriteFile(outPath, data, 0644); err != nil {
		return err
	}
	return nil
}
