package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strconv"
	"syscall"
	"time"
)

func stopContainer(containerName string) error {
	//find the pid of container named of containerName
	containerInfo, err := getContainerInfoByName(containerName)
	if err != nil {
		return fmt.Errorf("getContainerInfoByName(%s) error : %v", containerName, err)
	}
	ipid, err := strconv.Atoi(containerInfo.Pid)
	if err != nil {
		return fmt.Errorf("strconv.Atoi(%s) error : %v", containerInfo.Pid, err)
	}
	//firstly send SIGTERM to container
	err = syscall.Kill(-ipid, syscall.SIGTERM)
	if err != nil {
		return fmt.Errorf("kill -15 -%v error : %v", ipid, err)
	}

	//check if the process has terminated using ps -g -ipid
	for i := 0; i < 10; i++ {
		success, err := groupTerminated(containerInfo.Pid)
		if err != nil {
			return fmt.Errorf("groupTerminated error : %v", err)
		}
		//return immediately
		if success {
			goto update_info
		}
		time.Sleep(time.Second)
	}

	//when timeup but not being terminated, we will use SIGKILL to real kill the group
	err = syscall.Kill(-ipid, syscall.SIGKILL)
	if err != nil {
		return fmt.Errorf("kill -9 -%v error : %v", ipid, err)
	}

	//set the container pid to "" and update status to STOP
update_info:
	err = updateContainerInfo(containerInfo)
	if err != nil {
		return fmt.Errorf("updateContainerInfo(%s) error : %v", containerInfo.Name, err)
	}
	return nil
}

func updateContainerInfo(ci ContainerInfo) error {
	ci.Status = STOP
	ci.Pid = ""
	jsonBytes, err := json.Marshal(ci)
	if err != nil {
		return fmt.Errorf("json.Marshal(ci) error : %v", err)
	}
	configPath := path.Join(DefaultConfigLocation, ci.Name, ConfigName)
	err = os.WriteFile(configPath, jsonBytes, 0622)
	if err != nil {
		return fmt.Errorf("os.WriterFile error : %v", err)
	}
	return nil
}

func groupTerminated(groupId string) (bool, error) {

}

func getContainerInfoByName(containerName string) (ContainerInfo, error) {

}
