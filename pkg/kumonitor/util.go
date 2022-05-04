// Copyright 2022 Hyeon-Jun Jang, SSLab, Konkuk University
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kumonitor

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"k8s.io/klog"
)

func setFileUint(value uint64, path, file string) {
	err := ioutil.WriteFile(filepath.Join(path, file), []byte(strconv.FormatUint(uint64(value), 10)), os.FileMode(0777))
	if err != nil {
		klog.Infof("%s %d %s %s", err, value, path, file)
	}
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

func PathExists(path string) bool {
	if _, err := os.Stat(path); err != nil {
		return false
	}
	return true
}

func CheckPodExists(pi *PodInfo) bool {
	if !PathExists(filepath.Join(pi.cpuPath, "/cpu.cfs_quota_us")) {
		return false
		// } else if !PathExists(filepath.Join(pi.gpuPath, pi.podName) ) {
		// 	return false
		// } else if pi.interfaceName == "" {
		// 	klog.Infof("NO interface %s", pi.interfaceName)
		// 	return false
	} else {
		return true
	}
}

func CalAvg(array []uint64, windowSize int) float64 {

	monitoringCount := len(array) - 1
	if monitoringCount == 0 {
		return 0
	} else if monitoringCount < windowSize {
		windowSize = monitoringCount
	}

	prev := array[monitoringCount-windowSize]
	curr := array[monitoringCount]
	avg := (curr - prev) / uint64(windowSize)
	return float64(avg)
}

/* Get AcctUsage Functions */

func (pi *PodInfo) GetCpuAcctUsage() uint64 {
	return GetFileParamUint(pi.cpuPath, "/cpuacct.usage")
}

func GetGpuAcctUsage(gpuPath, podName string) uint64 {

	path := gpuPath + podName
	dat, _ := ioutil.ReadFile(path)
	read_line := strings.TrimSuffix(string(dat), "\n")
	num1, _ := strconv.ParseFloat(read_line, 64)
	return uint64(num1)
	// return GetFileParamUint(pi.gpuPath, pi.podName)
}

// func GetRxAcctUsage(pi *PodInfo) (uint64) {
// 	its, _ := GetnetworkStats(pi)
// 	return 8 * its[0].RxBytes
// }

/* Get Limit Functions */

func GetCpuLimitFromFile(pi *PodInfo) uint64 {
	return GetFileParamUint(pi.cpuPath, "/cpu.cfs_quota_us") / 1000
}

func GetGpuLimitFromFile(pi *PodInfo) uint64 {
	return GetFileParamUint(pi.gpuPath, "/quota")
}

/* Set Limit Functions */

func SetCpuLimit(pi *PodInfo, nextCpu float64) {
	setFileUint(uint64(nextCpu)*1000, pi.cpuPath, "/cpu.cfs_quota_us")
	pi.RIs["CPU"].SetLimit(nextCpu)
}

func SetGpuLimit(pi *PodInfo, nextGpu float64) {
	setFileUint(uint64(nextGpu), pi.gpuPath, "/quota")
	pi.RIs["GPU"].SetLimit(nextGpu)
}

// func SetRxLimit(pi *PodInfo, nextRx float64) {
// 	UpdateIngressQdisc(uint64(nextRx) * miliRX, 2 * uint64(nextRx) * miliRX, pi.interfaceName)
// 	pi.CI.RIs["RX"].SetLimit(nextRx)
// }

// func writeGpuGeminiConfig() {

// 	gpu_config_f, err := os.Create(SchedulerGPUConfigPath)
// 	if err != nil {
// 		klog.Errorf("Error when create config file on path: %s", SchedulerGPUConfigPath)
// 	}

// 	for i, pod := range pod_configs {
// 		pod_config := strings.Split(pod, " ")
// 		if len(pod_config) < 4 {
// 			break
// 		}

// 		minutil, maxutil, memlimit := pod_config[1], pod_config[2], pod_config[3]
// 		def := strings.Split(pod_config[0], "/")
// 		podname := def[1]
// 		klog.Infof("pod info[%d]: %s, %s, %s, %s, %s", i, def, podname, minutil, maxutil, memlimit)

// 		//pod key file
// 		gpu_config_f.WriteString(fmt.Sprintf("[%s]\n", podname))
// 		gpu_config_f.WriteString(fmt.Sprintf("clientid=%d\n", strings.Count(podlist, ",")))
// 		gpu_config_f.WriteString(fmt.Sprintf("MinUtil=%s\n", minutil))
// 		gpu_config_f.WriteString(fmt.Sprintf("MaxUtil=%s\n", maxutil))
// 		gpu_config_f.WriteString(fmt.Sprintf("MemoryLimit=%s\n", memlimit))
// 	}

// 	gpu_config_f.Sync()
// 	gpu_config_f.Close()
// }
