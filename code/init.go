package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"syscall"

	"github.com/sirupsen/logrus"
)

func RunContainerInitProcess(cmd string, args []string) error {
	logrus.Debugf("current process is %d", syscall.Getpid())
	//get r pipe
	r := os.NewFile(uintptr(3), "pipe")

	//look path
	ap, err := exec.LookPath(cmd)
	if err != nil {
		return fmt.Errorf("exec.LookPath(%s) = %v", cmd, err)
	}
	ctx := fmt.Sprintf("command=%s ,args=%v", ap, args)

	//mountpoint private
	err = syscall.Mount("", "/", "", syscall.MS_REC|syscall.MS_PRIVATE, "") //等价于mount --make-rprivate /
	if err != nil {
		return fmt.Errorf("%s : mount --make-rprivate error %v", ctx, err)
	}

	//start pivot_root
	newRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("os.Getwd error : %v", err)
	}

	//mkputold dir
	putold := path.Join(newRoot, ".putold")
	err = os.Mkdir(putold, 0755)
	if err != nil {
		return fmt.Errorf("mk putold error : %v", err)
	}

	//bind to itself to make newRoot as an mountpoint
	err = syscall.Mount(newRoot, newRoot, "bind", syscall.MS_BIND|syscall.MS_REC, "")
	if err != nil {
		return fmt.Errorf("bind newRoot error : %v", err)
	}

	//do PivotRoot
	err = syscall.PivotRoot(newRoot, putold)
	if err != nil {
		return fmt.Errorf("PivotRoot error: %v", err)
	}

	//ch and umount putold
	err = os.Chdir("/")
	if err != nil {
		return fmt.Errorf("chdir error : %v", err)
	}

	putold = path.Join("/", ".putold")
	err = syscall.Unmount(putold, syscall.MNT_DETACH)
	if err != nil {
		return fmt.Errorf("unmount error : %v", err)
	}

	//rm the putold dir
	err = os.Remove(putold)
	if err != nil {
		return fmt.Errorf("remove putold error : %v", err)
	}

	defaultMountFlags := syscall.MS_NOEXEC | syscall.MS_NOSUID | syscall.MS_NODEV
	err = syscall.Mount("proc", "/proc", "proc", uintptr(defaultMountFlags), "")
	if err != nil {
		return fmt.Errorf("%s : mount -t proc proc /proc error %v", ctx, err)
	}

	//mount /dev
	err = syscall.Mount("tmpfs", "/dev", "tmpfs", syscall.MS_NOSUID|syscall.MS_STRICTATIME, "")
	if err != nil {
		return fmt.Errorf("%s : mount -t tmpfs tmpfs /dev error %v", ctx, err)
	}

	//syscall.Exec
	argv := []string{cmd}
	argv = append(argv, args...)
	//logrus.Debugf("os.Environ is %v", os.Environ())

	signal, err := ioutil.ReadAll(r)
	if err != nil {
		return fmt.Errorf("%s : child process read signal error %v", ctx, err)
	}
	logrus.Debugf("%s : read from parent process %v", ctx, string(signal))

	if err := syscall.Exec(ap, argv, os.Environ()); err != nil {
		return fmt.Errorf("%s : execve error %v", ctx, err)
	}
	return nil
}
