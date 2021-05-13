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
	"github.com/redhat-marketplace/marketplace-csi-driver/operator/pkg/common"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

func (ra *ResourceApply) ApplyDriverConfigMap(cm *corev1.ConfigMap) (runtime.Object, error) {
	found := &corev1.ConfigMap{}
	key := types.NamespacedName{Namespace: cm.Namespace, Name: cm.Name}
	cm.SetOwnerReferences(append(cm.GetOwnerReferences(), ra.Owner))
	created, obj, err := ra.createResource(key, cm, found)
	if err != nil {
		return nil, err
	}
	if created {
		SetDriverUpdateNeeded(true)
		return obj, nil
	}
	updateRes := found.DeepCopy()
	updateRes.Data = cm.Data
	if !ra.isOwnerSet(updateRes.OwnerReferences) {
		updateRes.SetOwnerReferences(append(updateRes.GetOwnerReferences(), ra.Owner))
	}
	if !ra.Helper.DeepEqual(updateRes, found) {
		SetDriverUpdateNeeded(true)
		return ra.updateResource(key, updateRes)
	}
	return found, nil
}

func (ra *ResourceApply) applyPersistentVolumeclaim(pvc *corev1.PersistentVolumeClaim) (runtime.Object, error) {
	found := &corev1.PersistentVolumeClaim{}
	pvc.Namespace = ra.Namespace
	key := types.NamespacedName{Namespace: ra.Namespace, Name: pvc.Name}
	pvc.SetOwnerReferences(append(pvc.GetOwnerReferences(), ra.Owner))
	created, obj, err := ra.createResource(key, pvc, found)
	if err != nil {
		return nil, err
	}
	if created {
		return obj, nil
	}
	updateRes := found.DeepCopy()
	if len(updateRes.Annotations) == 0 {
		updateRes.Annotations = map[string]string{}
	}
	for key, specval := range pvc.Annotations {
		updateRes.Annotations[key] = specval
	}
	if !ra.isOwnerSet(updateRes.OwnerReferences) {
		updateRes.SetOwnerReferences(append(updateRes.GetOwnerReferences(), ra.Owner))
	}
	if !ra.Helper.DeepEqual(updateRes, found) {
		return ra.updateResource(key, updateRes)
	}
	return found, nil
}

func (ra *ResourceApply) applyService(service *corev1.Service) (runtime.Object, error) {
	found := &corev1.Service{}
	key := types.NamespacedName{Namespace: common.MarketplaceNamespace, Name: service.Name}
	service.SetOwnerReferences(append(service.GetOwnerReferences(), ra.Owner))
	created, obj, err := ra.createResource(key, service, found)
	if err != nil {
		return nil, err
	}
	if created {
		return obj, nil
	}
	updateRes := found.DeepCopy()
	updateRes.Spec.Selector = service.Spec.Selector
	updateRes.Spec.Ports[0].Port = service.Spec.Ports[0].Port
	updateRes.Spec.Ports[0].Name = service.Spec.Ports[0].Name
	if !ra.isOwnerSet(updateRes.OwnerReferences) {
		updateRes.SetOwnerReferences(append(updateRes.GetOwnerReferences(), ra.Owner))
	}
	if !ra.Helper.DeepEqual(updateRes, found) {
		return ra.updateResource(key, updateRes)
	}
	return found, nil
}

func (ra *ResourceApply) applyServiceAccount(serviceAccount *corev1.ServiceAccount) (runtime.Object, error) {
	found := &corev1.ServiceAccount{}
	key := types.NamespacedName{Namespace: common.MarketplaceNamespace, Name: serviceAccount.Name}
	serviceAccount.SetOwnerReferences(append(serviceAccount.GetOwnerReferences(), ra.Owner))
	created, obj, err := ra.createResource(key, serviceAccount, found)
	if err != nil {
		return nil, err
	}
	if created {
		return obj, nil
	}
	updateRes := found.DeepCopy()
	if !ra.isOwnerSet(updateRes.OwnerReferences) {
		updateRes.SetOwnerReferences(append(updateRes.GetOwnerReferences(), ra.Owner))
		return ra.updateResource(key, updateRes)
	}
	return found, nil
}
