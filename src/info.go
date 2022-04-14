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

import (
	"log"
	"math"
)

const miliCPU = 10000000
const miliGPU = 10
const miliRX = 80000 // miliNetworkBits = 10KB

type ResourceName  string
var defaultResources = []string{"CPU", "GPU", "RX"}


// Resource Info are considered by KuScale
type ResourceInfo struct {
	name			string
	path			string

	miliScale		int
	price			float64

	acctUsage 		[]uint64
	limit			float64
	usage   		float64
	avgUsage 		float64
	avgAvgUsage 	float64
}

func (ri *ResourceInfo) Init(name, path string, scale int, price float64) {
	
	ri.name, ri.miliScale, ri.path, ri.price  = name, scale, path, price
	ri.acctUsage = append(ri.acctUsage, 0)
	ri.limit, ri.usage, ri.avgUsage, ri.avgUsage, ri.avgAvgUsage  = 0, 0, 0, 0, 0
}

func (ri ResourceInfo) Limit() float64 { return ri.limit }
func (ri ResourceInfo) Usage() float64 { return math.Round(ri.usage) }
func (ri ResourceInfo) AvgUsage() float64 { return ri.avgUsage }
func (ri ResourceInfo) AvgAvgUsage() float64 { return ri.avgAvgUsage }
func (ri ResourceInfo) Price() float64 { return ri.price }
func (ri *ResourceInfo) SetLimit(limit float64) { ri.limit = limit }
func (ri *ResourceInfo) UpdateUsage() {

	ri.acctUsage = append(ri.acctUsage, ri.GetAcctUsage())
	ri.usage = CalAvg(ri.acctUsage, 1) / float64(ri.miliScale * config.MonitoringPeriod) // TODO: need to check CPU overflow
	ri.avgUsage = (7 * ri.avgUsage + ri.usage) / 8
	ri.avgAvgUsage = (7 * ri.avgAvgUsage + ri.avgUsage) / 8
}

func (ri ResourceInfo) GetAcctUsage() (uint64){
	
	switch ri.name {
	case "CPU":
		return GetFileParamUint(ri.path, "/cpuacct.usage")
	case "GPU":
		return GetFileParamUint(ri.path, "/total-usage-pod3")
	case "RX":
		ifaceStats, err := scanInterfaceStats(ri.path) // TODO : NEED TO READ HOST NET DEV
		if err != nil {
			log.Printf("couldn't read network stats: ", err)
			return 0
		}
		return 8 * ifaceStats[0].RxBytes // Make Bits
	}
	return 0
}

// Container Info are considered by KuScale
type ContainerInfo struct {
	
	// Resource Name
	RNs 						[]string 
	// Resource Info
	RIs							map[string]*ResourceInfo

    // Update Count from KuScale
	UpdateCount				 	int
}

var zeroCI = &ContainerInfo{}
func (ci *ContainerInfo) Reset() {
    *ci = *zeroCI
}

// Pod Info are managed by KuScale
type PodInfo struct {
	podName      	string
	namespace 		string
	containerName 	string
	initFlag		bool

	totalToken		uint64
	initCpu			uint64
	initGpu			uint64
	initRx			uint64

	cpuPath			string
	gpuPath			string
	rxPath			string

	pid				string
	lastIterModStart	int64
	interfaceName		string

	CI 				ContainerInfo
}

type PodMap map[string]PodInfo

var LivePodMap PodMap
var CompletedPodMap PodMap


