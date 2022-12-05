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

	"github.com/NTHU-LSALAB/KubeShare/pkg/signals"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"

	// kucontroller "github.com/sslab-konkuk/KuScale/pkg/kucontroller"

	bpfwatcher "github.com/sslab-konkuk/KuScale/pkg/bpfwatcher"
	kuexporter "github.com/sslab-konkuk/KuScale/pkg/kuexporter"
	"github.com/sslab-konkuk/KuScale/pkg/kumonitor"
	kutokenmanager "github.com/sslab-konkuk/KuScale/pkg/kutokenmanager"
	// "github.com/sslab-konkuk/KuScale/pkg/kumonitor/docker"
)

var (
	masterURL  string
	kubeconfig string
	nodeName   string

	threadNum        int64
	monitoringPeriod int64
	windowSize       int64

	monitoringMode bool
	exporterMode   bool
	bpfwatcherMode bool

	staticV float64
)

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&nodeName, "NodeName", "node4", "NodeName")

	flag.Int64Var(&threadNum, "threadness", 1, "The number of worker threads.")
	flag.Int64Var(&monitoringPeriod, "MonitoringPeriod", 2, "MonitoringPeriod")
	flag.Int64Var(&windowSize, "WindowSize", 15, "WindowSize")

	flag.BoolVar(&monitoringMode, "MonitoringMode", true, "MonitoringMode")
	flag.BoolVar(&exporterMode, "exporterMode", true, "exporterMode")
	flag.BoolVar(&bpfwatcherMode, "bpfwatcherMode", false, "bpfwatcherMode")

	flag.Float64Var(&staticV, "staticV", 0, "Static V Weight") //TODO: Need to Remove the static V
}

func main() {
	klog.InitFlags(nil)
	flag.Parse()
	stopCh := signals.SetupSignalHandler()

	// Run Ku Monitor
	monitor := kumonitor.NewMonitor(monitoringPeriod, windowSize, nodeName, monitoringMode, staticV, stopCh)
	go monitor.Run(stopCh)

	// Run Promethuse Exporter
	if exporterMode {
		go kuexporter.ExporterRun(monitor, nodeName, stopCh)
	}

	// Run Ku BPF Watcher
	if bpfwatcherMode {
		bpfWatcher := bpfwatcher.NewbpfWatcher()
		go bpfWatcher.Run(monitor, stopCh)
	}

	// Run KU Device Plugin
	tokenManager := kutokenmanager.NewKuTokenManager(
		"kuscale.com/token", 10,
		pluginapi.DevicePluginPath+"dorry-token.sock")
	go tokenManager.Run(stopCh)

	// if false {
	// 	cfg, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
	// 	if err != nil {
	// 		klog.Fatalf("Error building kubeconfig: %s", err.Error())
	// 	}
	// 	cfg.QPS = 1024.0
	// 	cfg.Burst = 1024

	// 	kubeClient, err := kubernetes.NewForConfig(cfg)
	// 	if err != nil {
	// 		klog.Fatalf("Error building kubernetes clientset: %s", err.Error())
	// 	}

	// 	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, time.Second*30)

	// 	controller := kucontroller.NewController(kubeClient, kubeInformerFactory.Core().V1().Pods(), monitor)

	// 	// notice that there is no need to run Start methods in a separate goroutine. (i.e. go kubeInformerFactory.Start(stopCh)
	// 	// Start method is non-blocking and runs all registered informers in a dedicated goroutine.
	// 	kubeInformerFactory.Start(stopCh)

	// 	if err = controller.Run(threadNum, stopCh); err != nil {
	// 		klog.Fatalf("Error running controller: %s", err.Error())
	// 	}
	// }

	klog.V(4).Info("Started Kuscale")
	<-stopCh
	klog.V(4).Info("Shutting All Down")
	// monitor.WaitAllContainers()
	time.Sleep(time.Second * 2)
	klog.V(4).Info("Shutted All Down")

}
