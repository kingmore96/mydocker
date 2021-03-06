package main

import (
	"fmt"
	"io"
	"os"
	"path"
)

func LogContainer(cname string) (err error) {
	logPath := path.Join(fmt.Sprintf(DefaultConfigLocation, cname), LogName)
	f, err := os.Open(logPath)
	if err != nil {
		return fmt.Errorf("os.Open(%s) error : %v", logPath, err)
	}
	defer func() {
		if er := f.Close(); er != nil {
			err = fmt.Errorf("file close error : %v", er)
		}
	}()

	bs, err := io.ReadAll(f)
	if err != nil {
		return fmt.Errorf("io.ReadAll(file) error : %v", err)
	}
	fmt.Println(string(bs))
	return
}
