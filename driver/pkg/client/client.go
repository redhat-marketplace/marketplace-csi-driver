package client

import (
	"context"
	"fmt"
	"strings"

	"github.com/kubernetes-csi/csi-lib-utils/protosanitizer"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/redhat-marketplace/marketplace-csi-driver/driver/pkg/common"
	"github.com/redhat-marketplace/marketplace-csi-driver/driver/pkg/helper"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog"
)

// KubeClient ...
type KubeClient struct {
	clientSet       kubernetes.Interface
	dynamicClient   dynamic.Interface
	datasetResource schema.GroupVersionResource
	driverResource  schema.GroupVersionResource
	context         context.Context
	helper          helper.DriverHelper
}

type patchOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

// NewKubeClient returns a new instance of KubeClient
func NewKubeClient(ctx context.Context, cs kubernetes.Interface, dc dynamic.Interface, helper helper.DriverHelper) *KubeClient {
	dsr := schema.GroupVersionResource{Group: common.CustomResourceGroup, Version: common.CustomResourceVersion, Resource: common.ResourceNameDatasets}
	drr := schema.GroupVersionResource{Group: common.CustomResourceGroup, Version: common.CustomResourceVersion, Resource: common.ResourceNameS3Drivers}
	return &KubeClient{
		clientSet:       cs,
		dynamicClient:   dc,
		datasetResource: dsr,
		driverResource:  drr,
		context:         ctx,
		helper:          helper,
	}
}

// GetDatasets ...
func (client *KubeClient) GetVolumeMountDetails(req *csi.NodePublishVolumeRequest) (common.VolumeMountDetails, error) {
	klog.V(4).Infof("Method GetVolumeMountDetails called with: %s", protosanitizer.StripSecrets(req))

	volDetails := common.VolumeMountDetails{Datasets: []common.Dataset{}}
	pod, err := client.GetPodDetails(req)
	if err != nil {
		return volDetails, err
	}
	volDetails.Pod = pod

	allDatasets, err := client.GetMarketplaceDatasets(pod.Namespace)
	if err != nil {
		klog.Error(err.Error())
		return volDetails, fmt.Errorf("error retrieving marketplace datasets in namespace %s: %s", pod.Namespace, err.Error())
	}
	volDetails.Datasets = allDatasets

	volDetails, err = client.GetDriverConfiguration(volDetails)
	if err != nil {
		klog.Error(err.Error())
		return volDetails, err
	}

	return volDetails, nil
}

func (client *KubeClient) GetDriverConfiguration(volDetails common.VolumeMountDetails) (common.VolumeMountDetails, error) {
	klog.V(4).Infof("Method GetDriverConfiguration called")
	driver, err := client.dynamicClient.Resource(client.driverResource).Namespace(common.MarketplaceNamespace).Get(client.context, common.ResourceNameS3Driver, metav1.GetOptions{})
	if err != nil {
		return volDetails, fmt.Errorf("unable to retrieve driver configuration %s/%s, error: %s", common.MarketplaceNamespace, common.ResourceNameS3Driver, err.Error())
	}

	credentialType := GetStringValue(driver, common.CredentialTypeDefault, "spec", "credential", "type")
	credentialName := GetStringValue(driver, common.CredentialNameDefault, "spec", "credential", "name")
	secret, err := client.GetSecret(common.MarketplaceNamespace, credentialName)
	if err != nil {
		return volDetails, fmt.Errorf("unable to retrieve driver credential %s/%s, error: %s", common.MarketplaceNamespace, credentialName, err.Error())
	}

	volDetails.Endpoint = GetStringValue(driver, common.DriverEndpointDefault, "spec", "endpoint")
	volDetails.MountRootPath = GetStringValue(driver, common.DriverMountPathDefault, "spec", "mountRootPath")

	mntOptions, exists, err := unstructured.NestedSlice(driver.Object, "spec", "mountOptions")
	if err == nil && exists && len(mntOptions) > 0 {
		var options []string
		for _, val := range mntOptions {
			options = append(options, val.(string))
		}
		volDetails.MountOptions = options
	} else {
		if err != nil {
			klog.Warningf("Error reading mount options: %s", err)
		}
	}

	if credentialType == "hmac" {
		accessKey, err := getSecretValue(secret, common.CredentialAccessKey)
		if err != nil {
			return volDetails, err
		}
		secretAccessKey, err := getSecretValue(secret, common.CredentialSecretAccessKey)
		if err != nil {
			return volDetails, err
		}
		volDetails.Credential = common.Credential{
			AccessKey:    accessKey,
			AccessSecret: secretAccessKey,
			Type:         common.CredentialTypeHmac,
		}
	} else {
		jwtString, err := getSecretValue(secret, common.CredentialPullSecret)
		if err != nil {
			return volDetails, err
		}
		claims, err := client.helper.ParseJWTclaims(jwtString)
		if err != nil {
			return volDetails, err
		}
		if credentialType == "rhm-pull-secret-hmac" {
			accessKeyID, err := client.helper.GetClaimValue(claims, common.CredentialHmacKey)
			if err != nil {
				return volDetails, err
			}
			secretAccessKey, err := client.helper.GetClaimValue(claims, common.CredentialHmacSecret)
			if err != nil {
				return volDetails, err
			}
			volDetails.Credential = common.Credential{
				AccessKey:    accessKeyID,
				AccessSecret: secretAccessKey,
				Type:         common.CredentialTypeHmac,
			}
		} else {
			accessKeyID, err := client.helper.GetClaimValue(claims, common.CredentialIamKey)
			if err != nil {
				return volDetails, err
			}
			volDetails.Credential = common.Credential{
				AccessKey:    accessKeyID,
				AccessSecret: "",
				Type:         common.CredentialTypeDefault,
			}
		}
	}
	return volDetails, nil
}

