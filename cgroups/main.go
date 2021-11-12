package main

import (
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"strconv"
	"syscall"
)

func CpuStress() {
	count := 0
	for i := 0; i < 1024*1024*1024; i++ {
		count++
	}
}

const cgMemPath = "/sys/fs/cgroup/memory"

func MockCgroups() {
	if os.Args[0] == "/proc/self/exe" {
		log.Printf("current process is %d", syscall.Getpid())
		cmd := exec.Command("stress", "--vm", "2", "--vm-keep", "--vm-bytes", "300m", "--quiet")
		cmd.SysProcAttr = &syscall.SysProcAttr{}
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		//the container process block in the stress
		if err := cmd.Run(); err != nil {
			log.Println(syscall.Getpid(), cmd.ProcessState, err)
		}
		log.Println(syscall.Getpid(), "Done")
	} else {
		cmd := exec.Command("/proc/self/exe")
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Cloneflags: syscall.CLONE_NEWUTS |
				syscall.CLONE_NEWIPC |
				syscall.CLONE_NEWPID |
				syscall.CLONE_NEWNS |
				syscall.CLONE_NEWUSER |
				syscall.CLONE_NEWNET,
			UidMappings: []syscall.SysProcIDMap{
				{
					ContainerID: 10086,
					HostID:      syscall.Getuid(),
					Size:        1,
				},
			},
			GidMappings: []syscall.SysProcIDMap{
				{
					ContainerID: 10086,
					HostID:      syscall.Getgid(),
					Size:        1,
				},
			},
		}

		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Start(); err != nil {
			log.Fatal(err)
		}

		log.Printf("new process id is %d", cmd.Process.Pid)

		//mkdir
		dirPath := path.Join(cgMemPath, strconv.Itoa(cmd.Process.Pid))
		os.Mkdir(dirPath, 0755)

		//write pid into tasks
		taskPath := path.Join(dirPath, "tasks")
		err := ioutil.WriteFile(taskPath, []byte(strconv.Itoa(cmd.Process.Pid)), 0644)
		if err != nil {
			log.Println(err)
		}

		//write restrictions
		memlimitPath := path.Join(dirPath, "memory.limit_in_bytes")
		err = ioutil.WriteFile(memlimitPath, []byte("100m"), 0644)
		if err != nil {
			log.Println(err)
		}

		//main goroutine wait
		if err := cmd.Wait(); err != nil {
			log.Println(syscall.Getpid(), cmd.ProcessState, err)
		}
		log.Println(syscall.Getpid(), "Done")
	}
}

func main() {
	// CpuStress()
	MockCgroups()
}
