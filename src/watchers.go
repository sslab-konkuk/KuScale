package main

import (

	"os"
	"os/signal"
	"github.com/fsnotify/fsnotify"

	// "context"
	// "fmt"
	// "net"
	// "time"
	// "path"
	// "google.golang.org/grpc"
	// podresourcesapi "k8s.io/kubelet/pkg/apis/podresources/v1alpha1"
)

func newFSWatcher(files ...string) (*fsnotify.Watcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	for _, f := range files {
		err = watcher.Add(f)
		if err != nil {
			watcher.Close()
			return nil, err
		}
	}

	return watcher, nil
}

func newOSWatcher(sigs ...os.Signal) chan os.Signal {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, sigs...)

	return sigChan
}

func getPodMap(pm PodMap) (bool, error) {
	// devicePods, err := getListOfPodsFromKubelet(podsocketPath)
	// if err != nil {
	// 	return false, fmt.Errorf("failed to get devices Pod information: %v", err)
	// }
	// new := updatePodMap(pm, *devicePods)
	// return new, nil
	return false, nil
}

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

// func updatePodMap(pm PodMap, devicePods podresourcesapi.ListPodResourcesResponse) (bool) {
// 	var new bool = false
// 	var tokenSize = uint64(0)

// 	for _, pod := range devicePods.GetPodResources() {
// 		podName := pod.GetName()
// 		if _, ok := pm[podName]; ok   {
// 			continue
// 		}

// 		if _, ok := CompletedPodMap[podName]; ok   {
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
// 				// println("Pod %s, Container %s ",pod.GetName(), container.GetName(), resourceToken, check)
				
// 				PodInfo := PodInfo{
// 					podName:      		podName,
// 					namespace: 			pod.GetNamespace(),
// 					containerName: 		container.GetName(),
// 					totalToken : 		tokenSize,
// 					initFlag : 			false,
// 					cpuPath : 			getCpuPath(podName),
// 					gpuPath : 			getGpuPath(podName),
// 					rxPath  : 			path.Join("/home/proc/", getPid(podName), "/net/dev"),
// 					interfaceName : 	getInterfaceName(podName),
// 					iterModPath : 		getIterModPath(podName),
// 				}
// 				pm[podName] = PodInfo
// 				new = true
// 				tokenSize = 0
// 			}
// 		}
// 	}



// 	return new
// }
