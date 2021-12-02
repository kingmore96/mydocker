package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strconv"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
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
	//get pgid to stop the group(which is the pid of mydocker run)
	pgid, err := syscall.Getpgid(ipid)
	if err != nil {
		return fmt.Errorf("syscall.Getpgid(%d) error : %v", ipid, err)
	}
	logrus.Debugf("container process pgid = %d", pgid)
	//firstly send SIGTERM to container
	err = syscall.Kill(-pgid, syscall.SIGTERM)
	if err != nil {
		return fmt.Errorf("kill -15 -%v error : %v", ipid, err)
	}

	//check if the process has terminated using ps -Afj | grep groupid
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

func updateContainerInfo(ci *ContainerInfo) error {
	ci.Status = STOP
	ci.Pid = ""
	jsonBytes, err := json.Marshal(ci)
	if err != nil {
		return fmt.Errorf("json.Marshal(ci) error : %v", err)
	}
	configPath := path.Join(fmt.Sprintf(DefaultConfigLocation, ci.Name), ConfigName)
	err = os.WriteFile(configPath, jsonBytes, 0622)
	if err != nil {
		return fmt.Errorf("os.WriterFile error : %v", err)
	}
	return nil
}

func groupTerminated(groupId string) (bool, error) {
	cmd := exec.Command("/bin/bash", "-c", "ps -Afj | grep "+groupId)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return false, fmt.Errorf("get ps command stdoutpipe error : %v", err)
	}

	if err := cmd.Start(); err != nil {
		return false, fmt.Errorf("cmd start error : %v", err)
	}

	defer cmd.Wait()

	//read until the EOF
	input := bufio.NewScanner(stdout)
	line := 0
	for input.Scan() {
		logrus.Debugf("%s", input.Text())
		// fmt.Println(input.Text())
		line += 1
	}
	// fmt.Println("====")
	logrus.Debugln("======")

	//logrus.Debugf("line = %d", line)
	if input.Err() != nil {
		return false, fmt.Errorf("stdout pipe scan error : %v", err)
	}

	if line == 2 {
		return true, nil
	}
	return false, nil
}

func getContainerInfoByName(containerName string) (*ContainerInfo, error) {
	configPath := path.Join(fmt.Sprintf(DefaultConfigLocation, containerName), ConfigName)
	jsonBytes, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("os.ReadFile(%s) error : %v", configPath, err)
	}

	var containerInfo ContainerInfo
	err = json.Unmarshal(jsonBytes, &containerInfo)
	if err != nil {
		return nil, fmt.Errorf("json.Unmarshal error : %v", err)
	}
	return &containerInfo, nil
}
