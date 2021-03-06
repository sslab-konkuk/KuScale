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
	"math"
	"time"

	"k8s.io/klog"
)

const miliCPU = 10000000
const miliGPU = 10

// const miliRX = 80000 // miliNetworkBits = 10KB

type ResourceName string
type acctUsageAndTime struct {
	timeStamp int64
	acctUsage uint64
}

var defaultResources = []string{"CPU", "GPU", "RX"}

// Resource Info are considered by KuScale
type ResourceInfo struct {
	name      ResourceName
	path      string
	initLimit float64

	miliScale int
	price     float64

	test        []acctUsageAndTime
	acctUsage   []uint64
	limit       float64
	usage       float64
	avgUsage    float64
	avgAvgUsage float64
}

func (ri *ResourceInfo) Init(name ResourceName, scale int, price float64) {

	ri.name, ri.miliScale, ri.price = name, scale, price
	ri.acctUsage = append(ri.acctUsage, 0)
	ri.limit, ri.usage, ri.avgUsage, ri.avgUsage, ri.avgAvgUsage = 0, 0, 0, 0, 0
	ri.test = append(ri.test, acctUsageAndTime{timeStamp: time.Now().UnixNano(), acctUsage: 0})

}

func (ri *ResourceInfo) Limit() float64         { return ri.limit }
func (ri *ResourceInfo) Usage() float64         { return math.Round(ri.usage) }
func (ri *ResourceInfo) AvgUsage() float64      { return ri.avgUsage }
func (ri *ResourceInfo) AvgAvgUsage() float64   { return ri.avgAvgUsage }
func (ri *ResourceInfo) Price() float64         { return ri.price }
func (ri *ResourceInfo) SetLimit(limit float64) { ri.limit = limit }
func (ri *ResourceInfo) GetAcctUsage() uint64 {

	switch ri.name {
	case "CPU":
		acctUsage, timeStamp := GetCpuAcctUsage(ri.path)
		ri.test = append(ri.test, acctUsageAndTime{timeStamp: timeStamp, acctUsage: acctUsage})
		return acctUsage
	case "GPU":
		acctUsage, timeStamp := GetGpuAcctUsage(ri.path)
		ri.test = append(ri.test, acctUsageAndTime{timeStamp: timeStamp, acctUsage: acctUsage * 1000000})
		return acctUsage
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

func (ri *ResourceInfo) getCurrentUsage() float64 {
	array := ri.test
	monitoringCount := len(array) - 1
	prev := array[monitoringCount-1]
	curr := array[monitoringCount]

	return float64(curr.acctUsage-prev.acctUsage) * 100. / float64(curr.timeStamp-prev.timeStamp)
}

type PodStatus string

const (
	PodInitializing PodStatus = "initializing"
	PodReady        PodStatus = "ready"
	PodRunning      PodStatus = "running"
	PodFinished     PodStatus = "finished"
)

// Pod Info are managed by KuScale
type PodInfo struct {
	PodName    string
	ID         string
	imageName  string
	podStatus  PodStatus
	totalToken uint64

	// pid           string
	// interfaceName string

	// Resource Names & Resource Infos
	RNs []ResourceName
	RIs map[ResourceName]*ResourceInfo

	// Update Count from KuScale
	UpdateCount int
}

func NewPodInfo(podName string, RNs []ResourceName) *PodInfo {

	klog.V(5).Infof("Makeing New Pod Info %s", podName)
	podInfo := PodInfo{
		PodName:   podName,
		podStatus: PodInitializing,
	}

	podInfo.RNs = RNs
	podInfo.RIs = make(map[ResourceName]*ResourceInfo)
	for _, name := range podInfo.RNs {
		ri := ResourceInfo{name: name}
		switch name {
		case "CPU":
			ri.Init(name, miliCPU, 1)
		case "GPU":
			ri.Init(name, miliGPU, 3)
		}
		// ri.UpdateUsage(1) //TODO: Need Change 1
		podInfo.RIs[name] = &ri
	}

	klog.V(5).Infof("Made New Pod Info %s", podName)
	return &podInfo
}

func (pi *PodInfo) UpdateUsage(monitoringPeriod int) {

	for _, ri := range pi.RIs {
		ri.acctUsage = append(ri.acctUsage, ri.GetAcctUsage())
		ri.usage = CalAvg(ri.acctUsage, 1) / float64(ri.miliScale*monitoringPeriod) // TODO: need to check CPU overflow
		if ri.usage > 1000 {
			ri.usage = 0
		}
		ri.avgUsage = (7*ri.avgUsage + ri.usage) / 8
		ri.avgAvgUsage = (7*ri.avgAvgUsage + ri.avgUsage) / 8
	}
}
