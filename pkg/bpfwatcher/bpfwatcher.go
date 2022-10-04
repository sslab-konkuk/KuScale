package bpfmonitor

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	bpf "github.com/iovisor/gobpf/bcc"
	kumonitor "github.com/sslab-konkuk/KuScale/pkg/kumonitor"
	"k8s.io/klog"
)

const source string = `
	#include <uapi/linux/ptrace.h>
	struct ebpf_event_t {
			u32 pid;
			u32 type;
			u64 ts;
	} __attribute__((packed));
	BPF_HASH(pidmap, u32);
	BPF_PERF_OUTPUT(ebpf_events);
`

const defaultFuncEbpf string = `
	int TRACE_FUNC(void *ctx) {
			struct ebpf_event_t event = {};
			u32 pid;
			pid = bpf_get_current_pid_tgid();
			u64 *deep;
			deep = pidmap.lookup(&pid);
			// if(deep == NULL){
				u64 delta = 1;
				pidmap.update(&pid, &delta);
				event.pid = pid;
				event.type = TRACE_NUMBER;
				event.ts = bpf_ktime_get_ns();
				ebpf_events.perf_submit(ctx, &event, sizeof(event));
			// }
			return 0;
	}
`

type cuLaunchKernelEvent struct {
	Pid  uint32
	Type uint32
	Ts   uint64
}

type BpfWatcher struct {
	tracePID []uint32
}

func NewbpfWatcher() *BpfWatcher {
	bpfWatcher := BpfWatcher{}

	return &bpfWatcher
}

func (bw *BpfWatcher) Run(m *kumonitor.Monitor, stopCh <-chan struct{}) {

	savelogs := []string{}

	funcNames := []string{
		"cuMemAlloc_v2",
		"cuMemAllocManaged",
		"cuMemAllocPitch_v2",
		"cuMemFree_v2",
		"cuArrayCreate_v2",
		"cuArray3DCreate_v2",
		"cuArrayDestroy",
		"cuMipmappedArrayCreate",
		"cuMipmappedArrayDestroy",
		"cuLaunchKernel",
		"cuLaunchCooperativeKernel",
		"cuMemGetInfo_v2",
		"cuCtxSynchronize",
		"cuMemcpyAtoH_v2",
		"cuMemcpyDtoH_v2",
		"cuMemcpyHtoA_v2",
		"cuMemcpyHtoD_v2"}

	ebpfSource := source
	for i, name := range funcNames {
		traceName := "trace_" + name
		a := strings.Replace(defaultFuncEbpf, "TRACE_FUNC", traceName, 1)
		b := strings.Replace(a, "TRACE_NUMBER", fmt.Sprintf("%d", i), 1)
		ebpfSource = ebpfSource + b
	}
	// fmt.Print(ebpfSource)

	bpfModule := bpf.NewModule(ebpfSource, []string{})
	defer bpfModule.Close()

	for _, name := range funcNames {
		traceName := "trace_" + name
		Uprobe, err := bpfModule.LoadUprobe(traceName)
		if err != nil {
			klog.V(4).Infof("Failed to load cuLaunchKernel: %s\n", err)
			os.Exit(1)
		}

		err = bpfModule.AttachUprobe("/kubeshare/library/libgemhook.so.1", name, Uprobe, -1)
		if err != nil {
			klog.V(4).Infof("Failed to attach return_value: %s\n", err)
			os.Exit(1)
		}
	}
	// err = m.AttachUprobe("/kubeshare/library/libgemhook.so.1", "cuLaunchKernel", Uprobe, -1)
	//_Z22cuLaunchKernel_prehookP9CUfunc_stjjjjjjjP11CUstream_stPPvS4_
	// err = m.AttachUprobe("/kubeshare/library/libgemhook.so.1", "_Z22cuLaunchKernel_prehookP9CUfunc_stjjjjjjjP11CUstream_stPPvS4_", Uprobe, -1)
	//_Z22cuLaunchKernel_prehookP9CUfunc_stjjjjjjjP11CUstream_stPPvS4_

	table := bpf.NewTable(bpfModule.TableId("ebpf_events"), bpfModule)

	channel := make(chan []byte)

	perfMap, err := bpf.InitPerfMap(table, channel, nil)
	if err != nil {
		klog.V(4).Infof("Failed to init perf map: %s\n", err)
		os.Exit(1)
	}

	klog.V(4).Infof("Insert Ebpef")
	var first cuLaunchKernelEvent
	first.Type = 0
	count := 0
	go func() {
		var event cuLaunchKernelEvent
		for {
			data := <-channel
			err := binary.Read(bytes.NewBuffer(data), binary.LittleEndian, &event)
			if err != nil {
				klog.V(4).Infof("failed to decode received data: %s\n", err)
				continue
			}
			// klog.V(4).Infof("%ld Detected %d's First cuLaunchKernel [ID : %s]", event.Ts, event.Pid, bw.getIDfromPID(event.Pid))
			if count == 0 {
				first = event
				count = 1
			} else if first.Type == event.Type {
				count = count + 1
			} else {

				// fmt.Print(first.Ts, ":", funcNames[first.Type], ":", count, "\n")
				// savelogs = append(savelogs, fmt.Sprint(first.Ts, ":", funcNames[first.Type], ":", count, "\n"))
				if first.Ts != 0 {
					savelogs = append(savelogs, fmt.Sprint(first.Ts, ",0,0,", funcNames[first.Type], ":", count, "\n"))
				}
				first = event
				count = 1
				if event.Type == 12 {
					m.MontiorAllPods()
				}
			}

		}
	}()

	perfMap.Start()
	klog.V(4).Info("Starting bpfWatcher")
	<-stopCh
	for _, t := range savelogs {
		fmt.Print(t)
	}
	perfMap.Stop()
	klog.V(4).Info("Shutting bpfWatcher Down")
}

func (bw *BpfWatcher) getIDfromPID(pid uint32) string {
	data, _ := ioutil.ReadFile(fmt.Sprintf("/home/proc/%d/cgroup", pid))
	lines := strings.Split(string(data), "\n")
	for i := range lines {
		if strings.Contains(lines[i], "cpu") {
			docker := strings.Split(lines[i], "-")
			pidString := strings.Split(docker[1], ".")
			klog.V(5).Info(pidString[0])
			return pidString[0]
		}
	}
	return ""
}
