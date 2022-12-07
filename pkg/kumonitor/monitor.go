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
	"strings"
	"time"

	"github.com/docker/docker/client"
	"github.com/sslab-konkuk/KuScale/pkg/kuprofiler"
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
	ctx     context.Context
	cli     *client.Client
	config  Configuraion
	staticV float64

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
	staticV float64) *Monitor {

	klog.V(4).Info("Creating New Monitor")
	config := Configuraion{monitoringPeriod, windowSize, nodeName, monitoringMode}
	klog.V(4).Info("Configuration ", config)
	monitor := &Monitor{config: config,
		RunningPodMap:   make(PodInfoMap),
		CompletedPodMap: make(PodInfoMap),
		podIDtoNameMap:  make(PodIDtoNameMap),
		staticV:         staticV}

	return monitor
}

/*
Func Name : FindPodNameById()
Objective : 1) Find the Pod Name in RunningPodMap Using ID
*/
func (m *Monitor) FindPodNameById(id string) *PodInfo {
	for _, pi := range m.RunningPodMap {
		if pi.dockerID == id {
			return pi
		}
	}

	return nil
}

/*
Func Name : UpdateNewPod()
Objective : 1) Initalize New Pod
			2) Pulling and Wait for New Pod
*/
func (m *Monitor) UpdateNewPod(podName string, cpuLimit, gpuLimit float64) {
	if podName == "" {
		return
	}
	// Check This Pod is Not in RunningPodMap
	if _, ok := m.RunningPodMap[podName]; !ok {
		klog.V(4).Info("No Need to Update Live Pod ", podName)
		return
	}

	// Prepare The Pod Info Structure
	podInfo := NewPodInfo(podName, []ResourceName{"CPU", "GPU"})
	podInfo.RIs["CPU"].initLimit = cpuLimit
	podInfo.RIs["GPU"].initLimit = gpuLimit
	podInfo.TokenReservation = 2*cpuLimit + 6*gpuLimit
	podInfo.TokenQueue = 0 // podInfo.TokenReservation / 2

	// Wait for The New Pod to Prepare the Cgroup
	for {
		podInfo.RIs["CPU"].path, podInfo.RIs["GPU"].path, _ = m.getPath(podName)
		if CheckPodPath(podInfo) {
			klog.V(5).Info("Ready and Start", podName)
			docker := strings.Split(podInfo.RIs["CPU"].path, "/")
			podInfo.dockerID = strings.Split(docker[len(docker)-1], "-")[1][:12]
			klog.V(1).Info("New Pod : docker ID : ", podInfo.dockerID)
			podInfo.SetInitLimit()
			podInfo.InitUpdateUsage()
			m.RunningPodMap[podName] = podInfo
			return
		}
		klog.V(10).Info("Not Ready because no Path ", podName)
	}
}

/*
Func Name : PullNewPods()
Objective : 1) Pull New Pods From KuWatcher connected with kubelet
*/
func (m *Monitor) PullNewPods() {
	startTime := kuprofiler.StartTime()
	defer kuprofiler.Record("PullNewPods", startTime)

	podNameList, _ := kuwatcher.Scan()

	for _, name := range podNameList {
		go m.UpdateNewPod(name, 300, 10)
	}
}

/*
Func Name : MontiorAllPods()
Objective : 1) Monitoring the pods in RunningPodMap
			2) Check and Remove Completed Pods
*/
func (m *Monitor) MontiorAllPods() {
	startTime := kuprofiler.StartTime()
	defer kuprofiler.Record("MontiorAllPods", startTime)

	rpmLen := len(m.RunningPodMap)
	monitorCh := make(chan *PodInfo, rpmLen)

	for _, pi := range m.RunningPodMap {
		go func(pi *PodInfo, mch chan *PodInfo) {

			if !CheckPodPath(pi) {
				klog.V(10).Info("Completed ", pi.PodName)
				pi.status = PodCompleted
			} else {
				pi.UpdateUsage()
				if pi.status == PodInitializing {
					pi.status = PodRunning
				}
				klog.V(10).Info("Usage ", pi.PodName, " ", pi.RIs["CPU"].Usage(), pi.RIs["GPU"].Usage())
			}
			mch <- pi
		}(pi, monitorCh)
	}

	for i := 0; i < rpmLen; i++ {
		pi := <-monitorCh
		if pi.status == PodCompleted {
			delete(m.RunningPodMap, pi.PodName)
			m.CompletedPodMap[pi.PodName] = pi
		} else {
			m.RunningPodMap[pi.PodName] = pi
		}
	}
}

func (m *Monitor) Monitoring(ebpfCh chan string, stopCh <-chan struct{}) {
	klog.V(5).Info("Monitoring Start")
	timerCh := time.Tick(time.Second * time.Duration(m.config.monitoringPeriod))
	for {
		select {
		case <-stopCh:
			return
		case <-ebpfCh:
			lastExpiredTime := time.Now().UnixNano()
			klog.V(10).Info("AutoScaling By EBPF")
			// pi := m.FindPodNameById(dockerId)
			// if pi != nil {
			// 	pi.
			// }
			m.PullNewPods()
			m.MontiorAllPods()
			m.Autoscale()
			m.lastUpdatedTime = lastExpiredTime
		case timeTick := <-timerCh:
			m.lastExpiredTime = timeTick.UnixNano()
			m.PullNewPods()
			m.MontiorAllPods()
			m.Autoscale()
			m.lastUpdatedTime = m.lastExpiredTime
		}
	}
}

func (m *Monitor) Run(ebpfCh chan string, stopCh <-chan struct{}) {

	m.ConnectDocker()

	klog.V(4).Info("Starting Monitor")
	go m.Monitoring(ebpfCh, stopCh)
	klog.V(4).Info("Started Monitor")
	<-stopCh
	klog.V(4).Info("Shutting monitor down")
}
