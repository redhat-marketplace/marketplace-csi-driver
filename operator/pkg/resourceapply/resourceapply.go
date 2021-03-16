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
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	"github.com/redhat-marketplace/marketplace-csi-driver/operator/pkg/common"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1beta1 "k8s.io/api/storage/v1beta1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	genericScheme = runtime.NewScheme()
	genericCodecs = serializer.NewCodecFactory(genericScheme)
	genericCodec  = genericCodecs.UniversalDeserializer()
)

func init() {
	utilruntime.Must(apiextensionsv1beta1.AddToScheme(genericScheme))
	utilruntime.Must(apiextensionsv1.AddToScheme(genericScheme))
	utilruntime.Must(storagev1beta1.AddToScheme(genericScheme))
	utilruntime.Must(appsv1.AddToScheme(genericScheme))
	utilruntime.Must(corev1.AddToScheme(genericScheme))
	utilruntime.Must(rbacv1.AddToScheme(genericScheme))
}

type ResourceApply struct {
	Client    client.Client
	Context   context.Context
	Helper    common.ControllerHelperInterface
	Owner     metav1.OwnerReference
	Log       logr.Logger
	Namespace string
}

var assetPlaceHolderPairs = []string{}

func init() {
	assetPlaceHolderPairs = append(assetPlaceHolderPairs, []string{"${NAMESPACE}", common.MarketplaceNamespace}...)
}

func AddAssetPlaceHolder(key, value string) {
	assetPlaceHolderPairs = append(assetPlaceHolderPairs, []string{key, value}...)
}

func ResetAssetPlaceHolder() {
	assetPlaceHolderPairs = []string{}
	assetPlaceHolderPairs = append(assetPlaceHolderPairs, []string{"${NAMESPACE}", common.MarketplaceNamespace}...)
}

// ReplaceAssetPlaceholders replace variables in yaml
func ReplaceAssetPlaceholders(manifest []byte) []byte {
	replaced := strings.NewReplacer(assetPlaceHolderPairs...).Replace(string(manifest))
	return []byte(replaced)
}

// AddOwnerRefToObject ...
func AddOwnerRefToObject(o metav1.Object, r metav1.OwnerReference) {
	o.SetOwnerReferences(append(o.GetOwnerReferences(), r))
}

func (ra *ResourceApply) GetResource(asset string) (runtime.Object, error) {
	objBytes, err := ra.Helper.Asset(asset)
	if err != nil {
		ra.Log.Error(err, fmt.Sprintf("read failed for asset: %s", asset))
		return nil, err
	}
	objBytes = ReplaceAssetPlaceholders(objBytes)
	objGeneric, _, err := genericCodec.Decode(objBytes, nil, nil)
	if err != nil {
		ra.Log.Error(err, fmt.Sprintf("decode failed for asset: %s", asset))
		return nil, err
	}
	return objGeneric, nil
}

func (ra *ResourceApply) Apply(asset string) (runtime.Object, error) {

	objGeneric, err := ra.GetResource(asset)
	if err != nil {
		return nil, err
	}

	switch obj := objGeneric.(type) {
	case *appsv1.DaemonSet:
		return ra.applyDaemonSet(obj)
	case *appsv1.StatefulSet:
		return ra.applyStatefulSet(obj)
	case *corev1.PersistentVolumeClaim:
		return ra.applyPersistentVolumeclaim(obj)
	case *corev1.Service:
		return ra.applyService(obj)
	case *corev1.ServiceAccount:
		return ra.applyServiceAccount(obj)
	case *rbacv1.ClusterRole:
		return ra.applyClusterRole(obj)
	case *rbacv1.ClusterRoleBinding:
		return ra.applyClusterRoleBinding(obj)
	case *rbacv1.Role:
		return ra.applyRole(obj)
	case *rbacv1.RoleBinding:
		return ra.applyRoleBinding(obj)
	case *storagev1beta1.CSIDriver:
		return ra.applyCSIDriver(obj)
	case *storagev1beta1.StorageClass:
		return ra.applyStorageClass(obj)
	default:
		return nil, fmt.Errorf("unhandled type %T", obj)
	}
}

func (ra *ResourceApply) createResource(key types.NamespacedName, source runtime.Object, found runtime.Object) (bool, runtime.Object, error) {
	err := ra.Client.Get(ra.Context, key, found)
	if err != nil {
		if !errors.IsNotFound(err) {
			ra.Log.Error(err, fmt.Sprintf("unable to retrieve %T, namespace: %s, name: %s", source, key.Namespace, key.Name))
			return false, nil, err
		}
		err = ra.Client.Create(ra.Context, source)
		if err != nil {
			ra.Log.Error(err, fmt.Sprintf("failed to create %T, namespace: %s, name: %s", source, key.Namespace, key.Name))
			return false, nil, err
		}
		ra.Log.Info(fmt.Sprintf("Created %T, namespace: %s, name: %s", source, key.Namespace, key.Name))
		return true, source, nil
	}
	return false, nil, nil
}

func (ra *ResourceApply) updateResource(key types.NamespacedName, updateRes runtime.Object) (runtime.Object, error) {
	err := ra.Client.Update(ra.Context, updateRes)
	if err != nil {
		ra.Log.Error(err, fmt.Sprintf("failed to update %T, namespace: %s, name: %s", updateRes, key.Namespace, key.Name))
		return nil, err
	}
	ra.Log.Info(fmt.Sprintf("updated %T, namespace: %s, name: %s", updateRes, key.Namespace, key.Name))
	return updateRes, nil
}

func (ra *ResourceApply) isOwnerSet(ownerReferences []metav1.OwnerReference) bool {
	if len(ownerReferences) == 0 {
		return false
	}
	for _, owner := range ownerReferences {
		if owner.Name == ra.Owner.Name && owner.Kind == ra.Owner.Kind {
			return true
		}
	}
	return false
}

func GetLabelValue(lbls map[string]string, key string) string {
	if len(lbls) == 0 {
		return ""
	}
	val, ok := lbls[key]
	if !ok {
		return ""
	}
	return val
}
