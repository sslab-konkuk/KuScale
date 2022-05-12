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

type PodMap map[string]*PodInfo

type Monitor struct {
	ctx             context.Context
	cli             *client.Client
	config          Configuraion
	RunningPodMap   PodMap
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
	monitor := &Monitor{config: config, RunningPodMap: make(PodMap), completedPodMap: make(PodMap)}

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
	m.RunningPodMap[podName] = podInfo
	writeGpuGeminiConfig(m.RunningPodMap)
}

func (m *Monitor) Monitoring() {
	klog.V(5).Info("MonitorPod Start")

	for {
		timer1 := time.NewTimer(time.Second * time.Duration(m.config.monitoringPeriod))
		m.MonitorPod()
		if !m.config.monitoring {
			m.Autoscale()
		}
		<-timer1.C
	}
}

func (m *Monitor) MonitorPod() {

	for name, pi := range m.RunningPodMap {

		klog.Info("monitor ", name, pi)

		// if pi.podStatus == PodReady && CheckPodPath(pi) {
		// 	klog.Info("monitor running", name)

		// 	pi.podStatus = PodRunning
		// } else if pi.podStatus == PodRunning && !CheckPodPath(pi) {
		// 	klog.Info("Completed ", name)
		// 	pi.podStatus = PodFinished
		// 	m.completedPodMap[name] = pi
		// 	delete(m.RunningPodMap, name)
		// 	continue
		// }

		// if pi.podStatus != PodRunning {
		// 	klog.Info("monitor notrunning ", name)

		// 	continue
		// }

		// Monitor Pod
		klog.Info("monitor star ", name)

		pi.UpdateUsage(m.config.monitoringPeriod)

		klog.V(5).Info(pi.PodName, " ", pi.RIs["CPU"].Usage(), pi.RIs["GPU"].Usage())
		m.RunningPodMap[name] = pi
	}
}

func (m *Monitor) Run(stopCh <-chan struct{}) {
	m.ConnectDocker()
	go m.UpdateNewPod("pod3", 100.0, 50.0)
	// m.UpdateNewPod("pod3")
	// m.UpdateNewPod("pod4")
	// m.PrintPodList()
	// ctx, cli := ConnectDocker()
	// RunDockerContainer(ctx, cli, "pod5", "guswns531/jobs:matrix-001", 100)

	klog.V(4).Info("Starting Monitor")
	go m.Monitoring()
	klog.V(4).Info("Started Monitor")
	<-stopCh
	klog.V(4).Info("Shutting down Monitor")
}
