package main

import (
	"fmt"
	"os"

	"github.com/coreos/etcd-play/terminal"
	"github.com/coreos/etcd-play/web"
	"github.com/spf13/cobra"
)

var (
	rootCommand = &cobra.Command{
		Use:        "etcd-play",
		Short:      "etcd-play runs etcd.",
		SuggestFor: []string{"etcd-play", "etcd-play", "etc-play"},
	}
)

func init() {
	cobra.EnablePrefixMatching = true
}

func init() {
	rootCommand.AddCommand(terminal.Command)
	rootCommand.AddCommand(web.Command)
}

func main() {
	if err := rootCommand.Execute(); err != nil {
		fmt.Fprintln(os.Stdout, err)
		os.Exit(1)
	}
}
