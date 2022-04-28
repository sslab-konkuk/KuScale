package kuscale

// import (
// 	"context"

// 	"strconv"
// 	"bufio"
// 	"os"
// 	"log"
// 	"strings"

// 	"github.com/docker/docker/api/types"
// 	"github.com/docker/docker/api/types/filters"
// 	"github.com/docker/docker/client"
// )

// func getCpuPath(podName string) string {
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
	
// 	cgroupPath := "/home/cgroup/cpu/kubepods.slice/kubepods-besteffort.slice/" + data.HostConfig.CgroupParent + "/docker-" + containers[0].ID + ".scope"
	
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