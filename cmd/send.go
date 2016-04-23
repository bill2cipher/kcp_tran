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
	shost *string
	sname *string
	sport *uint32
)
// sendCmd represents the send command
var sendCmd = &cobra.Command{
	Use:   "send",
	Short: "send file from local to remote",
	Long: `send is used to send local file to remote side.`,
	Run: ExecuteSend,
}

func init() {
	RootCmd.AddCommand(sendCmd)

	shost = sendCmd.Flags().StringP("host", "H", "", "the host of remote side server")
	sport = sendCmd.Flags().Uint32P("port", "p", 0, "the port of remote side server")
	sname = sendCmd.Flags().StringP("name", "n", "", "name of the file to send")
}

func ExecuteSend(cmd *cobra.Command, args []string) {
	if *shost == "" || *sport == 0 || *sname == "" {
		cmd.Usage()
		return
	}
	raddr := fmt.Sprintf("%s:%d", *shost, *sport)
	transfer.SendFile(*sname, raddr)
}
