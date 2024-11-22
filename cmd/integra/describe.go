package main

import (
	"fmt"
	"log"
	"os"
	"slices"
	"sort"
	"strings"
	"text/tabwriter"

	"tractor.dev/integra"
	"tractor.dev/toolkit-go/engine/cli"
)

var (
	showInfo      bool
	showResources bool
	showProps     bool
	showOps       bool

	accountData bool
)

func describeTabWriter() *tabwriter.Writer {
	return tabwriter.NewWriter(os.Stdout, 0, 4, 4, ' ', 0)
}

func describeCmd() *cli.Command {
	cmd := &cli.Command{
		Usage: "describe <selector>",
		Short: "describe aspects of a service API",
		Args:  cli.MaxArgs(1),
		Run: func(ctx *cli.Context, args []string) {
			if len(args) == 0 {
				describeServices()
				return
			}

			selector, version := integra.SplitSelectorVersion(args[0])
			sel := strings.Split(selector, ".")

			s, err := integra.LoadService(sel[0], version)
			if err != nil {
				log.Fatal(err)
			}

			if len(sel) == 1 {
				describeService(s)
				return
			}

			r, err := s.Resource(sel[1])
			if err != nil {
				log.Fatal(err)
			}

			if len(sel) == 2 {
				describeResource(r)
				return
			}

			// TODO: select into resource properties
			op, err := r.Operation(sel[2])
			if err != nil {
				log.Fatal(err)
			}

			if len(sel) == 3 {
				describeOperation(op)
				return
			}

			// opSelector := sel[3]
			// if opSelector == "in" {
			// 	describeOperationParams(op)
			// 	return
			// }
			// if opSelector == "out" {
			// 	describeOperationResponse(op)
			// 	return
			// }

			fmt.Println("invalid selector")
			os.Exit(1)
		},
	}
	cmd.Flags().BoolVar(&accountData, "account", false, "only account data")
	cmd.Flags().BoolVar(&showInfo, "info", false, "show info")
	cmd.Flags().BoolVar(&showResources, "resources", false, "show resources")
	cmd.Flags().BoolVar(&showOps, "methods", false, "show methods")
	cmd.Flags().BoolVar(&showProps, "props", false, "show properties")
	return cmd
}

func describeServices() {
	for _, s := range integra.AvailableServices() {
		fmt.Println(s)
	}
}

func describeServiceInfo(s integra.Service) {
	w := describeTabWriter()
	defer w.Flush()

	fmt.Fprintf(w, "Title:\t%s\n", s.Title())
	fmt.Fprintf(w, "Provider:\t%s\n", s.Provider())
	fmt.Fprintf(w, "Version:\t%s\n", s.Version())
	fmt.Fprintf(w, "Orientation:\t%s\n", s.Orientation())
	fmt.Fprintf(w, "Categories:\t%s\n", strings.Join(s.Categories(), ", "))
	fmt.Fprintf(w, "Security:\t%s\n", strings.Join(s.Security(), ", "))
	fmt.Fprintf(w, "Base URL:\t%s\n", s.BaseURL())
	fmt.Fprintf(w, "Docs URL:\t%s\n", s.DocsURL())
}

func describeServiceResources(s integra.Service) {
	res := s.Resources()
	if accountData {
		res = slices.DeleteFunc(res, func(r integra.Resource) bool {
			return r.Orientation() != "relative"
		})
	}
	tagGroups := make(map[string][]integra.Resource)
	for _, r := range res {
		for _, tag := range r.Tags() {
			tagGroups[tag] = append(tagGroups[tag], r)
		}
	}

	// if no tags, just list resources
	if len(tagGroups) == 0 {
		for _, r := range res {
			fmt.Println(r.Name())
		}
		fmt.Println()
		return
	}

	// otherwise, list resources grouped by tag
	var sortedTags []string
	for tag := range tagGroups {
		sortedTags = append(sortedTags, tag)
	}
	sort.Strings(sortedTags)
	for _, tag := range sortedTags {
		resources := tagGroups[tag]
		sort.Slice(resources, func(i, j int) bool {
			return resources[i].Name() < resources[j].Name()
		})

		fmt.Println(strings.ToUpper(tag))
		for _, resource := range resources {
			fmt.Printf("  %s\n", resource.Name())
		}
		fmt.Println()
	}
}

func describeService(service integra.Service) {
	if showInfo {
		// only show info
		describeServiceInfo(service)
		return
	}

	if showResources {
		// only show resources
		describeServiceResources(service)
		return
	}

	fmt.Printf("=== SERVICE INFO\n")
	describeServiceInfo(service)
	fmt.Println()

	fmt.Printf("=== SERVICE RESOURCES\n")
	describeServiceResources(service)

}

func describeResourceOperations(r integra.Resource) {
	w := describeTabWriter()
	defer w.Flush()
	for _, op := range r.Operations() {
		fmt.Fprintf(w, "%s\t%s\n", op.Name(), shortText(op.Description()))
	}
	fmt.Fprintln(w)
}

