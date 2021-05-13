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
	"github.com/redhat-marketplace/marketplace-csi-driver/operator/pkg/status"
)

const (
	// Asset paths
	AssetPathCsiDriver                 = "assets/csi-driver.yaml"
	AssetPathCsiControllerService      = "assets/csi-controller-service.yaml"
	AssetPathCsiNodeService            = "assets/csi-node-service.yaml"
	AssetPathCsiService                = "assets/csi-service.yaml"
	AssetPathCsiReconcileService       = "assets/csi-reconciler.yaml"
	AssetPathCsiWebhook                = "assets/csi-webhook.yaml"
	AssetPathStorageClass              = "assets/storage-class.yaml"
	AssetPathRbacServiceAccount        = "assets/rbac-service-account.yaml"
	AssetPathRbacCsiClusterRole        = "assets/rbac-csi-cluster-role.yaml"
	AssetPathRbacCsiClusterRoleBinding = "assets/rbac-csi-cluster-role-binding.yaml"
	AssetPathRbacCsiRole               = "assets/rbac-csi-role.yaml"
	AssetPathRbacCsiRoleBinding        = "assets/rbac-csi-role-binding.yaml"
	AssetPathRbacOlmClusterRole        = "assets/rbac-olm-dataset-cluster-role.yaml"
	AssetPathRbacOlmClusterRoleBinding = "assets/rbac-olm-dataset-cluster-role-binding.yaml"
	AssetPathRbacOlmRole               = "assets/rbac-olm-leader-role.yaml"
	AssetPathRbacOlmRoleBinding        = "assets/rbac-olm-leader-role-binding.yaml"
	AssetPathDatasetPVC                = "assets/dataset-pvc.yaml"
	AssetPathDriverEnvironment         = "assets/csi-driver-env.yaml"

	//Credential keys
	CredentialAccessKey       = "AccessKeyID"
	CredentialSecretAccessKey = "SecretAccessKey"
	CredentialPullSecret      = "PULL_SECRET"

	// Install condition types
	ConditionTypeInstall     status.ConditionType = "Install"
	ConditionTypeDriver      status.ConditionType = "DriverStatus"
	ConditionTypeHealthCheck status.ConditionType = "HealthCheck"

	DatasetPVCName          = "csi-rhm-cos-pvc"
	DatasetDefaultMountPath = "/var/rhm/datasets"

	LabelApp           = "app"
	LabelAppNode       = "csi-rhm-cos-ds"
	LabelAppRegister   = "csi-rhm-cos-ss"
	LabelAppReconcile  = "csi-rhm-cos-ctrl"
	LabelDatasetID     = "csi.marketplace.redhat.com/uid"
	LabelSelectorHash  = "csi.marketplace.redhat.com/selectorHash"
	LabelDriverVersion = "csi.marketplace.redhat.com/driverVersion"
	LabelUninstall     = "marketplace.redhat.com/uninstall"
	LabelUpdateTime    = "deploymentUpdateTime"
	LabelTimeFormat    = "20060102150405"

	// MarketplaceNamespace redhat marketplace namespace
	MarketplaceNamespace = "openshift-redhat-marketplace"

	NameNodeHealthCheck = "driver-health-check-"

	// Condition reason list
	ReasonInstalling        status.ConditionReason = "Installing"
	ReasonInstallComplete   status.ConditionReason = "InstallComplete"
	ReasonInstallFailed     status.ConditionReason = "InstallFailed"
	ReasonCredentialError   status.ConditionReason = "CredentialError"
	ReasonOperational       status.ConditionReason = "Operational"
	ReasonRepairInitiated   status.ConditionReason = "RepairInitiated"
	ReasonRepairComplete    status.ConditionReason = "RepairComplete"
	ReasonRepairInProgress  status.ConditionReason = "RepairInProgress"
	ReasonRepairFailed      status.ConditionReason = "RepairFailed"
	ReasonHealthCheckFailed status.ConditionReason = "DriverHealthCheckFailed"
	ReasonValidationError   status.ConditionReason = "ValidationFailed"

	// Resource names
	ResourceNameS3Driver = "marketplacecsidriver"
	ResourceNameDataset  = "marketplacedataset"
	ResourceNameVolume   = "redhat-marketplace-datasets"
)
