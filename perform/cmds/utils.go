/*
 * Copyright 2018, CS Systemes d'Information, http://www.c-s.fr
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package cmds

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"

	//log "github.com/sirupsen/logrus"

	"github.com/CS-SI/SafeScale/deploy/cluster"
	clusterapi "github.com/CS-SI/SafeScale/deploy/cluster/api"
	"github.com/urfave/cli"

	"github.com/CS-SI/SafeScale/perform/enums/ExitCode"
)

var (
	clusterName     string
	clusterInstance clusterapi.Cluster
	nodeName        string
	serviceName     string
	featureName     string

	// RebrandingPrefix is used to store the optional prefix to use when calling external SafeScale commands
	RebrandingPrefix string
)

// RebrandCommand allows to prefix a command with cmds.RebrandingPrefix
// ie: with cmds.RebrandingPrefix == "safe "
//     "deploy ..." becomes "safe deploy ..."
//     with cmds.RebrandingPrefix == "my"
//     "perform ..." becomes "myperform ..."
func RebrandCommand(command string) string {
	return fmt.Sprintf("%s%s", RebrandingPrefix, command)
}

// func runCommand(cmdStr string) error {
// 	cmd := exec.Command("bash", "-c", cmdStr)
// 	err := cmd.Run()
// 	if err != nil {
// 		output, _, err := system.ExtractRetCode(err)
// 		if err != nil {
// 			msg := fmt.Sprintf("Failed to extract return code: %s", err.Error())
// 			return cli.NewExitError(msg, int(ExitCode.Run))
// 		}
// 		msg := fmt.Sprintf("Failed to execute command: %s", output)
// 		return cli.NewExitError(msg, int(ExitCode.Run))
// 	}
// 	return nil
// }

func runCommand(cmdStr string) error {
	cmd := exec.Command("bash", "-c", cmdStr)
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()
	stdoutScanner := bufio.NewScanner(stdout)
	stderrScanner := bufio.NewScanner(stderr)
	go func() {
		for stdoutScanner.Scan() {
			fmt.Println(stdoutScanner.Text())
		}
	}()
	go func() {
		for stderrScanner.Scan() {
			fmt.Fprintln(os.Stderr, stderrScanner.Text())
		}
	}()

	cmd.Start()
	err := cmd.Wait()
	if err != nil {
		return cli.NewExitError(err.Error(), int(ExitCode.Run))
	}
	return nil
}

func extractClusterArgument(c *cli.Context) error {
	var err error
	if !c.Command.HasName("list") {
		clusterName = c.Args().First()
		if clusterName == "" {
			return cli.NewExitError("Invalid argument CLUSTERNAME", int(ExitCode.InvalidArgument))
		}
		clusterInstance, err = cluster.Get(clusterName)
		if c.Command.HasName("create") && clusterInstance != nil {
			msg := fmt.Sprintf("Cluster '%s' already exists", clusterName)
			return cli.NewExitError(msg, int(ExitCode.Duplicate))
		}
		if err != nil {
			msg := fmt.Sprintf("Failed to get cluster '%s' information: %s\n", clusterName, err.Error())
			return cli.NewExitError(msg, int(ExitCode.RPC))
		}
		if clusterInstance == nil {
			msg := fmt.Sprintf("Cluster '%s' not found\n", clusterName)
			return cli.NewExitError(msg, int(ExitCode.NotFound))
		}
	}
	return nil
}

func extractNodeArgument(c *cli.Context) error {
	if !c.Command.HasName("list") {
		nodeName = c.Args().Get(1)
		if nodeName == "" {
			return cli.NewExitError("Invalid argument NODENAME", int(ExitCode.InvalidArgument))
		}
	}
	return nil
}

func extractFeatureArgument(c *cli.Context) error {
	if !c.Command.HasName("list") {
		featureName := c.Args().Get(1)
		if featureName == "" {
			return cli.NewExitError("Invalid argument FEATURENAME", int(ExitCode.InvalidArgument))
		}
	}
	return nil
}
