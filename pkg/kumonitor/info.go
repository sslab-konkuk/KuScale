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
	"k8s.io/klog"
	"math"
)

type Configuraion struct {
	monitoringPeriod int
	windowSize       int
	nodeName         string
	monitoring       bool
	exportMode       bool
}

const miliCPU = 10000000
const miliGPU = 10

// const miliRX = 80000 // miliNetworkBits = 10KB

type ResourceName string

var defaultResources = []string{"CPU", "GPU", "RX"}

// Resource Info are considered by KuScale
type ResourceInfo struct {
	name string
	path string

	miliScale int
	price     float64

	acctUsage   []uint64
	limit       float64
	usage       float64
	avgUsage    float64
	avgAvgUsage float64
}

func (ri *ResourceInfo) Init(name, path string, scale int, price float64) {

	ri.name, ri.miliScale, ri.path, ri.price = name, scale, path, price
	ri.acctUsage = append(ri.acctUsage, 0)
	ri.limit, ri.usage, ri.avgUsage, ri.avgUsage, ri.avgAvgUsage = 0, 0, 0, 0, 0
}

func (ri ResourceInfo) Limit() float64          { return ri.limit }
func (ri ResourceInfo) Usage() float64          { return math.Round(ri.usage) }
func (ri ResourceInfo) AvgUsage() float64       { return ri.avgUsage }
func (ri ResourceInfo) AvgAvgUsage() float64    { return ri.avgAvgUsage }
func (ri ResourceInfo) Price() float64          { return ri.price }
func (ri *ResourceInfo) SetLimit(limit float64) { ri.limit = limit }
func (ri *ResourceInfo) UpdateUsage(podName string, monitoringPeriod int) {

	ri.acctUsage = append(ri.acctUsage, ri.GetAcctUsage(podName))
	ri.usage = CalAvg(ri.acctUsage, 1) / float64(ri.miliScale*monitoringPeriod) // TODO: need to check CPU overflow
	klog.V(4).Info(ri.name, " ", ri.usage)
	if ri.usage > 1000 {
		ri.usage = 0
	}
	ri.avgUsage = (7*ri.avgUsage + ri.usage) / 8
	ri.avgAvgUsage = (7*ri.avgAvgUsage + ri.avgUsage) / 8
}

func (ri ResourceInfo) GetAcctUsage(podName string) uint64 {

	switch ri.name {
	case "CPU":
		return GetFileParamUint(ri.path, "/cpuacct.usage")
	case "GPU":
		return GetGpuAcctUsage(ri.path, podName)
		// return GetFileParamUint(ri.path, podName)
		// case "RX":
		// 	ifaceStats, err := scanInterfaceStats(ri.path) // TODO : NEED TO READ HOST NET DEV
		// 	if err != nil {
		// 		klog.Infof("couldn't read network stats: ", err)
		// 		return 0
		// 	}
		// 	return 8 * ifaceStats[0].RxBytes // Make Bits
	}
	return 0
}

// Pod Info are managed by KuScale
type PodInfo struct {
	podName string
	// namespace     string
	containerName string
	initFlag      bool // TODO: Need to check how to use

	totalToken uint64
	initCpu    float64
	initGpu    float64
	// initRx     uint64

	cpuPath string
	gpuPath string
	// rxPath  string

	// pid           string
	// interfaceName string

	// Resource Names
	RNs []string
	// Resource Infos
	RIs map[string]*ResourceInfo

	// Update Count from KuScale
	UpdateCount int
}

func NewPodInfo(podName string, RNs []string) *PodInfo {

	klog.V(5).Infof("Makeing New Pod Info %s", podName)

	cpuPath := getCpuPath(podName)
	if cpuPath == "" {
		return nil
	}

	podInfo := PodInfo{
		podName:  podName,
		initFlag: false,
		cpuPath:  cpuPath,
		gpuPath:  "/kubeshare/scheduler/total-usage-",
	}
	podInfo.RNs = RNs
	podInfo.RIs = make(map[string]*ResourceInfo)
	for _, name := range podInfo.RNs {
		ri := ResourceInfo{name: name}
		switch name {
		case "CPU":
			ri.Init(name, podInfo.cpuPath, miliCPU, 1)
		case "GPU":
			ri.Init(name, podInfo.gpuPath, miliGPU, 3)
		}
		ri.UpdateUsage(name, 1) //TODO: Need Change 1
		podInfo.RIs[name] = &ri
	}

	klog.V(5).Infof("Made New Pod Info %s", podName)
	return &podInfo
}
