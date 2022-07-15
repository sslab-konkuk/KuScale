package kudeviceplugin

import (
	"os"
	"os/exec"
	"strconv"
	"time"

	"k8s.io/klog"
)

func PathExists(path string) bool {
	if _, err := os.Stat(path); err != nil {
		return false
	}
	return true
}

func checkEnableGpuModPath() bool {
	return PathExists("/sys/kernel/gpu")
}

func createGpuMod(nm int) {
	cmd := exec.Command("/home/dev/echo-gpu.sh", strconv.FormatUint(uint64(nm), 10))
	err := cmd.Run()
	if err != nil {
		klog.Info("createGpuMod", err)
	}

	/* Validate GPU Module Enable */
	for {
		if checkEnableGpuModPath() {
			break
		}
		time.Sleep(time.Second / 10)
	}
}

func rmmodGpuMod() {
	cmd := exec.Command("/usr/sbin/rmmod", "main")
	err := cmd.Run()
	if err != nil {
		klog.Info("rmmodGpuMod", err)
	}
}

func insmodGpuMod() {
	if checkEnableGpuModPath() {
		rmmodGpuMod()
	}
	cmd := exec.Command("/usr/sbin/insmod", "/home/main.ko")
	err := cmd.Run()
	if err != nil {
		klog.Info("insmodGpuMod", err)
	}
}
