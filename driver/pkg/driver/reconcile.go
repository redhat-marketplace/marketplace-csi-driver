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

package driver

import (
	"context"
	"time"

	"github.com/redhat-marketplace/marketplace-csi-driver/driver/pkg/client"
	"github.com/redhat-marketplace/marketplace-csi-driver/driver/pkg/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"
)

// ReconcileContext data structure
type ReconcileContext struct {
	driver    *COSDriver
	client    *client.KubeClient
	startTime time.Time
}

func NewReconcileContext(driver *COSDriver) (*ReconcileContext, error) {
	cs, dc, err := driver.helper.GetClientSet()
	if err != nil {
		klog.Errorf("Unable to initialize client, error: %s", err)
		return nil, err
	}
	kc := client.NewKubeClient(context.Background(), cs, dc, driver.helper)
	now := time.Now()
	return &ReconcileContext{
		driver:    driver,
		client:    kc,
		startTime: time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute(), now.Second(), 0, now.Location()),
	}, nil
}

func (rc *ReconcileContext) StartInformers(stopCh <-chan struct{}) {
	informerFactory := rc.client.GetInformerFactory()
	hcHander := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			rc.processHealthCheck(obj)
		},
		UpdateFunc: func(obj interface{}, newObj interface{}) {
		},
		DeleteFunc: func(obj interface{}) {
		},
	}
	hcGVR := rc.client.GetHealthCheckGVR()
	hcInformer := informerFactory.ForResource(hcGVR).Informer()
	hcInformer.AddEventHandler(hcHander)

	informerFactory.Start(stopCh)
	klog.V(4).Info("Wait for health check informer cache sync")
	if synced := informerFactory.WaitForCacheSync(stopCh); !synced[hcGVR] {
		klog.Errorf("informer for %s hasn't synced", hcGVR)
	}
}

func (rc *ReconcileContext) processHealthCheck(obj interface{}) {
	hc := obj.(*unstructured.Unstructured)
	klog.V(4).Infof("Processing health check request: %s/%s, created at %v", hc.GetNamespace(), hc.GetName(), hc.GetCreationTimestamp())

	if rc.startTime.After(hc.GetCreationTimestamp().Time) {
		klog.V(4).Infof("Skip health check, %s/%s request time %v before watch time %v", hc.GetNamespace(), hc.GetName(), hc.GetCreationTimestamp().Time, rc.startTime)
		return
	}

	nodes := client.GetStringSliceValue(hc, []string{}, "spec", "affectedNodes")
	if !rc.isCheckNeededForThisNode(nodes) {
		klog.V(4).Infof("Skip health check, %s/%s created does not affect this node %s", hc.GetNamespace(), hc.GetName(), rc.driver.nodeName)
		rc.updateHealthCheckStatusSkip(hc, common.HealthCheckActionNone)
		return
	}

	datasetName := client.GetStringValue(hc, "", "spec", "dataset")
	if datasetName == "" {
		rc.processHealthCheckAll(hc)
	} else {
		rc.processHealthCheckForDataset(hc, datasetName)
	}
}

func (rc *ReconcileContext) processHealthCheckForDataset(hc *unstructured.Unstructured, datasetName string) {
	klog.V(4).Infof("Processing health check request for dataset: %s/%s", hc.GetNamespace(), datasetName)
	ds, err := rc.client.GetMarketplaceDataset(hc.GetNamespace(), datasetName)
	if err != nil {
		klog.Errorf("Unable to retrieve dataset %s/%s, error: %s", hc.GetNamespace(), datasetName, err)
		return
	}
	volDetails := common.VolumeMountDetails{Datasets: []common.Dataset{*ds}}
	volDetails, err = rc.client.GetDriverConfiguration(volDetails)
	if err != nil {
		klog.Errorf("Unable to retrieve driver configuration, error: %s", err)
		rc.updateHealthCheckStatus(hc, 0, err)
		return
	}

	pods, err := rc.client.GetPodsToReconcile(rc.driver.nodeName, hc.GetNamespace())
	if err != nil {
		klog.Errorf("Unable to retrieve affected pods in node :%s, error: %s", rc.driver.nodeName, err)
		rc.updateHealthCheckStatus(hc, 0, err)
		return
	}
	if len(pods) == 0 {
		klog.V(4).Infof("No active pods to process in node %s, namespace: %s", rc.driver.nodeName, hc.GetNamespace())
		rc.updateHealthCheckStatusSkip(hc, common.HealthCheckActionNoEligiblePods)
		return
	}

	var podsChecked int64 = 0
	for _, pod := range pods {
		if pod.Status.Phase != "Running" || len(pod.Annotations) == 0 {
			klog.Warningf("Ignoring pod: %s/%s with status %s, annotation count: %d", pod.Namespace, pod.Name, pod.Status.Phase, len(pod.Annotations))
			continue
		}
		path, ok := pod.Annotations[common.PodAnnotationPath]
		if !ok {
			klog.Errorf("Ignoring pod: %s/%s with missing annotation %s ", pod.Namespace, pod.Name, common.PodAnnotationPath)
			continue
		}
		volDetails.Pod = &pod
		count, err := ReconcileDatasets(rc.driver.helper, volDetails, path)
		rc.client.UpdatePodStatusOnReconcile(volDetails, count, err)
		if err != nil {
			klog.Errorf("Error repairing datasets in pod: %s/%s, error: %s ", pod.Namespace, pod.Name, err)
		} else {
			podsChecked++
		}
	}
	rc.updateHealthCheckStatus(hc, podsChecked, nil)
}

