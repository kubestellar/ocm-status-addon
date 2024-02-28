package agent

import (
	"context"
	"fmt"
	"reflect"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/kubestellar/ocm-status-addon/api/v1alpha1"
	"github.com/kubestellar/ocm-status-addon/pkg/ocm"
	"github.com/kubestellar/ocm-status-addon/pkg/util"
)

const (
	ManagedByKSLabelKeyPrefix = "managed-by.kubestellar.io"
	TransportLabelPrefix      = "transport.kubestellar.io"
	SingletonstatusLabelKey   = "managed-by.kubestellar.io/singletonstatus"
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

	mObj := obj.(metav1.Object)
	// stop processing if not created by a manifest work and not deleted
	_, ok := a.trackedObjects.Get(string(mObj.GetUID()))
	if !ok && !isBeingDeleted {
		return false, nil
	}

	a.logger.Info("going to update status:", "object", util.GenerateObjectInfoString(obj))
	if err := a.updateWorkStatus(obj, isBeingDeleted); err != nil {
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

		if len(gvrs) == 0 {
			// requeue because it may take time for applied manifest to get updated with GVRs
			return true, nil
		}
		// need to maintain gvrs in a map indexed by name as the gvrs are deleted before we get them in the manifest
		uids := ocm.GetTrackedObjectsUID(aWork)
		info := util.AppliedManifestInfo{
			ObjectUIDs: uids,
			GVRs:       gvrs,
		}

		// check if this applied manifest is already tracked(manifest update)
		oldinfoIntf, ok := a.trackedAppliedManifests.Get(mObj.GetName())
		if ok {
			oldinfo := oldinfoIntf.(util.AppliedManifestInfo)
			if reflect.DeepEqual(info, oldinfo) {
				return false, nil
			}
			a.logger.Info("processing changes in the applied manifest resources list", "manifest-name", mObj.GetName())
			addedInfo, removedInfo := util.GetAddedRemovedInfo(oldinfo, info)
			a.trackedObjects.AddTrackedObjectsUID(addedInfo.ObjectUIDs, aWork.Spec.ManifestWorkName)
			a.trackedObjects.RemoveTrackedObjectsUID(removedInfo.ObjectUIDs)

			a.trackedAppliedManifests.Set(mObj.GetName(), info)
			// start/stop the informers if needed
			go a.startInformers(addedInfo.GVRs, addedInfo.ObjectUIDs)
			a.stopInformers(removedInfo)

			return false, nil
		}

		// track objects set by manifest & start informers
		a.trackedObjects.AddTrackedObjectsUID(info.ObjectUIDs, aWork.Spec.ManifestWorkName)
		a.trackedAppliedManifests.Set(mObj.GetName(), info)
		go a.startInformers(gvrs, uids)
	} else {
		appliedManifestWorkInfoIntf, ok := a.trackedAppliedManifests.Get(mObj.GetName())
		if !ok {
			a.logger.Info("could not find appliedManifestWorkInfo", "key", mObj.GetName())
			return false, nil
		}
		appliedManifestWorkInfo := appliedManifestWorkInfoIntf.(util.AppliedManifestInfo)
		a.trackedObjects.RemoveTrackedObjectsUID(appliedManifestWorkInfo.ObjectUIDs)
		a.stopInformers(appliedManifestWorkInfo)
		a.trackedAppliedManifests.Delete(mObj.GetName())
	}

	return false, nil
}

func (a *Agent) updateWorkStatus(obj runtime.Object, isBeingDeleted bool) error {
	mObj := obj.(metav1.Object)
	namespace := a.clusterName

	workStatus := &v1alpha1.WorkStatus{
		ObjectMeta: metav1.ObjectMeta{
			Name:      util.BuildWorkstatusName(obj),
			Namespace: namespace,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// delete WorkStatus if exists, when the workload object is deleted
	if isBeingDeleted {
		err := a.hubClient.Get(ctx, client.ObjectKeyFromObject(workStatus), workStatus, &client.GetOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		err = a.hubClient.Delete(ctx, workStatus, &client.DeleteOptions{})
		if err != nil {
			a.logger.Info("workStatus was previously deleted", "workStatus-name", workStatus.Name)
			return nil
		}
		a.logger.Info("workStatus deleted", "workStatus-name", workStatus.Name)
		return nil
	}

	name, ok := a.trackedObjects.Get(string(mObj.GetUID()))
	if !ok {
		return fmt.Errorf("object not found in tracked objects: uid=%s", string(mObj.GetUID()))
	}

	// check if WorkStatus exists and if not create it
	err := a.hubClient.Get(ctx, client.ObjectKeyFromObject(workStatus), workStatus, &client.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			// get the manifest work for this workstatus, so that we can set a owner ref
			manifestWork, err := ocm.GetManifestWork(a.hubClient, name, namespace)
			if err != nil {
				return fmt.Errorf("failed to get manifestWork: %w", err)
			}

			// only update status for KS-managed (by bindingpolicies here) objects
			// TODO remove the check for ManagedByKSLabelKeyPrefix after we switch to new transport
			if !util.HasPrefixInMap(manifestWork.Labels, ManagedByKSLabelKeyPrefix) && !util.HasPrefixInMap(manifestWork.Labels, TransportLabelPrefix) {
				a.logger.Info("object not managed by a KS bindingpolicy, status not updated", "object", name, "namespace", namespace)
				return nil
			}

			if err := controllerutil.SetControllerReference(manifestWork, workStatus, a.hubClient.Scheme()); err != nil {
				return fmt.Errorf("failed to set controller reference: %w", err)
			}

			// copy labels from manifest work to workstatus - this will be useful for tracking source bindingpolicy
			// TODO - need to do this also when labels are updated on manifest work
			// TODO - there are currently no labels on workstatus but should consider merging in case labels are set
			workStatus.Labels = manifestWork.Labels

			// copy singleton label from the object, if exist
			objLabels := mObj.GetLabels()
			if val, ok := objLabels[SingletonstatusLabelKey]; ok {
				workStatus.Labels[SingletonstatusLabelKey] = val
			}

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

	// patch the workStatus with singleton label if the object was labeled
	objLabels := mObj.GetLabels()
	if val, ok := objLabels[SingletonstatusLabelKey]; ok {
		if _, ok := workStatus.Labels[SingletonstatusLabelKey]; !ok {
			patchString := fmt.Sprintf(`{"metadata":{"labels":{"%s":"%s"}}}`, SingletonstatusLabelKey, val)
			err = a.hubClient.Patch(ctx, workStatus, client.RawPatch(types.MergePatchType, []byte(patchString)))
			if err != nil {
				return err
			}
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
