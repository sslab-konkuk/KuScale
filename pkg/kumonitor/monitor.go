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
	"io/ioutil"
	"strconv"
	"strings"
	"k8s.io/klog"
)


// func appAction(c *cli.Context) error {

// 	config.MonitoringPeriod = 1
// 	config.WindowSize = 15
// 	config.NodeName = c.String("hostname")
// 	config.MonitoringMode = c.Bool("monitoring")

// 	// FS system Watcher 
// 	// klog.Infof("Starting FS watcher.")
// 	// watcher, err := newFSWatcher(pluginapi.DevicePluginPath)
// 	// if err != nil {
// 	// 	return fmt.Errorf("failed to create FS watcher: %v", err)
// 	// }
// 	// defer watcher.Close()

// 	// OS signal Watcher
// 	// klog.Infof("Starting OS watcher.")
// 	sigs := newOSWatcher(syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

// restart:
// 	// Start Monitor Thread
// 	go Routine()

// events:
// 	for {
// 		select {

// 		// case event := <-watcher.Events:
// 		// 	if event.Name == pluginapi.KubeletSocket && event.Op&fsnotify.Create == fsnotify.Create {
// 		// 		klog.Infof("inotify: %s created, restarting.", pluginapi.KubeletSocket)
// 		// 		goto restart
// 		// 	}

// 		// case err := <-watcher.Errors:
// 		// 	klog.Infof("inotify: %s", err)

// 		case s := <-sigs:
// 			switch s {
// 			case syscall.SIGHUP:
// 				klog.Infof("Received SIGHUP, restarting.")
// 				goto restart
// 			default:
// 				klog.Infof("Received signal \"%v\", shutting down.", s)
// 				break events
// 			}
// 		}
// 	}
// 	return nil
// }


func LookUpNewPod (pm PodMap) {

	new, err := getPodMap(pm)
	if err != nil {
		klog.Infof("failed to get devices Pod information: %v", err)
	}

	if new {
		for name , pod := range pm {
			// If Pod is a new one , initialize it.
			if !pod.initFlag {

				// If Cgroup Path doesn't exist, Delete it
				if !CheckPodExists(pod) {
					klog.Infof("Not Yet Create ", name)
					delete(pm, name)
					continue
				}
				 
				// TODO: WE NEED TO CHOOSE RESOURCES
				pod.CI.RNs = defaultResources
				pod.CI.RIs = make(map[string]*ResourceInfo)
				for _, name := range pod.CI.RNs {
					ri := ResourceInfo{name : name,}
					switch name {
					case "CPU":
						ri.Init(name, pod.cpuPath, miliCPU, 1)
					case "GPU":
						ri.Init(name, pod.gpuPath, miliGPU, 3)
					case "RX":
						ri.Init(name, pod.rxPath, miliRX, 0.1)
					}
					pod.CI.RIs[name] = &ri
				}

				pm[name] = pod
			}
		}
	}			
}

func MonitorPod(pm PodMap) {
	
	for name , pod := range pm {

		// If Resource Path doesn't exist, Delete it
		if !CheckPodExists(pod) {
			klog.Infof("Completed ", name)
			CompletedPodMap[name] = pod
			delete(pm, name)
			continue
		}
		
		// Monitor Pod
		for _, ri := range pod.CI.RIs {
			ri.UpdateUsage()
		}
		
		pm[name] = pod

		klog.Infof("[",pod.podName,"] : ", pod.CI.RIs["CPU"].Usage(), pod.CI.RIs["CPU"].Limit(), ":", pod.CI.RIs["GPU"].Usage(), pod.CI.RIs["GPU"].Limit(), ":",pod.CI.RIs["RX"].Usage(), pod.CI.RIs["RX"].Limit())
	}
}

