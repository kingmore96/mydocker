package main

import (
	"fmt"
	"os"
	"syscall"
)

func RunContainerInitProcess(cmd string, args []string) error {
	ctx := fmt.Sprintf("command=%q ,args=%v", cmd, args)
	//mount
	err := syscall.Mount("", "/", "", syscall.MS_REC|syscall.MS_PRIVATE, "") //等价于mount --make-rprivate /
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
	if err := syscall.Exec(cmd, argv, os.Environ()); err != nil {
		return fmt.Errorf("%s : execve error %v", ctx, err)
	}
	return nil
}
