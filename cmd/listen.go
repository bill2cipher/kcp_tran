// Copyright © 2016 NAME HERE <EMAIL ADDRESS>
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

package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/jellybean4/kcp_tran/transfer"
)

var (
	lport *uint32
	ldest *string
)

// listenCmd represents the listen command
var listenCmd = &cobra.Command{
	Use:   "listen",
	Short: "listen on a specifiled port for client.",
	Long: `listen on a specifiled port for client.`,
	Run: ExecuteListen,
}

func init() {
	RootCmd.AddCommand(listenCmd)
	lport = listenCmd.Flags().Uint32P("port", "p", 0, "the port of server to listen to")
	ldest = listenCmd.Flags().StringP("dest", "d", "./", "the destination directory name to save file")
}

func ExecuteListen(cmd *cobra.Command, args []string) {
	if *lport == 0 {
		cmd.Usage()
		return
	}
	host := fmt.Sprintf("0.0.0.0:%d", *lport)
	transfer.Serve(host, *ldest)
}
