package main

import (
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

const usage = `mydocker is a simple container runtime implementation.
			   The purpose of this project is to learn how docker works and how to write a docker by ourselves
			   Enjoy it, just for fun.`

func main() {
	app := cli.NewApp()
	app.Name = "mydocker"
	app.Usage = usage

	app.Commands = []cli.Command{
		runCommand,
		initCommand,
	}

	app.Before = func(c *cli.Context) error {
		log.SetFormatter(&log.JSONFormatter{})
		log.SetOutput(os.Stdout)
		log.SetLevel(log.DebugLevel)
		return nil
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

var runCommand = cli.Command{
	Name: "run",
	Usage: `create a container with namespace and cgroup limits 
	mydocker run -ti [command]`,
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "ti",
			Usage: "enable tty",
		},
		cli.StringFlag{
			Name:  "m",
			Usage: "limit memory",
		},
		cli.StringFlag{
			Name:  "cpuset",
			Usage: "limit cpuset",
		},
		cli.StringFlag{
			Name:  "cpushare",
			Usage: "limit cpushare",
		},
	},
	Action: func(c *cli.Context) error {
		tty := c.Bool("ti")
		log.Debugf("ti is %t", tty)
		//get command and tty
		if len(c.Args()) == 0 {
			return fmt.Errorf("missing container command")
		}
		cmd := c.Args().Get(0)
		//get cgroups flags
		// rc := &ResourceConfig{
		// 	MemoryLimit: c.String("m"),
		// 	Cpuset:      c.String("cpuset"),
		// 	Cpushare:    c.String("cpushare"),
		// }
		ResourceConfigMap["memory"].Value = c.String("m")
		ResourceConfigMap["cpuset"].Value = c.String("cpuset")
		ResourceConfigMap["cpushare"].Value = c.String("cpushare")
		//Run it
		Run(tty, cmd)
		return nil
	},
}

type ResourceConfig struct {
	Value    string
	RootPath string
}

var ResourceConfigMap = map[string]*ResourceConfig{
	"memory": &ResourceConfig{
		RootPath: "/sys/fs/cgroup/memory",
	},
	"cpuset": &ResourceConfig{
		RootPath: "/sys/fs/cgroup/cpuset",
	},
	"cpushare": &ResourceConfig{
		RootPath: "/sys/fs/cgroup/cpu",
	},
}

var initCommand = cli.Command{
	Name:   "init",
	Usage:  "init the container to run user's process",
	Hidden: true,
	Action: func(c *cli.Context) error {
		log.Debugln("start to init")
		return RunContainerInitProcess(c.Args()[0], nil)
	},
}
