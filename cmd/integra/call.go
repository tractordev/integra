package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/progrium/clon-go"
	"tractor.dev/integra"
	"tractor.dev/toolkit-go/engine/cli"
)

func callCmd() *cli.Command {
	cmd := &cli.Command{
		Usage: "call <selector>",
		Short: "perform an operation on a service resource",
		Args:  cli.MinArgs(1),
		Run: func(ctx *cli.Context, args []string) {
			selector, version := integra.SplitSelectorVersion(args[0])
			sel := strings.Split(selector, ".")

			s, err := integra.LoadService(sel[0], version)
			if err != nil {
				log.Fatal(err)
			}

			if len(sel) == 1 {
				fmt.Printf("missing resource in selector. use `integra describe %s` to list resources.\n", sel[0])
				os.Exit(1)
				return
			}

			r, err := s.Resource(sel[1])
			if err != nil {
				log.Fatal(err)
			}

			if len(sel) == 2 {
				fmt.Printf("missing operation in selector. use `integra describe %s` to list operations.\n", args[0])
				os.Exit(1)
				return
			}

			op, err := r.Operation(sel[2])
			if err != nil {
				log.Fatal(err)
			}

			required := requiredParams(op)
			params := map[string]any{}

			if len(args) > 1 {
				parsed, err := clon.Parse(args[1:])
				if err != nil {
					log.Fatal(err)
				}
				params = parsed.(map[string]any)
			}

			for _, name := range required {
				_, ok := params[name]
				if !ok {
					log.Fatal("missing all required parameters:", required)
				}
			}

			// TODO: validation here?

			// var body io.Reader
			// if slices.Contains([]string{"create", "set", "update"}, op.Name()) {
			// 	b, err := json.Marshal(params)
			// 	if err != nil {
			// 		log.Fatal(err)
			// 	}
			// 	body = bytes.NewBuffer(b)
			// }

			// todo: need to populate url since implementing new model
			req, err := http.NewRequest(op.Method(), op.URL(), nil)
			if err != nil {
				log.Fatal(err)
			}

			token := integra.ServiceToken(s.Name())
			if token != "" {
				req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				log.Fatal(err)
			}
			defer resp.Body.Close()

			if resp.StatusCode > 299 {
				fmt.Println(resp.Status)
				io.Copy(os.Stdout, resp.Body)
				fmt.Println()
				return
			}

			var reply any
			dec := json.NewDecoder(resp.Body)
			if err := dec.Decode(&reply); err != nil {
				log.Fatal(err)
			}

			b, err := json.MarshalIndent(reply, "", "  ")
			if err != nil {
				log.Fatal(err)
			}
			fmt.Println(string(b))

		},
	}
	return cmd
}

func requiredParams(op integra.Operation) (required []string) {
	// from params
	for _, p := range op.Parameters() {
		if p.Required() {
			required = append(required, p.Name())
		}
	}
	return
}
