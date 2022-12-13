/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"flag"
	"time"

	"k8s.io/klog"

	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"

	// kucontroller "github.com/sslab-konkuk/KuScale/pkg/kucontroller"

	kuexporter "github.com/sslab-konkuk/KuScale/pkg/kuexporter"
	"github.com/sslab-konkuk/KuScale/pkg/kumonitor"
	kuprofiler "github.com/sslab-konkuk/KuScale/pkg/kuprofiler"
	kutokenmanager "github.com/sslab-konkuk/KuScale/pkg/kutokenmanager"
	kuwatcher "github.com/sslab-konkuk/KuScale/pkg/kuwatcher"
)

var (
	nodeName         string
	monitoringPeriod int64
	windowSize       int64

	monitoringMode bool
	exporterMode   bool
	bpfwatcherMode bool

	staticV float64
)

func init() {
	flag.StringVar(&nodeName, "NodeName", "node4", "NodeName")

	flag.Int64Var(&monitoringPeriod, "MonitoringPeriod", 2, "MonitoringPeriod")
	flag.Int64Var(&windowSize, "WindowSize", 15, "WindowSize")

	flag.BoolVar(&monitoringMode, "MonitoringMode", false, "MonitoringMode")
	flag.BoolVar(&exporterMode, "exporterMode", false, "exporterMode")
	flag.BoolVar(&bpfwatcherMode, "bpfwatcherMode", true, "bpfwatcherMode")

	flag.Float64Var(&staticV, "staticV", 10, "Static V Weight")
}

func main() {
	klog.InitFlags(nil)
	flag.Parse()
	kuprofiler.NewLatencyInfo(true)
	tokenReqCh := make(chan string, 10)
	newPodCh := make(chan string, 10)

	/* Run Signal Watcher */
	stopCh := kuwatcher.SignalWatcher()

	/* Run Ku Pod Watcher */
	go kuwatcher.PodWatcher(stopCh, tokenReqCh, newPodCh)

	// Run Ku BPF Watcher
	ebpfCh := make(chan string, 1000)
	if bpfwatcherMode {
		go kuwatcher.BpfWatcher(ebpfCh, stopCh)
	}

	// Run Ku Monitor
	monitor := kumonitor.NewMonitor(monitoringPeriod, windowSize, nodeName, monitoringMode, staticV)
	go monitor.Run(stopCh, ebpfCh, newPodCh)

	// Run Promethuse Exporter
	if exporterMode {
		go kuexporter.ExporterRun(monitor, nodeName, stopCh)
	}

	// Run KU Device Plugin
	tokenManager := kutokenmanager.NewKuTokenManager(
		"kuscale.com/token", 10,
		pluginapi.DevicePluginPath+"dorry-token.sock")
	go tokenManager.Run(stopCh, tokenReqCh)

	klog.V(4).Info("Started Kuscale")
	<-stopCh
	klog.V(4).Info("Shutting All Down")
	// monitor.WaitAllContainers()
	kuprofiler.Summary()
	time.Sleep(time.Second * 2)
	klog.V(4).Info("Shutted All Down")
}
