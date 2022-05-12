package kumonitor

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"

	// "github.com/docker/docker/api/types/filters"
	"io"
	"os"

	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"k8s.io/klog"
)

// https://pkg.go.dev/github.com/docker/docker/api/types/container#HostConfig
// https://github.com/moby/moby/blob/4433bf67ba0a3f686ffffce04d0709135e0b37eb/api/types/container/config.go#L43
// https://github.com/hashicorp/waypoint/blob/main/builtin/docker/platform.go

func (m *Monitor) ConnectDocker() {
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic(err)
	}
	m.ctx, m.cli = ctx, cli
}

func (m *Monitor) RunNewContainer(podInfo *PodInfo) {

	out, err := m.cli.ImagePull(m.ctx, podInfo.imageName, types.ImagePullOptions{})
	if err != nil {
		panic(err)
	}
	defer out.Close()
	io.Copy(os.Stdout, out)

	hostConfig := container.HostConfig{}
	var mounts []mount.Mount
	mount1 := mount.Mount{
		Type:   mount.TypeBind,
		Source: "/kubeshare/library/",
		Target: "/kubeshare/library/",
	}
	mounts = append(mounts, mount1)
	mount2 := mount.Mount{
		Type:   mount.TypeBind,
		Source: "/kubeshare/scheduler/ipc/",
		Target: "/kubeshare/scheduler/ipc/",
	}
	mounts = append(mounts, mount2)
	hostConfig.Mounts = mounts
	cpu := podInfo.RIs["CPU"].limit
	if cpu > 0 {
		var resources container.Resources
		resources.CPUShares = int64(cpu / 100 * 1024)
		resources.CPUQuota = int64(cpu * 10000)
		resources.CPUPeriod = 1000000
		hostConfig.Resources = resources
	}

	resp, err := m.cli.ContainerCreate(m.ctx, &container.Config{
		Image: podInfo.imageName,
		Cmd:   []string{"./matrix", "2048", "1000"},
		Tty:   false,
		Env: []string{"LD_PRELOAD=/kubeshare/library/libgemhook.so.1",
			"LD_LIBRARY_PATH=/kubeshare/library/:$LD_LIBRARY_PATH",
			"GEMINI_IPC_DIR=/kubeshare/scheduler/ipc/",
			fmt.Sprintf("GEMINI_GROUP_NAME=%s", podInfo.PodName)},
		Labels: map[string]string{
			"owner": "kuscale",
		},
	}, &hostConfig, nil, nil, "")

	if err != nil {
		panic(err)
	}

	if err := m.cli.ContainerStart(m.ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		panic(err)
	}

	klog.V(5).Info("Created Container: ", podInfo.PodName, " ID: ", resp.ID)
	podInfo.ID = resp.ID
	podInfo.RIs["CPU"].path = getCpuPath(podInfo.ID)
	podInfo.RIs["GPU"].path = fmt.Sprintf("/kubeshare/scheduler/total-usage-%s", podInfo.PodName)

	// statusCh, errCh := m.cli.ContainerWait(m.ctx, resp.ID, container.WaitConditionNotRunning)
	// select {
	// case err := <-errCh:
	// 	if err != nil {
	// 		panic(err)
	// 	}
	// case <-statusCh:
	// }
}

// func getCpuPath(ctx context.Context, cli *client.Client, podName string) string {
func getCpuPath(ID string) string {
	cgroupPath := "/home/cgroup/cpu/system.slice/docker-" + ID + ".scope"
	// cgroupPath := "/sys/fs/cgroup/cpu/system.slice/docker-" + ID + ".scope"
	klog.V(5).Info("getCPUPath ", ID, " ", cgroupPath)
	return cgroupPath
}

// func getCpuPath(ctx context.Context, cli *client.Client, podName string) string {
// filterlabel := "io.kubernetes.pod.name=" + podName
// filterlabel2 := "io.kubernetes.docker.type=container"
// filters := filters.NewArgs()
// filters.Add("label", filterlabel)
// filters.Add("label", filterlabel2)

// containers, err := cli.ContainerList(ctx, types.ContainerListOptions{Filters: filters})
// if err != nil {
// 	panic(err)
// }

// if len(containers) == 0 {
// 	klog.V(5).Info("getCPUPath , no path ", podName)
// 	return ""
// }

