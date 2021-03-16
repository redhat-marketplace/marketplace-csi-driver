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
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

func (ra *ResourceApply) ApplyOLMRbac() {
	ra.Log.Info("Initialize dataset manager roles")
	_, err := ra.Apply(common.AssetPathRbacOlmClusterRole)
	if err != nil {
		ra.Log.Error(err, "Error creating clusterrole")
	}
	_, err = ra.Apply(common.AssetPathRbacOlmClusterRoleBinding)
	if err != nil {
		ra.Log.Error(err, "Error creating clusterrolebinding")
	}
	_, err = ra.Apply(common.AssetPathRbacOlmRole)
	if err != nil {
		ra.Log.Error(err, "Error creating role")
	}
	_, err = ra.Apply(common.AssetPathRbacOlmRoleBinding)
	if err != nil {
		ra.Log.Error(err, "Error creating rolebinding")
	}
}

func (ra *ResourceApply) applyClusterRole(clusterRole *rbacv1.ClusterRole) (runtime.Object, error) {
	found := &rbacv1.ClusterRole{}
	key := types.NamespacedName{Namespace: metav1.NamespaceAll, Name: clusterRole.Name}
	if ra.Owner != (metav1.OwnerReference{}) {
		clusterRole.SetOwnerReferences(append(clusterRole.GetOwnerReferences(), ra.Owner))
	}
	created, obj, err := ra.createResource(key, clusterRole, found)
	if err != nil {
		return nil, err
	}
	if created {
		return obj, nil
	}
	updateRes := found.DeepCopy()
	updateRes.Rules = clusterRole.Rules
	if ra.Owner != (metav1.OwnerReference{}) && !ra.isOwnerSet(updateRes.OwnerReferences) {
		updateRes.SetOwnerReferences(append(updateRes.GetOwnerReferences(), ra.Owner))
	}
	if !ra.Helper.DeepEqual(updateRes, found) {
		return ra.updateResource(key, updateRes)
	}
	return found, nil
}

func (ra *ResourceApply) applyClusterRoleBinding(clusterRoleBinding *rbacv1.ClusterRoleBinding) (runtime.Object, error) {
	found := &rbacv1.ClusterRoleBinding{}
	key := types.NamespacedName{Namespace: metav1.NamespaceAll, Name: clusterRoleBinding.Name}
	if ra.Owner != (metav1.OwnerReference{}) {
		clusterRoleBinding.SetOwnerReferences(append(clusterRoleBinding.GetOwnerReferences(), ra.Owner))
	}
	created, obj, err := ra.createResource(key, clusterRoleBinding, found)
	if err != nil {
		return nil, err
	}
	if created {
		return obj, nil
	}
	updateRes := found.DeepCopy()
	updateRes.Subjects = clusterRoleBinding.Subjects
	updateRes.RoleRef = clusterRoleBinding.RoleRef
	if ra.Owner != (metav1.OwnerReference{}) && !ra.isOwnerSet(updateRes.OwnerReferences) {
		updateRes.SetOwnerReferences(append(updateRes.GetOwnerReferences(), ra.Owner))
	}
	if !ra.Helper.DeepEqual(updateRes, found) {
		return ra.updateResource(key, updateRes)
	}
	return found, nil
}

func (ra *ResourceApply) applyRole(role *rbacv1.Role) (runtime.Object, error) {
	found := &rbacv1.Role{}
	key := types.NamespacedName{Namespace: common.MarketplaceNamespace, Name: role.Name}
	if ra.Owner != (metav1.OwnerReference{}) {
		role.SetOwnerReferences(append(role.GetOwnerReferences(), ra.Owner))
	}
	created, obj, err := ra.createResource(key, role, found)
	if err != nil {
		return nil, err
	}
	if created {
		return obj, nil
	}
	updateRes := found.DeepCopy()
	updateRes.Rules = role.Rules
	if ra.Owner != (metav1.OwnerReference{}) && !ra.isOwnerSet(updateRes.OwnerReferences) {
		updateRes.SetOwnerReferences(append(updateRes.GetOwnerReferences(), ra.Owner))
	}
	if !ra.Helper.DeepEqual(updateRes, found) {
		return ra.updateResource(key, updateRes)
	}
	return found, nil
}

func (ra *ResourceApply) applyRoleBinding(roleBinding *rbacv1.RoleBinding) (runtime.Object, error) {
	found := &rbacv1.RoleBinding{}
	key := types.NamespacedName{Namespace: common.MarketplaceNamespace, Name: roleBinding.Name}
	if ra.Owner != (metav1.OwnerReference{}) {
		roleBinding.SetOwnerReferences(append(roleBinding.GetOwnerReferences(), ra.Owner))
	}
	created, obj, err := ra.createResource(key, roleBinding, found)
	if err != nil {
		return nil, err
	}
	if created {
		return obj, nil
	}
	updateRes := found.DeepCopy()
	updateRes.Subjects = roleBinding.Subjects
	updateRes.RoleRef = roleBinding.RoleRef
	if ra.Owner != (metav1.OwnerReference{}) && !ra.isOwnerSet(updateRes.OwnerReferences) {
		updateRes.SetOwnerReferences(append(updateRes.GetOwnerReferences(), ra.Owner))
	}
	if !ra.Helper.DeepEqual(updateRes, found) {
		return ra.updateResource(key, updateRes)
	}
	return found, nil
}
