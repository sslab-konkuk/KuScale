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

	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"

	"github.com/NTHU-LSALAB/KubeShare/pkg/signals"

	kucontroller "github.com/sslab-konkuk/KuScale/pkg/kucontroller"
	kumonitor "github.com/sslab-konkuk/KuScale/pkg/kumonitor"
)

var (
	masterURL        string
	kubeconfig       string
	threadNum        int
	monitoringPeriod int
	windowSize       int
	nodeName         string
	monitoringMode   bool
	exporterMode     bool
)

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	flag.IntVar(&threadNum, "threadness", 1, "The number of worker threads.")

	flag.IntVar(&monitoringPeriod, "MonitoringPeriod", 1, "MonitoringPeriod")
	flag.IntVar(&windowSize, "WindowSize", 15, "WindowSize")
	flag.StringVar(&nodeName, "NodeName", "node4", "NodeName")
	flag.BoolVar(&monitoringMode, "MonitoringMode", true, "MonitoringMode")
	flag.BoolVar(&exporterMode, "exporterMode", true, "exporterMode")
}

func main() {
	klog.InitFlags(nil)
	flag.Parse()

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()

	monitor := kumonitor.NewMonitor(monitoringPeriod, windowSize, nodeName, monitoringMode, exporterMode, stopCh)
	go monitor.Run(stopCh)

	cfg, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
	if err != nil {
		klog.Fatalf("Error building kubeconfig: %s", err.Error())
	}
	cfg.QPS = 1024.0
	cfg.Burst = 1024

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building kubernetes clientset: %s", err.Error())
	}

	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, time.Second*30)

	controller := kucontroller.NewController(kubeClient, kubeInformerFactory.Core().V1().Pods(), monitor)

	// notice that there is no need to run Start methods in a separate goroutine. (i.e. go kubeInformerFactory.Start(stopCh)
	// Start method is non-blocking and runs all registered informers in a dedicated goroutine.
	kubeInformerFactory.Start(stopCh)

	if err = controller.Run(threadNum, stopCh); err != nil {
		klog.Fatalf("Error running controller: %s", err.Error())
	}
}
