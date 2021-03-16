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

package resourceapply

import (
	storagev1beta1 "k8s.io/api/storage/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

func (ra *ResourceApply) applyCSIDriver(csiDriver *storagev1beta1.CSIDriver) (runtime.Object, error) {
	found := &storagev1beta1.CSIDriver{}
	key := types.NamespacedName{Namespace: metav1.NamespaceAll, Name: csiDriver.Name}
	csiDriver.SetOwnerReferences(append(csiDriver.GetOwnerReferences(), ra.Owner))
	created, obj, err := ra.createResource(key, csiDriver, found)
	if err != nil {
		return nil, err
	}
	if created {
		return obj, nil
	}
	updateRes := found.DeepCopy()
	updateRes.Spec.AttachRequired = csiDriver.Spec.AttachRequired
	updateRes.Spec.PodInfoOnMount = csiDriver.Spec.PodInfoOnMount
	updateRes.Spec.VolumeLifecycleModes = csiDriver.Spec.VolumeLifecycleModes
	if !ra.isOwnerSet(updateRes.OwnerReferences) {
		updateRes.SetOwnerReferences(append(updateRes.GetOwnerReferences(), ra.Owner))
	}
	if !ra.Helper.DeepEqual(updateRes, found) {
		return ra.updateResource(key, updateRes)
	}
	return found, nil
}

func (ra *ResourceApply) applyStorageClass(storageClass *storagev1beta1.StorageClass) (runtime.Object, error) {
	found := &storagev1beta1.StorageClass{}
	key := types.NamespacedName{Namespace: metav1.NamespaceAll, Name: storageClass.Name}
	storageClass.SetOwnerReferences(append(storageClass.GetOwnerReferences(), ra.Owner))
	created, obj, err := ra.createResource(key, storageClass, found)
	if err != nil {
		return nil, err
	}
	if created {
		return obj, nil
	}
	updateNeeded := false
	updateSC := found.DeepCopy()
	if !ra.isOwnerSet(updateSC.OwnerReferences) {
		updateSC.SetOwnerReferences(append(updateSC.GetOwnerReferences(), ra.Owner))
	}
	updateNeeded = updateNeeded || updateSC.Provisioner != storageClass.Provisioner
	updateSC.Provisioner = storageClass.Provisioner
	if updateNeeded {
		return ra.updateResource(key, updateSC)
	}
	return found, nil
}