func (client *KubeClient) GetMarketplaceDataset(namespace string, name string) (*common.Dataset, error) {
	klog.V(4).Infof("Method GetMarketplaceDataset called with: %s/%s", namespace, name)
	dataset, err := client.dynamicClient.Resource(client.datasetResource).Namespace(namespace).Get(client.context, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	ds, err := client.ParseDatasetSpec(dataset)
	if err != nil {
		return nil, err
	}
	return ds, nil
}

func (client *KubeClient) GetMarketplaceDatasets(namespace string) ([]common.Dataset, error) {
	klog.V(4).Infof("Method GetMarketplaceDatasets called with: %s", namespace)
	var datasets []common.Dataset
	dsList, err := client.dynamicClient.Resource(client.datasetResource).Namespace(namespace).List(client.context, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	if len(dsList.Items) == 0 {
		klog.Warningf("No marketplace datasets in namespace: %s", namespace)
		return datasets, nil
	}

	var dsReadError error
	for _, ds := range dsList.Items {
		dataset, err := client.ParseDatasetSpec(&ds)
		if err != nil {
			dsReadError = err
			continue
		}
		datasets = append(datasets, *dataset)
	}
	if len(datasets) == 0 && dsReadError != nil {
		return nil, dsReadError
	}
	return datasets, nil
}

func (client *KubeClient) UpdatePodOnNodePublishVolume(req *csi.NodePublishVolumeRequest, volMountDetails common.VolumeMountDetails, mountedDatasetCount int, mountError error) {
	klog.V(4).Infof("Method UpdatePodOnNodePublishVolume called with: %s", protosanitizer.StripSecrets(req))

	if volMountDetails.Pod == nil {
		return
	}

	client.TagPodForTracking(req, volMountDetails)

	pod, err := client.clientSet.CoreV1().Pods(volMountDetails.Pod.Namespace).Get(client.context, volMountDetails.Pod.Name, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("unable to update pod status for %s/%s, error: %s", volMountDetails.Pod.Namespace, volMountDetails.Pod.Name, err)
		return
	}
	var volStatus corev1.ConditionStatus
	var message string
	var reason string
	if mountError != nil {
		volStatus = corev1.ConditionFalse
		if mountedDatasetCount > 0 {
			reason = common.PodConditionReasonWarning
			message = fmt.Sprintf("Failed to mount %d out of %d datasets, error: %s",
				len(volMountDetails.Datasets)-mountedDatasetCount, len(volMountDetails.Datasets), mountError)
		} else {
			reason = common.PodConditionReasonError
			message = fmt.Sprintf("Error mounting datasets: %s", mountError)
		}
	} else {
		volStatus = corev1.ConditionTrue
		if mountedDatasetCount > 0 {
			reason = common.PodConditionReasonComplete
			message = fmt.Sprintf("Mounted %d datasets", mountedDatasetCount)
		} else {
			reason = common.PodConditionReasonNoAction
			message = fmt.Sprintf("No matching datasets found in namespace: %s", volMountDetails.Pod.Namespace)
		}
	}

	driverCondition := corev1.PodCondition{
		Type:    common.PodConditionType,
		Status:  volStatus,
		Reason:  reason,
		Message: message,
	}
	upd := updatePodCondition(&pod.Status, &driverCondition)
	if upd {
		_, err = client.clientSet.CoreV1().Pods(volMountDetails.Pod.Namespace).UpdateStatus(client.context, pod, metav1.UpdateOptions{})
		if err != nil {
			klog.Errorf("unable to update pod status for %s/%s, error: %s", volMountDetails.Pod.Namespace, volMountDetails.Pod.Name, err)
		}
	}
}

func (client *KubeClient) UpdatePodStatusOnReconcile(volMountDetails common.VolumeMountDetails, mountedDatasetCount int, mountError error) {
	klog.V(4).Info("Method UpdatePodStatusOnReconcile called")
	pod, err := client.clientSet.CoreV1().Pods(volMountDetails.Pod.Namespace).Get(client.context, volMountDetails.Pod.Name, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("unable to update pod %s/%s, error: %s", volMountDetails.Pod.Namespace, volMountDetails.Pod.Name, err)
		return
	}

	var volStatus corev1.ConditionStatus
	var message string
	var reason string
	if mountError != nil {
		volStatus = corev1.ConditionFalse
		if mountedDatasetCount > 0 {
			reason = common.PodConditionReasonWarning
			message = fmt.Sprintf("Reconcile, failed to add %d out of %d datasets, error: %s",
				len(volMountDetails.Datasets)-mountedDatasetCount, len(volMountDetails.Datasets), mountError)
		} else {
			reason = common.PodConditionReasonError
			message = fmt.Sprintf("Reconcile error mounting datasets: %s", mountError)
		}
	} else {
		volStatus = corev1.ConditionTrue
		reason = common.PodConditionReasonComplete
		message = fmt.Sprintf("Reconcile mounted %d datasets", mountedDatasetCount)
	}

	driverCondition := corev1.PodCondition{
		Type:    common.PodConditionType,
		Status:  volStatus,
		Reason:  reason,
		Message: message,
	}
	upd := updatePodCondition(&pod.Status, &driverCondition)
	if upd {
		_, err = client.clientSet.CoreV1().Pods(volMountDetails.Pod.Namespace).UpdateStatus(client.context, pod, metav1.UpdateOptions{})
		if err != nil {
			klog.Errorf("unable to update pod status %s/%s, error: %s", volMountDetails.Pod.Namespace, volMountDetails.Pod.Name, err)
		}
	}
}

func (client *KubeClient) TagPodForTracking(req *csi.NodePublishVolumeRequest, volMountDetails common.VolumeMountDetails) {
	klog.V(4).Infof("Method TagPodForTracking called with: %s", protosanitizer.StripSecrets(req))
	tagErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		pod, err := client.clientSet.CoreV1().Pods(volMountDetails.Pod.Namespace).Get(client.context, volMountDetails.Pod.Name, metav1.GetOptions{})
		if err != nil {
			klog.Errorf("unable to update pod %s/%s, error: %s", volMountDetails.Pod.Namespace, volMountDetails.Pod.Name, err)
			return err
		}

		var patch []patchOperation
		if len(pod.Labels) > 0 {
			patch = append(patch, patchOperation{Op: "add", Path: "/metadata/labels/" + strings.ReplaceAll(common.PodLabelWatchKey, "/", "~1"), Value: common.PodLabelWatchValue})
		} else {
			patch = append(patch, patchOperation{Op: "add", Path: "/metadata/labels", Value: map[string]string{common.PodLabelWatchKey: common.PodLabelWatchValue}})
		}
		if len(pod.Annotations) > 0 {
			patch = append(patch, patchOperation{Op: "add", Path: "/metadata/annotations/" + strings.ReplaceAll(common.PodAnnotationPath, "/", "~1"), Value: req.GetTargetPath()})
		} else {
			patch = append(patch, patchOperation{Op: "add", Path: "/metadata/annotations", Value: map[string]string{common.PodAnnotationPath: req.GetTargetPath()}})
		}

		patchBytes, err := client.helper.MarshaljSON(patch)
		if err != nil {
			klog.Errorf("unable to marshal patch for pod %s/%s, error: %s", volMountDetails.Pod.Namespace, volMountDetails.Pod.Name, err)
			return err
		}
		klog.V(4).Infof("Patching pod %s/%s, patch=%v", volMountDetails.Pod.Namespace, volMountDetails.Pod.Name, string(patchBytes))
		_, err = client.clientSet.CoreV1().Pods(volMountDetails.Pod.Namespace).Patch(client.context, volMountDetails.Pod.Name, types.JSONPatchType, patchBytes, metav1.PatchOptions{})
		if err != nil {
			klog.Errorf("unable to patch pod %s/%s, error: %s", volMountDetails.Pod.Namespace, volMountDetails.Pod.Name, err)
			return err
		}
		return nil
	})
	if tagErr != nil {
		klog.Errorf("unable to patch pod %s/%s, error: %s", volMountDetails.Pod.Namespace, volMountDetails.Pod.Name, tagErr)
	}
}

