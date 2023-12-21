package agent

import (
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"

	"github.ibm.com/dettori/status-addon/pkg/util"
)

func (a *Agent) startAppliedManifestWorkInformer(stopper chan struct{}) {
	gvr := schema.GroupVersionResource{
		Group:    util.AppliedManifestWorkGroup,
		Version:  util.AppliedManifestWorkVersion,
		Resource: util.AppliedManifestWorkResource}

	gvk := schema.GroupVersionKind{
		Group:   util.AppliedManifestWorkGroup,
		Version: util.AppliedManifestWorkVersion,
		Kind:    util.AppliedManifestWorkKind}

	a.startInformer(gvr, gvk, stopper, false)
}

func (a *Agent) startInformers(gvrs []*schema.GroupVersionResource) {
	for _, gvr := range gvrs {

		gvk, err := a.restMapper.KindFor(*gvr)
		if err != nil {
			a.logger.Error(err, "could not get kind for gvr")
			return
		}

		// we do not need to start informers for objects that do not have status
		if _, ok := excludedGVKs[gvk.String()]; ok {
			return
		}

		key := util.KeyForGroupVersionKind(gvk.Group, gvk.Version, gvk.Kind)
		a.objectsCount.IncrementValueForKey(key)

		count := a.objectsCount.Get(key)
		if count == 1 {
			a.logger.Info("starting informer", "key", key)
			stopper := make(chan struct{})
			defer close(stopper)
			a.startInformer(*gvr, gvk, stopper, true)
		}

	}
	// block to avoid closing channel
	select {}
}

func (a *Agent) startInformer(gvr schema.GroupVersionResource, gvk schema.GroupVersionKind, stopper chan struct{}, restartable bool) {
	key := util.KeyForGroupVersionKind(
		gvk.Group,
		gvk.Version,
		gvk.Kind)

	// SharedIndexInformer in client-go is not designed to be stopped and restarted.
	// Once a SharedIndexInformer is started, it’s intended to run for the lifetime
	// of the controller process. The only way to make it restartable is to recreate
	// the factory
	managedDynamicFactory := a.managedDynamicFactory
	if restartable {
		managedDynamicFactory = dynamicinformer.NewDynamicSharedInformerFactory(a.managedDynamicClient, 0*time.Minute)
	}
	informer := managedDynamicFactory.ForResource(gvr).Informer()
	a.informers[key] = &informer

	// add the event handler functions
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: a.handleObject,
		UpdateFunc: func(old, new interface{}) {
			if shouldSkipUpdate(old, new) {
				return
			}
			a.handleObject(new)
		},
		DeleteFunc: func(obj interface{}) {
			if shouldSkipDelete(obj) {
				return
			}
			a.handleObject(obj)
		},
	})

	// create and index the lister
	lister := cache.NewGenericLister(informer.GetIndexer(), gvr.GroupResource())
	a.listers[key] = &lister

	// run the informer
	a.stoppers[key] = stopper
	go informer.Run(stopper)
}

func (a *Agent) stopInformers(gvrs []*schema.GroupVersionResource) {
	for _, gvr := range gvrs {

		gvk, err := a.restMapper.KindFor(*gvr)
		if err != nil {
			a.logger.Error(err, "could not get kind for gvr")
			return
		}

		key := util.KeyForGroupVersionKind(gvk.Group, gvk.Version, gvk.Kind)

		// only consider existing informers, as some key may refer to informers for object
		// with GVK not being considered
		if _, ok := a.informers[key]; !ok {
			continue
		}

		a.objectsCount.DecrementValueForKey(key)
		count := a.objectsCount.Get(key)
		if count == 0 {
			a.stopInformer(key)
		}
	}
}

func (a *Agent) stopInformer(key string) {
	a.logger.Info("All instances deployed by hub removed, stopping informer", "key", key)
	stopper, ok := a.stoppers[key]
	if !ok {
		a.logger.Info("could not get stopper channel", "key", key)
	}
	// close channel
	close(stopper)
	// remove entries for key
	delete(a.informers, key)
	delete(a.listers, key)
	delete(a.stoppers, key)
}
