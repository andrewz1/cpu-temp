package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

const (
	maxDiff  = 5
	checkInt = maxDiff * time.Second
	sysPath  = "/sys/class/hwmon/hwmon*"
	namePath = "/name"
	cpuName  = "coretemp"
	ctrlName = "nct"

	cpuTempPath = "/temp1_input"
	cpuTempMin  = 30000
	cpuTempMax  = 70000

	pwmMin = 0
	pwmMax = 255
)

var (
	fans = []string{
		"/pwm1",
		"/pwm2",
	}
	cpus    []string
	ctrl    string
	pwmLast int
)

func readInt(path string) (rv int, err error) {
	var f *os.File
	if f, err = os.Open(path); err != nil {
		return
	}
	defer f.Close()
	var t, n int
	if n, err = fmt.Fscanf(f, "%d", &t); err != nil {
		return
	}
	if n != 1 {
		err = errors.New("invalid file content")
		return
	}
	rv = t
	return
}

func writeInt(path string, v int) (err error) {
	var f *os.File
	if f, err = os.OpenFile(path, os.O_WRONLY, 0644); err != nil {
		return
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "%d", v)
	return
}

func getCpuTemp() (rv int, err error) {
	var (
		tmp int
		sum float64
	)
	for _, p := range cpus {
		if tmp, err = readInt(p + cpuTempPath); err != nil {
			return
		}
		sum += float64(tmp)
	}
	rv = int(sum / float64(len(cpus)))
	return
}

func setFanPwm(pwm int) (err error) {
	for _, p := range fans {
		if err = writeInt(ctrl+p, pwm); err != nil {
			return
		}
	}
	return
}

func calcFanPwm() (rv int) {
	var (
		err  error
		temp int
	)
	if temp, err = getCpuTemp(); err != nil {
		rv = pwmMax
		return
	}
	switch {
	case temp <= cpuTempMin:
		rv = pwmMin
		return
	case temp >= cpuTempMax:
		rv = pwmMax
		return
	}
	tPrc := float64(temp-cpuTempMin) / float64(cpuTempMax-cpuTempMin)
	pwm := float64(pwmMax-pwmMin) * tPrc
	rv = pwmMin + int(pwm)
	return
}

func calcFanPwmWithDiff() (rv int) {
	calc := calcFanPwm()
	switch {
	case calc == pwmLast:
		rv = calc
	case calc > pwmLast:
		diff := calc - pwmLast
		if diff > maxDiff {
			rv = pwmLast + maxDiff
		} else {
			rv = calc
		}
	case calc < pwmLast:
		diff := pwmLast - calc
		if diff > maxDiff {
			rv = pwmLast - maxDiff
		} else {
			rv = calc
		}
	}
	pwmLast = rv
	return
}

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
	pwmLast = calcFanPwm()
	if err = setFanPwm(pwmLast); err != nil {
		fmt.Fprintln(os.Stderr, "can't set FAN PWM:", err)
		os.Exit(1)
	}
	sc := make(chan os.Signal, 1)
	signal.Notify(sc,
		syscall.SIGTERM,
		syscall.SIGQUIT,
		syscall.SIGINT,
		syscall.SIGHUP,
	)
	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go worker(done, &wg)
	fmt.Println("exit:", <-sc)
	close(done)
	wg.Wait()
	setFanPwm(pwmMax / 2)
}

func worker(done chan struct{}, wg *sync.WaitGroup) {
	defer wg.Done()
	ticker := time.NewTicker(checkInt)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			if err := setFanPwm(calcFanPwmWithDiff()); err != nil {
				fmt.Fprintln(os.Stderr, "can't set FAN PWM:", err)
			}
		}
	}
}
