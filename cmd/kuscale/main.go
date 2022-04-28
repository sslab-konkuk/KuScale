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
	"os"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"

	// Uncomment the following line to load the gcp plugin (only required to authenticate against GKE clusters).
	// _ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	clientset "github.com/NTHU-LSALAB/KubeShare/pkg/client/clientset/versioned"
	informers "github.com/NTHU-LSALAB/KubeShare/pkg/client/informers/externalversions"
	kubesharecontroller "github.com/sslab-konkuk/KuScale/pkg/kuescalecontroller"
)

type Configuraion struct {
	MonitoringPeriod 	int			
	WindowSize			int			
	NodeName			string
	MonitoringMode		bool
}

var config Configuraion

var (
	masterURL  string
	kubeconfig string
	threadNum  int
)

func init() {
	config.MonitoringPeriod = flag.Int("MonitoringPeriod", 1, "MonitoringPeriod")
	config.WindowSize = flag.Int("WindowSize", 15, "WindowSize")
	config.NodeName = flag.String("NodeName", "node4", "NodeName")
	config.MonitoringMode = flag.BoolVar("MonitoringMode", true, "MonitoringMode")

	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	flag.IntVar(&threadNum, "threadness", 1, "The number of worker threads.")
}


func main() {
	klog.InitFlags(nil)
	flag.Parse()

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()

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

	kubeshareClient, err := clientset.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building example clientset: %s", err.Error())
	}

	if !checkCRD(kubeshareClient) {
		klog.Error("CRD doesn't exist. Exiting")
		os.Exit(1)
	}

	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, time.Second*30)
	kubeshareInformerFactory := informers.NewSharedInformerFactory(kubeshareClient, time.Second*30)

	controller := kubesharecontroller.NewController(kubeClient, kubeshareClient,
		kubeInformerFactory.Core().V1().Pods(),
		kubeshareInformerFactory.Kubeshare().V1().SharePods())

	// notice that there is no need to run Start methods in a separate goroutine. (i.e. go kubeInformerFactory.Start(stopCh)
	// Start method is non-blocking and runs all registered informers in a dedicated goroutine.
	kubeInformerFactory.Start(stopCh)
	kubeshareInformerFactory.Start(stopCh)

	if err = controller.Run(threadNum, stopCh); err != nil {
		klog.Fatalf("Error running controller: %s", err.Error())
	}
}




func checkCRD(kubeshareClientSet *clientset.Clientset) bool {
	_, err := kubeshareClientSet.KubeshareV1().SharePods("").List(metav1.ListOptions{})
	if err != nil {
		klog.Error(err)
		if _, ok := err.(*errors.StatusError); ok {
			if errors.IsNotFound(err) {
				return false
			}
		}
	}
	return true
}
