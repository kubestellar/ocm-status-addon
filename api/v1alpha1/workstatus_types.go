/*
Copyright 2023 The KubeStellar Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// WorkStatus is the Schema for the work status
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName={ws,wss}
type WorkStatus struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec          WorkStatusSpec `json:"spec,omitempty"`
	Status        RawStatus      `json:"status,omitempty"`
	StatusDetails StatusDetails  `json:"statusDetails,omitempty"`
}

// Workstatus spec
type WorkStatusSpec struct {
	SourceRef SourceRef `json:"sourceRef,omitempty"`
}

// StatusDetails contains information about downsync propagations, which may or may not have been applied
type StatusDetails struct {
	// `lastGeneration` is that last `ObjectMeta.Generation` from the WDS that
	// propagated to the WEC. This is not to imply that it was successfully applied there;
	// for that, see `lastGenerationIsApplied`.
	// Zero means that none has yet propagated there.
	LastGeneration int64 `json:"lastGeneration"`
	// `lastGenerationIsApplied` indicates whether `lastGeneration` has been successfully applied
	LastGenerationIsApplied bool `json:"lastGenerationIsApplied"`
	// `lastCurrencyUpdateTime` is the time of the latest update to either
	// `lastGeneration` or `lastGenerationIsApplied`. More precisely, it is
	// the time when the core became informed of the update.
	// Before the first such update, this holds `time.Unix(0, 0)`
	LastCurrencyUpdateTime metav1.Time `json:"lastCurrencyUpdateTime"`
}

// +kubebuilder:object:root=true
// WorkStatusList contains a list of WorkStatus
type WorkStatusList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []WorkStatus `json:"items"`
}

// Manifest represents a resource to be deployed
type RawStatus struct {
	// +kubebuilder:pruning:PreserveUnknownFields
	runtime.RawExtension `json:",inline"`
}

type SourceRef struct {
	Group     string `json:"group"`
	Version   string `json:"version,omitempty"`
	Resource  string `json:"resource,omitempty"`
	Kind      string `json:"kind,omitempty"`
	Name      string `json:"name,omitempty"`
	Namespace string `json:"namespace"`
}

func init() {
	SchemeBuilder.Register(&WorkStatus{}, &WorkStatusList{})
}
