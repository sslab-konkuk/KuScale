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

package kuscale

import (
	"io/ioutil"
	"path/filepath"
	"strconv"
	"strings"
	"os"
	"log"
	"fmt"
)

func setFileUint(value uint64, path, file string) {
	err := ioutil.WriteFile(filepath.Join(path, file), []byte(strconv.FormatUint(uint64(value), 10)), os.FileMode(777)) 
	if err != nil {
		log.Println(err, value, path, file)
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
		fmt.Printf("couldn't GetFileParamUint: %v", err)
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
		log.Println("NO interface", pi.interfaceName)
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