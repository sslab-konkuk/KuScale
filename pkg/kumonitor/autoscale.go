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

	"github.com/sslab-konkuk/KuScale/pkg/kuprofiler"
	"k8s.io/klog"
)

func (m *Monitor) Autoscale2() {

	matrixInfo, matrix := makeMatrix(m.RunningPodMap)
	if matrixInfo.nmOfPods == 0 {
		klog.V(4).Info("matrixInfo.nmOfPods is Zero")
		return
	}

	m.fillreservedTokens(matrixInfo, matrix)

	// m.fillSlackConditons(matrixInfo, matrix)
	// m.fillTokenConditons(matrixInfo, matrix)

gauss:
	printMatrix(matrix)

	result, err := gaussJordan(matrix, matrixInfo.totalColumnSize, matrixInfo.totalRowSize)
	if err != nil {
		klog.Info("Error = ", err)
	}
	klog.Info("Result = ", result)

	t, k := m.checkTotalToken(matrixInfo, result)
	if t == true {
		m.fillTokenConditons2(matrixInfo, matrix, k)
		goto gauss
	}

	// checkConditions(matrixInfo, matrix, result)
	// ret, podNm, resourceNm := checkConditions(matrixInfo, matrix, result)
	// if test == 6 {
	// 	klog.Info("TOO Error")
	// } else if ret == 1 {
	// 	m.fillMinConditions(matrixInfo, matrix, podNm, resourceNm)
	// 	test += 1
	// 	goto gauss
	// } else if ret == 2 {
	// 	m.fillMaxConditions(matrixInfo, matrix, resourceNm)
	// 	test += 1
	// 	goto gauss
	// }

	m.updatePodInfo(matrixInfo, result)
}

func (m *Monitor) Autoscale() {
	startTime := kuprofiler.StartTime()
	defer kuprofiler.Record("Autoscale", startTime)

	m.updateAllpods()
}

func (m *Monitor) updatePodInfo(mI MatrixInfo, result []float64) {

	nmOfResources := 2

	for podNm, name := range mI.podNmMap {
		pod := m.RunningPodMap[name]
		pod.SetLimit(result[nmOfResources*podNm], result[nmOfResources*podNm+1])
		m.RunningPodMap[name] = pod
	}
}

// updateAllpods
func (m *Monitor) updateAllpods() {
	elaspedTimePerSecond := float64(time.Now().UnixNano()-m.lastUpdatedTime) / 1000000000.
	elaspedTimePerSecond = float64(m.config.monitoringPeriod)
	cpu := []float64{0., 0., 0., 0.}
	gpu := []float64{0., 0., 0., 0.}

	podNm := 0
	for name, pi := range m.RunningPodMap {
		pi.UpdateTokenQueue(elaspedTimePerSecond)
		pi.UpdateDynamicWeight(m.staticV)
		cpu[podNm], gpu[podNm] = m.getNextLimit(elaspedTimePerSecond, pi)
		m.RunningPodMap[name] = pi
		podNm += 1
	}

	//
	if gpu[0]+gpu[1] < 100 {
		podNm = 0
		for name, pi := range m.RunningPodMap {
			pi.SetLimit(cpu[podNm], gpu[podNm])
			m.RunningPodMap[name] = pi
			podNm += 1
		}
	} else {
		matrixInfo, matrix := makeMatrix(m.RunningPodMap)
		if matrixInfo.nmOfPods == 0 {
			klog.V(4).Info("matrixInfo.nmOfPods is Zero")
			return
		}
		m.fill(matrixInfo, matrix)
		printMatrix(matrix)
		result, _ := gaussJordan(matrix, matrixInfo.totalColumnSize, matrixInfo.totalRowSize)
		klog.V(10).Info("Result = ", result)
		m.updatePodInfo(matrixInfo, result)
	}
}

func (m *Monitor) getNextLimit(elaspedTimePerSecond float64, pi *PodInfo) (float64, float64) {

	/*
		TokenQueue := pod.TokenQueue
		var next []float64
		i := 0
		for _, ri := range pod.RIs {
			usage, weight, price := ri.Usage(), ri.DynamicWeight(), ri.Price()
			next[i] = usage * price * TokenQueue / (2 * weight)
			i += 1
		}
	*/

	remainedTimePerSecond := float64(m.lastExpiredTime)/1000000000. + float64(m.config.monitoringPeriod) - elaspedTimePerSecond
	remainedTimePerSecond = float64(m.config.monitoringPeriod)

	k := 1.
	pi.availableToken = k*pi.TokenQueue + pi.TokenReservation*remainedTimePerSecond
	// Need To Change
	pi.availableToken = pi.TokenReservation * remainedTimePerSecond

	/*** Caclulate Next Limit wihtout Any Conditions ***/
	cpuRI, gpuRI := pi.RIs["CPU"], pi.RIs["GPU"]
	cpuNextLimit := cpuRI.usage + cpuRI.price*pi.availableToken/(2*cpuRI.dynamicWeight)
	gpuNextLimit := gpuRI.usage + gpuRI.price*pi.availableToken/(2*gpuRI.dynamicWeight)
	klog.V(10).Info("Pod(", pi.PodName, ")'s remainedTimePerSecond : ", remainedTimePerSecond)

	tokenCondition := pi.availableToken - (cpuRI.price*cpuNextLimit+gpuRI.price*gpuNextLimit)*remainedTimePerSecond
	klog.V(4).Info("Pod(", pi.PodName, ")'s Next Reseravation :", int64(cpuNextLimit), " , ", int64(gpuNextLimit), " Token Enough : tokenCondition : ", int64(tokenCondition))
	if tokenCondition >= 0 {
		return cpuNextLimit, gpuNextLimit
	}

	/*** Caclulate Next Limit wiht Token Conditions ***/
	up := pi.availableToken/remainedTimePerSecond - cpuRI.price*cpuRI.usage - gpuRI.price*gpuRI.usage
	below := gpuRI.dynamicWeight*cpuRI.price*cpuRI.price + cpuRI.dynamicWeight*gpuRI.price*gpuRI.price
	cpuNextLimit = cpuRI.usage + cpuRI.price*gpuRI.dynamicWeight*up/below
	gpuNextLimit = gpuRI.usage + gpuRI.price*cpuRI.dynamicWeight*up/below
	tokenCondition = pi.availableToken - (cpuRI.price*cpuNextLimit+gpuRI.price*gpuNextLimit)*remainedTimePerSecond

	klog.V(4).Info("Pod(", pi.PodName, ")'s Next Reseravation :", int64(cpuNextLimit), " , ", int64(gpuNextLimit), " Token Not Enough : tokenCondition : ", int64(tokenCondition))

	return cpuNextLimit, gpuNextLimit
}

// func (m *Monitor) DriftPlutPenaltyAlgorithm() {

// 	matrixInfo, matrix := makeMatrix(m.RunningPodMap)
// 	if matrixInfo.nmOfPods == 0 {
// 		klog.V(4).Info("matrixInfo.nmOfPods is Zero")
// 		return
// 	}
// }
