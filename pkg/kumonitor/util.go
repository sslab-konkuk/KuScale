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
	"path/filepath"
	"strconv"
	"strings"
	"os"

	"k8s.io/klog"
)

func setFileUint(value uint64, path, file string) {
	err := ioutil.WriteFile(filepath.Join(path, file), []byte(strconv.FormatUint(uint64(value), 10)), os.FileMode(777)) 
	if err != nil {
		klog.Infof("%s %s %s %s", err, value, path, file)
	}
}

func parseUint(s string, base, bitSize int) (uint64) {
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

func GetFileParamUint(Path, File string) (uint64) {
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

func CheckPodExists(pi PodInfo) bool {
	if !PathExists(filepath.Join(pi.cpuPath, "/cpu.cfs_quota_us") ){
		return false
	} else if !PathExists(filepath.Join(pi.gpuPath, "/total_runtime") ) {
		return false
	} else if pi.interfaceName == "" {
		klog.Infof("NO interface %s", pi.interfaceName)
		return false
	} else {
		return true
	}
}

func CalAvg(array []uint64, windowSize int) (float64){
	
	monitoringCount := len(array) - 1
	if monitoringCount == 0 {
		return 0;
	} else if monitoringCount < windowSize {
		windowSize = monitoringCount
	} 

	prev := array[monitoringCount - windowSize]
	curr := array[monitoringCount]
	avg := (curr - prev) / uint64(windowSize)
	return float64(avg)
}




/* Get AcctUsage Functions */

func GetCpuAcctUsage(pi *PodInfo) (uint64) {
	return GetFileParamUint(pi.cpuPath, "/cpuacct.usage")
}

func GetGpuAcctUsage(pi *PodInfo) (uint64) {
	return GetFileParamUint(pi.gpuPath, "/total_runtime")
}

// func GetRxAcctUsage(pi *PodInfo) (uint64) {
// 	its, _ := GetnetworkStats(pi)
// 	return 8 * its[0].RxBytes
// }

/* Get Limit Functions */

func GetCpuLimitFromFile(pi *PodInfo) (uint64) {
	return GetFileParamUint(pi.cpuPath, "/cpu.cfs_quota_us") / 1000
}

func GetGpuLimitFromFile(pi *PodInfo) (uint64) {
	return GetFileParamUint(pi.gpuPath, "/quota")
}

/* Set Limit Functions */

func SetCpuLimit(pi *PodInfo, nextCpu float64) {
	setFileUint(uint64(nextCpu) * 1000, pi.cpuPath, "/cpu.cfs_quota_us")
	pi.CI.RIs["CPU"].SetLimit(nextCpu)
}

func SetGpuLimit(pi *PodInfo, nextGpu float64) {
	setFileUint(uint64(nextGpu), pi.gpuPath, "/quota")
	pi.CI.RIs["GPU"].SetLimit(nextGpu)
}

// func SetRxLimit(pi *PodInfo, nextRx float64) {
// 	UpdateIngressQdisc(uint64(nextRx) * miliRX, 2 * uint64(nextRx) * miliRX, pi.interfaceName)
// 	pi.CI.RIs["RX"].SetLimit(nextRx)
// }

func getPodMap(pm PodMap) (bool, error) {
	// devicePods, err := getListOfPodsFromKubelet(podsocketPath)
	// if err != nil {
	// 	return false, fmt.Errorf("failed to get devices Pod information: %v", err)
	// }
	// new := updatePodMap(pm, *devicePods)
	// return new, nil
	return false, nil
}