// data, err := cli.ContainerInspect(ctx, containers[0].ID)

// if err != nil {
// 	panic(err)
// }

// cgroupPath := "/home/cgroup/cpu/kubepods.slice/kubepods-besteffort.slice/" + data.HostConfig.CgroupParent + "/docker-" + containers[0].ID + ".scope"
// 		klog.V(5).Info("getCPUPath ", ID, " ", cgroupPath)

// 	return cgroupPath
// }

// func getGpuPath(podName string) string {
// 	ctx := context.Background()
// 	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
// 	if err != nil {
// 		panic(err)
// 	}
// 	filterlabel := "io.kubernetes.pod.name=" + podName
// 	filters := filters.NewArgs()
// 	filters.Add("label", filterlabel)

// 	containers, err := cli.ContainerList(ctx, types.ContainerListOptions{Filters: filters})
// 	if err != nil {
// 		panic(err)
// 	}

// 	if len(containers) == 0 {
// 		return ""
// 	}

// 	data, err := cli.ContainerInspect(ctx, containers[0].ID)
// 	if err != nil {
// 		panic(err)
// 	}
// 	var gpuPath string

// 	for _, m := range data.Mounts {
// 		if m.Destination == "/home/gpu" {
// 			// gpuPath = "/home/" + m.Source[12:]
// 			gpuPath = m.Source
// 		}
// 	}
// 	// fmt.Println(data.Mounts[0].Source)

// 	return gpuPath
// }

// func getPid(podName string) string {
// 	ctx := context.Background()
// 	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
// 	if err != nil {
// 		panic(err)
// 	}
// 	filterlabel := "io.kubernetes.pod.name=" + podName
// 	filters := filters.NewArgs()
// 	filters.Add("label", filterlabel)

// 	containers, err := cli.ContainerList(ctx, types.ContainerListOptions{Filters: filters})
// 	if err != nil {
// 		panic(err)
// 	}

// 	if len(containers) == 0 {
// 		return ""
// 	}

// 	data, err := cli.ContainerInspect(ctx, containers[0].ID)
// 	if err != nil {
// 		panic(err)
// 	}

// 	pid := data.State.Pid
// 	// fmt.Println(pid)
// 	// enterNsPrintFromPid(pid)

// 	return strconv.Itoa(pid)
// }

// func getInterfaceName(podName string) string {
// 	ctx := context.Background()
// 	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
// 	if err != nil {
// 		panic(err)
// 	}
// 	filterlabel := "io.kubernetes.pod.name=" + podName
// 	filters := filters.NewArgs()
// 	filters.Add("label", filterlabel)

// 	containers, err := cli.ContainerList(ctx, types.ContainerListOptions{Filters: filters})
// 	if err != nil {
// 		panic(err)
// 	}

// 	if len(containers) == 0 {
// 		return ""
// 	}

// 	data, err := cli.ContainerInspect(ctx, containers[0].ID)
// 	if err != nil {
// 		panic(err)
// 	}
// 	var interfacePath string
// 	found := 0
// 	for _, m := range data.Mounts {
// 		if m.Destination == "/etc/hosts" {
// 			// gpuPath = "/home/" + m.Source[12:]
// 			interfacePath = m.Source
// 			found = 1
// 			// klog.Infof("FOUND")
// 		}
// 	}
// 	if found == 0 {
// 		// klog.Infof("NOT FOUND")
// 		return ""
// 	}
// 	IP := getIPFromFile(interfacePath, podName)
// 	index := getIfindexFromRoute(IP)
// 	name := getIfnameByIndex(index)

// 	// fmt.Println(data.Mounts[0].Source)
// 	return name
// }

// func getIPFromFile(path, podName string) string {
// 	file, err := os.Open(path)
//     if err != nil {
//         log.Fatal(err)
//     }
//     defer file.Close()

//     scanner := bufio.NewScanner(file)
//     for scanner.Scan() {
//         // fmt.Println(scanner.Text())
// 		slice := strings.Split(scanner.Text(), "\t")
// 		for _, str := range slice {
// 			// fmt.Println(str)
// 			if str == podName {
// 				// klog.Infof("Found IP :", slice[0])
// 				return slice[0]
// 			}
// 		}
//     }

//     if err := scanner.Err(); err != nil {
//         log.Fatal(err)
//     }
// 	return ""
// }
