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
	if _, ok := m.RunningPodMap[podName]; ok {
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
			podInfo.CPU().usagePath = podInfo.CPU().path + "/cpuacct.usage"
			podInfo.GPU().usagePath = podInfo.GPU().path + "/total_runtime"
			docker := strings.Split(podInfo.RIs["CPU"].path, "/")
			podInfo.dockerID = strings.Split(docker[len(docker)-1], "-")[1][:12]
			klog.V(1).Info("New Pod : docker ID : ", podInfo.dockerID)
			podInfo.SetInitLimit()
			podInfo.UpdatePodUsage()
			m.RunningPodMap[podName] = podInfo
			return
		}
		klog.V(10).Info("Not Ready because no Path ", podName)
	}
}

/*
Func Name : MontiorAllPods()
	Objective :
	1) Monitoring the pods in RunningPodMap
	2) Check and Remove Completed Pods
*/
func (m *Monitor) MontiorAllPods() {
	// startTime := kuprofiler.StartTime()
	// defer kuprofiler.Record("MontiorAllPods", startTime)

	rpmLen := len(m.RunningPodMap)
	monitorCh := make(chan *PodInfo, rpmLen)

	for _, pi := range m.RunningPodMap {
		go func(pi *PodInfo, mch chan *PodInfo) {

			if !CheckPodPath(pi) {
				klog.V(10).Info("Completed ", pi.PodName)
				pi.status = PodCompleted
			} else {
				pi.UpdatePodUsage()
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
			startTime := kuprofiler.StartTime()
			delete(m.RunningPodMap, pi.PodName)
			m.CompletedPodMap[pi.PodName] = pi
			defer kuprofiler.Record("PodCompleted", startTime)
		} else {
			m.RunningPodMap[pi.PodName] = pi
		}
	}
}

/*
Func Name : MonitorPod(pi *PodInfo)
	Objective :
	1) Monitoring the pods in RunningPodMap
	2) Check and Remove Completed Pods
*/
func updatePod(pi *PodInfo) {
	if pi.status == PodInitializing {
		pi.status = PodRunning
	}
	pi.lastElaspedTime = float64(time.Now().UnixNano()-pi.lastUpdatedTime) / 1000000000.
	pi.UpdatePodUsage()
	pi.UpdateTokenQueue()
}

/*
Func Name : MonitorAndAutoScale()
	Objective :
	1) Monitoring the pods in RunningPodMap
	2) Check and Remove Completed Pods
*/
func (m *Monitor) MonitorAndAutoScale() {
	// startTime := kuprofiler.StartTime()
	// defer kuprofiler.Record("MonitorAndAutoScale", startTime)

	/* Return If there is no pods in RunningPodMap */
	if len(m.RunningPodMap) == 0 {
		return
	}
	/* Monitor and Update Pod */
	for _, pi := range m.RunningPodMap {
		updatePod(pi)
		if pi.status == PodCompleted {
			delete(m.RunningPodMap, pi.PodName)
			m.CompletedPodMap[pi.PodName] = pi
		} else {
			m.RunningPodMap[pi.PodName] = pi
		}
	}

	/* Get Next Limit in Simple conditions */
	for _, pi := range m.RunningPodMap {
		pi.UpdateDynamicWeight(m.staticV)
		pi.getNextLimit(float64(m.config.monitoringPeriod))
		pi.setNextLimit()
		m.RunningPodMap[pi.PodName] = pi
	}

	// matrixInfo, matrix := makeMatrix(m.RunningPodMap)
	// if matrixInfo.nmOfPods == 0 {
	// 	klog.V(4).Info("matrixInfo.nmOfPods is Zero")
	// 	return
	// }
	// m.fill(matrixInfo, matrix)
	// result, _ := gaussJordan(matrix, matrixInfo.totalColumnSize, matrixInfo.totalRowSize)
	// klog.V(10).Info("Result = ", result)
	// m.updatePodInfo(matrixInfo, result)
}

func (m *Monitor) Run(stopCh, ebpfCh, newPodCh chan string) {

	m.ConnectDocker()

	klog.V(4).Info("Starting Monitor")
	timerCh := time.Tick(time.Second * time.Duration(m.config.monitoringPeriod))
	for {
		select {
		case <-stopCh:
			klog.V(4).Info("Shutting monitor down")
			return
		case podName := <-newPodCh:
			klog.V(10).Info("Get New PodCh : ", podName)
			go m.UpdateNewPod(podName, 300, 10)
		case <-ebpfCh:
			klog.V(10).Info("MonitorAndAutoScale By EBPF")
			m.MonitorAndAutoScale()
			kuprofiler.RecordEnd("SchedulingLatency")
		case <-timerCh:
			klog.V(10).Info("MonitorAndAutoScale By Timer")
			m.MonitorAndAutoScale()
		}
	}
}
