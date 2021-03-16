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

// MarketplaceDriverHealthCheckSpec is used by Operator to trigger mounted volume check and repair
// +k8s:openapi-gen=true
type MarketplaceDriverHealthCheckSpec struct {
	// AffectedNodes is list of nodes to check and repair
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.displayName="Affected nodes"
	AffectedNodes []string `json:"affectedNodes,omitempty"`

	// Dataset is an optional input that can used to limit health check to a given dataset in the namespace of this custom resource
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.displayName="Affected nodes"
	Dataset string `json:"dataset,omitempty"`
}

// HealthCheckStatus defines health check responses for each node
type HealthCheckStatus struct {
	// DriverResponse is the response from CSI driver, possible values are NoAction, NoEligiblePods, Repaired or Error
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	DriverResponse string `json:"driverResponse,omitempty"`

	// PodCount is the number of pods that were checked for repair
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	PodCount int32 `json:"podCount,omitempty"`

	// Message is optional message associated with action taken on the node
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	Message string `json:"message,omitempty"`
}

// MarketplaceDriverHealthCheckStatus defines the observed state of MarketplaceDriverHealthCheck
// +k8s:openapi-gen=true
type MarketplaceDriverHealthCheckStatus struct {
	// Conditions represent the latest available observations of the health check state
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.x-descriptors="urn:alm:descriptor:io.kubernetes.conditions"
	// +optional
	Conditions status.Conditions `json:"conditions,omitempty"`

	// DriverHealthCheckStatus is driver health check status for each node, map key is the node name
	DriverHealthCheckStatus map[string]HealthCheckStatus `json:"driverHealthCheckStatus,omitempty"`
}

// MarketplaceDriverHealthCheck is used by the Operator to trigger mounted volume check and repair
// +kubebuilder:object:root=true
//
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=marketplacedriverhealthchecks,scope=Namespaced
// +kubebuilder:printcolumn:name="HEALTH_CHECK_STATUS",type=string,JSONPath=`.status.conditions[?(@.type == "ConditionTypeHealthCheck")].status`
// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.displayName="Marketplace Driver Health Check"
// +operator-sdk:gen-csv:customresourcedefinitions.displayName="Marketplace Driver Health Check"
// +operator-sdk:gen-csv:customresourcedefinitions.resources=`PersistentVolumeClaim,v1,"csi-rhm-cos"`
type MarketplaceDriverHealthCheck struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MarketplaceDriverHealthCheckSpec   `json:"spec,omitempty"`
	Status MarketplaceDriverHealthCheckStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// MarketplaceDriverHealthCheckList contains a list of MarketplaceDriverHealthCheck
type MarketplaceDriverHealthCheckList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MarketplaceDriverHealthCheck `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MarketplaceDriverHealthCheck{}, &MarketplaceDriverHealthCheckList{})
}
