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

package main

/* Get AcctUsage Functions */

func GetCpuAcctUsage(pi *PodInfo) (uint64) {
	return GetFileParamUint(pi.cpuPath, "/cpuacct.usage")
}

func GetGpuAcctUsage(pi *PodInfo) (uint64) {
	return GetFileParamUint(pi.gpuPath, "/total_runtime")
}

func GetRxAcctUsage(pi *PodInfo) (uint64) {
	its, _ := GetnetworkStats(pi)
	return 8 * its[0].RxBytes
}

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

func SetRxLimit(pi *PodInfo, nextRx float64) {
	UpdateIngressQdisc(uint64(nextRx) * miliRX, 2 * uint64(nextRx) * miliRX, pi.interfaceName)
	pi.CI.RIs["RX"].SetLimit(nextRx)
}






