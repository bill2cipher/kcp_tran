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
	rhost *string
	rname *string
	rdest *string
	rport *uint32
)

// receiveCmd represents the receive command
var receiveCmd = &cobra.Command{
	Use:   "receive",
	Short: "receive file from remote side to local",
	Long: `receive file from remote side to local`,
	Run: ExecuteReceive,
}

func init() {
	RootCmd.AddCommand(receiveCmd)
	rhost = receiveCmd.Flags().StringP("host", "H", "", "the host of remote side server")
	rport = receiveCmd.Flags().Uint32P("port", "p", 0, "the port of remote side server")
	rdest = receiveCmd.Flags().StringP("dest", "d", "./", "the destination directory name")
	rname = receiveCmd.Flags().StringP("name", "n", "", "name of the file to send")
}

func ExecuteReceive(cmd *cobra.Command, args []string) {
	if *rhost == "" || *rport == 0 || *rname == "" {
		cmd.Usage()
		return
	}
	raddr := fmt.Sprintf("%s:%d", *rhost, *rport)
	transfer.RecvFile(*rname, *rdest, raddr)
}
