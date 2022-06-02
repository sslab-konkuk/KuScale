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
	"context"
	"time"

	"github.com/docker/docker/client"
	"k8s.io/klog"
)

type Configuraion struct {
	monitoringPeriod int
	windowSize       int
	nodeName         string
	monitoringMode   bool
}

type PodInfoMap map[string]*PodInfo
type PodIDtoNameMap map[string]string

type Monitor struct {
	ctx             context.Context
	cli             *client.Client
	config          Configuraion
	RunningPodMap   PodInfoMap
	completedPodMap PodInfoMap
	podIDtoNameMap  PodIDtoNameMap
	stopFlag        bool
}

func NewMonitor(
	monitoringPeriod, windowSize int,
	nodeName string,
	monitoringMode bool,
	stopCh <-chan struct{}) *Monitor {

	klog.V(4).Info("Creating New Monitor")
	config := Configuraion{monitoringPeriod, windowSize, nodeName, monitoringMode}
	klog.V(4).Info("Configuration ", config)
	monitor := &Monitor{config: config, RunningPodMap: make(PodInfoMap), completedPodMap: make(PodInfoMap), podIDtoNameMap: make(PodIDtoNameMap), stopFlag: false}

	return monitor
}

func (m *Monitor) PrintPodList() {
	for name := range m.RunningPodMap {
		klog.V(5).Info("Live Pod Name: ", name)
	}

	for name := range m.completedPodMap {
		klog.V(5).Info("Completed Pod Name: ", name)
	}
}

func (m *Monitor) UpdateNewPod(podName string, cpuLimit, gpuLimit float64) {

	if _, ok := m.RunningPodMap[podName]; ok {
		klog.V(4).Info("No Need to Update Live Pod ", podName)
		return
	}

	klog.V(5).Info("UpdateNewPod ", podName)
	podInfo := NewPodInfo(podName, []ResourceName{"CPU", "GPU"})

	podInfo.RIs["CPU"].initLimit = cpuLimit
	podInfo.RIs["GPU"].initLimit = gpuLimit
	podInfo.totalToken = uint64(cpuLimit + 3*gpuLimit)

	podInfo.RIs["CPU"].SetLimit(cpuLimit)
	podInfo.RIs["GPU"].SetLimit(gpuLimit)

	podInfo.imageName = "guswns531/jobs:matrix-001"
	m.RunNewContainer(podInfo)
	podInfo.podStatus = PodReady
	podInfo.UpdateUsage(m.config.monitoringPeriod)
	m.RunningPodMap[podName] = podInfo
	writeGpuGeminiConfig(m.RunningPodMap)
}

func (m *Monitor) Monitoring() {
	klog.V(5).Info("Monitoring Start")

	for !m.stopFlag {
		timer1 := time.NewTimer(time.Second * time.Duration(m.config.monitoringPeriod))
		m.MonitorPod()
		if !m.config.monitoringMode {
			m.Autoscale()
		}
		<-timer1.C
	}
}

func (m *Monitor) MonitorPod() {

	for name, pi := range m.RunningPodMap {

		if pi.podStatus == PodReady && CheckPodPath(pi) {
			pi.podStatus = PodRunning
		} else if pi.podStatus == PodRunning && !CheckPodPath(pi) {
			klog.V(4).Info("Completed ", name)
			pi.podStatus = PodFinished
			m.completedPodMap[name] = pi
			delete(m.RunningPodMap, name)
			continue
		}

		if pi.podStatus != PodRunning {
			continue
		}

		// Monitor Pod
		pi.UpdateUsage(m.config.monitoringPeriod)

		klog.V(5).Info(pi.PodName, " ", pi.RIs["CPU"].Usage(), pi.RIs["GPU"].Usage())
		klog.V(5).Infof("%s, %.4f %.4f", pi.PodName, pi.RIs["CPU"].getCurrentUsage(), pi.RIs["GPU"].getCurrentUsage())
		m.RunningPodMap[name] = pi
	}
}

func (m *Monitor) Run(stopCh <-chan struct{}) {
	m.ConnectDocker()
	go m.UpdateNewPod("pod3", 100.0, 50.0)
	go m.UpdateNewPod("pod4", 100.0, 50.0)

	klog.V(4).Info("Starting Monitor")
	go m.Monitoring()
	klog.V(4).Info("Started Monitor")
	<-stopCh
	m.stopFlag = true
	klog.V(4).Info("Shutting monitor down")
}