func Routine() {

	config.MonitoringPeriod = 1
	config.WindowSize = 15
	config.NodeName = "node4"
	config.MonitoringMode = false

	// monitoringPeriod := config.MonitoringPeriod
	LivePodMap = make(PodMap)

	podName:= "pod3"
	pod := PodInfo{
		podName:      		podName,
		initFlag : 			false,
		// cpuPath : 			getCpuPath(podName),
		gpuPath : 			"/kubeshare/scheduler",
		// rxPath  : 			path.Join("/home/proc/", getPid(podName), "/net/dev"),
		// interfaceName : 	getInterfaceName(podName),
	}
	pod.CI.RNs = []string{"GPU"}
	pod.CI.RIs = make(map[string]*ResourceInfo)
	for _, name := range pod.CI.RNs {
		ri := ResourceInfo{name : name,}
		switch name {
		case "CPU":
			ri.Init(name, pod.cpuPath, miliCPU, 1)
		case "GPU":
			ri.Init(name, pod.gpuPath, miliGPU, 3)
		}
		ri.UpdateUsage()
		pod.CI.RIs[name] = &ri
	}
    LivePodMap[podName] = pod

	podName= "pod4"
	pod = PodInfo{
		podName:      		podName,
		initFlag : 			false,
		// cpuPath : 			getCpuPath(podName),
		gpuPath : 			"/kubeshare/scheduler",
		// rxPath  : 			path.Join("/home/proc/", getPid(podName), "/net/dev"),
		// interfaceName : 	getInterfaceName(podName),
	}
	pod.CI.RNs = []string{"GPU"}
	pod.CI.RIs = make(map[string]*ResourceInfo)
	for _, name := range pod.CI.RNs {
		ri := ResourceInfo{name : name,}
		switch name {
		case "CPU":
			ri.Init(name, pod.cpuPath, miliCPU, 1)
		case "GPU":
			ri.Init(name, pod.gpuPath, miliGPU, 3)
		}
		ri.UpdateUsage()
		pod.CI.RIs[name] = &ri
	}
    LivePodMap[podName] = pod


	last := 0.
	last2 := 0.
	for {
		// timer1 := time.NewTimer(time.Second * time.Duration(float64(monitoringPeriod)))
		tt := 1.0
		timer1 := time.NewTimer(time.Second * time.Duration(tt))

		// LookUpNewPod(LivePodMap)
		// MonitorPod(LivePodMap)		

		dat, _ := ioutil.ReadFile("/kubeshare/scheduler/total-usage-pod3")
		read_line := strings.TrimSuffix(string(dat), "\n")
		num1, _ := strconv.ParseFloat(read_line, 64)
		dd := LivePodMap["pod3"]
		dd.CI.RIs["GPU"].acctUsage = append(dd.CI.RIs["GPU"].acctUsage, uint64(num1))
		dd.CI.RIs["GPU"].usage = num1
		dd.CI.RIs["GPU"].avgUsage = (num1 - last)/1000.
		last = num1
		klog.Infof("GPU total usage: ", dd.CI.RIs["GPU"].usage, dd.CI.RIs["GPU"].avgUsage)
		LivePodMap["pod3"]= dd

		dat2, _ := ioutil.ReadFile("/kubeshare/scheduler/total-usage-pod4")
		read_line2 := strings.TrimSuffix(string(dat2), "\n")
		num2, _ := strconv.ParseFloat(read_line2, 64)
		dd2 := LivePodMap["pod4"]
		dd2.CI.RIs["GPU"].acctUsage = append(dd2.CI.RIs["GPU"].acctUsage, uint64(num2))
		dd2.CI.RIs["GPU"].usage = num2
		dd2.CI.RIs["GPU"].avgUsage = (num2 - last2)/1000.
		last2 = num2
		klog.Infof("GPU total usage: ", dd2.CI.RIs["GPU"].usage, dd2.CI.RIs["GPU"].avgUsage)
		LivePodMap["pod4"]= dd2

		<-timer1.C
	}
}