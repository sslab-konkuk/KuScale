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
	"fmt"
	"math"
	"time"

	"k8s.io/klog"
)

var defaultResources = []string{"CPU", "GPU", "RX"}

type Matrix [][]float64
type MatrixInfo struct {
	nmOfPods        int
	nmOfResources   int
	nmOfConditions  int
	totalColumnSize int
	totalRowSize    int
	conditionStates []ConditionState
	podNmMap        map[int]string
}

func printMatrix(matrix Matrix) {
	for _, ai := range matrix {
		klog.V(10).Info(ai)
	}
}

func gaussJordan(matrix0 Matrix, columnSize, rowSize int) ([]float64, error) {

	matrix := make(Matrix, columnSize)
	for i := 0; i < columnSize; i++ {
		matrix[i] = make([]float64, rowSize)
		copy(matrix[i], matrix0[i])
	}

	for k := 0; k < columnSize; k++ {

		iMax := 0
		max := -1.

		for i := k; i < columnSize; i++ {
			row := matrix[i]

			s := -1.
			for j := k; j < columnSize; j++ {
				x := math.Abs(row[j])
				if x > s {
					s = x
				}
			}

			if abs := math.Abs(row[k]) / s; abs > max {
				iMax = i
				max = abs
			}
		}

		if matrix[iMax][k] == 0 {
			continue
		}

		matrix[k], matrix[iMax] = matrix[iMax], matrix[k]

		for i := k + 1; i < columnSize; i++ {
			for j := k + 1; j <= rowSize-1; j++ {
				matrix[i][j] -= matrix[k][j] * (matrix[i][k] / matrix[k][k])
			}
			matrix[i][k] = 0
		}
	}

	x := make([]float64, columnSize)
	for i := columnSize - 1; i >= 0; i-- {
		x[i] = matrix[i][rowSize-1]
		if matrix[i][i] == 0 {
			if x[i] != 0 { // There exists no solution
				klog.Info("######### Init Matrix #########")
				printMatrix(matrix0)
				klog.Info("********** No Soultion **********")
				printMatrix(matrix)
				return nil, fmt.Errorf("there exists no solution")
			}
			continue
		}
		for j := i + 1; j < rowSize-1; j++ {
			x[i] -= matrix[i][j] * x[j]
		}
		x[i] /= matrix[i][i]
	}

	return x, nil

}

func makeMatrix(pm PodInfoMap) (MatrixInfo, Matrix) {

	mi := MatrixInfo{}
	mi.podNmMap = make(map[int]string)
	podNm := 0
	for name, pod := range pm {
		if pod.podStatus == PodReady {
			continue
		}
		mi.podNmMap[podNm] = name
		podNm = podNm + 1
	}

	// Number of Conditions : Token Condition(=nmOfPods) + Max Resource Condition(=nmOfResources) + Min Resource Conditions (=nmOfPods*nmOfResources)
	mi.nmOfPods = podNm
	mi.nmOfResources = 2
	mi.nmOfConditions = mi.nmOfPods + mi.nmOfResources + mi.nmOfPods*mi.nmOfResources
	mi.totalColumnSize = mi.nmOfPods*mi.nmOfResources + mi.nmOfConditions
	mi.totalRowSize = mi.totalColumnSize + 1

	/* Make Matrix */
	mt := make(Matrix, mi.totalColumnSize)
	for i := 0; i < mi.totalColumnSize; i++ {
		row := make([]float64, mi.totalRowSize)
		mt[i] = row
	}

	return mi, mt
}

