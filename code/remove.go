package main

import (
	"fmt"
	"path"
)

func removeContainer(containerName string) error {
	//check container status
	cinfo, err := getContainerInfoByName(containerName)
	if err != nil {
		return fmt.Errorf("getContainerInfoByName(%s) error : %v", containerName, err)
	}

	if cinfo.Status == RUNNING {
		return fmt.Errorf("you can't remove a running container %s, should stop it first", cinfo.Id)
	}

	//other two status can be removed
	deleteRootfs(path.Join(fmt.Sprintf(DefaultRootfsLocation, containerName), "newrootfs"), cinfo.Volume)
	deleteContainerInfo(containerName)
	fmt.Println(cinfo.Id)
	return nil
}
