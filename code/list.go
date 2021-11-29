package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"text/tabwriter"
)

func ListContainer() error {
	dirPath := fmt.Sprintf(DefaultConfigLocation, "")
	dirs, err := os.ReadDir(dirPath)
	if err != nil {
		return fmt.Errorf("os.ReadDir(%s) error : %v", dirPath, err)
	}

	var containerInfos []ContainerInfo
	for _, dir := range dirs {
		configFilePath := path.Join(dirPath, dir.Name(), ConfigName)
		f, err := os.Open(configFilePath)
		if err != nil {
			return fmt.Errorf("os.Open(%s) error : %v", configFilePath, err)
		}
		jsonBytes, err := ioutil.ReadAll(f)
		if err != nil {
			return fmt.Errorf("ioutil.RealAll() error : %v", err)
		}
		var containerInfo ContainerInfo
		if err := json.Unmarshal(jsonBytes, &containerInfo); err != nil {
			return fmt.Errorf("unmarshal failed %v", err)
		}
		containerInfos = append(containerInfos, containerInfo)
	}

	//print to terminal
	w := tabwriter.NewWriter(os.Stdout, 12, 1, 3, ' ', 0)
	fmt.Fprint(w, "ID\tNAME\tPID\tSTATUS\tCOMMAND\tCREATETIME\n")
	for _, c := range containerInfos {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			c.Id,
			c.Name,
			c.Pid,
			c.Status,
			c.Command,
			c.CreateTime,
		)
	}
	if err := w.Flush(); err != nil {
		return fmt.Errorf("tabwriter flush error : %v", err)
	}
	return nil
}
