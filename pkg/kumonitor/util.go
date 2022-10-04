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
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"
	"k8s.io/klog"
)

func postive(x float64) float64 {
	if x < 0 {
		return 0
	}
	return x
}

func setFileUint(value uint64, path, file string) {
	err := ioutil.WriteFile(filepath.Join(path, file), []byte(strconv.FormatUint(uint64(value), 10)), os.FileMode(0777))
	if err != nil {
		klog.Infof("%s %d %s %s", err, value, path, file)
	}
}

func parseUint(s string, base, bitSize int) uint64 {
	value, err := strconv.ParseUint(s, base, bitSize)
	if err != nil {
		intValue, intErr := strconv.ParseInt(s, base, bitSize)
		if intErr == nil && intValue < 0 {
			return 0
		} else if intErr != nil && intErr.(*strconv.NumError).Err == strconv.ErrRange && intValue < 0 {
			return 0
		}
		return value
	}
	return value
}

func GetFileParamUint(Path, File string) uint64 {
	contents, err := ioutil.ReadFile(filepath.Join(Path, File))
	if err != nil {
		klog.V(2).Infof("couldn't GetFileParamUint: %s", err)
		return 0
	}
	return parseUint(strings.TrimSpace(string(contents)), 10, 64)
}

func PathExists(path string) bool {
	if _, err := os.Stat(path); err != nil {
		return false
	}
	return true
}

func CheckPodPath(pi *PodInfo) bool {
	for _, ri := range pi.RIs {
		if !PathExists(ri.path) {
			return false
		}
	}
	return true
}

func CalAvg(array []uint64, windowSize int) float64 {

	monitoringCount := len(array) - 1
	if monitoringCount == 0 {
		return 0
	} else if monitoringCount < windowSize {
		windowSize = monitoringCount
	}

	prev := array[monitoringCount-windowSize]
	curr := array[monitoringCount]
	avg := (curr - prev) / uint64(windowSize)
	return float64(avg)
}

func GetMtime() (uint64, error) {
	var ts unix.Timespec

	err := unix.ClockGettime(unix.CLOCK_MONOTONIC, &ts)
	if err != nil {
		return 0, fmt.Errorf("Unable get time: %s", err)
	}

	return uint64(unix.TimespecToNsec(ts)), nil
}

/* Get AcctUsage Functions From Cgroup or GPU Virt */
func GetCpuAcctUsage(cpuPath string) (uint64, uint64) {
	now := uint64(time.Now().UnixNano())
	// now, _ := GetMtime()
	return GetFileParamUint(cpuPath, "/cpuacct.usage"), now
}

func GetGpuAcctUsage(gpuPath string) (uint64, uint64) {
	now := uint64(time.Now().UnixNano())
	// now, _ := GetMtime()
	return GetFileParamUint(gpuPath, "/total_runtime"), now
}

// func GetRxAcctUsage(pi *PodInfo) (uint64) {
// 	its, _ := GetnetworkStats(pi)
// 	return 8 * its[0].RxBytes
// }

/* Get Limit Functions */

// func GetCpuLimitFromFile(pi *PodInfo) uint64 {
// 	return GetFileParamUint(pi.cpuPath, "/cpu.cfs_quota_us") / 1000
// }

// func GetGpuLimitFromFile(pi *PodInfo) uint64 {
// 	return GetFileParamUint(pi.gpuPath, "/quota")
// }

// /* Set Limit Functions */

func UpdateGemini() {
	setFileUint(0, "/sys/kernel/gpu/gemini", "/resource_conf")
}

// func SetCpuLimit(pi *PodInfo, nextCpu float64) {
// 	if nextCpu > 1000 || nextCpu < 0 {
// 		return
// 	}

// 	setFileUint(uint64(nextCpu)*1000, pi.RIs["CPU"].path, "/cpu.cfs_quota_us")
// 	pi.RIs["CPU"].SetLimit(nextCpu)
// }

// func SetGpuLimit(pi *PodInfo, nextGpu float64) {
// 	if nextGpu > 1000 || nextGpu < 0 {
// 		return
// 	}
// 	setFileUint(uint64(nextGpu)*10, pi.RIs["GPU"].path, "/gpu_limit")
// 	setFileUint(uint64(nextGpu)*10, pi.RIs["GPU"].path, "/gpu_request")
// 	UpdateGemini()
// 	pi.RIs["GPU"].SetLimit(nextGpu)
// }

// func SetRxLimit(pi *PodInfo, nextRx float64) {
// 	UpdateIngressQdisc(uint64(nextRx) * miliRX, 2 * uint64(nextRx) * miliRX, pi.interfaceName)
// 	pi.CI.RIs["RX"].SetLimit(nextRx)
// }

// func writeGpuGeminiConfig(RunningPodMap PodInfoMap) {