/* Slack Conditions */
func (m *Monitor) fillSlackConditons(mI MatrixInfo, inputMatrix Matrix) {

	nmR := mI.nmOfResources
	tRS := mI.totalRowSize

	V1 := float64(10)
	V2 := float64(1)

	for podNm, name := range mI.podNmMap {
		pod := m.RunningPodMap[name]

		/* SumAvg |A|^2 */
		sumAvg := 0.
		for _, ri := range pod.RIs {
			avg := ri.AvgUsage()
			sumAvg += (avg * avg)
		}

		for i := 0; i < nmR; i++ {
			for j := 0; j < nmR; j++ {
				if i == j {
					/* 2V2 + 2V1 - (2*V1*a_i^2)/sumAvg */
					A := pod.RIs[pod.RNs[i]].AvgUsage()
					inputMatrix[nmR*podNm+i][nmR*podNm+j] = 2 * (V1 + V2 - (V1*A*A)/sumAvg)
				} else {
					/*  -(2*V1*a_i*a_j)/sumAvg */
					Ai := pod.RIs[pod.RNs[i]].AvgUsage()
					Aj := pod.RIs[pod.RNs[j]].AvgUsage()
					inputMatrix[nmR*podNm+i][nmR*podNm+j] = -2 * V1 * Ai * Aj / sumAvg
				}
			}
		}

		/* 2*V2*l_n - q_n*w_n */
		for i, name := range pod.RNs {
			ri := pod.RIs[name]
			L, U, W := ri.Limit(), ri.Usage(), ri.Price()
			S := postive(L - U)
			inputMatrix[nmR*podNm+i][tRS-1] = 2*V2*L - S*W
		}
	}
}

/* Resource Token Conditions */
func (m *Monitor) fillTokenConditons(mI MatrixInfo, inputMatrix Matrix) {

	nmP := mI.nmOfPods
	nmR := mI.nmOfResources
	tRS := mI.totalRowSize

	for podNm, name := range mI.podNmMap {
		pod := m.RunningPodMap[name]

		for i, name := range pod.RNs {
			W := pod.RIs[name].Price()
			inputMatrix[nmR*podNm+i][nmP*nmR+podNm] = W
			inputMatrix[nmR*nmP+podNm][nmR*podNm+i] = W
		}

		inputMatrix[nmR*nmP+podNm][tRS-1] = float64(pod.reservedToken)
	}
}

func (m *Monitor) fillMinConditions(mI MatrixInfo, inputMatrix Matrix, podNm, resourceNm int) {

	nmP := mI.nmOfPods
	nmR := mI.nmOfResources
	tRS := mI.totalRowSize
	minRow := nmP*nmR + nmP + nmR
	i := podNm*nmR + resourceNm

	inputMatrix[minRow+i][i] = 1
	inputMatrix[minRow+i][tRS-1] = 10.
	inputMatrix[i][minRow+i] = 1
}

func (m *Monitor) fillMaxConditions(mI MatrixInfo, inputMatrix Matrix, resourceNm int) {

	nmP := mI.nmOfPods
	nmR := mI.nmOfResources
	tRS := mI.totalRowSize
	max := []float64{600., 100.}
	maxResourceRow := nmP*nmR + nmP + resourceNm

	for i := 0; i < nmP; i++ {
		inputMatrix[maxResourceRow][i*nmR+resourceNm] = 1
		inputMatrix[i*nmR+resourceNm][maxResourceRow] = 1
	}
	inputMatrix[maxResourceRow][tRS-1] = max[resourceNm]
}

type ConditionState struct {
	MinOrMax     int
	ResourceName int
	PodNm        int
}

func checkConditions(mI MatrixInfo, inputMatrix Matrix, result []float64) (int, int, int) {

	nmP := mI.nmOfPods
	nmR := mI.nmOfResources
	sum := make([]float64, nmR)
	max := []float64{600., 100.}

	// Check Min Conditions
	for i := 0; i < nmP; i++ {
		for j := 0; j < nmR; j++ {
			nm := i*nmR + j
			sum[j] += result[nm]
			if (result[nm] - 10) < -0.1 {
				klog.V(4).Info("Checked Min Condition PodNm := ", i, "Resource := ", defaultResources[j])
				mI.conditionStates = append(mI.conditionStates, ConditionState{1, j, i})
			}
		}
	}

	// Check Max Conditions
	for j := 0; j < nmR; j++ {
		if (sum[j] - max[j]) > 0.1 {
			klog.V(4).Info("Checked MAX Condition Resource := ", defaultResources[j])
			mI.conditionStates = append(mI.conditionStates, ConditionState{2, j, -1})
		}
	}

	klog.V(4).Info("Conditions Summary ", mI.conditionStates)
	return 0, 0, 0
}

