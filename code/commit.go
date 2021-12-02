package main

import (
	"fmt"
	"os/exec"
	"path"
)

func CommitContainer(cid string, tin string) error {
	mntURL := path.Join("/root/mnt", cid, "mnt")
	tinURL := path.Join("/root", tin)
	if err := exec.Command("tar", "-cvf", tinURL, "-C", mntURL, ".").Run(); err != nil {
		return fmt.Errorf("tar -cvf %s %s error : %v", tinURL, mntURL, err)
	}
	return nil
}