func (client *KubeClient) GetSecret(namespace string, name string) (*corev1.Secret, error) {
	sec, err := client.clientSet.CoreV1().Secrets(namespace).Get(client.context, name, metav1.GetOptions{})
	return sec, err
}

func (client *KubeClient) GetPodsToReconcile(nodeName, namespace string) ([]corev1.Pod, error) {
	labelSelector := metav1.LabelSelector{MatchLabels: map[string]string{common.PodLabelWatchKey: common.PodLabelWatchValue}}
	listOptions := metav1.ListOptions{
		LabelSelector: labels.Set(labelSelector.MatchLabels).String(),
		FieldSelector: fmt.Sprintf("%s=%s,%s=%s", "spec.nodeName", nodeName, "status.phase", "Running"),
	}
	podList, err := client.clientSet.CoreV1().Pods(namespace).List(client.context, listOptions)
	if err != nil {
		return nil, err
	}
	return podList.Items, nil
}

func (client *KubeClient) GetInformerFactory() dynamicinformer.DynamicSharedInformerFactory {
	return dynamicinformer.NewFilteredDynamicSharedInformerFactory(client.dynamicClient, 0, metav1.NamespaceAll, nil)
}

func (client *KubeClient) GetHealthCheckGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: common.CustomResourceGroup, Version: common.CustomResourceVersion, Resource: common.ResourceNameDriverCheck}
}

