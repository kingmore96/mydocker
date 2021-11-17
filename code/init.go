package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/sirupsen/logrus"
)

func RunContainerInitProcess(cmd string, args []string) error {
	logrus.Debugf("current process is %d", syscall.Getpid())
	ap, err := exec.LookPath(cmd)
	if err != nil {
		return fmt.Errorf("exec.LookPath(%s) = %v", cmd, err)
	}
	ctx := fmt.Sprintf("command=%s ,args=%v", ap, args)
	//mount
	err = syscall.Mount("", "/", "", syscall.MS_REC|syscall.MS_PRIVATE, "") //等价于mount --make-rprivate /
	if err != nil {
		return fmt.Errorf("%s : mount --make-rprivate error %v", ctx, err)
	}
	defaultMountFlags := syscall.MS_NOEXEC | syscall.MS_NOSUID | syscall.MS_NODEV
	err = syscall.Mount("proc", "/proc", "proc", uintptr(defaultMountFlags), "")
	if err != nil {
		return fmt.Errorf("%s : mount -t proc proc /proc error %v", ctx, err)
	}
	//syscall.Exec
	argv := []string{cmd}
	argv = append(argv, args...)
	//logrus.Debugf("os.Environ is %v", os.Environ())
	if err := syscall.Exec(ap, argv, os.Environ()); err != nil {
		return fmt.Errorf("%s : execve error %v", ctx, err)
	}
	return nil
}