package main

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"syscall"

	"github.com/sirupsen/logrus"
)

func Run(tty bool, comArr []string, volume string) error {
	logrus.Debugf("comArr is %v", comArr)

	r, w, err := os.Pipe()
	defer func() {
		r.Close()
		w.Close()
	}()

	if err != nil {
		return fmt.Errorf("pipe failed %v", err)
	}

	var ca []string
	ca = append(ca, "init")
	ca = append(ca, comArr...)
	container := exec.Command("/proc/self/exe", ca...)
	//send the r to container
	container.ExtraFiles = []*os.File{r}

	//change the container pwd to /root/busybox
	// container.Dir = "/root/busybox"
	//4.2 change the container pwd to /root/mnt which is the aufs file system
	rootURL := "/root/mnt"
	if err := os.Mkdir(rootURL, 0755); err != nil {
		if !os.IsExist(err) {
			return fmt.Errorf("mkdir rootURL %s error %v", rootURL, err)
		}
	}

	newRootfsURL := path.Join("/root/mnt", strconv.Itoa(syscall.Getpid()))
	//make /mnt
	if err := buildNewRootfs(newRootfsURL, volume); err != nil {
		return fmt.Errorf("build new rootfs failed %v", err)
	}
	container.Dir = path.Join(newRootfsURL, "mnt")

	logrus.Debugf("ca is %v", ca)
	container.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS |
			syscall.CLONE_NEWIPC |
			syscall.CLONE_NEWPID |
			syscall.CLONE_NEWNS |
			syscall.CLONE_NEWUSER |
			syscall.CLONE_NEWNET,
		UidMappings: []syscall.SysProcIDMap{
			{
				ContainerID: 0,
				HostID:      syscall.Getuid(),
				Size:        1,
			},
		},
		GidMappings: []syscall.SysProcIDMap{
			{
				ContainerID: 0,
				HostID:      syscall.Getgid(),
				Size:        1,
			},
		},
	}

	if tty {
		container.Stdin = os.Stdin
		container.Stdout = os.Stdout
		container.Stderr = os.Stderr
	}

	if err := container.Start(); err != nil {
		return fmt.Errorf("container start failed")
	}

	//cgroups code
	logrus.Infof("new process id is %s", strconv.Itoa(container.Process.Pid))

	context := fmt.Sprintf("tty=%t,cmd=%v", tty, comArr)
	for k, v := range ResourceConfigMap {
		if v.Value != "" {
			configString := fmt.Sprintf("k=%v,v=%v", k, v)
			//mkdir
			dirPath := path.Join(v.RootPath, strconv.Itoa(container.Process.Pid))
			if err := os.Mkdir(dirPath, 0755); err != nil {
				err = fmt.Errorf("%s %s mkdir error %v", context, configString, err)
				return err
			}
			//clean code
			defer os.RemoveAll(dirPath)
			//write cgroup limit
			limitPath := path.Join(dirPath, v.FileName)
			if err := os.WriteFile(limitPath, []byte(v.Value), 0644); err != nil {
				err = fmt.Errorf("%s %s write to limit error %v", context, configString, err)
				return err
			}
			//cpuset need to fill in the cpu.mems
			if v.FileName == "cpuset.cpus" {
				cpumemPath := path.Join(dirPath, "cpuset.mems")
				if err := os.WriteFile(cpumemPath, []byte("0"), 0644); err != nil {
					err = fmt.Errorf("%s %s write to limit error %v", context, configString, err)
					return err
				}
			}
			//write pid
			taskPath := path.Join(dirPath, "tasks")
			if err := os.WriteFile(taskPath, []byte(strconv.Itoa(container.Process.Pid)), 0644); err != nil {
				err = fmt.Errorf("%s %s write to tasks error %v", context, configString, err)
				return err
			}
		}
	}
	//finish cgroup and send the signal to child process
	_, err = w.WriteString("o")
	logrus.Debug("write signal finish")
	if err != nil {
		return fmt.Errorf("parent process write finish signal false %v", err)
	}
	w.Close()
	container.Wait()
	//delete all resource
	deleteRootfs(newRootfsURL, volume)
	return nil
}

