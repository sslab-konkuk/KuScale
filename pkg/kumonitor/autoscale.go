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

	"k8s.io/klog"
)

func printMatrix(matrix Matrix) {
	for _, ai := range matrix {
		klog.Info(ai)
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

type Matrix [][]float64
type MatrixInfo struct {
	nmOfPods        int
	nmOfResources   int
	nmOfConditions  int
	totalColumnSize int
	totalRowSize    int
}

func makeMatrix(pm PodInfoMap) (MatrixInfo, Matrix) {

	// Number of Conditions : Token Condition(=nmOfPods) + Max Resource Condition(=nmOfResources) + Min Resource Conditions (=nmOfPods*nmOfResources)
	nmOfPods := len(pm)
	nmOfResources := 2
	nmOfConditions := nmOfPods + nmOfResources + nmOfPods*nmOfResources
	totalColumnSize := nmOfPods*nmOfResources + nmOfConditions
	totalRowSize := totalColumnSize + 1
	matrixInfo := MatrixInfo{nmOfPods, nmOfResources, nmOfConditions, totalColumnSize, totalRowSize}

	/* Make Matrix */
	m := make(Matrix, totalColumnSize)
	for i := 0; i < totalColumnSize; i++ {
		row := make([]float64, totalRowSize)
		m[i] = row
	}

	return matrixInfo, m
}

/* Slack Conditions */
func (m *Monitor) fillSlackConditons(mI MatrixInfo, inputMatrix Matrix, podNmMap PodNmMap) {

	podNm := 0
	nmR := mI.nmOfResources
	tRS := mI.totalRowSize

	V1 := float64(10)
	V2 := float64(1)

	for name, pod := range m.RunningPodMap {
		podNmMap[name] = podNm

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

		podNm = podNm + 1
	}
}

/* Resource Token Conditions */
func (m *Monitor) fillTokenConditons(mI MatrixInfo, inputMatrix Matrix, podNmMap PodNmMap) {

	nmP := mI.nmOfPods
	nmR := mI.nmOfResources
	tRS := mI.totalRowSize

	for podName, pod := range m.RunningPodMap {
		podNm := podNmMap[podName]

		for i, name := range pod.RNs {
			W := pod.RIs[name].Price()
			inputMatrix[nmR*podNm+i][nmP*nmR+podNm] = W
			inputMatrix[nmR*nmP+podNm][nmR*podNm+i] = W
		}

		inputMatrix[nmR*nmP+podNm][tRS-1] = float64(pod.totalToken)
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

	// /* Check Min Resource Condition */
	// for i := 0; i < nmP * nmR; i++ {
	// 	if (result[i] - 10) < -0.1 {
	// 		/* Setup Min Resource Condition */

	// 		inputMatrix[minRow + i][i] = 1
	// 		inputMatrix[minRow + i][tRS-1] = 10.
	// 		inputMatrix[i][minRow + i] = 1
	// 		// goto gauss
	// 	}
	// }
}

func (m *Monitor) fillMaxConditions(mI MatrixInfo, inputMatrix Matrix, resourceNm int) {

	nmP := mI.nmOfPods
	nmR := mI.nmOfResources
	tRS := mI.totalRowSize
	max := []float64{600., 100., 10000.}
	maxResourceRow := nmP*nmR + nmP + resourceNm

	for i := 0; i < nmP; i++ {
		inputMatrix[maxResourceRow][i*nmR+resourceNm] = 1
		inputMatrix[i*nmR+resourceNm][maxResourceRow] = 1
	}
	inputMatrix[maxResourceRow][tRS-1] = max[resourceNm]
}

func checkConditions(mI MatrixInfo, inputMatrix Matrix, result []float64) (int, int, int) {

	nmP := mI.nmOfPods
	nmR := mI.nmOfResources
	sum := make([]float64, nmR)
	max := []float64{600., 100., 10000.}

	for i := 0; i < nmP; i++ {
		for j := 0; j < nmR; j++ {
			nm := i*nmR + j
			sum[j] += result[nm]
			if (result[nm] - 10) < -0.1 {
				klog.Info("Min PodNm := ", i, "Resource := ", defaultResources[j])
				return 1, i, j
			}
		}
	}

	for j := 0; j < nmR; j++ {
		if (sum[j] - max[j]) > 0.1 {
			klog.Info("MAX Resource := ", defaultResources[j])
			return 2, 0, j
		}
	}
	return 0, 0, 0
}

type PodNmMap map[string]int

func (m *Monitor) Autoscale() {
	if len(m.RunningPodMap) == 0 {
		return
	}

	// test := 0
	podNmMap := make(PodNmMap)
	matrixInfo, matrix := makeMatrix(m.RunningPodMap)
	m.fillSlackConditons(matrixInfo, matrix, podNmMap)
	m.fillTokenConditons(matrixInfo, matrix, podNmMap)
	// gauss:
	// printMatrix(m)
	result, err := gaussJordan(matrix, matrixInfo.totalColumnSize, matrixInfo.totalRowSize)
	if err != nil {
		klog.Info("Error = ", err)
	}
	klog.Info("Result = ", result)
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

	m.updatePodInfo(podNmMap, result)
}

func (m *Monitor) updatePodInfo(podNmMap PodNmMap, result []float64) {

	nmOfResources := 2

	for name, pod := range m.RunningPodMap {
		podNm := podNmMap[name]

		if pod.UpdateCount == 0 {
			pod.UpdateCount = 1
			m.RunningPodMap[name] = pod
			continue
		}

		SetCpuLimit(pod, math.Round(result[nmOfResources*podNm]))
		// SetGpuLimit(pod, math.Round(result[nmOfResources*podNm+1]))
		// pod.RIs["GPU"].SetLimit(math.Round(result[nmOfResources*podNm+1]))
		// writeGpuGeminiConfig(m.RunningPodMap)
		// SetRxLimit(&pod, math.Round(result[nmOfResources*podNm+2]))
		// log.Println("[",pod.podName,"]", pod.CI.RIs["CPU"], pod.CI.RIs["GPU"], pod.CI.RIs["RX"])
		pod.UpdateCount = pod.UpdateCount + 1

		m.RunningPodMap[name] = pod
	}

}
