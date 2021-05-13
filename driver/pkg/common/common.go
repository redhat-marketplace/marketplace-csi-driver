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

package common

import (
	"crypto/sha1"
	"encoding/hex"
	"io"
	"strings"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Dataset ...
type Credential struct {
	Type         string
	AccessKey    string
	AccessSecret string
}

type Selector struct {
	MatchLabels      map[string]string             `json:"matchLabels,omitempty"`
	MatchExpressions []v1.LabelSelectorRequirement `json:"matchExpressions,omitempty"`
}

type Dataset struct {
	Bucket       string     `json:"bucket"`
	PodSelectors []Selector `json:"podSelectors,omitempty"`
	Folder       string     `json:"folder,omitempty"`
}

type VolumeMountDetails struct {
	Credential    Credential
	Datasets      []Dataset
	Endpoint      string
	MountRootPath string
	MountOptions  []string
	Pod           *corev1.Pod
}

type HealthCheckStatus struct {
	DriverResponse string
	PodCount       int64
	Message        string
}

const (
	// Credential constants
	CredentialTypeHmac        = "hmac"
	CredentialTypeDefault     = "rhm-pull-secret"
	CredentialNameDefault     = "redhat-marketplace-pull-secret"
	CredentialAccessKey       = "AccessKeyID"
	CredentialSecretAccessKey = "SecretAccessKey"
	CredentialPullSecret      = "PULL_SECRET"
	CredentialHmacKey         = "entitlement.storage.hmac.access_key_id"
	CredentialHmacSecret      = "entitlement.storage.hmac.secret_access_key"
	CredentialIamKey          = "iam_apikey"

	// CR constants
	CustomResourceGroup   = "marketplace.redhat.com"
	CustomResourceVersion = "v1alpha1"

	DriverEndpointDefault  = "s3.us.cloud-object-storage.appdomain.cloud"
	DriverMountPathDefault = "/var/rhm/datasets"

	HealthCheckActionNone           = "NoAction"
	HealthCheckActionNoEligiblePods = "NoEligiblePods"
	HealthCheckActionRepaired       = "Repaired"
	HealthCheckActionError          = "Error"

	// MarketplaceNamespace redhat marketplace namespace
	MarketplaceNamespace = "openshift-redhat-marketplace"

	// Pod condition, label and annotation constants
	PodConditionType           corev1.PodConditionType = "csi.marketplace.redhat.com/datasets"
	PodConditionReasonComplete                         = "Complete"
	PodConditionReasonError                            = "Error"
	PodConditionReasonWarning                          = "Warning"
	PodConditionReasonNoAction                         = "NoAction"
	PodLabelWatchKey                                   = "csi.marketplace.redhat.com/watch"
	PodLabelWatchValue                                 = "enabled"

	PodAnnotationPath = "csi.marketplace.redhat.com/dspath"

	// Resource names
	ResourceNameS3Driver    = "marketplacecsidriver"
	ResourceNameS3Drivers   = "marketplacecsidrivers"
	ResourceNameDatasets    = "marketplacedatasets"
	ResourceNameDriverCheck = "marketplacedriverhealthchecks"

	//Volume context constants
	VolumeContextPvcName      = "csi.storage.k8s.io/pvc/name"
	VolumeContextPvcNameSpace = "csi.storage.k8s.io/pvc/namespace"
	VolumeContextPodName      = "csi.storage.k8s.io/pod.name"
	VolumeContextPodNameSpace = "csi.storage.k8s.io/pod.namespace"
)

func SanitizeKubernetesIdentifier(identifier string) string {
	identifier = strings.ToLower(identifier)
	if len(identifier) > 63 {
		h := sha1.New()
		io.WriteString(h, identifier)
		identifier = hex.EncodeToString(h.Sum(nil))
	}
	return identifier
}