func deleteRootfs(newRootfsURL string, volume string) {
	mntURL := path.Join(newRootfsURL, "mnt")
	//umount container volume : need to check param, so we can move the check step to firstone function
	if volume != "" {
		vs := strings.Split(volume, ":")
		if len(vs) == 2 && vs[0] != "" && vs[1] != "" && strings.HasPrefix(vs[0], "/") && strings.HasPrefix(vs[1], "/") {
			//umount container volume before remove the dir
			cv := path.Join(mntURL, vs[1])
			if err := syscall.Unmount(cv, syscall.MNT_DETACH); err != nil {
				logrus.Errorf("umount container volume %s failed : %v", cv, err)
			}
		}
	}

	//umount newRootfsURL/mnt
	if err := syscall.Unmount(mntURL, syscall.MNT_DETACH); err != nil {
		logrus.Errorf("umount mntURL %s failed : %v ", mntURL, err)
	}

	//rm -r newRootfsURL
	if err := os.RemoveAll(newRootfsURL); err != nil {
		logrus.Errorf("rm -r %s failed : %v", newRootfsURL, err)
	}
}

func buildNewRootfs(newRootURL string, volume string) error {
	//mkdir newRoolURL
	_, err := os.Stat(newRootURL)
	if err == nil {
		//rm the dir
		os.RemoveAll(newRootURL)
		logrus.Debugf("remove newRootURL dir %s", newRootURL)
	}

	err = os.Mkdir(newRootURL, 0755)
	if err != nil {
		return fmt.Errorf("mkdir newRootURL %s error %v", newRootURL, err)
	}

	//mkdir readonly layer
	busyboxURL := path.Join(newRootURL, "busybox")
	err = os.Mkdir(busyboxURL, 0755)
	if err != nil {
		return fmt.Errorf("mkdir newRootURL %s/busybox error %v", newRootURL, err)
	}

	//tar -xvf /root/busybox.tar -C newRootURL/busybox/
	if err := exec.Command("tar", "-xvf", "/root/busybox.tar", "-C", busyboxURL).Run(); err != nil {
		return fmt.Errorf("untar /root/busybox failed %v", err)
	}

	//mkdir write layer
	writeLayer := path.Join(newRootURL, "writeLayer")
	err = os.Mkdir(writeLayer, 0755)
	if err != nil {
		return fmt.Errorf("mkdir newRootURL %s/writeLayer error %v", newRootURL, err)
	}

	//mount
	mntURL := path.Join(newRootURL, "mnt")
	err = os.Mkdir(mntURL, 0755)
	if err != nil {
		return fmt.Errorf("mkdir newRootURL %s/mnt error %v", newRootURL, err)
	}
	if err := exec.Command("mount", "-t", "aufs", "-o", "dirs="+writeLayer+":"+busyboxURL, "none", mntURL).Run(); err != nil {
		return fmt.Errorf("mount error %v", err)
	}

	//volume code
	if volume != "" {
		logrus.Debugf("volume section start : %s", volume)
		// eg: /root/volume:/container/volume
		//verify volume
		vs := strings.Split(volume, ":")
		if len(vs) == 2 && vs[0] != "" && vs[1] != "" && strings.HasPrefix(vs[0], "/") && strings.HasPrefix(vs[1], "/") {
			//mkdir /root/volume
			if err := os.MkdirAll(vs[0], 0755); err != nil {
				return fmt.Errorf("mkdirAll error : %s", vs[0])
			}
			//mkdir /container/volume
			containerVolumePath := path.Join(mntURL, vs[1])
			if err := os.MkdirAll(containerVolumePath, 0755); err != nil {
				return fmt.Errorf("mkdirAll error : %s", containerVolumePath)
			}
			//mount bind
			if err := syscall.Mount(vs[0], containerVolumePath, "bind", syscall.MS_BIND, ""); err != nil {
				return fmt.Errorf("mount --bind error :%s %s", vs[0], containerVolumePath)
			}
		} else {
			return fmt.Errorf("wrong volume %s", volume)
		}
	}
	return nil
}
