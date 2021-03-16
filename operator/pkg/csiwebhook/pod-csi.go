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
package csiwebhook

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	marketplacev1alpha1 "github.com/redhat-marketplace/marketplace-csi-driver/operator/api/v1alpha1"
	"github.com/redhat-marketplace/marketplace-csi-driver/operator/pkg/common"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// +kubebuilder:webhook:path=/rhm-csi-mutate-v1-pod,mutating=true,failurePolicy=ignore,groups="",resources=pods,verbs=create,versions=v1,name=webhook.csi.rhm,sideEffects=none

type PodCSIVolumeMounter struct {
	Client  client.Client
	decoder *admission.Decoder
	Log     logr.Logger
	Helper  common.ControllerHelperInterface
}

var protectedNamespaces = make(map[string]struct{})

func init() {
	protectedNamespaces[metav1.NamespaceSystem] = struct{}{}
	protectedNamespaces[metav1.NamespacePublic] = struct{}{}
	protectedNamespaces[common.MarketplaceNamespace] = struct{}{}
}

func (a *PodCSIVolumeMounter) Handle(ctx context.Context, req admission.Request) admission.Response {
	pod := &corev1.Pod{}
	err := a.decoder.Decode(req, pod)
	if err != nil {
		a.Log.Error(err, "Error decoding pod from admission request")
		return admission.Allowed("Unable to decode pod")
	}

	podNamespace := pod.Namespace
	if podNamespace == "" {
		podNamespace = req.Namespace
	}
	a.Log.Info(fmt.Sprintf("Check mutation needed for %s/%s", podNamespace, pod.Name))
	_, ok := protectedNamespaces[podNamespace]
	if ok {
		a.Log.Info(fmt.Sprintf("Namespace protected, mutation not needed for %s/%s", podNamespace, pod.Name))
		return admission.Allowed("No mutation required")
	}

	driverConfig := &marketplacev1alpha1.MarketplaceCSIDriver{}
	err = a.Client.Get(ctx, client.ObjectKey{Namespace: common.MarketplaceNamespace, Name: common.ResourceNameS3Driver}, driverConfig)
	if err != nil {
		a.Log.Error(err, fmt.Sprintf("Cannot find driver config %s/%s, skipping mutation",
			common.MarketplaceNamespace, common.ResourceNameS3Driver))
		return admission.Allowed("Driver not set up")
	}

	if !a.isMutationRequired(pod, ctx, podNamespace) {
		a.Log.Info(fmt.Sprintf(fmt.Sprintf("No mutation required for %s/%s", podNamespace, pod.Name)))
		return admission.Allowed("No mutation required")
	}

	a.mutate(pod, driverConfig)
	marshaledPod, err := a.Helper.Marshal(pod)
	if err != nil {
		a.Log.Error(err, "Error marshalling pod json")
		return admission.Allowed("Unable to marshall pod")
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledPod)
}

func (a *PodCSIVolumeMounter) InjectDecoder(d *admission.Decoder) error {
	a.decoder = d
	return nil
}

func (a *PodCSIVolumeMounter) isMutationRequired(pod *corev1.Pod, ctx context.Context, namespace string) bool {
	if len(pod.Spec.Volumes) > 0 {
		for _, vol := range pod.Spec.Volumes {
			if vol.Name == common.ResourceNameVolume || (vol.PersistentVolumeClaim != nil && vol.PersistentVolumeClaim.ClaimName == common.DatasetPVCName) {
				a.Log.Info(fmt.Sprintf("Pod %s/%s has dataset volume, skip mutate", pod.Namespace, pod.Name))
				return false
			}
		}
	}

	csiPVC := &corev1.PersistentVolumeClaim{}
	err := a.Client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: common.DatasetPVCName}, csiPVC)
	if err != nil {
		if !errors.IsNotFound(err) {
			a.Log.Error(err, fmt.Sprintf("Unable to check marketplace dataset PVC %s/%s", namespace, common.DatasetPVCName))
		}
		a.Log.Info(fmt.Sprintf("No pvc in namespace, skip mutate: %s", namespace))
		return false
	}
	return true
}

func (a *PodCSIVolumeMounter) mutate(pod *corev1.Pod, driverConfig *marketplacev1alpha1.MarketplaceCSIDriver) {
	mountPath := common.DatasetDefaultMountPath
	if driverConfig.Spec.MountRootPath != "" {
		mountPath = driverConfig.Spec.MountRootPath
	}

	volSource := corev1.VolumeSource{
		PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: common.DatasetPVCName},
	}
	volume := corev1.Volume{
		Name:         common.ResourceNameVolume,
		VolumeSource: volSource,
	}
	if len(pod.Spec.Volumes) == 0 {
		pod.Spec.Volumes = []corev1.Volume{volume}
	} else {
		pod.Spec.Volumes = append(pod.Spec.Volumes, volume)
	}

	mode := corev1.MountPropagationHostToContainer
	for idx, container := range pod.Spec.Containers {
		if len(container.VolumeMounts) == 0 {
			container.VolumeMounts = []corev1.VolumeMount{{Name: "redhat-marketplace-datasets", MountPath: mountPath, MountPropagation: &mode}}
		} else {
			container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{Name: "redhat-marketplace-datasets", MountPath: mountPath, MountPropagation: &mode})
		}
		pod.Spec.Containers[idx] = container
	}
}
