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
	"time"

	"github.com/redhat-marketplace/marketplace-csi-driver/operator/pkg/common"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

func (ra *ResourceApply) applyDaemonSet(daemonSet *appsv1.DaemonSet) (runtime.Object, error) {
	found := &appsv1.DaemonSet{}
	key := types.NamespacedName{Namespace: common.MarketplaceNamespace, Name: daemonSet.Name}
	daemonSet.SetOwnerReferences(append(daemonSet.GetOwnerReferences(), ra.Owner))
	created, obj, err := ra.createResource(key, daemonSet, found)
	if err != nil {
		return nil, err
	}
	if created {
		return obj, nil
	}
	updateRes := found.DeepCopy()
	assetVersion := GetLabelValue(daemonSet.Spec.Template.Labels, common.LabelDriverVersion)
	deployedVersion := GetLabelValue(updateRes.Spec.Template.Labels, common.LabelDriverVersion)
	if assetVersion != deployedVersion {
		updateRes.Spec = daemonSet.Spec
	}
	if !ra.isOwnerSet(updateRes.OwnerReferences) {
		updateRes.SetOwnerReferences(append(updateRes.GetOwnerReferences(), ra.Owner))
	}
	if IsDriverUpdateNeeded() || !ra.Helper.DeepEqual(updateRes, found) {
		updateRes.Spec.Template.Labels[common.LabelUpdateTime] = time.Now().Format(common.LabelTimeFormat)
		return ra.updateResource(key, updateRes)
	}
	return found, nil
}

func (ra *ResourceApply) applyStatefulSet(statefulSet *appsv1.StatefulSet) (runtime.Object, error) {
	found := &appsv1.StatefulSet{}
	key := types.NamespacedName{Namespace: common.MarketplaceNamespace, Name: statefulSet.Name}
	statefulSet.SetOwnerReferences(append(statefulSet.GetOwnerReferences(), ra.Owner))
	created, obj, err := ra.createResource(key, statefulSet, found)
	if err != nil {
		return nil, err
	}
	if created {
		return obj, nil
	}
	updateRes := found.DeepCopy()
	assetVersion := GetLabelValue(statefulSet.Spec.Template.Labels, common.LabelDriverVersion)
	deployedVersion := GetLabelValue(updateRes.Spec.Template.Labels, common.LabelDriverVersion)
	if assetVersion != deployedVersion {
		updateRes.Spec = statefulSet.Spec
	}
	if !ra.isOwnerSet(updateRes.OwnerReferences) {
		updateRes.SetOwnerReferences(append(updateRes.GetOwnerReferences(), ra.Owner))
	}
	if IsDriverUpdateNeeded() || !ra.Helper.DeepEqual(updateRes, found) {
		updateRes.Spec.Template.Labels[common.LabelUpdateTime] = time.Now().Format(common.LabelTimeFormat)
		return ra.updateResource(key, updateRes)
	}
	return found, nil
}
