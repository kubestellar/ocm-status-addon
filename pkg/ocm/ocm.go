package ocm

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	workv1 "open-cluster-management.io/api/work/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ToAppliedManifestWork(obj *unstructured.Unstructured) (*workv1.AppliedManifestWork, error) {
	// Convert unstructured.Unstructured to an AppliedManifestWork object
	aWork := &workv1.AppliedManifestWork{}
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), aWork)
	if err != nil {
		return nil, err
	}
	return aWork, nil
}

func ListGVRs(aWork *workv1.AppliedManifestWork) []*schema.GroupVersionResource {
	gvrs := make([]*schema.GroupVersionResource, 0)
	for _, appliedResource := range aWork.Status.AppliedResources {
		gvr := &schema.GroupVersionResource{
			Group:    appliedResource.Group,
			Version:  appliedResource.Version,
			Resource: appliedResource.Resource,
		}
		gvrs = append(gvrs, gvr)
	}
	return gvrs
}

func GetTrackedObjectsUID(aWork *workv1.AppliedManifestWork) []string {
	uids := []string{}
	for _, appliedResource := range aWork.Status.AppliedResources {
		uids = append(uids, appliedResource.UID)
	}
	return uids
}

func GetManifestWork(c client.Client, name, namespace string) (*workv1.ManifestWork, error) {
	manifestWork := &workv1.ManifestWork{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}

	if err := c.Get(context.TODO(), client.ObjectKeyFromObject(manifestWork), manifestWork, &client.GetOptions{}); err != nil {
		return nil, err
	}

	return manifestWork, nil
}
