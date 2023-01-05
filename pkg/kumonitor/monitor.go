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
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
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
	config  Configuraion
	staticV float64
	ctx     context.Context
	cli     *client.Client

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

	var err error
	monitor.ctx = context.Background()
	monitor.cli, err = client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic(err)
	}
	return monitor
}

/*
Func Name : waitContainerStart()
Objective : 1) Wait for new container with vgpuId
			2) Get the information about the new container such as podName, cpuPath, gpuPath
*/
func (m *Monitor) waitContainerStart(vgpuId string) (string, string, string, string) {

	var containers []types.Container
	var err error

	filter := "annotation.kuauto.vgpu=" + vgpuId
	filters := filters.NewArgs()
	filters.Add("label", filter)

	klog.V(10).Info("waitContainerStart vgpuId : ", vgpuId)

	for {
		containers, err = m.cli.ContainerList(m.ctx, types.ContainerListOptions{Filters: filters})
		if err != nil {
			panic(err) // TODO: erorr handling
		}
		if len(containers) != 0 {
			break
		}
	}

	klog.V(5).Info("Found the new container with vgpu ", vgpuId)
	data, err := m.cli.ContainerInspect(m.ctx, containers[0].ID)

	if err != nil {
		panic(err)
	}

	var podName, cpuPath, gpuPath, dockerId string

	for label, value := range data.Config.Labels {
		if label == "io.kubernetes.pod.name" {
			podName = value
			break
		}
	}

	cpuPath = "/home/cgroup/cpu/kubepods.slice/kubepods-besteffort.slice/" + data.HostConfig.CgroupParent + "/docker-" + containers[0].ID + ".scope"
	gpuPath = "/sys/kernel/gpu/IDs/" + vgpuId
	dockerId = containers[0].ID[:12]

	klog.V(5).Info("Cgroup Path:", cpuPath, ",  gpuPath : ", gpuPath)

	return podName, cpuPath, gpuPath, dockerId
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
func (m *Monitor) UpdateNewPod(vgpuNToken string) {
	startTime := kuprofiler.StartTime()
	defer kuprofiler.Record("UpdateNewPod", startTime)

	data := strings.Split(vgpuNToken, ":")
	tokenRes, _ := strconv.ParseFloat(data[1], 64)
	vgpuId := data[0]

	podName, cpuPath, gpuPath, dockerId := m.waitContainerStart(vgpuId)

	// Prepare The Pod Info Structure
	podInfo := NewPodInfo(podName, []ResourceName{"CPU", "GPU"})
	podInfo.dockerID = dockerId
	podInfo.TokenReservation = tokenRes
	podInfo.TokenQueue = 0

	podInfo.RIs["CPU"].path, podInfo.RIs["GPU"].path = cpuPath, gpuPath
	podInfo.CPU().usagePath = podInfo.CPU().path + "/cpuacct.usage"
	podInfo.GPU().usagePath = podInfo.GPU().path + "/total_runtime"

	podInfo.SetInitLimit()
	podInfo.UpdatePodUsage()
	podInfo.UpdatePodUsage()

	klog.V(5).Info("Ready and Start", podName)
	m.RunningPodMap[podName] = podInfo
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
	startTime := kuprofiler.StartTime()
	defer kuprofiler.Record("MonitorAndAutoScale", startTime)

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

	klog.V(4).Info("Starting Monitor")
	timerCh := time.Tick(time.Second * time.Duration(m.config.monitoringPeriod))
	for {
		select {
		case <-stopCh:
			klog.V(4).Info("Shutting monitor down")
			return
		case vgpuNToken := <-newPodCh:
			klog.V(10).Info("Get New PodCh : ", vgpuNToken)
			go m.UpdateNewPod(vgpuNToken)
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
