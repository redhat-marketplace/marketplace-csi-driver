/*
Copyright 2021 IBM Corp.

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
	"github.com/redhat-marketplace/marketplace-csi-driver/operator/pkg/status"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Selector defines one set of pod targeting terms for this dataset.
// All conditions in `match label` and `match expressions` must be satisfied for a pod to be selected.
// +kubebuilder:object:generate:=true
type Selector struct {

	// MatchLabels is a set of label value pairs used to target pods for this dataset
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.displayName="Match expressions"
	MatchLabels map[string]string `json:"matchLabels,omitempty"`

	// MatchExpressions is a set of expressions used to target pods for this dataset.
	// Supported operators are In, NotIn, Exists and DoesNotExist
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.displayName="Match expressions"
	MatchExpressions []metav1.LabelSelectorRequirement `json:"matchExpressions,omitempty"`
}

// MarketplaceDatasetSpec represents a dataset subscription in Red Hat Marketplace
// +k8s:openapi-gen=true
type MarketplaceDatasetSpec struct {

	// Bucket is the dataset bucket name
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.displayName="Bucket name"
	Bucket string `json:"bucket"`

	// PodSelectors is a a list of rules to select pods for this dataset based on pod labels.
	// In addition to labels, rules also support fields `metadata.name` and `metadata.generateName`
	// If one of the pod selector rules is satisfied by a pod, this dataset will be mounted to that pod.
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.displayName="Pod selectors"
	PodSelectors []Selector `json:"podSelectors,omitempty"`
}

// MarketplaceDatasetStatus defines the observed state of MarketplaceDataset
// +k8s:openapi-gen=true
type MarketplaceDatasetStatus struct {
	// Conditions represent the latest available observations of dataset state
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.x-descriptors="urn:alm:descriptor:io.kubernetes.conditions"
	// +optional
	Conditions status.Conditions `json:"conditions,omitempty"`
}

// MarketplaceDataset is the resource identifying a purchased dataset from Red Hat Marketplace
// +kubebuilder:object:root=true
//
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=marketplacedatasets,scope=Namespaced
// +kubebuilder:printcolumn:name="INSTALL_STATUS",type=string,JSONPath=`.status.conditions[?(@.type == "Install")].status`
// +kubebuilder:printcolumn:name="CLEANUP_STATUS",type=string,JSONPath=`.status.conditions[?(@.type == "CleanUpStatus")].status`
// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.displayName="Marketplace Dataset"
// +operator-sdk:gen-csv:customresourcedefinitions.displayName="Marketplace Dataset"
// +operator-sdk:gen-csv:customresourcedefinitions.resources=`PersistentVolumeClaim,v1,"csi-rhm-cos"`
type MarketplaceDataset struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MarketplaceDatasetSpec   `json:"spec,omitempty"`
	Status MarketplaceDatasetStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// MarketplaceDatasetList contains a list of MarketplaceDataset
type MarketplaceDatasetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MarketplaceDataset `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MarketplaceDataset{}, &MarketplaceDatasetList{})
}
