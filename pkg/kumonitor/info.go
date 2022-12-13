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
	name      ResourceName
	path      string
	usagePath string
	miliScale int
	price     float64

	/* Limit */
	initLimit float64
	limit     float64
	nextLimit float64

	/* Usage */
	acctUsageAndTime []AcctUsageAndTime
	usage            float64
	avgUsage         float64 // Weighted Average : (7*ri.avgUsage + ri.usage) / 8
	dynamicWeight    float64 // Dynamic Weight for this resource 	: price / {avgUsage / sum of avgUsage}
}

func (ri *ResourceInfo) Init(name ResourceName, scale int, price float64) {

	ri.name, ri.miliScale, ri.price = name, scale, price
	ri.limit, ri.usage, ri.avgUsage, ri.avgUsage = 0, 0, 0, 0
	ri.acctUsageAndTime = append(ri.acctUsageAndTime, AcctUsageAndTime{timeStamp: uint64(time.Now().UnixNano()), acctUsage: 0})
}

func (ri *ResourceInfo) Limit() float64         { return ri.limit }
func (ri *ResourceInfo) Usage() float64         { return ri.usage }
func (ri *ResourceInfo) AvgUsage() float64      { return ri.avgUsage }
func (ri *ResourceInfo) DynamicWeight() float64 { return ri.dynamicWeight }
func (ri *ResourceInfo) Price() float64         { return ri.price }

