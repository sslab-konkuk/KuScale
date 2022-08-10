package kutokenmanager

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"k8s.io/klog"
)

func PathExists(path string) bool {
	if _, err := os.Stat(path); err != nil {
		return false
	}
	return true
}

func updateResourceConf() {
	echoCommand := "echo 0 > /sys/kernel/gpu/gemini/resource_conf"
	cmd := exec.Command("sh", "-c", echoCommand)
	err := cmd.Run()
	if err != nil {
		klog.Error("updateResourceConf", err)
	}
}

func CreateGPUID(path string) {

	echoCommand := "echo " + path + " > /sys/kernel/gpu/configs/init"
	cmd := exec.Command("sh", "-c", echoCommand)
	err := cmd.Run()
	if err != nil {
		klog.Error("Error Creating GPU ID", err)
	}
	updateResourceConf()
	klog.V(5).Info("Created Sucessfully GPU Lyaer ID :", path)
}

func RmmodGpuMod() {
	cmd := exec.Command("/usr/sbin/rmmod", "ku-gpu-layer")
	err := cmd.Run()
	if err != nil {
		klog.Error("Error rmmodGpuMod", err)
	}
	klog.V(5).Info("Removed KU GPU Lyaer Module")
}

func InsmodGpuMod() {
	if PathExists("/sys/kernel/gpu") {
		// RmmodGpuMod()
		klog.V(5).Info("Already Inserted KU GPU Lyaer Module")
		return
	}
	cmd := exec.Command("/usr/sbin/insmod", "./ku-gpu-layer.ko")
	err := cmd.Run()
	if err != nil {
		klog.Error("Erorr insmodGpuMod", err)
	}
	klog.V(5).Info("Inserted KU GPU Lyaer Module")

}

func parseUint(s string, base, bitSize int) uint64 {
	value, err := strconv.ParseUint(s, base, bitSize)
	if err != nil {
		intValue, intErr := strconv.ParseInt(s, base, bitSize)
		if intErr == nil && intValue < 0 {
			return 0
		} else if intErr != nil && intErr.(*strconv.NumError).Err == strconv.ErrRange && intValue < 0 {
			return 0
		}
		return value
	}
	return value
}

func GetFileParamUint(Path, File string) uint64 {
	contents, err := ioutil.ReadFile(filepath.Join(Path, File))
	if err != nil {
		klog.Infof("couldn't GetFileParamUint: %s", err)
		return 0
	}
	return parseUint(strings.TrimSpace(string(contents)), 10, 64)
}
