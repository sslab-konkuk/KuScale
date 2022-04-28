package kuescalecontroller

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	//appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	// "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"

	kubesharev1 "github.com/NTHU-LSALAB/KubeShare/pkg/apis/kubeshare/v1"
	clientset "github.com/NTHU-LSALAB/KubeShare/pkg/client/clientset/versioned"
	kubesharescheme "github.com/NTHU-LSALAB/KubeShare/pkg/client/clientset/versioned/scheme"
	informers "github.com/NTHU-LSALAB/KubeShare/pkg/client/informers/externalversions/kubeshare/v1"
	listers "github.com/NTHU-LSALAB/KubeShare/pkg/client/listers/kubeshare/v1"
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
	kubeclientset      kubernetes.Interface
	kubeshareclientset clientset.Interface

	podsLister      corelisters.PodLister
	podsSynced      cache.InformerSynced
	sharepodsLister listers.SharePodLister
	sharepodsSynced cache.InformerSynced

	// workqueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	workqueue workqueue.RateLimitingInterface
	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	recorder record.EventRecorder
}

// NewController returns a new sample controller
func NewController(
	kubeclientset kubernetes.Interface,
	kubeshareclientset clientset.Interface,
	podInformer coreinformers.PodInformer,
	kubeshareInformer informers.SharePodInformer) *Controller {

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
		kubeclientset:      kubeclientset,
		kubeshareclientset: kubeshareclientset,
		podsLister:         podInformer.Lister(),
		podsSynced:         podInformer.Informer().HasSynced,
		sharepodsLister:    kubeshareInformer.Lister(),
		sharepodsSynced:    kubeshareInformer.Informer().HasSynced,
		workqueue:          workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "SharePods"),
		recorder:           recorder,
	}

	klog.Info("Setting up event handlers")
	// Set up an event handler for when SharePod resources change
	kubeshareInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueSharePod,
		UpdateFunc: func(old, new interface{}) {
			controller.enqueueSharePod(new)
		},
		DeleteFunc: controller.handleDeletedSharePod,
	})
	// Set up an event handler for when Deployment resources change. This
	// handler will lookup the owner of the given Deployment, and if it is
	// owned by a SharePod resource will enqueue that SharePod resource for
	// processing. This way, we don't need to implement custom logic for
	// handling Deployment resources. More info on this pattern:
	// https://github.com/kubernetes/community/blob/8cafef897a22026d42f5e5bb3f104febe7e29830/contributors/devel/controllers.md
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
func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	klog.Info("Starting SharePod controller")

	// Wait for the caches to be synced before starting workers
	klog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.podsSynced, c.sharepodsSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	// clientHandler in ConfigManager must have correct PodList of every SharePods,
	// so call it after initNodeClient
	go StartConfigManager(stopCh, c.kubeclientset)

	klog.Info("Starting workers")
	// Launch two workers to process SharePod resources
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	klog.Info("Started workers")
	<-stopCh
	klog.Info("Shutting down workers")

	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *Controller) processNextWorkItem() bool {
	obj, shutdown := c.workqueue.Get()

	if shutdown {
		return false
	}

	// We wrap this block in a func so we can defer c.workqueue.Done.
	err := func(obj interface{}) error {
		// We call Done here so the workqueue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the workqueue and attempted again after a back-off
		// period.
		defer c.workqueue.Done(obj)
		var key string
		var ok bool
		// We expect strings to come off the workqueue. These are of the
		// form namespace/name. We do this as the delayed nature of the
		// workqueue means the items in the informer cache may actually be
		// more up to date that when the item was initially put onto the
		// workqueue.
		if key, ok = obj.(string); !ok {
			// As the item in the workqueue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid.
			c.workqueue.Forget(obj)
			utilruntime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		// SharePod resource to be synced.
		if err := c.syncHandler(key); err != nil {
			// Put the item back on the workqueue to handle any transient errors.
			c.workqueue.AddRateLimited(key)
			return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.workqueue.Forget(obj)
		klog.Infof("Successfully synced '%s'", key)
		return nil
	}(obj)

	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}

// syncHandler returns error when we want to re-process the key, otherwise returns nil
func (c *Controller) syncHandler(key string) error {

	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	sharepod, err := c.sharepodsLister.SharePods(namespace).Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("SharePod '%s' in work queue no longer exists", key))
			return nil
		}
		return err
	}

	if sharepod.Spec.NodeName == "" {
		utilruntime.HandleError(fmt.Errorf("SharePod '%s' must be scheduled! Spec.NodeName is empty.", key))
		return nil
	}

	isGPUPod := false
	gpu_request := 0.0
	gpu_limit := 0.0
	gpu_mem := int64(0)
	GPUID := ""
	physicalGPUuuid := ""
	physicalGPUport := 0

	// GPU Pod needs to be filled with request, limit, memory, and GPUID, or none of them.
	// If something weird, reject it (record the reason to user then return nil)

	if sharepod.ObjectMeta.Annotations[kubesharev1.KubeShareResourceGPURequest] != "" ||
		sharepod.ObjectMeta.Annotations[kubesharev1.KubeShareResourceGPULimit] != "" ||
		sharepod.ObjectMeta.Annotations[kubesharev1.KubeShareResourceGPUMemory] != "" ||
		sharepod.ObjectMeta.Annotations[kubesharev1.KubeShareResourceGPUID] != "" {
		var err error
		gpu_limit, err = strconv.ParseFloat(sharepod.ObjectMeta.Annotations[kubesharev1.KubeShareResourceGPULimit], 64)
		if err != nil || gpu_limit > 1.0 || gpu_limit < 0.0 {
			utilruntime.HandleError(fmt.Errorf("SharePod %s/%s gpu_limit value error: %s", sharepod.ObjectMeta.Namespace, sharepod.ObjectMeta.Name, kubesharev1.KubeShareResourceGPULimit))
			c.recorder.Event(sharepod, corev1.EventTypeWarning, ErrValueError, "Value error: "+kubesharev1.KubeShareResourceGPULimit)
			return nil
		}
		gpu_request, err = strconv.ParseFloat(sharepod.ObjectMeta.Annotations[kubesharev1.KubeShareResourceGPURequest], 64)
		if err != nil || gpu_request > gpu_limit || gpu_request < 0.0 {
			utilruntime.HandleError(fmt.Errorf("SharePod %s/%s gpu_request value error: %s", sharepod.ObjectMeta.Namespace, sharepod.ObjectMeta.Name, kubesharev1.KubeShareResourceGPURequest))
			c.recorder.Event(sharepod, corev1.EventTypeWarning, ErrValueError, "Value error: "+kubesharev1.KubeShareResourceGPURequest)
			return nil
		}
		gpu_mem, err = strconv.ParseInt(sharepod.ObjectMeta.Annotations[kubesharev1.KubeShareResourceGPUMemory], 10, 64)
		if err != nil || gpu_mem < 0 {
			utilruntime.HandleError(fmt.Errorf("SharePod %s/%s gpu_mem value error: %s", sharepod.ObjectMeta.Namespace, sharepod.ObjectMeta.Name, kubesharev1.KubeShareResourceGPUMemory))
			c.recorder.Event(sharepod, corev1.EventTypeWarning, ErrValueError, "Value error: "+kubesharev1.KubeShareResourceGPUMemory)
			return nil
		}
		GPUID = sharepod.ObjectMeta.Annotations[kubesharev1.KubeShareResourceGPUID]
		if len(GPUID) == 0 {
			utilruntime.HandleError(fmt.Errorf("SharePod %s/%s GPUID value error: %s", sharepod.ObjectMeta.Namespace, sharepod.ObjectMeta.Name, kubesharev1.KubeShareResourceGPUID))
			c.recorder.Event(sharepod, corev1.EventTypeWarning, ErrValueError, "Value error: "+kubesharev1.KubeShareResourceGPUID)
			return nil
		}
		isGPUPod = true
	}

	// sharepod.Print()

	if isGPUPod && sharepod.Status.BoundDeviceID == "" {
		var errCode int
		physicalGPUuuid, errCode = c.getPhysicalGPUuuid(sharepod.Spec.NodeName, GPUID, gpu_request, gpu_limit, gpu_mem, key, &physicalGPUport)
		switch errCode {
		case 0:
			klog.Infof("SharePod %s is bound to GPU uuid: %s", key, physicalGPUuuid)
		case 1:
			klog.Infof("SharePod %s/%s is waiting for dummy Pod", sharepod.ObjectMeta.Namespace, sharepod.ObjectMeta.Name)
			return nil
		case 2:
			err := fmt.Errorf("Resource exceed!")
			utilruntime.HandleError(err)
			c.recorder.Event(sharepod, corev1.EventTypeWarning, ErrValueError, "Resource exceed")
			return err
		case 3:
			err := fmt.Errorf("Pod manager port pool is full!")
			utilruntime.HandleError(err)
			return err
		default:
			utilruntime.HandleError(fmt.Errorf("Unknown Error"))
			c.recorder.Event(sharepod, corev1.EventTypeWarning, ErrValueError, "Unknown Error")
			return nil
		}
		sharepod.Status.BoundDeviceID = physicalGPUuuid
	}

	pod, err := c.podsLister.Pods(sharepod.ObjectMeta.Namespace).Get(sharepod.ObjectMeta.Name)

	// If the resource doesn't exist, we'll create it, but don't create when we knew that Pod will not restart forever
	if errors.IsNotFound(err) && (sharepod.Status.PodStatus == nil ||
		sharepod.Spec.RestartPolicy == corev1.RestartPolicyAlways ||
		(sharepod.Spec.RestartPolicy == corev1.RestartPolicyOnFailure && sharepod.Status.PodStatus.Phase != corev1.PodSucceeded) ||
		(sharepod.Spec.RestartPolicy == corev1.RestartPolicyNever && (sharepod.Status.PodStatus.Phase != corev1.PodSucceeded && sharepod.Status.PodStatus.Phase != corev1.PodFailed))) {
		if n, ok := nodesInfo[sharepod.Spec.NodeName]; ok {
			pod, err = c.kubeclientset.CoreV1().Pods(sharepod.ObjectMeta.Namespace).Create(newPod(sharepod, isGPUPod, n.PodIP, physicalGPUport))
		}
	}

	if err != nil {
		return err
	}

	if !metav1.IsControlledBy(pod, sharepod) {
		msg := fmt.Sprintf(MessageResourceExists, pod.Name)
		c.recorder.Event(sharepod, corev1.EventTypeWarning, ErrResourceExists, msg)
		return fmt.Errorf(msg)
	}

	if (pod.Spec.RestartPolicy == corev1.RestartPolicyNever && (pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed)) ||
		(pod.Spec.RestartPolicy == corev1.RestartPolicyOnFailure && pod.Status.Phase == corev1.PodSucceeded) {
		go c.removeSharePodFromList(sharepod)
	}

	err = c.updateSharePodStatus(sharepod, pod, physicalGPUport)
	if err != nil {
		return err
	}

	c.recorder.Event(sharepod, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}

