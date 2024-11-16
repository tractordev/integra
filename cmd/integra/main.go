package main

import (
	"context"
	"log"
	"os"

	"tractor.dev/toolkit-go/engine/cli"
)

var Version = "dev"

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	root := &cli.Command{
		Version: Version,
		Usage:   "integra",
		Long:    `integra is an integrations toolchain and utility`,
	}

	root.AddCommand(authCmd())
	root.AddCommand(callCmd())
	root.AddCommand(describeCmd())
	root.AddCommand(generateCmd())
	root.AddCommand(devCmd())

	if err := cli.Execute(context.Background(), root, os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}
