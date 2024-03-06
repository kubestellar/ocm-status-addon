package util

import (
	"encoding/json"
	"fmt"
	"reflect"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
	workv1 "open-cluster-management.io/api/work/v1"
)

const (
	CRDKind                              = "CustomResourceDefinition"
	CRDGroup                             = "apiextensions.k8s.io"
	CRDVersion                           = "v1"
	ServiceVersion                       = "v1"
	ServiceKind                          = "Service"
	AnnotationToPreserveValuesKey        = "annotations.kubestellar.io/preserve"
	PreserveNodePortValue                = "nodeport"
	UnableToRetrieveCompleteAPIListError = "unable to retrieve the complete list of server APIs"

	AppliedManifestWorkKind     = "AppliedManifestWork"
	AppliedManifestWorkResource = "appliedmanifestworks"
)

func IsCRD(o interface{}) bool {
	return matchesGVK(o, CRDGroup, CRDVersion, CRDKind)
}

func IsAppliedManifestWork(o interface{}) bool {
	return matchesGVK(o, workv1.GroupVersion.Group, workv1.GroupVersion.Version, AppliedManifestWorkKind)
}

func matchesGVK(o interface{}, group, version, kind string) bool {
	gvk, err := getObjectGVK(o)
	if err != nil {
		return false
	}

	if gvk.Group == group &&
		gvk.Version == version &&
		gvk.Kind == kind {
		return true
	}
	return false
}

func getObjectGVK(o interface{}) (schema.GroupVersionKind, error) {
	gvk := schema.GroupVersionKind{}
	switch obj := o.(type) {
	case runtime.Object:
		gvk.Group = obj.GetObjectKind().GroupVersionKind().Group
		gvk.Version = obj.GetObjectKind().GroupVersionKind().Version
		gvk.Kind = obj.GetObjectKind().GroupVersionKind().Kind
	case unstructured.Unstructured:
		gvk.Group = obj.GetObjectKind().GroupVersionKind().Group
		gvk.Version = obj.GetObjectKind().GroupVersionKind().Version
		gvk.Kind = obj.GetObjectKind().GroupVersionKind().Kind
	default:
		return gvk, fmt.Errorf("object is of wrong type: %#v", obj)
	}
	return gvk, nil
}

func ZeroFields(obj runtime.Object) runtime.Object {
	zeroed := obj.DeepCopyObject()
	mObj := zeroed.(metav1.Object)
	mObj.SetManagedFields(nil)
	mObj.SetCreationTimestamp(metav1.Time{})
	mObj.SetGeneration(0)
	mObj.SetResourceVersion("")
	mObj.SetUID("")
	annotations := mObj.GetAnnotations()
	delete(annotations, "kubectl.kubernetes.io/last-applied-configuration")
	mObj.SetAnnotations(annotations)

	return zeroed
}

// create a minimal runtime.Object copy with no spec and status for use
// with the delete
func CopyObjectMetaAndType(obj runtime.Object) runtime.Object {
	dest := obj.DeepCopyObject()
	dest = ZeroFields(dest)
	val := reflect.ValueOf(dest).Elem()

	spec := val.FieldByName("Spec")
	if spec.IsValid() {
		spec.Set(reflect.Zero(spec.Type()))
	}

	status := val.FieldByName("Status")
	if status.IsValid() {
		status.Set(reflect.Zero(status.Type()))
	}

	return dest
}

func IsBeingDeleted(obj runtime.Object) bool {
	mObj := obj.(metav1.Object)
	return mObj.GetDeletionTimestamp() != nil
}

func GetObjectFromKey(listers *SafeMap, key Key) (runtime.Object, error) {
	pListerIntf, _ := listers.Get(key.GvkKey)
	if pListerIntf == nil {
		return nil, fmt.Errorf("could not get lister for key: %s", key.GvkKey)
	}
	lister := pListerIntf.(cache.GenericLister)

	namespace, name, err := cache.SplitMetaNamespaceKey(key.NamespaceNameKey)
	if err != nil {
		return nil, fmt.Errorf("invalid resource key: %s %s", key.NamespaceNameKey, err)
	}

	return getObject(lister, namespace, name)
}

func getObject(lister cache.GenericLister, namespace, name string) (runtime.Object, error) {
	if namespace != "" {
		return lister.ByNamespace(namespace).Get(name)
	}
	return lister.Get(name)
}

func GetObjectStatusAsBytes(obj runtime.Object) ([]byte, error) {
	unstructuredObj, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return nil, fmt.Errorf("object is not a *unstructured.Unstructured")
	}

	status, ok, err := unstructured.NestedFieldNoCopy(unstructuredObj.Object, "status")
	if err != nil {
		return nil, fmt.Errorf("error getting status: %v", err)
	}
	if !ok {
		return nil, fmt.Errorf("status field not found")
	}

	rawStatus, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&status)
	if err != nil {
		return nil, fmt.Errorf("error converting status to unstructured: %v", err)
	}

	return json.Marshal(rawStatus)
}

func ConvertRuntimeObjectToUnstructured(obj runtime.Object) (*unstructured.Unstructured, error) {
	unstructuredMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, fmt.Errorf("error converting runtime.Object to Unstructured: %v", err)
	}

	return &unstructured.Unstructured{
		Object: unstructuredMap,
	}, nil
}

func GetGVR(mapper meta.RESTMapper, gvk schema.GroupVersionKind) (schema.GroupVersionResource, error) {
	mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return schema.GroupVersionResource{}, fmt.Errorf("could not get REST mapping for GVK %v: %w", gvk, err)
	}

	return mapping.Resource, nil
}
