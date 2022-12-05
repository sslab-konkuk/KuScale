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
	monitoringPeriod int64
	windowSize       int64
	nodeName         string
	monitoringMode   bool
}

type PodInfoMap map[string]*PodInfo
type PodIDtoNameMap map[string]string

type Monitor struct {
	ctx      context.Context
	cli      *client.Client
	config   Configuraion
	staticV  float64
	stopFlag bool

	NotReadyPodMap  PodInfoMap
	RunningPodMap   PodInfoMap
	CompletedPodMap PodInfoMap
	podIDtoNameMap  PodIDtoNameMap

	lastExpiredTime int64 // Last Expired Time form Monitor Timer
	lastUpdatedTime int64 // Last Updated Time from KuScale
}

func NewMonitor(
	monitoringPeriod, windowSize int64,
	nodeName string,
	monitoringMode bool,
	staticV float64,
	stopCh <-chan struct{}) *Monitor {

	klog.V(4).Info("Creating New Monitor")
	config := Configuraion{monitoringPeriod, windowSize, nodeName, monitoringMode}
	klog.V(4).Info("Configuration ", config)
	monitor := &Monitor{config: config,
		NotReadyPodMap:  make(PodInfoMap),
		RunningPodMap:   make(PodInfoMap),
		CompletedPodMap: make(PodInfoMap),
		podIDtoNameMap:  make(PodIDtoNameMap),
		staticV:         staticV,
		stopFlag:        false}

	return monitor
}

func (m *Monitor) PrintPodList() {
	for name := range m.RunningPodMap {
		klog.V(5).Info("Live Pod Name: ", name)
	}

	for name := range m.CompletedPodMap {
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
	// TODO: Need To change
	podInfo.TokenReservation = 2*cpuLimit + 6*gpuLimit
	podInfo.TokenQueue = 0
	// podInfo.TokenQueue = podInfo.TokenReservation / 2
	podInfo.podStatus = PodNotReady

	podInfo.RIs["CPU"].path, podInfo.RIs["GPU"].path, _ = m.getPath(podName)

	if CheckPodPath(podInfo) {
		klog.V(5).Info("Ready and Start", podName)
		podInfo.SetInitLimit()
		podInfo.InitUpdateUsage()
		podInfo.podStatus = PodRunning
		m.RunningPodMap[podName] = podInfo
		return
	}
	m.NotReadyPodMap[podName] = podInfo
}

func (m *Monitor) Monitoring() {
	klog.V(5).Info("Monitoring Start")

	for !m.stopFlag {
		m.lastExpiredTime = time.Now().UnixNano()

		monitorTimer := time.NewTimer(time.Second * time.Duration(m.config.monitoringPeriod))

		podName, _ := kuwatcher.Scan()
		if podName != "" {
			// m.UpdateNewPod(podName, 50, 10)
			m.UpdateNewPod(podName, 300, 10)
		}

		m.MontiorAllPods()
		m.CheckNotReadyPods()

		// if !m.config.monitoringMode {
		m.Autoscale()
		// }

		m.lastUpdatedTime = m.lastExpiredTime
		<-monitorTimer.C
	}
}

func (m *Monitor) CheckNotReadyPods() {

	for name, pi := range m.NotReadyPodMap {

		pi.RIs["CPU"].path, pi.RIs["GPU"].path, _ = m.getPath(name)

		if CheckPodPath(pi) {
			klog.V(5).Info("Ready and Start", name)
			pi.SetInitLimit()
			pi.InitUpdateUsage()
			pi.podStatus = PodRunning
			delete(m.NotReadyPodMap, name)
			pi.InitUpdateUsage()
			m.RunningPodMap[name] = pi
			continue
		}
		klog.V(5).Info("Not Ready because no Path ", name)
	}
}

func (m *Monitor) MontiorAllPods() {

	for name, pi := range m.RunningPodMap {
		if !CheckPodPath(pi) {
			klog.V(5).Info("Completed ", name)
			pi.podStatus = PodCompleted
			delete(m.RunningPodMap, name)
			m.CompletedPodMap[name] = pi
			continue
		}
		pi.UpdateUsage()
		m.RunningPodMap[name] = pi
		klog.V(10).Info("Usage ", pi.PodName, " ", pi.RIs["CPU"].Usage(), pi.RIs["GPU"].Usage())
	}
}

func (m *Monitor) Run(stopCh <-chan struct{}) {
	m.ConnectDocker()

	klog.V(4).Info("Starting Monitor")
	go m.Monitoring()
	klog.V(4).Info("Started Monitor")
	<-stopCh
	m.stopFlag = true
	klog.V(4).Info("Shutting monitor down")
}

func (m *Monitor) printAcct() {
	for _, pi := range m.CompletedPodMap {
		var prev AcctUsageAndTime

		ri := pi.RIs["CPU"]
		for _, timeacct := range ri.acctUsageAndTime {
			curr := timeacct
			cpupercent := float64(curr.acctUsage-prev.acctUsage) * 100. / float64(curr.timeStamp-prev.timeStamp)
			/* Time, CPU, GPU, cuLaunchKernel, cuCtxSynchronize, cuMemAlloc_v2, cuMemFree_v2*/
			fmt.Print(timeacct.timeStamp, ",", cpupercent, ",0,0,0\n")
			prev = timeacct
		}
		ri = pi.RIs["GPU"]
		for _, timeacct := range ri.acctUsageAndTime {
			curr := timeacct
			gpupercent := float64(curr.acctUsage-prev.acctUsage) * 100. / float64(curr.timeStamp-prev.timeStamp)
			fmt.Print(timeacct.timeStamp, ",0,", gpupercent, ",0,0\n")
			prev = timeacct
		}

	}
}
