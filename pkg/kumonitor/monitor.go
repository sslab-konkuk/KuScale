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
	"time"
)

type PodMap map[string]*PodInfo

type Monitor struct {
	config          Configuraion
	livePodMap      PodMap
	completedPodMap PodMap
}

func NewMonitor(
	monitoringPeriod, windowSize int,
	nodeName string,
	monitoringMode bool,
	exporterMode bool,
	stopCh <-chan struct{}) *Monitor {

	klog.V(4).Info("Creating New Monitor")
	config := Configuraion{monitoringPeriod, windowSize, nodeName, monitoringMode, exporterMode}
	klog.V(4).Info("Configuration ", config)
	monitor := &Monitor{config: config, livePodMap: make(PodMap), completedPodMap: make(PodMap)}

	return monitor
}

func (m *Monitor) PrintPodList() {
	for name := range m.livePodMap {
		klog.V(5).Info("Live Pod Name: ", name)
	}

	for name := range m.completedPodMap {
		klog.V(5).Info("Completed Pod Name: ", name)
	}
}

func (m *Monitor) UpdateNewPod(podName string, cpuLimit, gpuLimit float64) {

	if _, ok := m.livePodMap[podName]; ok {
		klog.V(4).Info("No Need to Update Live Pod ", podName)
		return
	}

	klog.V(5).Info("UpdateNewPod ", podName)
	podInfo := NewPodInfo(podName, []string{"CPU", "GPU"})
	if podInfo == nil {
		klog.V(4).Info("Not Yet Start No Update")
		return
	}
	podInfo.initCpu = cpuLimit
	podInfo.initGpu = gpuLimit
	podInfo.totalToken = uint64(cpuLimit + 3*gpuLimit)
	m.livePodMap[podName] = podInfo
}

func (m *Monitor) Monitor() {
	klog.V(5).Info("MonitorPod Start")

	for {
		timer1 := time.NewTimer(time.Second * time.Duration(m.config.monitoringPeriod))
		m.MonitorPod()

		// if m.config.monitoring {
		// 	m.autoscale()
		// }
		<-timer1.C
	}
}

func (m *Monitor) MonitorPod() {

	for name, pi := range m.livePodMap {

		// If Resource Path doesn't exist, Delete it
		if !CheckPodExists(pi) {
			if !pi.initFlag {
				klog.V(4).Info("Not Yet Start ", name)
				continue
			}
			klog.Info("Completed ", name)
			m.completedPodMap[name] = pi
			delete(m.livePodMap, name)
			continue
		}
		if !pi.initFlag {
			klog.V(4).Info("Start ", name)
			SetCpuLimit(pi, pi.initCpu)
			pi.RIs["GPU"].SetLimit(pi.initGpu)
			writeGpuGeminiConfig(m.livePodMap)
			pi.initFlag = true
		}

		// Monitor Pod
		for _, ri := range pi.RIs {
			ri.UpdateUsage(name, m.config.monitoringPeriod)
		}

		klog.V(5).Info(pi.podName, " ", pi.RIs["CPU"].Usage(), pi.RIs["GPU"].Usage())
		// klog.V(5).Info("[",pi.podName,"] : ", pi.RIs["CPU"].Usage(), pi.RIs["CPU"].Limit(), ":", pi.RIs["GPU"].Usage(), pi.RIs["GPU"].Limit(), ":",pi.RIs["RX"].Usage(), pi.RIs["RX"].Limit())
		// Update Pod
		m.livePodMap[name] = pi

		m.autoscale()
	}

}

func (m *Monitor) Run(stopCh <-chan struct{}) {

	// m.UpdateNewPod("pod3")
	// m.UpdateNewPod("pod4")
	// m.PrintPodList()

	klog.V(4).Info("Starting Monitor")
	go m.Monitor()
	klog.V(4).Info("Started Monitor")

	// Run Promethuse Exporter
	if m.config.exportMode {
		go ExporterRun(m, stopCh)
	}

	<-stopCh
	klog.V(4).Info("Shutting down Monitor")
}
