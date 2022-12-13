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

// func (m *Monitor) Autoscale2() {

// 	matrixInfo, matrix := makeMatrix(m.RunningPodMap)
// 	if matrixInfo.nmOfPods == 0 {
// 		klog.V(4).Info("matrixInfo.nmOfPods is Zero")
// 		return
// 	}

// 	m.fillreservedTokens(matrixInfo, matrix)

// 	// m.fillSlackConditons(matrixInfo, matrix)
// 	// m.fillTokenConditons(matrixInfo, matrix)

// gauss:
// 	printMatrix(matrix)

// 	result, err := gaussJordan(matrix, matrixInfo.totalColumnSize, matrixInfo.totalRowSize)
// 	if err != nil {
// 		klog.Info("Error = ", err)
// 	}
// 	klog.Info("Result = ", result)

// 	t, k := m.checkTotalToken(matrixInfo, result)
// 	if t == true {
// 		m.fillTokenConditons2(matrixInfo, matrix, k)
// 		goto gauss
// 	}

// 	// checkConditions(matrixInfo, matrix, result)
// 	// ret, podNm, resourceNm := checkConditions(matrixInfo, matrix, result)
// 	// if test == 6 {
// 	// 	klog.Info("TOO Error")
// 	// } else if ret == 1 {
// 	// 	m.fillMinConditions(matrixInfo, matrix, podNm, resourceNm)
// 	// 	test += 1
// 	// 	goto gauss
// 	// } else if ret == 2 {
// 	// 	m.fillMaxConditions(matrixInfo, matrix, resourceNm)
// 	// 	test += 1
// 	// 	goto gauss
// 	// }

// 	m.updatePodInfo(matrixInfo, result)
// }

// func (m *Monitor) DriftPlutPenaltyAlgorithm() {

// 	matrixInfo, matrix := makeMatrix(m.RunningPodMap)
// 	if matrixInfo.nmOfPods == 0 {
// 		klog.V(4).Info("matrixInfo.nmOfPods is Zero")
// 		return
// 	}
// }