func (c *Controller) updateSharePodStatus(sharepod *kubesharev1.SharePod, pod *corev1.Pod, port int) error {
	sharepodCopy := sharepod.DeepCopy()
	sharepodCopy.Status.PodStatus = pod.Status.DeepCopy()
	sharepodCopy.Status.PodObjectMeta = pod.ObjectMeta.DeepCopy()
	if port != 0 {
		sharepodCopy.Status.PodManagerPort = port
	}

	_, err := c.kubeshareclientset.KubeshareV1().SharePods(sharepodCopy.Namespace).Update(sharepodCopy)
	return err
}

func (c *Controller) enqueueSharePod(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.workqueue.Add(key)
}

func (c *Controller) handleDeletedSharePod(obj interface{}) {
	sharepod, ok := obj.(*kubesharev1.SharePod)
	if !ok {
		utilruntime.HandleError(fmt.Errorf("handleDeletedSharePod: cannot parse object"))
		return
	}
	go c.removeSharePodFromList(sharepod)
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
		klog.V(4).Infof("Recovered deleted object '%s' from tombstone", object.GetName())
	}
	klog.V(4).Infof("Processing object: %s", object.GetName())

	if ownerRef := metav1.GetControllerOf(object); ownerRef != nil {
		// If this object is not owned by a SharePod, we should not do anything more
		// with it.
		if ownerRef.Kind != "SharePod" {
			return
		}

		foo, err := c.sharepodsLister.SharePods(object.GetNamespace()).Get(ownerRef.Name)
		if err != nil {
			klog.V(4).Infof("ignoring orphaned object '%s' of SharePod '%s'", object.GetSelfLink(), ownerRef.Name)
			return
		}

		c.enqueueSharePod(foo)
		return
	}
}