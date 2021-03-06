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

package main

// TODO NOTICE Side-effects imports here
import (
	"log"
	"os"
	"sort"
	"time"

	"github.com/urfave/cli"

	"github.com/CS-SI/SafeScale/perform/cmds"

	_ "github.com/CS-SI/SafeScale/providers/cloudwatt"      // Imported to initialise provider cloudwatt
	_ "github.com/CS-SI/SafeScale/providers/flexibleengine" // Imported to initialise provider flexibleengine
	_ "github.com/CS-SI/SafeScale/providers/opentelekom"    // Imported to initialise provider opentelekom
	_ "github.com/CS-SI/SafeScale/providers/ovh"            // Imported to initialise provider ovh
	_ "github.com/CS-SI/SafeScale/providers/cloudferro"     // Imported to initialise provider cloudferro
)

func main() {

	cli.VersionFlag = cli.BoolFlag{
		Name:  "version, V",
		Usage: "print version",
	}

	app := cli.NewApp()
	app.Name = "perform"
	app.Usage = "perform COMMAND"
	app.Version = "0.0.1"
	app.Copyright = "(c) 2018 CS-SI"
	app.Compiled = time.Now()
	app.Authors = []cli.Author{
		cli.Author{
			Name:  "CS-SI",
			Email: "safescale@c-s.fr",
		},
	}
	app.EnableBashCompletion = true

	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name: "verbose, v",
		},
		cli.BoolFlag{
			Name: "debug, d",
		},
		cli.IntFlag{
			Name:  "port, p",
			Usage: "Bind to specified port `PORT`",
			Value: 50051,
		},
	}

	app.Commands = []cli.Command{
		cmds.ClusterListCommand,
		cmds.ClusterCreateCommand,
		cmds.ClusterInspectCommand,
		cmds.ClusterDeleteCommand,
		cmds.ClusterStartCommand,
		cmds.ClusterStopCommand,
		cmds.ClusterStateCommand,
		cmds.ClusterExpandCommand,
		cmds.ClusterShrinkCommand,
		cmds.ClusterCallCommand,
		cmds.ClusterInspectNodeCommand,
		cmds.ClusterDeleteNodeCommand,
		cmds.ClusterStartNodeCommand,
		cmds.ClusterStopNodeCommand,
		cmds.ClusterProbeNodeCommand,
		cmds.ClusterAddFeatureCommand,
		cmds.ClusterDeleteFeatureCommand,
		cmds.ClusterProbeFeatureCommand,
	}
	sort.Sort(cli.CommandsByName(app.Commands))

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