func describeResourceInfo(r integra.Resource) {
	w := describeTabWriter()
	defer w.Flush()
	fmt.Fprintf(w, "Title:\t%s\n", r.Title())
	if r.Description() != "" {
		fmt.Fprintf(w, "Description:\t%s\n", shortText(r.Description()))
	}
	if r.Parent() != nil {
		fmt.Fprintf(w, "Parent:\t%s\n", r.Parent().Name())
	}
	if len(r.Tags()) > 0 {
		fmt.Fprintf(w, "Tags:\t%s\n", strings.Join(r.Tags(), ", "))
	}
	fmt.Fprintf(w, "Orientation:\t%s\n", r.Orientation())
	if len(r.CollectionURLs()) > 0 {
		fmt.Fprintf(w, "Collection URLs:\t%s\n", strings.Join(r.CollectionURLs(), ", "))
	}
	if len(r.ItemURLs()) > 0 {
		fmt.Fprintf(w, "Item URLs:\t%s\n", strings.Join(r.ItemURLs(), ", "))
	}
	fmt.Fprintln(w)
}

func describeResource(r integra.Resource) {
	if showInfo {
		// only show info
		describeResourceInfo(r)
		return
	}

	// if showProps {
	// 	// only show props
	// 	if r.Schema != nil {
	// 		describeProps(r.Schema)
	// 	}
	// 	return
	// }

	if showOps {
		// only show operations
		describeResourceOperations(r)
		return
	}

	fmt.Printf("=== RESOURCE INFO\n")
	describeResourceInfo(r)

	// if r.Schema != nil {
	// 	fmt.Println()
	// 	fmt.Printf("=== RESOURCE PROPERTIES\n")
	// 	describeProps(r.Schema)

	// 	fmt.Println()
	// }

	subs := r.Subresources()
	if len(subs) > 0 {
		fmt.Printf("=== RESOURCE SUB-RESOURCES\n")
		for _, sr := range subs {
			fmt.Println(sr.Name())
		}
		fmt.Println()
	}

	fmt.Printf("=== RESOURCE OPERATIONS\n")
	describeResourceOperations(r)

}

func describeOperationInfo(op integra.Operation) {
	w := describeTabWriter()
	defer w.Flush()

	if op.ID() != "" {
		fmt.Fprintf(w, "ID:\t%s\n", op.ID())
	}
	if op.Description() != "" {
		fmt.Fprintf(w, "Description:\t%s\n", shortText(op.Description()))
	}
	fmt.Fprintf(w, "Endpoint:\t%s\n", op.URL())
	fmt.Fprintf(w, "Method:\t%s\n", op.Method())
	if len(op.Tags()) > 0 {
		fmt.Fprintf(w, "Tags:\t%s\n", strings.Join(op.Tags(), ", "))
	}
	fmt.Fprintf(w, "Security:\t%s\n", strings.Join(op.Security(), ", "))

	if len(op.Scopes()) > 0 {
		fmt.Fprintf(w, "Scopes:\t%s\n", strings.Join(op.Scopes(), ", "))
	}

	if op.DocsURL() != "" {
		fmt.Fprintf(w, "Docs URL:\t%s\n", op.DocsURL())
	}

	fmt.Fprintln(w)
}

func describeOperation(op integra.Operation) {
	if showInfo {
		// only show info
		describeOperationInfo(op)
		return
	}

	fmt.Printf("=== OPERATION INFO\n")
	describeOperationInfo(op)

	if params := op.Parameters(); len(params) > 0 {
		fmt.Printf("=== OPERATION PARAMETERS\n")
		describePropSummary(params, "", true)
	}

	if input := op.Input(); input != nil {
		fmt.Printf("=== OPERATION INPUT\n")
		fmt.Printf("%s:\n", input.Type())
		describePropSummary(input.Properties(), "  ", true)
	}

	resp := op.Response()
	output := op.Output()

	if resp != nil && resp.Name() != output.Name() {
		fmt.Printf("=== OPERATION RESPONSE\n")
		fmt.Printf("%s:\n", resp.Type())
		describePropSummary(resp.Properties(), "  ", false)
	}

	if output != nil {
		fmt.Printf("=== OPERATION OUTPUT\n")
		if output.Type() == "array" {
			fmt.Printf("%s of %s:\n", output.Type(), output.Items().Type())
			output = output.Items()
		} else {
			fmt.Printf("%s:\n", output.Type())
		}
		describePropSummary(output.Properties(), "  ", false)
	}
}

func describeProps(props []integra.Schema) {
	w := describeTabWriter()
	defer w.Flush()
	for _, prop := range props {
		var features []string
		features = append(features, schemaFeatures(prop)...)

		if len(features) > 0 {
			fmt.Fprintf(w, "%s:\t%s (%s)\n", prop.Name(), prop.Type(), strings.Join(features, ", "))
		} else {
			fmt.Fprintf(w, "%s:\t%s\n", prop.Name(), prop.Type())
		}
		if prop.Description() != "" {
			fmt.Fprintf(w, "  %s\n\n", shortText(prop.Description()))
		}
	}
	fmt.Fprintln(w)
}

func describePropSummary(props []integra.Schema, indent string, showOptional bool) {
	w := describeTabWriter()
	defer w.Flush()
	for _, prop := range props {
		optional := ""
		if showOptional && !prop.Required() {
			optional = ", optional"
		}
		fmt.Fprintf(w, "%s%s\t%s%s\t%s\n", indent, prop.Name(), prop.Type(), optional, shortText(prop.Description()))
	}
	fmt.Fprintln(w)
}
