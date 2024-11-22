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
	"tractor.dev/toolkit-go/engine/cli"
)

func fetchCmd() *cli.Command {
	// var (
	// 	methodFilter string
	// 	opFilter     string
	// )
	cmd := &cli.Command{
		Usage: "fetch <service> <dir>",
		Short: "",
		Args:  cli.MinArgs(2),
		Run: func(ctx *cli.Context, args []string) {
			// hardcoding for now
			accountData = true

			selector, version := integra.SplitSelectorVersion(args[0])
			sel := strings.Split(selector, ".")

			s, err := integra.LoadService(sel[0], version)
			if err != nil {
				log.Fatal(err)
			}

			targetDir := filepath.Join(args[1], sel[0])
			os.MkdirAll(targetDir, 0755)

			// var filterMethods []string
			// var filterOps []string
			// if methodFilter != "" {
			// 	filterMethods = strings.Split(methodFilter, ",")
			// }
			// if opFilter != "" {
			// 	filterOps = strings.Split(opFilter, ",")
			// }

			w := describeTabWriter()
			defer w.Flush()

			nonAccountResources := make(map[string]integra.Resource)
			for _, r := range s.Resources() {
				if accountData && r.Orientation() != "relative" {
					nonAccountResources[r.Name()] = r
					continue
				}
				for _, op := range r.Operations() {
					// if filterMethods != nil && !slices.Contains(filterMethods, op.Method()) {
					// 	continue
					// }
					// if filterOps != nil && !slices.Contains(filterOps, op.Name()) {
					// 	continue
					// }
					if op.AbsName() != "list" && op.AbsName() != "get" {
						continue
					}

					params := slices.DeleteFunc(op.Parameters(), func(p integra.Schema) bool {
						return !p.Required()
					})

					if op.AbsName() == "get" && len(params) > 0 {
						// only singleton resource get operations
						continue
					}

					fmt.Fprintf(w, "%s.%s\n", r.Name(), op.Name())
					w.Flush()

					if len(params) > 0 {
						fmt.Fprintf(w, "  SKIP: needs params\n")
						w.Flush()
						continue
					}

					req, err := integra.MakeRequest(op, nil)
					if err != nil {
						fmt.Fprintf(w, "  ERROR: %s\n", err)
						w.Flush()
						continue
					}

					resp, err := http.DefaultClient.Do(req)
					if err != nil {
						fmt.Fprintf(w, "  ERROR: %s\n", err)
						w.Flush()
						continue
					}

					if resp.StatusCode == 200 {

						b, err := io.ReadAll(resp.Body)
						if err != nil {
							log.Fatal(err)
						}

						if err := writeResponse(targetDir, s, req, b); err != nil {
							log.Fatal(err)
						}

						var data any
						if err := json.Unmarshal(b, &data); err != nil {
							log.Fatal(err)
						}

						if op.AbsName() == "list" {
							handleList(w, targetDir, op, jsonaccess.New(data), nonAccountResources)
						} else {
							fmt.Fprintf(w, "  singleton: %s \n", resp.Status)
						}

						w.Flush()
					} else {
						fmt.Fprintf(w, "  %s\n", resp.Status)
						w.Flush()
					}
					resp.Body.Close()

				}

			}

		},
	}
	// cmd.Flags().BoolVar(&accountData, "account", false, "only account data")
	// cmd.Flags().StringVar(&methodFilter, "methods", "", "filter by http methods (comma separated)")
	// cmd.Flags().StringVar(&opFilter, "ops", "", "filter by op names (comma separated)")
	return cmd
}

func handleList(w *tabwriter.Writer, targetDir string, op integra.Operation, resp *jsonaccess.Value, nonAccountResources map[string]integra.Resource) {
	if op.Response().Name() == op.Output().Name() && op.Response().Type() != "array" {
		fmt.Fprintf(w, "  ERROR: no listing response\n")
		return
	}
	listSchema := op.Response()
	var listItems []*jsonaccess.Value
	if listSchema.Type() == "array" {
		listItems = resp.Items()
	} else {
		listSchema = op.Output()
		listItems = resp.Get(listSchema.Name()).Items()
	}

	getOp, err := op.Resource().Operation("get")
	if err != nil {
		res, ok := nonAccountResources[strings.TrimLeft(op.Resource().Name(), "~")]
		if !ok {
			fmt.Fprintf(w, "  %d %s, no get, no other resource \n", len(listItems), listSchema.Name())
			return
		}
		otherGet, err := res.Operation("get")
		if err != nil {
			fmt.Fprintf(w, "  %d %s, no get on either resource \n", len(listItems), listSchema.Name())
			return
		}
		getOp = otherGet
	}

	keyProps := make(map[string]integra.Schema)
	for _, p := range getOp.Parameters() {
		if p.Required() {
			keyProps[p.Name()] = nil
		}
	}

	listURLParts := strings.Split(op.URL(), "/")
	resShortName := inflection.Singular(listURLParts[len(listURLParts)-1])

	var itemProps []string
	for _, p := range listSchema.Items().Properties() {
		if strings.HasSuffix(p.Name(), "id") {
			itemProps = append(itemProps, p.Name())
		}
		if p.Name() == "id" {
			if len(keyProps) == 1 {
				// we are guessing the 1 param is the id
				for k := range keyProps {
					keyProps[k] = p
				}
			} else {
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

	hasKeys := true
	for k, prop := range keyProps {
		if prop == nil {
			hasKeys = false
			break
		}
		if !slices.Contains([]string{"string", "integer"}, prop.Type()) {
			fmt.Fprintln(w, "!! needs prop type:", prop.Type(), k)
			hasKeys = false
			break
		}
	}

	fmt.Fprintf(w, "  %d %s, %v => %v \n", len(listItems), listSchema.Name(), itemProps, hasKeys)
	if hasKeys {
		for _, item := range listItems {
			params := make(map[string]any)
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

			req, err := integra.MakeRequest(op, params)
			if err != nil {
				fmt.Fprintf(w, "  - %v: ERROR: %s\n", params, err)
				w.Flush()
				continue
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				fmt.Fprintf(w, "  - %v: ERROR: %s\n", params, err)
				w.Flush()
				continue
			}

			if resp.StatusCode == 200 {
				data, err := io.ReadAll(resp.Body)
				if err != nil {
					log.Fatal(err)
				}
				if err := writeResponse(targetDir, op.Resource().Service(), req, data); err != nil {
					log.Fatal(err)
				}
			}
			resp.Body.Close()

			fmt.Fprintf(w, "  - %v: %s \n", params, resp.Status)
		}

	}
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
