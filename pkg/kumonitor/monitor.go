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

type PodMap map[string]*PodInfo

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
    m.livePodMap[podName] = NewPodInfo(podName, []string{"CPU","GPU"})
	podName = "pod4"
    m.livePodMap[podName] = NewPodInfo(podName, []string{"CPU","GPU"})
	
	// if _, ok := m.livePodMap[podName]; ok   {
	// 	return
	// }
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

func (m *Monitor) Monitor() {
	for {
		timer1 := time.NewTimer(time.Second * time.Duration(m.config.monitoringPeriod))
		klog.V(5).Info("Monitor Start")
		m.MonitorPod()
		<-timer1.C
	}
}

func (m *Monitor) MonitorPod() {

	klog.V(5).Info("MonitorPod Start")

	for name , pod := range m.livePodMap {

		// klog.V(5).Info("MonitorPod Name: ", name)

		// If Resource Path doesn't exist, Delete it
		if !CheckPodExists(pod) {
			klog.Info("Completed ", name)
			m.completedPodMap[name] = pod
			delete(m.livePodMap, name)
			continue
		}
		
		// Monitor Pod
		for _, ri := range pod.CI.RIs {
			ri.UpdateUsage(name, m.config.monitoringPeriod)
		}
		
		m.livePodMap[name] = pod
		
		// klog.Info(pod)
		klog.Info(pod.podName, " ", pod.CI.RIs["CPU"].Usage(), pod.CI.RIs["GPU"].Usage())
		// klog.V(5).Info("[",pod.podName,"] : ", pod.CI.RIs["CPU"].Usage(), pod.CI.RIs["CPU"].Limit(), ":", pod.CI.RIs["GPU"].Usage(), pod.CI.RIs["GPU"].Limit(), ":",pod.CI.RIs["RX"].Usage(), pod.CI.RIs["RX"].Limit())
	}
}

func (m *Monitor) Run(stopCh <-chan struct{}) {

	m.UpdateNewPod()
	for name , _ := range m.livePodMap {
		klog.V(5).Info("Run Name: ", name)
	}
	go m.Monitor()		
	
	klog.Info("Started Monitor")
	<-stopCh
	klog.Info("Shutting down Monitor")
}