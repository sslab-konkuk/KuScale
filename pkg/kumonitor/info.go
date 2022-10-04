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
	"time"

	"k8s.io/klog"
)

const miliCPU = 10000000
const miliGPU = 10

// const miliRX = 80000 // miliNetworkBits = 10KB

type ResourceName string
type AcctUsageAndTime struct {
	timeStamp uint64
	acctUsage uint64
}

type ResourceInfo struct {
	name             ResourceName
	path             string
	miliScale        int
	acctUsageAndTime []AcctUsageAndTime
	price            float64
	initLimit        float64
	limit            float64
	usage            float64
	avgUsage         float64 // Weighted Average : (7*ri.avgUsage + ri.usage) / 8
	dynamicWeight    float64 // Dynamic Weight for this resource 	: price / {avgUsage / sum of avgUsage}
	avgAvgUsage      float64
}

func (ri *ResourceInfo) Init(name ResourceName, scale int, price float64) {

	ri.name, ri.miliScale, ri.price = name, scale, price
	ri.limit, ri.usage, ri.avgUsage, ri.avgUsage, ri.avgAvgUsage = 0, 0, 0, 0, 0
	ri.acctUsageAndTime = append(ri.acctUsageAndTime, AcctUsageAndTime{timeStamp: uint64(time.Now().UnixNano()), acctUsage: 0})
}

func (ri *ResourceInfo) Limit() float64         { return ri.limit }
func (ri *ResourceInfo) Usage() float64         { return ri.usage }
func (ri *ResourceInfo) AvgUsage() float64      { return ri.avgUsage }
func (ri *ResourceInfo) DynamicWeight() float64 { return ri.dynamicWeight }
func (ri *ResourceInfo) AvgAvgUsage() float64   { return ri.avgAvgUsage }
func (ri *ResourceInfo) Price() float64         { return ri.price }

func (ri *ResourceInfo) SetLimit(limit float64) {
	klog.V(5).Info("Set ", ri.name, ": ", limit)
	switch ri.name {
	case "CPU":
		setFileUint(uint64(limit)*1000, ri.path, "/cpu.cfs_quota_us")
		ri.limit = limit
		return
	case "GPU":
		setFileUint(uint64(limit)*10, ri.path, "/gpu_limit")
		setFileUint(uint64(limit)*10, ri.path, "/gpu_request")
		UpdateGemini()
		ri.limit = limit
		return
	}
}

func (ri *ResourceInfo) getAcctUsage() {
	switch ri.name {
	case "CPU":
		acctUsage, timeStamp := GetCpuAcctUsage(ri.path)
		ri.acctUsageAndTime = append(ri.acctUsageAndTime, AcctUsageAndTime{timeStamp: timeStamp, acctUsage: acctUsage})
		return
	case "GPU":
		acctUsage, timeStamp := GetGpuAcctUsage(ri.path)
		ri.acctUsageAndTime = append(ri.acctUsageAndTime, AcctUsageAndTime{timeStamp: timeStamp, acctUsage: acctUsage})
		return
		// case "RX":
		// 	ifaceStats, err := scanInterfaceStats(ri.path) // TODO : NEED TO READ HOST NET DEV
		// 	if err != nil {
		// 		klog.Infof("couldn't read network stats: ", err)
		// 		return 0
		// 	}
		// 	return 8 * ifaceStats[0].RxBytes // Make Bits
	}
	return
}

func (ri *ResourceInfo) getCurrentUsage() float64 {
	ri.getAcctUsage()
	array := ri.acctUsageAndTime
	monitoringCount := len(array) - 1
	curr, prev := array[monitoringCount], array[monitoringCount-1]

	return float64(curr.acctUsage-prev.acctUsage) * 100. / float64(curr.timeStamp-prev.timeStamp)
}

type PodStatus string

const (
	PodInitializing PodStatus = "initializing"
	PodNotReady     PodStatus = "not ready"
	PodRunning      PodStatus = "running"
	PodCompleted    PodStatus = "completed"
)

// Pod Info are managed by KuScale
type PodInfo struct {
	PodName       string
	ID            string
	imageName     string
	podStatus     PodStatus
	reservedToken uint64
	totalToken    float64
	expectedToken float64

	tokenQueue       float64
	tokenReservation float64
	availableToken   float64

	UpdatedCount int64 // Update Count from KuScale

	// pid           string
	// interfaceName string

	// Resource Names & Resource Infos
	RNs []ResourceName
	RIs map[ResourceName]*ResourceInfo
}

func NewPodInfo(podName string, RNs []ResourceName) *PodInfo {

	klog.V(5).Infof("Makeing New Pod Info of %s", podName)
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
		podInfo.RIs[name] = &ri
	}

	klog.V(5).Infof("Made New Pod Info of %s", podName)
	return &podInfo
}

func (pi *PodInfo) UpdateUsage() {

	for _, ri := range pi.RIs {
		ri.usage = ri.getCurrentUsage()
		ri.avgUsage = (7*ri.avgUsage + ri.usage) / 8
		ri.avgAvgUsage = (7*ri.avgAvgUsage + ri.avgUsage) / 8
	}
}

// Caclulate Token Queue for pod
func (pi *PodInfo) UpdateTokenQueue(elaspedTimePerSecond float64) {

	tokenQueue := pi.tokenQueue + float64(pi.tokenReservation)*elaspedTimePerSecond
	for _, ri := range pi.RIs {
		limit, price := ri.Limit(), ri.Price()
		tokenQueue = tokenQueue - price*limit*elaspedTimePerSecond
	}
	if tokenQueue < 0 {
		tokenQueue = 0
	}
	pi.tokenQueue = tokenQueue

	klog.V(10).Info("Update tokenQueue for Pod: ", pi.PodName, " Total Token : ", pi.tokenQueue, " Token Reservation : ", pi.tokenReservation)
}

// Caclulate Dynamic Weight for each resource in the pod
func (pi *PodInfo) UpdateDynamicWeight() {

	// Get Sum of AvgUsage
	sumAvgUsage := 0.
	for _, ri := range pi.RIs {
		sumAvgUsage += ri.AvgUsage() + 1
	}

	// Update Dynamic Weight
	for rn, ri := range pi.RIs {
		avgUsage, price := ri.AvgUsage()+1, ri.Price()
		ri.dynamicWeight = price * sumAvgUsage / avgUsage
		klog.V(10).Info("Update DynamicWeight for Pod: ", pi.PodName, ", ", rn, "'s Dynamic Weight : ", ri.dynamicWeight)
	}

}

func (pi *PodInfo) SetInitLimit() {
	for _, ri := range pi.RIs {
		ri.SetLimit(ri.initLimit)
	}
}

func (pi *PodInfo) SetLimit(cpuLimit, gpuLimit float64) {
	klog.V(5).Info("SetLimit ", pi.PodName, " CPU: ", cpuLimit, ", GPU: ", gpuLimit)
	if pi.RIs["CPU"].Limit() == cpuLimit && pi.RIs["GPU"].Limit() == gpuLimit {
		return
	}
	pi.RIs["CPU"].SetLimit(cpuLimit)
	pi.RIs["GPU"].SetLimit(gpuLimit)
	pi.UpdatedCount = pi.UpdatedCount + 1
	// pi.lastUpdatedTime = time.Now().UnixNano()
}
