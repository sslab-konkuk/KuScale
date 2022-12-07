package bpfmonitor

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	bpf "github.com/iovisor/gobpf/bcc"
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

type cuEvent struct {
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

func (bw *BpfWatcher) Run(ebpfCh chan string, stopCh <-chan struct{}) {

	klog.V(4).Infof("Run BpfWatcher")

	pidToIdMap := make(map[uint32]string)

	funcNames := []string{"cuLaunchKernel"}

	ebpfSource := source
	for i, name := range funcNames {
		traceName := "trace_" + name
		a := strings.Replace(defaultFuncEbpf, "TRACE_FUNC", traceName, 1)
		b := strings.Replace(a, "TRACE_NUMBER", fmt.Sprintf("%d", i), 1)
		ebpfSource = ebpfSource + b
	}

	bpfModule := bpf.NewModule(ebpfSource, []string{})
	defer bpfModule.Close()

	for _, name := range funcNames {
		traceName := "trace_" + name
		Uprobe, err := bpfModule.LoadUprobe(traceName)
		if err != nil {
			klog.Errorf("Failed to load cuLaunchKernel: %s\n", err)
			os.Exit(1)
		}

		err = bpfModule.AttachUprobe("/kubeshare/library/libgemhook.so.1", name, Uprobe, -1)
		if err != nil {
			klog.Errorf("Failed to attach return_value: %s\n", err)
			os.Exit(1)
		}
	}

	table := bpf.NewTable(bpfModule.TableId("ebpf_events"), bpfModule)

	channel := make(chan []byte)

	perfMap, err := bpf.InitPerfMap(table, channel, nil)
	if err != nil {
		klog.Errorf("Failed to init perf map: %s\n", err)
		os.Exit(1)
	}

	klog.V(4).Infof("Insert Ebpef")
	go func(ebpfCh chan string) {
		var event cuEvent
		for {
			data := <-channel
			err := binary.Read(bytes.NewBuffer(data), binary.LittleEndian, &event)
			if err != nil {
				klog.Errorf("failed to decode received data: %s\n", err)
				continue
			}
			id, ok := pidToIdMap[event.Pid]
			if ok {
				klog.V(4).Info("Found New PID")
				id = getIDfromPID(event.Pid)
				pidToIdMap[event.Pid] = id
			}
			ebpfCh <- id

		}
	}(ebpfCh)

	perfMap.Start()
	klog.V(4).Info("Starting bpfWatcher")
	<-stopCh
	perfMap.Stop()
	klog.V(4).Info("Shutting bpfWatcher Down")
}

func getIDfromPID(pid uint32) string {
	data, _ := ioutil.ReadFile(fmt.Sprintf("/home/proc/%d/cgroup", pid))
	lines := strings.Split(string(data), "\n")
	for i := range lines {
		if strings.Contains(lines[i], "cpu") {
			docker := strings.Split(lines[i], "/")
			dockerID := strings.Split(docker[len(docker)-1], "-")[1][:12]
			return dockerID
		}
	}
	return ""
}
