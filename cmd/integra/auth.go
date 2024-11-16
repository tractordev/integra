package main

import (
	"fmt"

	"tractor.dev/toolkit-go/engine/cli"
)

func authCmd() *cli.Command {
	cmd := &cli.Command{
		Usage: "auth",
		Short: "authenticate with a service",
		// Args:  cli.MinArgs(1),
		Run: func(ctx *cli.Context, args []string) {
			fmt.Println("TODO")

		},
	}
	return cmd
}
