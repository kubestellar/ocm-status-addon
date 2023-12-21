package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"golang.org/x/time/rate"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlm "sigs.k8s.io/controller-runtime/pkg/manager"

	"github.ibm.com/dettori/status-addon/pkg/ocm"
	"github.ibm.com/dettori/status-addon/pkg/util"
)

// generally all resources not creating a status should be excluded
var excludedGVKs = map[string]bool{
	"rbac.authorization.k8s.io/v1, Kind=ClusterRoleBinding": true,
	"rbac.authorization.k8s.io/v1, Kind=ClusterRole":        true,
	"rbac.authorization.k8s.io/v1, Kind=Role":               true,
	"rbac.authorization.k8s.io/v1, Kind=RoleBinding":        true,
	"/v1, Kind=Secret":         true,
	"/v1, Kind=ConfigMap":      true,
	"/v1, Kind=Namespace":      true,
	"/v1, Kind=ServiceAccount": true,
	"/v1, Kind=Service":        true,
}

// Agent watches all objects, find associated placements, when mached a placement wraps and
// place objects ino mailboxes
type Agent struct {
	agentName               string
	clusterName             string
	ctx                     context.Context
	logger                  logr.Logger
	managedDynamicClient    *dynamic.DynamicClient
	managedKubernetesClient *kubernetes.Clientset
	managedDynamicFactory   dynamicinformer.DynamicSharedInformerFactory
	restMapper              meta.RESTMapper
	hubClient               client.Client
	listers                 map[string]*cache.GenericLister
	informers               map[string]*cache.SharedIndexInformer
	trackedObjects          map[string]string
	trackedAppliedManifests util.SafeAppliedManifestMap
	objectsCount            util.SafeIntMap
	stoppers                map[string]chan struct{}
	workqueue               workqueue.RateLimitingInterface
	initializedTs           time.Time
}

// Create a new agent controller
func NewAgent(mgr ctrlm.Manager, managedRestConfig *rest.Config, hubRestConfig *rest.Config, clusterName, agentName string) (*Agent, error) {
	ratelimiter := workqueue.NewMaxOfRateLimiter(
		workqueue.NewItemExponentialFailureRateLimiter(5*time.Millisecond, 1000*time.Second),
		&workqueue.BucketRateLimiter{Limiter: rate.NewLimiter(rate.Limit(50), 300)},
	)

	managedDynamicClient, err := dynamic.NewForConfig(managedRestConfig)
	if err != nil {
		return nil, err
	}

	hubClient, err := ocm.NewClient(hubRestConfig)
	if err != nil {
		return nil, err
	}

	managedKubernetesClient, err := kubernetes.NewForConfig(managedRestConfig)
	if err != nil {
		return nil, err
	}

	managedDynamicFactory := dynamicinformer.NewDynamicSharedInformerFactory(managedDynamicClient, 0*time.Minute)

	discoveryClient := managedKubernetesClient.Discovery()
	groupResources, err := restmapper.GetAPIGroupResources(discoveryClient)
	if err != nil {
		return nil, err
	}

	agent := &Agent{
		agentName:               agentName,
		clusterName:             clusterName,
		logger:                  mgr.GetLogger(),
		managedDynamicClient:    managedDynamicClient,
		managedKubernetesClient: managedKubernetesClient,
		managedDynamicFactory:   managedDynamicFactory,
		hubClient:               *hubClient,
		restMapper:              restmapper.NewDiscoveryRESTMapper(groupResources),
		listers:                 make(map[string]*cache.GenericLister),
		informers:               make(map[string]*cache.SharedIndexInformer),
		trackedAppliedManifests: *util.NewSafeAppliedManifestMap(),
		objectsCount:            *util.NewSafeIntMap(),
		stoppers:                make(map[string]chan struct{}),
		trackedObjects:          make(map[string]string),
		workqueue:               workqueue.NewRateLimitingQueue(ratelimiter),
	}

	return agent, nil
}

// Start the agent
func (a *Agent) Start(workers int) error {
	ctx, cancel := context.WithCancel(context.Background())
	a.ctx = ctx
	defer cancel()

	errChan := make(chan error, 1)
	go func() {
		errChan <- a.run(workers)
	}()

	// check for errors at startup, after all started we let it continue
	// so we can start the controller-runtime manager
	select {
	case err := <-errChan:
		return err
	case <-time.After(3 * time.Second):
		return nil
	}
}

