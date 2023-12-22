package agent

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.ibm.com/dettori/status-addon/api/v1alpha1"
	"github.ibm.com/dettori/status-addon/pkg/ocm"
	"github.ibm.com/dettori/status-addon/pkg/util"
)

const (
	ManagedByPlacementPrefix = "managed-by.kubestellar.io"
)

// main reconciliation loop. The returned bool value allows to re-enque even if no errors
func (a *Agent) reconcile(ctx context.Context, key util.Key) (bool, error) {
	var obj runtime.Object
	var err error
	isBeingDeleted := false
	if key.DeletedObject == nil {
		obj, err = util.GetObjectFromKey(a.listers, key)
		if err != nil {
			// The resource no longer exist, which means it has been deleted.
			if apierrors.IsNotFound(err) {
				utilruntime.HandleError(fmt.Errorf("resource '%s' for lister '%s' in work queue no longer exists", key.NamespaceNameKey, key.GvkKey))
				return true, err
			}
			return true, err
		}
	} else {
		isBeingDeleted = true
		obj = *key.DeletedObject
	}

	// special handling for selected API resources
	// note that object is *unstructured.Unstructured so we cannot
	// just use "switch obj.(type)"
	if util.IsAppliedManifestWork(obj) {
		return a.handleAppliedManifestWork(obj, isBeingDeleted)
	}

	// avoid further processing for keys of objects being deleted that do not have a deleted object
	if util.IsBeingDeleted(obj) && key.DeletedObject == nil {
		return false, nil
	}

	mObj := obj.(metav1.Object)
	// stop processing if not created by a manifest work
	if _, ok := a.trackedObjects[string(mObj.GetUID())]; !ok {
		return false, nil
	}

	a.logger.Info("going to update status:", "object", util.GenerateObjectInfoString(obj))
	if err := a.updateWorkStatus(obj); err != nil {
		return false, err
	}

	return false, nil
}

// returned bool is used to requeue without throwing an error
func (a *Agent) handleAppliedManifestWork(obj runtime.Object, isBeingDeleted bool) (bool, error) {
	mObj := obj.(metav1.Object)
	a.logger.Info("Got applied manifest work", "name", mObj.GetName(), "isBeingDeleted", isBeingDeleted)

	aWork, err := ocm.ToAppliedManifestWork(obj.(*unstructured.Unstructured))
	if err != nil {
		return true, err
	}

	if !isBeingDeleted {
		// list of GVR requiring to start informers for
		gvrs := ocm.ListGVRs(aWork)

		// need to check only if being deleted as the list is removed before the applied manifest is removed
		if len(gvrs) == 0 {
			// requeue because it may take time for applied manifest to get updated with GVRs
			return true, nil
		}
		// track objects set by manifest & start informers
		ocm.AddTrackedObjectsUID(aWork, a.trackedObjects)
		// need to maintain gvrs in a map indexed by name as the gvrs are deleted before we get them in the manifest
		uids := ocm.GetTrackedObjectsUID(aWork)
		info := util.AppliedManifestInfo{
			ObjectUIDs: uids,
			GVRs:       gvrs,
		}
		a.trackedAppliedManifests.Set(mObj.GetName(), info)
		go a.startInformers(gvrs, uids)
	} else {
		appliedManifestWorkInfo, ok := a.trackedAppliedManifests.Get(mObj.GetName())
		if !ok {
			a.logger.Info("could not find appliedManifestWorkInfo", "key", mObj.GetName())
		}
		ocm.RemoveTrackedObjectsUID(appliedManifestWorkInfo.ObjectUIDs, a.trackedObjects)
		a.stopInformers(appliedManifestWorkInfo)
		a.trackedAppliedManifests.Delete(mObj.GetName())
	}

	return false, nil
}

func (a *Agent) updateWorkStatus(obj runtime.Object) error {
	mObj := obj.(metav1.Object)
	namespace := a.clusterName
	name, ok := a.trackedObjects[string(mObj.GetUID())]
	if !ok {
		return fmt.Errorf("object not found in tracked objects: uid=%s", string(mObj.GetUID()))
	}

	// check if WorkStatus exists and if not create it
	workStatus := &v1alpha1.WorkStatus{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	err := a.hubClient.Get(ctx, client.ObjectKeyFromObject(workStatus), workStatus, &client.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			// get the manifest work for this workstatus, so that we can set a owner ref
			manifestWork, err := ocm.GetManifestWork(a.hubClient, name, namespace)
			if err != nil {
				return fmt.Errorf("failed to get manifestWork: %w", err)
			}

			// only update status for KS placement-managed objects
			if !util.HasPrefixInMap(manifestWork.Labels, ManagedByPlacementPrefix) {
				a.logger.Info("object not managed by a KS placement, status not updated", "object", name, "namespace", namespace)
				return nil
			}

			if err := controllerutil.SetControllerReference(manifestWork, workStatus, a.hubClient.Scheme()); err != nil {
				return fmt.Errorf("failed to set controller reference: %w", err)
			}

			// copy labels from manifest work to workstatus - this will be useful for tracking source placement
			// TODO - need to do this also when labels are updated on manifest work
			// TODO - there are currently no labels on workstatus but should consider merging in case labels are set
			workStatus.Labels = manifestWork.Labels

			// set object ref
			gvk := schema.GroupVersionKind{
				Group:   obj.GetObjectKind().GroupVersionKind().Group,
				Version: obj.GetObjectKind().GroupVersionKind().Version,
				Kind:    obj.GetObjectKind().GroupVersionKind().Kind}

			// TODO - restMapper may not be updated for new APIs - need to do that or use different approach
			gvr, err := util.GetGVR(a.restMapper, gvk)
			if err != nil {
				return fmt.Errorf("could not get gvr from restmapper for object: %s", err)
			}
			workStatus.Spec.SourceRef = v1alpha1.SourceRef{
				Group:     gvr.Group,
				Version:   gvr.Version,
				Resource:  gvr.Resource,
				Kind:      gvk.Kind,
				Name:      mObj.GetName(),
				Namespace: mObj.GetNamespace(),
			}

			if err = a.hubClient.Create(ctx, workStatus, &client.CreateOptions{}); err != nil {
				return fmt.Errorf("failed to create workStatus: %w", err)
			}
		} else {
			return err
		}
	}

	// generate status & update
	rawStatus, err := util.GetObjectStatusAsBytes(obj)
	if err != nil {
		return err
	}

	workStatus.Status.Raw = rawStatus
	err = a.hubClient.Status().Update(ctx, workStatus, &client.SubResourceUpdateOptions{})
	if err != nil {
		return err
	}

	return nil
}
