package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
)

type ContainerInfo struct {
	Id         string `json:"id"`
	Name       string `json:"name"`
	Pid        string `json:"pid"`
	CreateTime string `json:"create_time"`
	Command    string `json:"command"`
	Status     string `json:"status"`
}

const (
	RUNNING string = "running"
	STOP    string = "stopped"
	EXIT    string = "exited"
)

var DefaultConfigLocation = "/var/run/mydocker/%s"
var ConfigName = "config.json"
var LogName = "container.log"

func Run(tty bool, comArr []string, volume string, name string) error {
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
		// Setpgid: true, //pgid = 0 so the container pgid is its pid
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

	// var container_id string
	//generate container id
	t := time.Now().UTC().Unix()
	buf := make([]byte, 5)
	rand.Read(buf)
	container_id := fmt.Sprintf("%x%x", t, buf[0:])
	if name == "" {
		name = container_id
	}

	if tty {
		container.Stdin = os.Stdin
		container.Stdout = os.Stdout
		container.Stderr = os.Stderr
	} else {
		//need to redirect to other files
		logDirPath := fmt.Sprintf(DefaultConfigLocation, name)
		if err := os.MkdirAll(logDirPath, 0777); err != nil {
			return fmt.Errorf("os.MkdirAll(%s) error : %v", logDirPath, err)
		}
		logPath := path.Join(logDirPath, LogName)
		f, err := os.Create(logPath)
		if err != nil {
			return fmt.Errorf("os.Create %s error %v", logPath, err)
		}
		//need to close?
		//defer f.Close()

		container.Stdout = f
		container.Stderr = f
		//print the container id to console
		fmt.Println(container_id)
	}

	if err := container.Start(); err != nil {
		return fmt.Errorf("container start failed")
	}

	//save the container info into /var/run/mydocker/container_name/config.json
	createTime := time.Now().Format("2006-01-02 15:04:05")
	command := fmt.Sprintf("%s", comArr)
	ci := &ContainerInfo{
		Name:       name,
		Id:         container_id,
		Pid:        strconv.Itoa(container.Process.Pid),
		CreateTime: createTime,
		Command:    command,
		Status:     RUNNING,
	}
	jsonBytes, err := json.Marshal(ci)
	if err != nil {
		return fmt.Errorf("json.Marshal error : %v", err)
	}
	confiDirPath := fmt.Sprintf(DefaultConfigLocation, name)
	if err := os.MkdirAll(confiDirPath, 0622); err != nil {
		return fmt.Errorf("mkdirAll(%s) error : %v", confiDirPath, err)
	}
	configPath := path.Join(confiDirPath, ConfigName)
	f, err := os.OpenFile(configPath, os.O_CREATE|os.O_RDWR, 0622)
	if err != nil {
		return fmt.Errorf("create configFile %s error :%v", configPath, err)
	}
	defer f.Close()
	jsonString := string(jsonBytes)
	if _, err := f.WriteString(jsonString); err != nil {
		return fmt.Errorf("write config json error : %v", err)
	}
	logrus.Debugf("write to config file success %s", jsonString)

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

	if tty {
		container.Wait()
		//delete all resource
		deleteRootfs(newRootfsURL, volume)
		//delete containerInfo
		deleteContainerInfo(name)
	}
	return nil
}

func deleteContainerInfo(containerName string) {
	dirURL := fmt.Sprintf(DefaultConfigLocation, containerName)
	if err := os.RemoveAll(dirURL); err != nil {
		logrus.Errorf("remove container info %s failed error :%v", dirURL, err)
	}
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

var RootLogURL = "/root/logs"

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