func (rc *ReconcileContext) processHealthCheckAll(hc *unstructured.Unstructured) {
	klog.V(4).Infof("Processing health check request for all datasets")
	volDetails := common.VolumeMountDetails{}
	volDetails, err := rc.client.GetDriverConfiguration(volDetails)
	if err != nil {
		klog.Errorf("Unable to retrieve driver configuration, error: %s", err)
		rc.updateHealthCheckStatus(hc, 0, err)
		return
	}

	pods, err := rc.client.GetPodsToReconcile(rc.driver.nodeName, metav1.NamespaceAll)
	if err != nil {
		klog.Errorf("Unable to retrieve affected pods in node :%s, error: %s", rc.driver.nodeName, err)
		rc.updateHealthCheckStatus(hc, 0, err)
		return
	}
	if len(pods) == 0 {
		klog.V(4).Infof("No active pods to process in node %s", rc.driver.nodeName)
		rc.updateHealthCheckStatusSkip(hc, common.HealthCheckActionNoEligiblePods)
		return
	}

	datasetsMap := make(map[string][]common.Dataset)
	var podsChecked int64 = 0
	for _, pod := range pods {
		if pod.Status.Phase != "Running" || len(pod.Annotations) == 0 {
			klog.Warningf("Ignoring pod: %s/%s with status %s, annotation count: %d", pod.Namespace, pod.Name, pod.Status.Phase, len(pod.Annotations))
			continue
		}
		path, ok := pod.Annotations[common.PodAnnotationPath]
		if !ok {
			klog.Errorf("Ignoring pod: %s/%s with missing annotation %s ", pod.Namespace, pod.Name, common.PodAnnotationPath)
			continue
		}
		volDetails.Pod = &pod
		datasets, ok := datasetsMap[pod.Namespace]
		klog.V(4).Infof("Get datasets for %s", pod.Namespace)
		if !ok {
			ds, err := rc.client.GetMarketplaceDatasets(pod.Namespace)
			if err != nil {
				klog.Errorf("Error retrieving datasets in namespace: %s, error: %s ", pod.Namespace, err)
				continue
			}
			klog.V(4).Infof("Get datasets for %s, returned %d", pod.Namespace, len(ds))
			datasetsMap[pod.Namespace] = ds
			datasets = datasetsMap[pod.Namespace]
		}
		if len(datasets) == 0 {
			klog.Warningf("No datasets in namespace: %s, skip reconcile", pod.Namespace)
			continue
		}
		volDetails.Datasets = datasets
		count, err := ReconcileDatasets(rc.driver.helper, volDetails, path)
		rc.client.UpdatePodStatusOnReconcile(volDetails, count, err)
		if err != nil {
			klog.Errorf("Error repairing datasets in pod: %s/%s, error: %s ", pod.Namespace, pod.Name, err)
		} else {
			podsChecked++
		}
	}
	rc.updateHealthCheckStatus(hc, podsChecked, nil)
}

func (rc *ReconcileContext) isCheckNeededForThisNode(nodes []string) bool {
	if len(nodes) == 0 {
		return true
	}
	for _, n := range nodes {
		if n == rc.driver.nodeName || n == "" {
			return true
		}
	}
	return false
}

func (rc *ReconcileContext) updateHealthCheckStatusSkip(hc *unstructured.Unstructured, action string) {
	rc.client.UpdateHealthCheckStatus(rc.driver.nodeName, hc.GetNamespace(), hc.GetName(), common.HealthCheckStatus{
		DriverResponse: action,
		PodCount:       0,
		Message:        "",
	})
}

func (rc *ReconcileContext) updateHealthCheckStatus(hc *unstructured.Unstructured, count int64, err error) {
	var action string = common.HealthCheckActionRepaired
	var message string = common.PodConditionReasonComplete
	if err != nil {
		action = common.HealthCheckActionError
		message = err.Error()
	}
	rc.client.UpdateHealthCheckStatus(rc.driver.nodeName, hc.GetNamespace(), hc.GetName(), common.HealthCheckStatus{
		DriverResponse: action,
		PodCount:       count,
		Message:        message,
	})
}