func (client *KubeClient) UpdateHealthCheckStatus(nodeName string, namespace string, name string, hcStatus common.HealthCheckStatus) {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		hc, updErr := client.dynamicClient.Resource(client.GetHealthCheckGVR()).Namespace(namespace).Get(client.context, name, metav1.GetOptions{}, "status")
		if updErr != nil {
			klog.V(4).Infof("get error: %s", updErr)
			return updErr
		}
		drvStatus, exists, updErr := unstructured.NestedMap(hc.Object, "status", "driverHealthCheckStatus")
		if updErr != nil {
			klog.V(4).Infof("error reading status: %s", updErr)
			return updErr
		}

		if !exists {
			drvStatus = make(map[string]interface{})
		}
		drvCond := map[string]interface{}{
			"driverResponse": hcStatus.DriverResponse,
			"podCount":       hcStatus.PodCount,
			"status":         hcStatus.Message,
		}

		drvStatus[nodeName] = drvCond
		_ = unstructured.SetNestedMap(hc.Object, drvStatus, "status", "driverHealthCheckStatus")
		_, updErr = client.dynamicClient.Resource(client.GetHealthCheckGVR()).Namespace(namespace).UpdateStatus(client.context, hc, metav1.UpdateOptions{})
		if updErr != nil {
			klog.V(4).Infof("error updating status: %s", updErr)
			return updErr
		}
		return nil
	})
	if err != nil {
		klog.Errorf("error updating status: %s", err)
	}
}

