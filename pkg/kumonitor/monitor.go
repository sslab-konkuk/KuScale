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
	"fmt"
	"time"

	"github.com/docker/docker/client"
	kuwatcher "github.com/sslab-konkuk/KuScale/pkg/kuwatcher"
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

	podInfo := NewPodInfo(podName, []ResourceName{"CPU", "GPU"})
	podInfo.RIs["CPU"].initLimit = cpuLimit
	podInfo.RIs["GPU"].initLimit = gpuLimit
	podInfo.reservedToken = uint64(cpuLimit + 3*gpuLimit)
	podInfo.podStatus = PodReady

	podInfo.RIs["CPU"].path, podInfo.RIs["GPU"].path, _ = m.getPath(podName)

	if CheckPodPath(podInfo) {
		podInfo.SetInitLimit()
		podInfo.UpdateUsage()
	}
	m.RunningPodMap[podName] = podInfo
}

func (m *Monitor) Monitoring() {
	klog.V(5).Info("Monitoring Start")

	for !m.stopFlag {
		timer1 := time.NewTimer(time.Second * time.Duration(m.config.monitoringPeriod))
		podName, _ := kuwatcher.Scan()
		if podName != "" {
			m.UpdateNewPod(podName, 600, 100)
		}

		if len(m.RunningPodMap) != 0 {
			m.MonitorPod()
			if !m.config.monitoringMode {
				m.Autoscale()
			}
		}

		<-timer1.C
	}
}

func (m *Monitor) MonitorPod() {

	for name, pi := range m.RunningPodMap {

		if pi.podStatus == PodReady && !CheckPodPath(pi) {

			klog.V(5).Info("Ready but no Path ", name)
			pi.RIs["CPU"].path, pi.RIs["GPU"].path, _ = m.getPath(name)
			m.RunningPodMap[name] = pi
			continue
		} else if pi.podStatus == PodReady && CheckPodPath(pi) {
			klog.V(5).Info("Ready and Start", name)
			pi.UpdateUsage()
			pi.SetInitLimit()
			pi.podStatus = PodRunning
		} else if pi.podStatus == PodRunning && !CheckPodPath(pi) {
			klog.V(5).Info("Completed ", name)
			pi.podStatus = PodFinished
			m.completedPodMap[name] = pi
			delete(m.RunningPodMap, name)
			continue
		}

		pi.UpdateUsage()
		klog.V(10).Info("Usage ", pi.PodName, " ", pi.RIs["CPU"].Usage(), pi.RIs["GPU"].Usage())
		m.RunningPodMap[name] = pi
	}
}

func (m *Monitor) Run(stopCh <-chan struct{}) {
	m.ConnectDocker()

	klog.V(4).Info("Starting Monitor")
	go m.Monitoring()
	klog.V(4).Info("Started Monitor")
	<-stopCh
	m.stopFlag = true

	for _, pi := range m.completedPodMap {
		var prev acctUsageAndTime

		ri := pi.RIs["CPU"]
		for _, timeacct := range ri.test {
			curr := timeacct
			cpupercent := float64(curr.acctUsage-prev.acctUsage) * 100. / float64(curr.timeStamp-prev.timeStamp)
			/* Time, CPU, GPU, cuLaunchKernel, cuCtxSynchronize, cuMemAlloc_v2, cuMemFree_v2*/
			fmt.Print(timeacct.timeStamp, ",", cpupercent, ",0,0,0\n")
			prev = timeacct
		}
		ri = pi.RIs["GPU"]
		for _, timeacct := range ri.test {
			curr := timeacct
			gpupercent := float64(curr.acctUsage-prev.acctUsage) * 100. / float64(curr.timeStamp-prev.timeStamp)
			fmt.Print(timeacct.timeStamp, ",0,", gpupercent, ",0,0\n")
			prev = timeacct
		}

	}

	klog.V(4).Info("Shutting monitor down")
}
