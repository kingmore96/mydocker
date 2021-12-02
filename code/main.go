package main

import (
	"fmt"
	"os"
	"strings"

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
		commitCommand,
		logCommand,
		listCommand,
		stopCommand,
		removeCommand,
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
			Usage: "run container in the foreground mode if false run it in background mode",
		},
		cli.StringFlag{
			Name:  "name",
			Usage: "container name",
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
		cli.StringFlag{
			Name:  "v",
			Usage: "volume",
		},
	},
	Action: func(c *cli.Context) error {
		tty := c.Bool("ti")
		log.Debugf("ti is %t", tty)

		//get command and tty
		if len(c.Args()) == 0 {
			return fmt.Errorf("missing container command")
		}
		var comArr []string
		for _, arg := range c.Args() {
			comArr = append(comArr, arg)
		}
		//get cgroups flags
		ResourceConfigMap["memory"].Value = c.String("m")
		ResourceConfigMap["cpuset"].Value = c.String("cpuset")
		ResourceConfigMap["cpushare"].Value = c.String("cpushare")
		//get volume config
		volume := c.String("v")
		name := c.String("name")
		//Run it
		return Run(tty, comArr, volume, name)
	},
}

type ResourceConfig struct {
	Value    string
	RootPath string
	FileName string
}

var ResourceConfigMap = map[string]*ResourceConfig{
	"memory": {
		RootPath: "/sys/fs/cgroup/memory",
		FileName: "memory.limit_in_bytes",
	},
	"cpuset": {
		RootPath: "/sys/fs/cgroup/cpuset",
		FileName: "cpuset.cpus",
	},
	"cpushare": {
		RootPath: "/sys/fs/cgroup/cpu",
		FileName: "cpu.shares",
	},
}

var initCommand = cli.Command{
	Name:   "init",
	Usage:  "init the container to run user's process",
	Hidden: true,
	Action: func(c *cli.Context) error {
		log.Debugln("start to init")
		return RunContainerInitProcess(c.Args()[0], c.Args()[1:])
	},
}

var commitCommand = cli.Command{
	Name:  "commit",
	Usage: "commit the container into image tar",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "cid",
			Usage: "container id",
		},
		cli.StringFlag{
			Name:  "tin",
			Usage: "tar image name",
		},
	},
	Action: func(c *cli.Context) error {
		log.Debugln("start to commit")
		//check params
		cid := c.String("cid")
		if cid == "" {
			return fmt.Errorf("missing container id")
		}

		tin := c.String("tin")
		if tin == "" {
			tin = cid + ".tar"
		} else {
			if !strings.HasSuffix(tin, ".tar") {
				tin += ".tar"
			}
		}
		return CommitContainer(cid, tin)
	},
}

var logCommand = cli.Command{
	Name:  "logs",
	Usage: "print the specified container log",
	Action: func(c *cli.Context) error {
		if len(c.Args()) == 0 {
			return fmt.Errorf("missing container id")
		}
		cname := c.Args().Get(0)
		return LogContainer(cname)
	},
}

var listCommand = cli.Command{
	Name:  "ps",
	Usage: "list all container",
	Action: func(c *cli.Context) error {
		return ListContainer()
	},
}

var stopCommand = cli.Command{
	Name:  "stop",
	Usage: "stop the container process and all the children process it forked",
	Action: func(c *cli.Context) error {
		//check if the container name params is passed
		if len(c.Args()) == 0 {
			return fmt.Errorf("missing container name")
		}
		cn := c.Args().Get(0)
		return stopContainer(cn)
	},
}

var removeCommand = cli.Command{
	Name:  "rm",
	Usage: "remove the stopped container",
	Action: func(c *cli.Context) error {
		if len(c.Args()) == 0 {
			return fmt.Errorf("missing container name")
		}
		cn := c.Args().Get(0)
		return removeContainer(cn)
	},
}
