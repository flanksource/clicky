package main

import (
	"fmt"
	"os"

	"github.com/flanksource/clicky"
	"github.com/spf13/cobra"
)

type ModelOptions struct {
	Model  string `flag:"model" category:"Model selection" help:"Model to use"`
	Access string `flag:"access" category:"Model permissions" help:"Access contract"`
}

type RenderOptions struct {
	ModelOptions
	File string `flag:"file" help:"Document to render"`
}

func main() {
	root := &cobra.Command{Use: "grouped-help"}
	clicky.BindAllFlagsToCommand(root, "format")
	clicky.AddNamedCommand("render", root, RenderOptions{}, func(options RenderOptions) (string, error) {
		return options.File, nil
	})
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
