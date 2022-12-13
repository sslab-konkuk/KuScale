package kucontroller

import (
	"fmt"
	"strconv"

	// "strings"
	// "time"

	//appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	// "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	// "k8s.io/apimachinery/pkg/labels"
	// "k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	// "k8s.io/apimachinery/pkg/util/wait"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"

	// kubesharev1 "github.com/NTHU-LSALAB/KubeShare/pkg/apis/kubeshare/v1"
	// clientset "github.com/NTHU-LSALAB/KubeShare/pkg/client/clientset/versioned"
	kubesharescheme "github.com/NTHU-LSALAB/KubeShare/pkg/client/clientset/versioned/scheme"
	// informers "github.com/NTHU-LSALAB/KubeShare/pkg/client/informers/externalversions/kubeshare/v1"
	kumonitor "github.com/sslab-konkuk/KuScale/pkg/kumonitor"
)

const controllerAgentName = "kubeshare-controller"

const (
	// SuccessSynced is used as part of the Event 'reason' when a SharePod is synced
	SuccessSynced = "Synced"
	// ErrResourceExists is used as part of the Event 'reason' when a SharePod fails
	// to sync due to a Deployment of the same name already existing.
	ErrResourceExists = "ErrResourceExists"

	ErrValueError = "ErrValueError"

	// MessageResourceExists is the message used for Events when a resource
	// fails to sync due to a Deployment already existing
	MessageResourceExists = "Resource %q already exists and is not managed by SharePod"
	// MessageResourceSynced is the message used for an Event fired when a SharePod
	// is synced successfully
	MessageResourceSynced = "SharePod synced successfully"

	KubeShareLibraryPath = "/kubeshare/library"
	SchedulerIpPath      = KubeShareLibraryPath + "/schedulerIP.txt"
	PodManagerPortStart  = 50050
)

type Controller struct {
	kubeclientset kubernetes.Interface

	podsLister corelisters.PodLister
	podsSynced cache.InformerSynced
	// workqueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	workqueue workqueue.RateLimitingInterface
	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	recorder record.EventRecorder

	km *kumonitor.Monitor
}

// NewController returns a new sample controller
func NewController(
	kubeclientset kubernetes.Interface,
	podInformer coreinformers.PodInformer,
	km *kumonitor.Monitor) *Controller {

	// Create event broadcaster
	// Add sample-controller types to the default Kubernetes Scheme so Events can be
	// logged for sample-controller types.
	utilruntime.Must(kubesharescheme.AddToScheme(scheme.Scheme))

	klog.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	controller := &Controller{
		kubeclientset: kubeclientset,
		podsLister:    podInformer.Lister(),
		podsSynced:    podInformer.Informer().HasSynced,
		workqueue:     workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "SharePods"),
		recorder:      recorder,
		km:            km,
	}

	klog.Info("Setting up event handlers")

	podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.handleObject,
		UpdateFunc: func(old, new interface{}) {
			newDepl := new.(*corev1.Pod)
			oldDepl := old.(*corev1.Pod)
			if newDepl.ResourceVersion == oldDepl.ResourceVersion {
				// Periodic resync will send update events for all known Deployments.
				// Two different versions of the same Deployment will always have different RVs.
				return
			}
			klog.Infof("Pod Informer Update Func with change")
			controller.handleObject(new)
		},
		DeleteFunc: controller.handleObject,
	})

	return controller
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *Controller) Run(threadiness int, stopCh chan string) error {
	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	klog.Info("Starting SharePod controller")

	// Wait for the caches to be synced before starting workers
	klog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.podsSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	klog.Info("Started workers")
	<-stopCh
	klog.Info("Shutting down workers")

	return nil
}

func (c *Controller) handleObject(obj interface{}) {
	var object metav1.Object
	var ok bool
	if object, ok = obj.(metav1.Object); !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("error decoding object, invalid type"))
			return
		}
		object, ok = tombstone.Obj.(metav1.Object)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("error decoding object tombstone, invalid type"))
			return
		}
		klog.V(5).Infof("Recovered deleted object '%s' from tombstone", object.GetName())
	}
	klog.V(5).Infof("Processing object: %s", object.GetName())

	if ownerRef := metav1.GetControllerOf(object); ownerRef != nil {
		// If this object is not owned by a SharePod, we should not do anything more
		// with it.
		if ownerRef.Kind != "SharePod" {
			return
		}

		klog.V(4).Infof("Pod Informer Handler , SharedPod %s", object.GetName())
		klog.V(5).Info("Annotation : ", object.GetAnnotations())

		annotations := object.GetAnnotations()
		klog.V(4).Info("CPU_Limit : ", annotations["kuscale/cpu_limit"], " GPU_Limit : ", annotations["kuscale/gpu_limit"])
		cpu_limit, err := strconv.ParseFloat(annotations["kuscale/cpu_limit"], 64)
		if err != nil || cpu_limit > 600.0 || cpu_limit <= 0.0 {
			klog.V(4).Info("cpu_limit annotations Error , Pod %s", object.GetName())
			return
		}
		gpu_limit, err := strconv.ParseFloat(annotations["kuscale/gpu_limit"], 64)
		if err != nil || gpu_limit > 100.0 || gpu_limit <= 0.0 {
			klog.V(4).Info("gpu_limit annotations Error , Pod %s", object.GetName())
			return
		}
		// conditions := obj.(metav1.Condition)
		// klog.V(4).Info(conditions.Status)

		c.km.UpdateNewPod(object.GetName(), cpu_limit, gpu_limit)

		return
	}
}