func (client *KubeClient) GetPodDetails(req *csi.NodePublishVolumeRequest) (*corev1.Pod, error) {
	podNamespace, ok := req.VolumeContext[common.VolumeContextPodNameSpace]
	if !ok {
		return nil, fmt.Errorf("pod namespace missing in volumecontext: %v", req.VolumeContext)
	}
	podName, ok := req.VolumeContext[common.VolumeContextPodName]
	if !ok {
		return nil, fmt.Errorf("pod name missing in volumecontext: %v", req.VolumeContext)
	}
	pod, err := client.clientSet.CoreV1().Pods(podNamespace).Get(client.context, podName, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("unable to retrieve pod %s/%s, error: %s", podNamespace, podName, err)
		return nil, err
	}
	return pod, nil
}

func GetStringValue(u *unstructured.Unstructured, defValue string, fldPath ...string) string {
	fld, exists, err := unstructured.NestedString(u.Object, fldPath...)
	if err != nil {
		klog.Warningf("Error reading %s for %s: %v", strings.Join(fldPath, "."), u.GetName(), err)
		return defValue
	}
	if !exists {
		klog.Warningf("Missing field %s for %s", strings.Join(fldPath, "."), u.GetName())
		return defValue
	}
	return fld
}

func GetStringSliceValue(u *unstructured.Unstructured, defValue []string, fldPath ...string) []string {
	strList, exists, err := unstructured.NestedSlice(u.Object, fldPath...)
	if err == nil && exists && len(strList) > 0 {
		var values []string
		for _, val := range strList {
			v, ok := val.(string)
			if ok {
				values = append(values, v)
			}
		}
		return values
	}
	if err != nil {
		klog.Warningf("Error reading field %s for %s", strings.Join(fldPath, "."), u.GetName())
	}
	return defValue
}

func (client *KubeClient) ParseDatasetSpec(dsu *unstructured.Unstructured) (*common.Dataset, error) {
	fld, exists, err := unstructured.NestedMap(dsu.Object, "spec")
	if err != nil {
		klog.Error(fmt.Sprintf("error parsing dataset spec for %s, error: %s", dsu.GetName(), err.Error()))
		return nil, err
	}
	if !exists {
		message := fmt.Sprintf("No spec for dataset %s", dsu.GetName())
		klog.Error(message)
		return nil, fmt.Errorf(message)
	}
	js, err := client.helper.MarshaljSON(fld)
	if err != nil {
		klog.Error(fmt.Sprintf("error marshaling spec: %v, error: %s", fld, err.Error()))
		return nil, err
	}
	dataset := common.Dataset{}
	err = client.helper.UnMarshaljSON(js, &dataset)
	if err != nil {
		klog.Error(fmt.Sprintf("error unmarshaling spec: %s", err.Error()))
		return nil, err
	}
	dataset.Folder = dsu.GetName()
	return &dataset, nil
}

func getSecretValue(secret *corev1.Secret, key string) (string, error) {
	val, ok := secret.Data[key]
	if !ok {
		return "", fmt.Errorf("field %s missing in secret %s/%s", key, secret.Namespace, secret.Name)
	}
	valStr := string(val)
	if valStr == "" {
		return "", fmt.Errorf("field %s missing in secret %s/%s", key, secret.Namespace, secret.Name)
	}
	return valStr, nil
}

func GetPodConditionFromList(conditions []corev1.PodCondition, conditionType corev1.PodConditionType) (int, *corev1.PodCondition) {
	if conditions == nil {
		return -1, nil
	}
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return i, &conditions[i]
		}
	}
	return -1, nil
}

func GetPodCondition(status *corev1.PodStatus, conditionType corev1.PodConditionType) (int, *corev1.PodCondition) {
	if status == nil {
		return -1, nil
	}
	return GetPodConditionFromList(status.Conditions, conditionType)
}

func updatePodCondition(status *corev1.PodStatus, condition *corev1.PodCondition) bool {
	condition.LastTransitionTime = metav1.Now()
	conditionIndex, oldCondition := GetPodCondition(status, condition.Type)

	if oldCondition == nil {
		status.Conditions = append(status.Conditions, *condition)
		return true
	}
	if condition.Status == oldCondition.Status {
		condition.LastTransitionTime = oldCondition.LastTransitionTime
	}

	isEqual := condition.Status == oldCondition.Status &&
		condition.Reason == oldCondition.Reason &&
		condition.Message == oldCondition.Message &&
		condition.LastProbeTime.Equal(&oldCondition.LastProbeTime) &&
		condition.LastTransitionTime.Equal(&oldCondition.LastTransitionTime)

	status.Conditions[conditionIndex] = *condition
	return !isEqual
}