// 	gpu_config_f, err := os.Create("/kubeshare/scheduler/config/resource.conf")
// 	if err != nil {
// 		klog.Errorf("Error when create config file on path: %s", "/kubeshare/scheduler/config/resource.conf")
// 	}

// 	for name, pod := range RunningPodMap {

// 		// minutil, maxutil, memlimit := pod_config[1], pod_config[2], pod_config[3]
// 		// def := strings.Split(pod_config[0], "/")
// 		// podname := def[1]
// 		// klog.Infof("pod info[%d]: %s, %s, %s, %s, %s", i, def, podname, minutil, maxutil, memlimit)
// 		maxutil := pod.RIs["GPU"].limit / 100
// 		//pod key file
// 		gpu_config_f.WriteString(fmt.Sprintf("[%s]\n", name))
// 		// gpu_config_f.WriteString(fmt.Sprintf("clientid=%d\n", strings.Count(podlist, ",")))
// 		// gpu_config_f.WriteString(fmt.Sprintf("MinUtil=%s\n", minutil))
// 		gpu_config_f.WriteString(fmt.Sprintf("MaxUtil=%f\n", maxutil))
// 		gpu_config_f.WriteString(fmt.Sprintf("MemoryLimit=%s\n", "10240MiB"))
// 	}

// 	gpu_config_f.Sync()
// 	gpu_config_f.Close()
// }

// func GetDevicePodInfoFromKubelet(pm PodMap) (bool, error) {
// 	devicePods, err := getListOfPodsFromKubelet(podsocketPath)
// 	if err != nil {
// 		return false, fmt.Errorf("failed to get devices Pod information: %v", err)
// 	}
// 	new := updatePodMap(pm, *devicePods)

// 	return new, nil
// }

// func getListOfPodsFromKubelet(socket string) (*podresourcesapi.ListPodResourcesResponse, error) {
// 	conn, err := connectToServer(socket)
// 	if err != nil {
// 		return nil, err
// 	}
// 	defer conn.Close()

// 	client := podresourcesapi.NewPodResourcesListerClient(conn)

// 	ctx, cancel := context.WithTimeout(context.Background(), connectionTimeout)
// 	defer cancel()

// 	resp, err := client.List(ctx, &podresourcesapi.ListPodResourcesRequest{})
// 	if err != nil {
// 		return nil, fmt.Errorf("failure getting pod resources %v", err)
// 	}
// 	return resp, nil
// }

// func connectToServer(socket string) (*grpc.ClientConn, error) {
// 	ctx, cancel := context.WithTimeout(context.Background(), connectionTimeout)
// 	defer cancel()

// 	conn, err := grpc.DialContext(ctx, socket, grpc.WithInsecure(), grpc.WithBlock(),
// 		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(podResourcesMaxSize)),
// 		grpc.WithDialer(func(addr string, timeout time.Duration) (net.Conn, error) {
// 			return net.DialTimeout("unix", addr, timeout)
// 		}),
// 	)
// 	if err != nil {
// 		return nil, fmt.Errorf("failure connecting to %s: %v", socket, err)
// 	}
// 	return conn, nil
// }

// func updatePodMap(pm PodMap, devicePods podresourcesapi.ListPodResourcesResponse) bool {
// 	var new bool = false
// 	var tokenSize = uint64(0)

// 	for _, pod := range devicePods.GetPodResources() {
// 		podName := pod.GetName()
// 		if _, ok := pm[podName]; ok {
// 			continue
// 		}

// 		if _, ok := CompletedPodMap[podName]; ok {
// 			continue
// 		}

// 		for _, container := range pod.GetContainers() {
// 			for _, device := range container.GetDevices() {
// 				resourceName := device.GetResourceName()
// 				if resourceName == resourceToken {
// 					tokenSize = tokenSize + 1
// 				}
// 			}
// 			if tokenSize > 0 {
// 				// // println("Pod %s, Container %s ",pod.GetName(), container.GetName(), resourceToken, check)

// 				// PodInfo := PodInfo{
// 				// 	podName:       podName,
// 				// 	namespace:     pod.GetNamespace(),
// 				// 	containerName: container.GetName(),
// 				// 	reservedToken:    tokenSize,
// 				// 	initFlag:      false,
// 				// 	cpuPath:       getCpuPath(podName),
// 				// 	gpuPath:       getGpuPath(podName),
// 				// 	rxPath:        path.Join("/home/proc/", getPid(podName), "/net/dev"),
// 				// 	interfaceName: getInterfaceName(podName),
// 				// 	iterModPath:   getIterModPath(podName),
// 				// }
// 				// pm[podName] = PodInfo
// 				// new = true
// 				// tokenSize = 0
// 			}
// 		}
// 	}

// 	return new
// }
