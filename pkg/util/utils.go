package util

import (
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// GetAddedRemovedInfo compares the updated AppliedManifestInfo to the base AppliedManifestInfo
// and returns AppliedManifestInfos for added and removed elements.
func GetAddedRemovedInfo(base AppliedManifestInfo, updated AppliedManifestInfo) (added, removed AppliedManifestInfo) {
	baseUIDs := base.ObjectUIDs
	updatedUIDs := updated.ObjectUIDs
	if len(baseUIDs) == 0 {
		return updated, removed
	}
	if len(updatedUIDs) == 0 {
		return added, base
	}
	baseGVRs := base.GVRs
	updatedGVRs := updated.GVRs
	// convert slices to maps for better performance
	mapBase := make(map[string]*schema.GroupVersionResource)
	mapUpdated := make(map[string]*schema.GroupVersionResource)
	for i, val := range baseUIDs {
		mapBase[val] = baseGVRs[i]
	}
	for i, val := range updatedUIDs {
		mapUpdated[val] = updatedGVRs[i]
	}

	for key := range mapBase {
		if _, ok := mapUpdated[key]; !ok {
			removed.ObjectUIDs = append(removed.ObjectUIDs, key)
			removed.GVRs = append(removed.GVRs, mapBase[key])
		} else {
			delete(mapUpdated, key)
		}
	}
	for key := range mapUpdated {
		added.ObjectUIDs = append(added.ObjectUIDs, key)
		added.GVRs = append(added.GVRs, mapUpdated[key])
	}
	return added, removed
}

// BuildWorkstatusName returns string in format: <groupversion>-<kind>-<namespace>-<name>
func BuildWorkstatusName(obj any) string {
	mObj := obj.(metav1.Object)
	rObj := obj.(runtime.Object)
	gvk := rObj.GetObjectKind().GroupVersionKind()
	return fmt.Sprintf("%s-%s-%s-%s",
		strings.ToLower(strings.ReplaceAll(gvk.GroupVersion().String(), "/", "")),
		strings.ToLower(gvk.Kind),
		mObj.GetNamespace(),
		mObj.GetName(),
	)
}
