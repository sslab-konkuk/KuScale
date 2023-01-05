package kuwatcher

import (
	"context"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"k8s.io/klog"
)

const (
	socketPath        = "/var/lib/kubelet/pod-resources/kubelet.sock"
	tokenName         = "kuscale.com/token"
	connectionTimeout = 10 * time.Second
)

type podWathcer struct {
	updatedPodMap map[string]string

	ctx context.Context
	cli *client.Client
}

var pw *podWathcer

func (pw *podWathcer) ConnectDocker() {
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic(err)
	}
	pw.ctx, pw.cli = ctx, cli
}

func (pw *podWathcer) WaitContianerStart(vgpuId string, newPodCh chan string) (string, string, string) {

	var containers []types.Container
	var err error

	filters := filters.NewArgs()
	filters.Add("label", "annotation.kuauto.vgpu="+vgpuId)

	for len(containers) == 1 {
		containers, err = pw.cli.ContainerList(pw.ctx, types.ContainerListOptions{Filters: filters})
		if err != nil {
			panic(err) // TODO: erorr handling
		}
	}

	klog.V(5).Info("Found the new container with vgpu ", vgpuId)
	data, err := pw.cli.ContainerInspect(pw.ctx, containers[0].ID)

	if err != nil {
		panic(err)
	}

	var podName, cpuPath, gpuPath string

	for label, value := range data.Config.Labels {
		if label == "io.kubernetes.pod.name" {
			podName = value
			break
		}
	}

	cpuPath = "/home/cgroup/cpu/kubepods.slice/kubepods-besteffort.slice/" + data.HostConfig.CgroupParent + "/docker-" + containers[0].ID + ".scope"

	gpuPath = "/sys/kernel/gpu/IDs/" + vgpuId

	klog.V(5).Info("Cgroup Path:", cpuPath, ",  gpuPath : ", gpuPath)

	return podName, cpuPath, gpuPath
}

/*
Func Name : PodWatcher()
Objective : 1) Initialize Pod Watcher
*/

func PodWatcher(stopCh, tokenReqCh, newPodCh chan string) {

	pw = &podWathcer{updatedPodMap: make(map[string]string)}
	pw.ConnectDocker()
	pw.updatedPodMap = make(map[string]string)

	klog.V(4).Info("Starting PodWatcher")
	for {
		select {
		case <-stopCh:
			klog.V(4).Info("Shutting PodWatcher Down")
			return
		case vgpuId := <-tokenReqCh:
			klog.V(4).Info("Get New Token Request : ", vgpuId)
			go pw.WaitContianerStart(vgpuId)
			newPodCh <- podName
		}
	}
}
