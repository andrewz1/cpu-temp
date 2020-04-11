package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"time"
)

const (
	maxDiff  = 5
	checkInt = maxDiff * time.Second
	sysPath  = "/sys/class/hwmon/hwmon*"
	namePath = "/name"
	cpuName  = "coretemp"
	ctrlName = "nct"
)

var (
	cpus []string
	ctrl string
)

func main() {
	var (
		err      error
		nameData []byte
	)
	paths, _ := filepath.Glob(sysPath)
	for _, p := range paths {
		if nameData, err = ioutil.ReadFile(p + namePath); err != nil {
			continue
		}
		if bytes.HasPrefix(nameData, []byte(cpuName)) {
			cpus = append(cpus, p)
		} else if bytes.HasPrefix(nameData, []byte(ctrlName)) {
			ctrl = p
		}
	}
	fmt.Println(cpus, ctrl)
}
