package main

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"strconv"
	"syscall"

	"github.com/sirupsen/logrus"
)

func Run(tty bool, comArr []string) error {
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
	return nil
}
