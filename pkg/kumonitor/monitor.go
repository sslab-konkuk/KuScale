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
	"io/ioutil"
	"strconv"
	"strings"
	"k8s.io/klog"
)

type PodMap map[string]PodInfo

type Monitor struct {
	config 					Configuraion
	livePodMap 				PodMap
 	completedPodMap 		PodMap
} 

func NewMonitor(
	monitoringPeriod, windowSize int,
	nodeName string,
	monitoringMode bool,
	exporterMode bool,
	stopCh <-chan struct{}) *Monitor {
	
	klog.Info("Creating New Monitor")
	config = Configuraion{monitoringPeriod, windowSize, nodeName, monitoringMode}
	monitor := &Monitor{config: config, livePodMap: make(PodMap), completedPodMap: make(PodMap)}
	
	// Run Promethuse Exporter
	if exporterMode {
		klog.Info("Creating Exporter")
		go ExporterRun(monitor, stopCh)
	}

	return monitor
}


func (m *Monitor) UpdateNewPod() {

	klog.Info("UpdateNewPod")
	
	podName:= "pod3"
	pod := PodInfo{
		podName:      		podName,
		initFlag : 			false,
		// cpuPath : 			getCpuPath(podName),
		gpuPath : 			"/kubeshare/scheduler",
	}
	pod.CI.RNs = []string{"GPU"}
	pod.CI.RIs = make(map[string]*ResourceInfo)
	for _, name := range pod.CI.RNs {
		ri := ResourceInfo{name : name,}
		switch name {
		case "CPU":
			ri.Init(name, pod.cpuPath, miliCPU, 1)
		case "GPU":
			ri.Init(name, pod.gpuPath, miliGPU, 3)
		}
		ri.UpdateUsage()
		pod.CI.RIs[name] = &ri
	}
    m.livePodMap[podName] = pod

	podName= "pod4"
	pod = PodInfo{
		podName:      		podName,
		initFlag : 			false,
		// cpuPath : 			getCpuPath(podName),
		gpuPath : 			"/kubeshare/scheduler",
	}
	pod.CI.RNs = []string{"GPU"}
	pod.CI.RIs = make(map[string]*ResourceInfo)
	for _, name := range pod.CI.RNs {
		ri := ResourceInfo{name : name,}
		switch name {
		case "CPU":
			ri.Init(name, pod.cpuPath, miliCPU, 1)
		case "GPU":
			ri.Init(name, pod.gpuPath, miliGPU, 3)
		}
		ri.UpdateUsage()
		pod.CI.RIs[name] = &ri
	}
    m.livePodMap[podName] = pod

	// new, err := getPodMap(pm)
	// if err != nil {
	// 	klog.Infof("failed to get devices Pod information: %v", err)
	// }

	// if new {
	// 	for name , pod := range pm {
	// 		// If Pod is a new one , initialize it.
	// 		if !pod.initFlag {

	// 			// If Cgroup Path doesn't exist, Delete it
	// 			if !CheckPodExists(pod) {
	// 				klog.Infof("Not Yet Create ", name)
	// 				delete(pm, name)
	// 				continue
	// 			}
				 
	// 			// TODO: WE NEED TO CHOOSE RESOURCES
	// 			pod.CI.RNs = defaultResources
	// 			pod.CI.RIs = make(map[string]*ResourceInfo)
	// 			for _, name := range pod.CI.RNs {
	// 				ri := ResourceInfo{name : name,}
	// 				switch name {
	// 				case "CPU":
	// 					ri.Init(name, pod.cpuPath, miliCPU, 1)
	// 				case "GPU":
	// 					ri.Init(name, pod.gpuPath, miliGPU, 3)
	// 				case "RX":
	// 					ri.Init(name, pod.rxPath, miliRX, 0.1)
	// 				}
	// 				pod.CI.RIs[name] = &ri
	// 			}

	// 			pm[name] = pod
	// 		}
	// 	}
	// }			
}

func (m *Monitor) MonitorPod() {

	last := 0.
	last2 := 0.
	for {
		timer1 := time.NewTimer(time.Second * time.Duration(m.config.monitoringPeriod))
		dat, _ := ioutil.ReadFile("/kubeshare/scheduler/total-usage-pod3")
		read_line := strings.TrimSuffix(string(dat), "\n")
		num1, _ := strconv.ParseFloat(read_line, 64)
		dd := m.livePodMap["pod3"]
		dd.CI.RIs["GPU"].acctUsage = append(dd.CI.RIs["GPU"].acctUsage, uint64(num1))
		dd.CI.RIs["GPU"].usage = num1
		dd.CI.RIs["GPU"].avgUsage = (num1 - last)/1000.
		last = num1
		klog.Infof("GPU total usage: %s %s", dd.CI.RIs["GPU"].usage, dd.CI.RIs["GPU"].avgUsage)
		m.livePodMap["pod3"]= dd
	
		dat2, _ := ioutil.ReadFile("/kubeshare/scheduler/total-usage-pod4")
		read_line2 := strings.TrimSuffix(string(dat2), "\n")
		num2, _ := strconv.ParseFloat(read_line2, 64)
		dd2 := m.livePodMap["pod4"]
		dd2.CI.RIs["GPU"].acctUsage = append(dd2.CI.RIs["GPU"].acctUsage, uint64(num2))
		dd2.CI.RIs["GPU"].usage = num2
		dd2.CI.RIs["GPU"].avgUsage = (num2 - last2)/1000.
		last2 = num2
		klog.Infof("GPU total usage: %s %s", dd2.CI.RIs["GPU"].usage, dd2.CI.RIs["GPU"].avgUsage)
		m.livePodMap["pod4"]= dd2
		<-timer1.C
	}
	

	// for name , pod := range m.livePodMap {

	// 	// If Resource Path doesn't exist, Delete it
	// 	if !CheckPodExists(pod) {
	// 		klog.Infof("Completed ", name)
	// 		m.completedPodMap[name] = pod
	// 		delete(pm, name)
	// 		continue
	// 	}
		
	// 	// Monitor Pod
	// 	for _, ri := range pod.CI.RIs {
	// 		ri.UpdateUsage()
	// 	}
		
	// 	pm[name] = pod

	// 	klog.Infof("[",pod.podName,"] : ", pod.CI.RIs["CPU"].Usage(), pod.CI.RIs["CPU"].Limit(), ":", pod.CI.RIs["GPU"].Usage(), pod.CI.RIs["GPU"].Limit(), ":",pod.CI.RIs["RX"].Usage(), pod.CI.RIs["RX"].Limit())
	// }
}

func (m *Monitor) Run(stopCh <-chan struct{}) {

	m.UpdateNewPod()
	go m.MonitorPod()		
	
	klog.Info("Started Monitor")
	<-stopCh
	klog.Info("Shutting down Monitor")
}