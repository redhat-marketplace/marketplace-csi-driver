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

// S3Credential defines a kubernetes Secret with dataset S3 credentials
// +kubebuilder:object:generate:=true
type S3Credential struct {

	// Name for the kubernetes Secret with S3 credentials.
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.displayName="Secret name"
	Name string `json:"name"`

	// Type of Secret. Supported types are hmac or rhm-pull-secret (default).
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.displayName="Secret type"
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.x-descriptors="urn:alm:descriptor:com.tectonic.ui:select:"
	// +kubebuilder:validation:Enum=hmac;rhm-pull-secret;rhm-pull-secret-hmac
	Type string `json:"type"`
}

// MarketplaceCSIDriverSpec defines the desired state of MarketplaceCSIDriver
// +k8s:openapi-gen=true
type MarketplaceCSIDriverSpec struct {

	// Endpoint is the dataset S3 public endpoint
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.displayName="S3 endpoint url"
	Endpoint string `json:"endpoint"`

	// Credential is configuration that contains dataset S3 credential details
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.displayName="Dataset credential"
	Credential S3Credential `json:"credential"`

	// HealthCheckIntervalInMinutes is the interval at which to run health check for mounted volumes.
	// A value of `0` disables periodic health check.
	// +kubebuilder:validation:Minimum=0
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.displayName="Health check interval (minutes)"
	HealthCheckIntervalInMinutes int32 `json:"healthCheckIntervalInMinutes"`

	// MountRootPath is the root mount path for all dataset buckets
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.displayName="Dataset mount path"
	MountRootPath string `json:"mountRootPath"`

	// MountOptions is an array of performance options for dataset driver. This is for internal use.
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.displayName="Dataset mount options"
	MountOptions []string `json:"mountOptions,omitempty"`
}

// DriverPod defines identifiers for a csi driver pod
type DriverPod struct {
	//CreateTime is the pod creation time
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	CreateTime string `json:"createTime,omitempty"`

	//Version is the resource version of a driver pod
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	Version string `json:"version,omitempty"`

	//NodeName is the node where a driver pod is running
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	NodeName string `json:"nodeName,omitempty"`
}

// MarketplaceCSIDriverStatus defines the observed state of MarketplaceCSIDriver
// +k8s:openapi-gen=true
type MarketplaceCSIDriverStatus struct {
	// Conditions represent the latest available observations of the driver state
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.x-descriptors="urn:alm:descriptor:io.kubernetes.conditions"
	// +optional
	Conditions status.Conditions `json:"conditions,omitempty"`

	// DriverPods holds details of currently running csi driver pods
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +optional
	DriverPods map[string]DriverPod `json:"driverPods,omitempty"`
}

// MarketplaceCSIDriver is the resource that deploys the marketplace CSI driver
// +kubebuilder:object:root=true
//
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=marketplacecsidrivers,scope=Namespaced
// +kubebuilder:printcolumn:name="INSTALL_STATUS",type=string,JSONPath=`.status.conditions[?(@.type == "Install")].status`
// +kubebuilder:printcolumn:name="DRIVER_STATUS",type=string,JSONPath=`.status.conditions[?(@.type == "DriverStatus")].status`
// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.displayName="CSI Driver"
// +operator-sdk:gen-csv:customresourcedefinitions.displayName="Marketplace dataset driver"
// +operator-sdk:gen-csv:customresourcedefinitions.resources=`CSIDriver,storage.k8s.io/v1,"csi.rhm.cos.ibm.com"`
// +operator-sdk:gen-csv:customresourcedefinitions.resources=`StorageClass,storage.k8s.io/v1,"csi-rhm-cos"`
// +operator-sdk:gen-csv:customresourcedefinitions.resources=`ServiceAccount,v1,"csi-rhm-cos"`
// +operator-sdk:gen-csv:customresourcedefinitions.resources=`ClusterRole,rbac.authorization.k8s.io/v1,"csi-rhm-cos-provisioner"`
// +operator-sdk:gen-csv:customresourcedefinitions.resources=`ClusterRoleBinding,rbac.authorization.k8s.io/v1,"csi-rhm-cos-provisioner"`
// +operator-sdk:gen-csv:customresourcedefinitions.resources=`StatefulSet,apps/v1,"csi-rhm-cos-ss"`
// +operator-sdk:gen-csv:customresourcedefinitions.resources=`DaemonSet,apps/v1,"csi-rhm-cos-ds"`
// +operator-sdk:gen-csv:customresourcedefinitions.resources=`DaemonSet,apps/v1,"csi-rhm-cos-ctrl"`
// +operator-sdk:gen-csv:customresourcedefinitions.resources=`Service,v1,"csi-rhm-cos-ss"`
type MarketplaceCSIDriver struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MarketplaceCSIDriverSpec   `json:"spec,omitempty"`
	Status MarketplaceCSIDriverStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// MarketplaceCSIDriverList contains a list of MarketplaceCSIDriver
type MarketplaceCSIDriverList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MarketplaceCSIDriver `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MarketplaceCSIDriver{}, &MarketplaceCSIDriverList{})
}
