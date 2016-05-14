// Copyright 2016 CoreOS, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// etcd-play plays etcd.
//
//	Usage:
//	  etcd-play [command]
//
//	Available Commands:
//	  web         web plays etcd in web browser.
//
//	Use "etcd-play [command] --help" for more information about a command.
//
package main

import (
	"fmt"
	"os"

	"github.com/coreos/etcd-play/backend"
	"github.com/spf13/cobra"
)

var (
	rootCommand = &cobra.Command{
		Use:        "etcd-play",
		Short:      "etcd-play plays etcd.",
		SuggestFor: []string{"etcd-play", "etcdplay", "etcdp", "etcd-play", "etc-play"},
	}
)

func init() {
	cobra.EnablePrefixMatching = true
}

func init() {
	rootCommand.AddCommand(backend.WebCommand)
}

func main() {
	if err := rootCommand.Execute(); err != nil {
		fmt.Fprintln(os.Stdout, err)
		os.Exit(1)
	}
}
