package main

import (
	"fmt"

	"tractor.dev/toolkit-go/engine/cli"
)

func generateCmd() *cli.Command {
	cmd := &cli.Command{
		Usage: "generate",
		Short: "generate asset from service schema",
		// Args:  cli.MinArgs(1),
		Run: func(ctx *cli.Context, args []string) {
			fmt.Println("TODO")

		},
	}
	return cmd
}