func makeMatrix2(pm PodInfoMap) (MatrixInfo, Matrix) {

	mi := MatrixInfo{}
	mi.podNmMap = make(map[int]string)
	podNm := 0
	for name, pod := range pm {
		if pod.podStatus == PodReady {
			continue
		}
		mi.podNmMap[podNm] = name
		podNm = podNm + 1
	}

	// Number of Conditions : Token Condition(=nmOfPods) + Max Resource Condition(=nmOfResources) + Min Resource Conditions (=nmOfPods*nmOfResources)
	mi.nmOfPods = podNm
	mi.nmOfResources = 2
	mi.nmOfConditions = mi.nmOfPods + mi.nmOfResources
	mi.totalColumnSize = mi.nmOfPods*mi.nmOfResources + mi.nmOfConditions
	mi.totalRowSize = mi.totalColumnSize + 1

	/* Make Matrix */
	mt := make(Matrix, mi.totalColumnSize)
	for i := 0; i < mi.totalColumnSize; i++ {
		row := make([]float64, mi.totalRowSize)
		mt[i] = row
	}

	return mi, mt
}

func (m *Monitor) fillreservedTokens(mI MatrixInfo, inputMatrix Matrix) {
	nmR := mI.nmOfResources
	tRS := mI.totalRowSize

	V1 := float64(50)

	for podNm, name := range mI.podNmMap {
		pod := m.RunningPodMap[name]
		currentTime := time.Now().UnixNano()
		elaspedTime := currentTime - pod.lastSetTime
		elaspedTimePerSecond := float64(elaspedTime) / 1000000000.
		pod.totalToken = pod.totalToken + float64(pod.reservedToken)*elaspedTimePerSecond
		for _, ri := range pod.RIs {
			L, W := ri.Limit(), ri.Price()
			pod.totalToken = pod.totalToken - W*L*elaspedTimePerSecond
		}
		pod.expectedToken = pod.totalToken + float64(pod.reservedToken)
		m.RunningPodMap[name] = pod
		klog.V(10).Info("Pod: ", name, " Total Token : ", pod.expectedToken)

		for i := 0; i < nmR; i++ {
			for j := 0; j < nmR; j++ {
				if i == j {
					inputMatrix[nmR*podNm+i][nmR*podNm+j] = 2 * V1
				}
			}
		}

		for i, name := range pod.RNs {
			U := pod.RIs[name].Usage()
			W := pod.RIs[name].Price()
			inputMatrix[nmR*podNm+i][tRS-1] = 2*V1*U + pod.totalToken*W
		}
	}
}

/* Resource Token Conditions */
func (m *Monitor) fillTokenConditons2(mI MatrixInfo, inputMatrix Matrix, podNm int) {

	nmP := mI.nmOfPods
	nmR := mI.nmOfResources
	tRS := mI.totalRowSize

	pod := m.RunningPodMap[mI.podNmMap[podNm]]

	for i, name := range pod.RNs {
		W := pod.RIs[name].Price()
		inputMatrix[nmR*podNm+i][nmP*nmR+podNm] = W
		inputMatrix[nmR*nmP+podNm][nmR*podNm+i] = W
	}

	inputMatrix[nmR*nmP+podNm][tRS-1] = float64(pod.expectedToken)

}
func (m *Monitor) checkTotalToken(mI MatrixInfo, result []float64) (bool, int) {

	nmP := mI.nmOfPods
	nmR := mI.nmOfResources

	// Check Min Conditions
	for i := 0; i < nmP; i++ {
		sum := result[i*nmR] + 3*result[i*nmR+1]
		if (sum - m.RunningPodMap[mI.podNmMap[i]].expectedToken) > 0.1 {
			klog.V(10).Info("Exceed Total Token ", sum)
			return true, i
		}
	}
	klog.V(10).Info("Fine Total Token")
	return false, 0
}

func (m *Monitor) Autoscale() {

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

func (m *Monitor) updatePodInfo(mI MatrixInfo, result []float64) {

	nmOfResources := 2

	for podNm, name := range mI.podNmMap {
		pod := m.RunningPodMap[name]
		pod.SetLimit(result[nmOfResources*podNm], result[nmOfResources*podNm+1])
		m.RunningPodMap[name] = pod
	}
}
