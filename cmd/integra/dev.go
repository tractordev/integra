package main

import (
	"fmt"
	"log"
	"slices"
	"strings"

	"tractor.dev/integra"
	"tractor.dev/integra/internal/jsonaccess"
	"tractor.dev/toolkit-go/engine/cli"
)

func devCmd() *cli.Command {
	cmd := &cli.Command{
		Usage:  "dev",
		Short:  "",
		Hidden: true,
		// Args:  cli.MinArgs(1),
		// Run: func(ctx *cli.Context, args []string) {
		// 	s, err := integra.LoadService("google-keep", "")
		// 	if err != nil {
		// 		log.Fatal(err)
		// 	}
		// 	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		// 	defer w.Flush()

		// 	fmt.Fprintf(w, "Title:\t%s\n", s.Title())
		// 	fmt.Fprintf(w, "Provider:\t%s\n", s.Provider())
		// 	fmt.Fprintf(w, "Version:\t%s\n", s.Version())
		// 	fmt.Fprintf(w, "Categories:\t%s\n", strings.Join(s.Categories(), ", "))
		// 	fmt.Fprintf(w, "Security:\t%s\n", strings.Join(s.Security(), ", "))
		// 	fmt.Fprintf(w, "Base URL:\t%s\n", s.BaseURL())
		// 	fmt.Fprintf(w, "Docs URL:\t%s\n", s.DocsURL())

		// },
	}
	cmd.AddCommand(openapiOpsCmd())
	cmd.AddCommand(openapiPathsCmd())
	return cmd
}

func openapiOpsCmd() *cli.Command {
	var (
		methodFilter string
	)
	cmd := &cli.Command{
		Usage: "openapi-operations <service>",
		Args:  cli.ExactArgs(1),
		Run: func(ctx *cli.Context, args []string) {
			s, err := integra.LoadService(args[0], "")
			if err != nil {
				log.Fatal(err)
			}

			w := describeTabWriter()
			defer w.Flush()

			var methods []string
			if methodFilter != "" {
				methods = strings.Split(methodFilter, ",")
			}

			// only works for OpenAPI services for now
			for _, path := range s.Schema().Get("paths").Keys() {
				for _, method := range s.Schema().Get("paths", path).Keys() {
					if len(methods) > 0 && !slices.Contains(methods, method) {
						continue
					}
					op := s.Schema().Get("paths", path, method, "operationId")
					if op.IsNil() {
						continue
					}
					res, _ := s.Resource(integra.ToResourceName(path))
					if res == nil {
						continue
					}
					fmt.Fprintf(w, "%s\t%s\t%s\n", jsonaccess.MustAs[string](op), res.Name(), res.Tags())
				}
			}
		},
	}
	cmd.Flags().StringVar(&methodFilter, "method", "", "filter by http methods (comma separated)")
	return cmd
}

func openapiPathsCmd() *cli.Command {
	var (
		methodFilter string
	)
	cmd := &cli.Command{
		Usage: "openapi-paths <service>",
		Args:  cli.ExactArgs(1),
		Run: func(ctx *cli.Context, args []string) {
			s, err := integra.LoadService(args[0], "")
			if err != nil {
				log.Fatal(err)
			}

			w := describeTabWriter()
			defer w.Flush()

			var filterMethods []string
			if methodFilter != "" {
				filterMethods = strings.Split(methodFilter, ",")
			}

			// only works for OpenAPI services for now
			for _, path := range s.Schema().Get("paths").Keys() {
				methods := slices.DeleteFunc(s.Schema().Get("paths", path).Keys(), func(e string) bool {
					return strings.HasPrefix(e, "x-")
				})
				if len(filterMethods) > 0 && !anyInCommon(filterMethods, methods) {
					continue
				}
				fmt.Fprintf(w, "%s\t%s\n", strings.Join(methods, ","), path)
			}
		},
	}
	cmd.Flags().StringVar(&methodFilter, "method", "", "filter by http methods (comma separated)")
	return cmd
}

func anyInCommon(slice1, slice2 []string) bool {
	// Create a map from the first slice
	set := make(map[string]struct{})
	for _, s := range slice1 {
		set[s] = struct{}{}
	}

	// Check if any element in the second slice exists in the map
	for _, s := range slice2 {
		if _, exists := set[s]; exists {
			return true
		}
	}
	return false
}