func (ri *ResourceInfo) SetLimit(limit float64) {
	// klog.V(5).Info("Set ", ri.name, ": ", limit)
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

/*
Func Name : (ri *ResourceInfo) updateUsage() bool
	Objective :
	1) Get AcctUsage From ri.usagePath
	2) Update usage, avgUsage, acctUsage
	3) Return completed(=true) when the filepath doesn't exist
*/
func (ri *ResourceInfo) updateUsage() bool {

	timeStamp := uint64(time.Now().UnixNano())
	acctUsage, completed := GetFileUint(ri.usagePath)
	if completed {
		return true
	}
	prev := ri.acctUsageAndTime[len(ri.acctUsageAndTime)-1]

	ri.usage = float64(acctUsage-prev.acctUsage) * 100. / float64(timeStamp-prev.timeStamp)
	ri.avgUsage = (7*ri.avgUsage + ri.usage) / 8
	ri.acctUsageAndTime = append(ri.acctUsageAndTime,
		AcctUsageAndTime{timeStamp: timeStamp, acctUsage: acctUsage})
	return false
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
	PodName   string
	ID        string
	dockerID  string
	imageName string

	status         PodStatus
	reservedToken  uint64
	totalToken     float64
	expectedToken  float64
	availableToken float64

	lastUpdatedTime  int64
	lastElaspedTime  float64
	TokenQueue       float64
	TokenReservation float64
	UpdatedCount     int64 // Update Count from KuScale

	RNs []ResourceName
	RIs map[ResourceName]*ResourceInfo
}

func (pi *PodInfo) CPU() *ResourceInfo {
	return pi.RIs["CPU"]
}
func (pi *PodInfo) GPU() *ResourceInfo {
	return pi.RIs["GPU"]
}

func NewPodInfo(podName string, RNs []ResourceName) *PodInfo {

	klog.V(5).Infof("Makeing New Pod Info of %s", podName)
	podInfo := PodInfo{
		PodName: podName,
		status:  PodInitializing,
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

/*
Func Name : (pi *PodInfo) UpdatePodUsage()
	Objective :
	1) Monitoring the pods in RunningPodMap
	2) Check and Remove Completed Pods
*/
func (pi *PodInfo) UpdatePodUsage() {
	// startTime := kuprofiler.StartTime()
	// defer kuprofiler.Record("UpdatePodUsage", startTime)

	for _, ri := range pi.RIs {
		completed := ri.updateUsage()
		if completed {
			klog.V(10).Info(pi.PodName, " may be finished because the filepath doesn't exist")
			pi.status = PodCompleted
		}
	}
	klog.V(4).Info(pi.PodName, "'s usages are ", int64(pi.RIs["CPU"].Usage()), int64(pi.RIs["GPU"].Usage()))
}

/*
Func Name : (pi *PodInfo) UpdateTokenQueue()
	Objective :
	1) Update TokenQueue
*/
func (pi *PodInfo) UpdateTokenQueue() {

	TokenQueue := pi.TokenQueue + pi.TokenReservation*pi.lastElaspedTime

	for _, ri := range pi.RIs {
		limit, price := ri.Limit(), ri.Price()
		TokenQueue = TokenQueue - price*limit*pi.lastElaspedTime
	}
	if TokenQueue < 0 {
		TokenQueue = 0
	} else if TokenQueue >= pi.TokenReservation {
		// TODO : Update Max
		TokenQueue = pi.TokenReservation
	}

	pi.TokenQueue = TokenQueue

	klog.V(10).Info(pi.PodName, " 's TokenQueue is updated to ", pi.TokenQueue, " with Token Reservation : ", pi.TokenReservation)
}

/*
Func Name : (pi *PodInfo) UpdateDynamicWeight(staticV float64)
	Objective :
	1) Update DynamicWeight
*/
func (pi *PodInfo) UpdateDynamicWeight(staticV float64) {

	if staticV > 0 {
		for _, ri := range pi.RIs {
			ri.dynamicWeight = ri.price * staticV
		}
		return
	}

	// Get Sum of AvgUsage
	sumAvgUsage := 0.
	for _, ri := range pi.RIs {
		sumAvgUsage += ri.AvgUsage() + 1
	}

	// Weight
	W := 15.
	AVGDIFF := 0.
	for _, ri := range pi.RIs {
		AVGDIFF += (ri.AvgUsage() + 1.) / (ri.Usage() + 1.)
	}
	W = AVGDIFF * W
	klog.V(10).Info("Update W : ", W)

	// Update Dynamic Weight
	for rn, ri := range pi.RIs {
		avgUsage, price := ri.AvgUsage()+1, ri.Price()
		ri.dynamicWeight = W * price * sumAvgUsage / avgUsage
		klog.V(10).Info("Update DynamicWeight for Pod: ", pi.PodName, ", ", rn, "'s Dynamic Weight : ", ri.dynamicWeight,
			" avg : ", ri.avgUsage)
	}

}

func (pi *PodInfo) SetInitLimit() {
	for _, ri := range pi.RIs {
		ri.SetLimit(ri.initLimit)
	}
}

func (pi *PodInfo) setNextLimit() {

	for _, ri := range pi.RIs {
		if ri.nextLimit < 10 {
			ri.nextLimit = 10
		}
		ri.SetLimit(ri.nextLimit)
	}
	pi.UpdatedCount = pi.UpdatedCount + 1
	pi.lastUpdatedTime = time.Now().UnixNano()
	klog.V(4).Info(pi.PodName, "'s limits are set to : ", int64(pi.CPU().nextLimit), int64(pi.GPU().nextLimit))
}

func (pi *PodInfo) getNextLimit(remainedTimePerSecond float64) {

	k := 0.
	pi.availableToken = k*pi.TokenQueue + pi.TokenReservation*remainedTimePerSecond

	/*** Caclulate Next Limit wihtout Any Conditions ***/
	cpuRI, gpuRI := pi.RIs["CPU"], pi.RIs["GPU"]
	cpuNextLimit := cpuRI.usage + cpuRI.price*pi.availableToken/(2*cpuRI.dynamicWeight)
	gpuNextLimit := gpuRI.usage + gpuRI.price*pi.availableToken/(2*gpuRI.dynamicWeight)

	tokenCondition := pi.availableToken - (cpuRI.price*cpuNextLimit+gpuRI.price*gpuNextLimit)*remainedTimePerSecond

	if tokenCondition >= 0 {
		klog.V(10).Info(pi.PodName, "'s Next Reseravation :", int64(cpuNextLimit), " , ", int64(gpuNextLimit), " Token Enough : tokenCondition : ", int64(tokenCondition))
		pi.CPU().nextLimit = cpuNextLimit
		pi.GPU().nextLimit = gpuNextLimit
		return
	}

	/*** Caclulate Next Limit wiht Token Conditions ***/
	up := pi.availableToken/remainedTimePerSecond - cpuRI.price*cpuRI.usage - gpuRI.price*gpuRI.usage
	below := gpuRI.dynamicWeight*cpuRI.price*cpuRI.price + cpuRI.dynamicWeight*gpuRI.price*gpuRI.price
	cpuNextLimit = cpuRI.usage + cpuRI.price*gpuRI.dynamicWeight*up/below
	gpuNextLimit = gpuRI.usage + gpuRI.price*cpuRI.dynamicWeight*up/below
	tokenCondition = pi.availableToken - (cpuRI.price*cpuNextLimit+gpuRI.price*gpuNextLimit)*remainedTimePerSecond

	klog.V(10).Info(pi.PodName, "'s Next Reseravation :", int64(cpuNextLimit), " , ", int64(gpuNextLimit), " Token Enough : tokenCondition : ", int64(tokenCondition))

	pi.CPU().nextLimit = cpuNextLimit
	pi.GPU().nextLimit = gpuNextLimit
	return
}