// Invoked by Start() to run the agent
func (a *Agent) run(workers int) error {
	defer a.workqueue.ShutDown()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// start only informer for appliedmanifestwork
	stopper := make(chan struct{})
	defer close(stopper)
	a.startAppliedManifestWorkInformer(stopper)

	// wait for all informers caches to be synced
	a.logger.Info("Waiting for caches to sync")
	for _, informer := range a.informers {
		if ok := cache.WaitForCacheSync(ctx.Done(), (*informer).HasSynced); !ok {
			return fmt.Errorf("failed to wait for caches to sync")
		}
	}
	a.logger.Info("All caches synced")

	a.logger.Info("Starting workers", "count", workers)
	for i := 0; i < workers; i++ {
		go wait.UntilWithContext(ctx, a.runWorker, time.Second)
	}
	a.logger.Info("Started workers")

	a.initializedTs = time.Now()

	<-ctx.Done()
	a.logger.Info("Shutting down workers")

	return nil
}

// Event handler: enqueues the objects to be processed
// At this time it is very simple, more complex processing might be required here
func (a *Agent) handleObject(obj any) {
	mObj := obj.(metav1.Object)
	rObj := obj.(runtime.Object)
	ok := rObj.GetObjectKind()
	gvk := ok.GroupVersionKind()
	a.logger.V(2).Info("Got object event", gvk.GroupVersion().String(), gvk.Kind, mObj.GetNamespace(), mObj.GetName())
	a.enqueueObject(obj, false)
}

// enqueueObject converts an object into a key struct which is then put onto the work queue.
func (a *Agent) enqueueObject(obj interface{}, skipCheckIsDeleted bool) {
	var key util.Key
	var err error
	if key, err = util.KeyForGroupVersionKindNamespaceName(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	if !skipCheckIsDeleted {
		// we need to check if object was deleted.
		// This does not break the best practice of only storing the keys so that
		// the latest version of the object is retrieved, as we know the object was deleted when
		// we get a copy of the runtime.Object and there is no longer a copy on the API server.
		_, err = util.GetObjectFromKey(a.listers, key)
		if err != nil {
			// The resource no longer exist, which means it has been deleted.
			if apierrors.IsNotFound(err) {
				// we only track delete for manifestwork, so we can determine
				// if informers need to be stopped
				if util.IsAppliedManifestWork(obj) {
					deletedObj := util.CopyObjectMetaAndType(obj.(runtime.Object))
					key.DeletedObject = &deletedObj
					a.workqueue.Add(key)
				}
				return
			}
			// TODO - return error here
			a.logger.Error(err, "error getting object from key")
			return
		}
	}
	a.workqueue.Add(key)
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (a *Agent) runWorker(ctx context.Context) {
	for a.processNextWorkItem(ctx) {
	}
}

// processNextWorkItem reads a single work item off the workqueue and
// attempt to process it by calling the reconcile.
func (a *Agent) processNextWorkItem(ctx context.Context) bool {
	obj, shutdown := a.workqueue.Get()
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
		defer a.workqueue.Done(obj)
		var key util.Key
		var ok bool
		// We expect util.Key to come off the workqueue. We do this as the delayed
		// nature of the workqueue means the items in the informer cache may actually be
		// more up to date that when the item was initially put onto the
		// workqueue.
		if key, ok = obj.(util.Key); !ok {
			// if the item in the workqueue is invalid, we call
			// Forget here to avoid process a work item that is invalid.
			a.workqueue.Forget(obj)
			utilruntime.HandleError(fmt.Errorf("expected util.Key in workqueue but got %#v", obj))
			return nil
		}
		// Run the reconciler, passing it the full key or the metav1 Object
		requeue, err := a.reconcile(ctx, key)
		if err != nil {
			// Put the item back on the workqueue to handle any transient errors.
			a.workqueue.AddRateLimited(obj)
			return fmt.Errorf("error syncing key '%#v': %s, requeuing", obj, err.Error())
		}
		if requeue {
			// requeue without returning error as this is dne to wait for some other event
			a.workqueue.AddRateLimited(obj)
			return nil
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		a.workqueue.Forget(obj)
		a.logger.V(2).Info("Successfully synced", "object", obj)
		return nil
	}(obj)

	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}

func shouldSkipDelete(obj interface{}) bool {
	// logic to determine if should ignore delete based on object GVK
	return false
}

func shouldSkipUpdate(old, new interface{}) bool {
	oldMObj := old.(metav1.Object)
	newMObj := new.(metav1.Object)
	// do not enqueue update events for objects that have not changed
	if newMObj.GetResourceVersion() == oldMObj.GetResourceVersion() {
		return true
	}
	// avoid enqueing events for updates to applied manifest
	if util.IsAppliedManifestWork(new) {
		return true
	}
	return false
}
