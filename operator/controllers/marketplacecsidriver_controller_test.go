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

package controllers

import (
	"context"
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	marketplacev1alpha1 "github.com/redhat-marketplace/marketplace-csi-driver/operator/api/v1alpha1"
	"github.com/redhat-marketplace/marketplace-csi-driver/operator/pkg/common"
	"github.com/redhat-marketplace/marketplace-csi-driver/operator/pkg/resourceapply"
	"github.com/redhat-marketplace/marketplace-csi-driver/operator/pkg/status"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1beta1 "k8s.io/api/storage/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var testStartTime = metav1.Time{Time: time.Now()}

const TestInfraManifest = `
apiVersion: config.openshift.io/v1
kind: Infrastructure
metadata:
  name: cluster
status:
  platformStatus:
    type: ibmcloud
`
const TestInfraManifestInvalid = `
apiVersion: config.openshift.io/v1
kind: Infrastructure
metadata:
  name: cluster
status:
  platformStatus:
    type: 1
`
const TestInfraManifestMissingData = `
apiVersion: config.openshift.io/v1
kind: Infrastructure
metadata:
  name: cluster
status:
  platformStatus:
    somekey: ibmcloud
`

var _ = Context("MarketplaceCSIDriver Controller", func() {
	var (
		ctx       = context.TODO()
		namespace = common.MarketplaceNamespace
		name      = "marketplacecsidriver"
		r         *MarketplaceCSIDriverReconciler
		driver    *marketplacev1alpha1.MarketplaceCSIDriver
		ra        resourceapply.ResourceApply
	)

	BeforeEach(func() {
		driver = &marketplacev1alpha1.MarketplaceCSIDriver{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Spec: marketplacev1alpha1.MarketplaceCSIDriverSpec{
				Endpoint:                     "s3.us.cloud-object-storage.appdomain.cloud",
				MountRootPath:                "/var/redhat-marketplace/datasets",
				HealthCheckIntervalInMinutes: 0,
				Credential: marketplacev1alpha1.S3Credential{
					Name: "redhat-marketplace-pull-secret",
					Type: "rhm-pull-secret",
				},
				EnvironmentVariables: []marketplacev1alpha1.NameValueMap{
					{Name: "test1", Value: "value1"},
				},
			},
		}
		r = &MarketplaceCSIDriverReconciler{
			Scheme: scheme.Scheme,
			Log:    logf.Log.WithName("controllers").WithName("MarketplaceCSIDriver"),
			Helper: common.NewControllerHelper(),
		}
		resourceapply.SetDriverImage("test")
		resourceapply.SetDriverVersion("test")
		resourceapply.ResetAssetPlaceHolder()
		trueVar := true
		falseVar := false
		ra = resourceapply.ResourceApply{
			Context: context.Background(),
			Helper:  r.Helper,
			Log:     r.Log,
			Client:  fake.NewFakeClient(),
			Owner: metav1.OwnerReference{
				APIVersion:         "v1alpha1",
				Kind:               "MarketplaceCSIDriver",
				Name:               name,
				Controller:         &trueVar,
				BlockOwnerDeletion: &falseVar,
			},
		}
	})

	It("CSI Driver incorrect name", func() {
		driver.ObjectMeta.Name = "marketplacecsidriver-sample"
		r.Client = fake.NewFakeClient(driver)

		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      driver.ObjectMeta.Name,
				Namespace: namespace,
			},
		}

		_, err := r.Reconcile(req)
		Expect(err).To(HaveOccurred(), "Reconcile did not return error for bad csi driver name")
		key := client.ObjectKey{
			Name:      driver.ObjectMeta.Name,
			Namespace: namespace,
		}
		reason := getMarketplaceCSIDriverStatusReason(ctx, key, r.Client, common.ConditionTypeInstall)
		Expect(reason).To(Equal("InstallFailed"))
	})

	It("CSI Driver incorrect namespace", func() {
		driver.ObjectMeta.Namespace = "default"
		r.Client = fake.NewFakeClient(driver)

		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: driver.ObjectMeta.Namespace,
			},
		}

		_, err := r.Reconcile(req)
		Expect(err).To(HaveOccurred(), "Reconcile did not return error for bad csi driver name")
		key := client.ObjectKey{
			Name:      name,
			Namespace: driver.ObjectMeta.Namespace,
		}
		reason := getMarketplaceCSIDriverStatusReason(ctx, key, r.Client, common.ConditionTypeInstall)
		Expect(reason).To(Equal("InstallFailed"))
	})

	It("CSI Driver delete", func() {
		r.Client = fake.NewFakeClient()
		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			},
		}

		_, err := r.Reconcile(req)
		Expect(err).NotTo(HaveOccurred(), "Reconcile returned error for delete")
	})

	It("CSI Driver get error", func() {
		r.Client = newMarketplaceCSIDriverClient(fake.NewFakeClient(driver), "get", "MarketplaceCSIDriver", driver.Name)
		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			},
		}

		_, err := r.Reconcile(req)
		Expect(err).To(HaveOccurred(), "Reconcile did not return error as expected")
	})

	It("CSI Driver installing", func() {
		r.Client = fake.NewFakeClient(driver)
		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			},
		}

		_, err := r.Reconcile(req)
		Expect(err).NotTo(HaveOccurred(), "Reconcile returned error for install")

		key := client.ObjectKey{
			Name:      name,
			Namespace: namespace,
		}
		reason := getMarketplaceCSIDriverStatusReason(ctx, key, r.Client, common.ConditionTypeInstall)
		Expect(reason).To(Equal("Installing"))
	})

	It("CSI Driver installing ibmcloud", func() {
		infrastructureResource := &unstructured.Unstructured{}
		dec := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
		dec.Decode([]byte(TestInfraManifest), nil, infrastructureResource)

		r.Client = fake.NewFakeClient(driver, infrastructureResource)
		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			},
		}

		_, err := r.Reconcile(req)
		Expect(err).NotTo(HaveOccurred(), "Reconcile returned error for install")

		key := client.ObjectKey{
			Name:      name,
			Namespace: namespace,
		}
		reason := getMarketplaceCSIDriverStatusReason(ctx, key, r.Client, common.ConditionTypeInstall)
		Expect(reason).To(Equal("Installing"))

		key = client.ObjectKey{
			Name:      "csi-rhm-cos-ds",
			Namespace: namespace,
		}
		ds := &appsv1.DaemonSet{}
		err = r.Client.Get(ctx, key, ds)
		Expect(err).NotTo(HaveOccurred(), "daemon set not found")
		volPathValid := true
		for _, vol := range ds.Spec.Template.Spec.Volumes {
			if vol.Name == "socket-dir" || vol.Name == "mountpoint-dir" || vol.Name == "registration-dir" {
				if !strings.HasPrefix(vol.HostPath.Path, "/var/data") {
					volPathValid = false
				}
			}
		}
		Expect(volPathValid).To(BeTrue(), "Invalid host path for ibmcloud")
	})

	It("CSI Driver installing - bad infrastructure data", func() {
		infrastructureResource := &unstructured.Unstructured{}
		dec := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
		dec.Decode([]byte(TestInfraManifestInvalid), nil, infrastructureResource)

		r.Client = fake.NewFakeClient(driver, infrastructureResource)
		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			},
		}

		_, err := r.Reconcile(req)
		Expect(err).NotTo(HaveOccurred(), "Reconcile returned error for install")

		key := client.ObjectKey{
			Name:      name,
			Namespace: namespace,
		}
		reason := getMarketplaceCSIDriverStatusReason(ctx, key, r.Client, common.ConditionTypeInstall)
		Expect(reason).To(Equal("Installing"))
	})

	It("CSI Driver installing - missing infrastructure data", func() {
		infrastructureResource := &unstructured.Unstructured{}
		dec := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
		dec.Decode([]byte(TestInfraManifestMissingData), nil, infrastructureResource)

		r.Client = fake.NewFakeClient(driver, infrastructureResource)
		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			},
		}

		_, err := r.Reconcile(req)
		Expect(err).NotTo(HaveOccurred(), "Reconcile returned error for install")

		key := client.ObjectKey{
			Name:      name,
			Namespace: namespace,
		}
		reason := getMarketplaceCSIDriverStatusReason(ctx, key, r.Client, common.ConditionTypeInstall)
		Expect(reason).To(Equal("Installing"))
	})

	It("CSI Driver installing - infrastructure get error", func() {
		infrastructureResource := &unstructured.Unstructured{}
		dec := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
		dec.Decode([]byte(TestInfraManifest), nil, infrastructureResource)

		cl := newMarketplaceCSIDriverClient(
			fake.NewFakeClient(driver, infrastructureResource),
			"get",
			"Infrastructure",
			"cluster",
		)
		r.Client = cl
		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			},
		}
		_, err := r.Reconcile(req)
		Expect(err).NotTo(HaveOccurred(), "Reconcile returned error for install")
		key := client.ObjectKey{
			Name:      name,
			Namespace: namespace,
		}
		reason := getMarketplaceCSIDriverStatusReason(ctx, key, r.Client, common.ConditionTypeInstall)
		Expect(reason).To(Equal("Installing"))
	})

	It("CSI Driver installing status update error", func() {
		cl := newMarketplaceCSIDriverClient(fake.NewFakeClient(driver), "", "", "")
		cl.setStatusErrorAction("update")
		r.Client = cl
		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			},
		}

		_, err := r.Reconcile(req)
		Expect(err).NotTo(HaveOccurred(), "Reconcile returned error for install")

		key := client.ObjectKey{
			Name:      name,
			Namespace: namespace,
		}
		reason := getMarketplaceCSIDriverStatusReason(ctx, key, r.Client, common.ConditionTypeInstall)
		Expect(reason).To(Equal(""))
	})

	It("CSI Driver install error status update error", func() {
		cl := newMarketplaceCSIDriverClient(fake.NewFakeClient(driver), "create", "CSIDriver", "csi.rhm.cos.ibm.com")
		cl.setStatusErrorAction("update")
		r.Client = cl
		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			},
		}

		_, err := r.Reconcile(req)
		Expect(err).To(HaveOccurred(), "Reconcile should return error for install")

		key := client.ObjectKey{
			Name:      name,
			Namespace: namespace,
		}
		reason := getMarketplaceCSIDriverStatusReason(ctx, key, r.Client, common.ConditionTypeInstall)
		Expect(reason).To(Equal(""))
	})

	It("CSI Driver install complete without pull secret", func() {
		obj, err := ra.GetResource(common.AssetPathCsiControllerService)
		Expect(err).NotTo(HaveOccurred(), "Test set up failed")
		ss := obj.(*appsv1.StatefulSet)
		ss.Status = appsv1.StatefulSetStatus{
			ReadyReplicas: 2,
		}

		r.Client = fake.NewFakeClient(driver, ss)
		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			},
		}

		_, err = r.Reconcile(req)
		Expect(err).NotTo(HaveOccurred(), "Reconcile returned error for install")

		key := client.ObjectKey{
			Name:      name,
			Namespace: namespace,
		}
		reason := getMarketplaceCSIDriverStatusReason(ctx, key, r.Client, common.ConditionTypeInstall)
		Expect(reason).To(Equal("InstallComplete"))
		reason = getMarketplaceCSIDriverStatusReason(ctx, key, r.Client, common.ConditionTypeDriver)
		Expect(reason).To(Equal("CredentialError"))
	})

	It("CSI Driver install complete with pull secret", func() {
		obj, err := ra.GetResource(common.AssetPathCsiControllerService)
		Expect(err).NotTo(HaveOccurred(), "Test set up failed")
		ss := obj.(*appsv1.StatefulSet)
		ss.Status = appsv1.StatefulSetStatus{
			ReadyReplicas: 2,
		}

		data := make(map[string][]byte)
		data["PULL_SECRET"] = []byte("test")
		sec := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      "redhat-marketplace-pull-secret",
			},
			Type: corev1.SecretTypeOpaque,
			Data: data,
		}

		r.Client = fake.NewFakeClient(driver, ss, sec)
		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			},
		}

		_, err = r.Reconcile(req)
		Expect(err).NotTo(HaveOccurred(), "Reconcile returned error for install")

		key := client.ObjectKey{
			Name:      name,
			Namespace: namespace,
		}
		reason := getMarketplaceCSIDriverStatusReason(ctx, key, r.Client, common.ConditionTypeInstall)
		Expect(reason).To(Equal("InstallComplete"))
		reason = getMarketplaceCSIDriverStatusReason(ctx, key, r.Client, common.ConditionTypeDriver)
		Expect(reason).To(Equal("Operational"))
	})

	It("CSI Driver install complete with pull secret hmac", func() {
		obj, err := ra.GetResource(common.AssetPathCsiControllerService)
		Expect(err).NotTo(HaveOccurred(), "Test set up failed")
		ss := obj.(*appsv1.StatefulSet)
		ss.Status = appsv1.StatefulSetStatus{
			ReadyReplicas: 2,
		}

		data := make(map[string][]byte)
		data["AccessKeyID"] = []byte("test")
		data["SecretAccessKey"] = []byte("test")
		sec := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      "redhat-marketplace-pull-secret",
			},
			Type: corev1.SecretTypeOpaque,
			Data: data,
		}
		driver.Spec.Credential.Type = "hmac"
		driver.Spec.HealthCheckIntervalInMinutes = 10

		r.Client = fake.NewFakeClient(driver, ss, sec)
		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			},
		}

		_, err = r.Reconcile(req)
		Expect(err).NotTo(HaveOccurred(), "Reconcile returned error for install")

		key := client.ObjectKey{
			Name:      name,
			Namespace: namespace,
		}
		reason := getMarketplaceCSIDriverStatusReason(ctx, key, r.Client, common.ConditionTypeInstall)
		Expect(reason).To(Equal("InstallComplete"))
		reason = getMarketplaceCSIDriverStatusReason(ctx, key, r.Client, common.ConditionTypeDriver)
		Expect(reason).To(Equal("Operational"))
	})

	It("CSI Driver install complete with bad pull secret", func() {
		obj, err := ra.GetResource(common.AssetPathCsiControllerService)
		Expect(err).NotTo(HaveOccurred(), "Test set up failed")
		ss := obj.(*appsv1.StatefulSet)
		ss.Status = appsv1.StatefulSetStatus{
			ReadyReplicas: 2,
		}

		data := make(map[string][]byte)
		data["PULL_SECRET_BAD"] = []byte("test")
		sec := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      "redhat-marketplace-pull-secret",
			},
			Type: corev1.SecretTypeOpaque,
			Data: data,
		}

		r.Client = fake.NewFakeClient(driver, ss, sec)
		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			},
		}

		_, err = r.Reconcile(req)
		Expect(err).NotTo(HaveOccurred(), "Reconcile returned error for install")

		key := client.ObjectKey{
			Name:      name,
			Namespace: namespace,
		}
		reason := getMarketplaceCSIDriverStatusReason(ctx, key, r.Client, common.ConditionTypeInstall)
		Expect(reason).To(Equal("InstallComplete"))
		reason = getMarketplaceCSIDriverStatusReason(ctx, key, r.Client, common.ConditionTypeDriver)
		Expect(reason).To(Equal("CredentialError"))
	})

	It("CSI Driver install complete status update error", func() {
		obj, err := ra.GetResource(common.AssetPathCsiControllerService)
		Expect(err).NotTo(HaveOccurred(), "Test set up failed")
		ss := obj.(*appsv1.StatefulSet)
		ss.Status = appsv1.StatefulSetStatus{
			ReadyReplicas: 2,
		}

		data := make(map[string][]byte)
		data["PULL_SECRET"] = []byte("test")
		sec := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      "redhat-marketplace-pull-secret",
			},
			Type: corev1.SecretTypeOpaque,
			Data: data,
		}

		cl := newMarketplaceCSIDriverClient(fake.NewFakeClient(driver, ss, sec), "", "", "")
		cl.setStatusErrorAction("update")
		r.Client = cl
		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			},
		}

		_, err = r.Reconcile(req)
		Expect(err).NotTo(HaveOccurred(), "Reconcile returned error for install")

		key := client.ObjectKey{
			Name:      name,
			Namespace: namespace,
		}
		reason := getMarketplaceCSIDriverStatusReason(ctx, key, r.Client, common.ConditionTypeInstall)
		Expect(reason).To(Equal(""))
		reason = getMarketplaceCSIDriverStatusReason(ctx, key, r.Client, common.ConditionTypeDriver)
		Expect(reason).To(Equal(""))
	})

	It("CSI Driver health check", func() {
		driver.Spec.HealthCheckIntervalInMinutes = 1
		driver.Status.DriverPods = map[string]marketplacev1alpha1.DriverPod{
			"csi-rhm-cos-ss-1": {
				CreateTime: testStartTime.Format(time.RFC3339),
				Version:    "1",
				NodeName:   "test-node",
			},
			"csi-rhm-cos-ds-1": {
				CreateTime: testStartTime.Format(time.RFC3339),
				Version:    "1",
				NodeName:   "test-node",
			},
			"csi-rhm-cos-ctrl-1": {
				CreateTime: testStartTime.Format(time.RFC3339),
				Version:    "1",
				NodeName:   "test-node",
			},
		}

		crobj, err := ra.GetResource(common.AssetPathRbacCsiClusterRole)
		Expect(err).NotTo(HaveOccurred(), "Test set up failed")
		cr := crobj.(*rbacv1.ClusterRole)

		crbobj, err := ra.GetResource(common.AssetPathRbacCsiClusterRoleBinding)
		Expect(err).NotTo(HaveOccurred(), "Test set up failed")
		crb := crbobj.(*rbacv1.ClusterRoleBinding)

		cl := newMarketplaceCSIDriverClient(createMarketplaceCSIDriverClientForUpdate(ra, driver), "", "", "")
		cl.addRuntimeObject(cr)
		cl.addRuntimeObject(crb)
		r.Client = cl
		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			},
		}
		lastHealthCheckTime = time.Now().Add(time.Minute * -5)

		_, err = r.Reconcile(req)
		Expect(err).NotTo(HaveOccurred(), "Reconcile returned error")

		key := client.ObjectKey{
			Name:      name,
			Namespace: namespace,
		}
		reason := getMarketplaceCSIDriverStatusReason(ctx, key, r.Client, common.ConditionTypeInstall)
		Expect(reason).To(Equal("InstallComplete"))
		reason = getMarketplaceCSIDriverStatusReason(ctx, key, r.Client, common.ConditionTypeDriver)
		Expect(reason).To(Equal("RepairInitiated"))
	})

	It("CSI Driver health check fail", func() {
		driver.Spec.HealthCheckIntervalInMinutes = 1
		driver.Status.DriverPods = map[string]marketplacev1alpha1.DriverPod{
			"csi-rhm-cos-ss-1": {
				CreateTime: testStartTime.Format(time.RFC3339),
				Version:    "1",
				NodeName:   "test-node",
			},
			"csi-rhm-cos-ds-1": {
				CreateTime: testStartTime.Format(time.RFC3339),
				Version:    "1",
				NodeName:   "test-node",
			},
			"csi-rhm-cos-ctrl-1": {
				CreateTime: testStartTime.Format(time.RFC3339),
				Version:    "1",
				NodeName:   "test-node",
			},
		}

		crobj, err := ra.GetResource(common.AssetPathRbacCsiClusterRole)
		Expect(err).NotTo(HaveOccurred(), "Test set up failed")
		cr := crobj.(*rbacv1.ClusterRole)

		crbobj, err := ra.GetResource(common.AssetPathRbacCsiClusterRoleBinding)
		Expect(err).NotTo(HaveOccurred(), "Test set up failed")
		crb := crbobj.(*rbacv1.ClusterRoleBinding)

		cl := newMarketplaceCSIDriverClient(createMarketplaceCSIDriverClientForUpdate(ra, driver), "create", "MarketplaceDriverHealthCheck", common.NameNodeHealthCheck)
		cl.addRuntimeObject(cr)
		cl.addRuntimeObject(crb)
		r.Client = cl
		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			},
		}
		lastHealthCheckTime = time.Now().Add(time.Minute * -5)

		_, err = r.Reconcile(req)
		Expect(err).To(HaveOccurred(), "Reconcile did not return error for health check failure")

		key := client.ObjectKey{
			Name:      name,
			Namespace: namespace,
		}
		reason := getMarketplaceCSIDriverStatusReason(ctx, key, r.Client, common.ConditionTypeInstall)
		Expect(reason).To(Equal("InstallComplete"))
		reason = getMarketplaceCSIDriverStatusReason(ctx, key, r.Client, common.ConditionTypeDriver)
		Expect(reason).To(Equal("DriverHealthCheckFailed"))
	})

	It("CSI Driver install error - storageclass", func() {
		r.Client = newMarketplaceCSIDriverClient(
			fake.NewFakeClient(driver),
			"create",
			"StorageClass",
			"csi-rhm-cos",
		)
		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			},
		}

		_, err := r.Reconcile(req)
		Expect(err).To(HaveOccurred(), "Reconcile should return error for install")

		key := client.ObjectKey{
			Name:      name,
			Namespace: namespace,
		}
		reason := getMarketplaceCSIDriverStatusReason(ctx, key, r.Client, common.ConditionTypeInstall)
		Expect(reason).To(Equal("InstallFailed"))
	})

	It("CSI Driver install error - clusterrole", func() {
		r.Client = newMarketplaceCSIDriverClient(
			fake.NewFakeClient(driver),
			"create",
			"ClusterRole",
			"csi-rhm-cos-csi-cluster-role",
		)
		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			},
		}

		_, err := r.Reconcile(req)
		Expect(err).To(HaveOccurred(), "Reconcile should return error for install")

		key := client.ObjectKey{
			Name:      name,
			Namespace: namespace,
		}
		reason := getMarketplaceCSIDriverStatusReason(ctx, key, r.Client, common.ConditionTypeInstall)
		Expect(reason).To(Equal("InstallFailed"))
	})

	It("CSI Driver install error - ClusterRoleBinding", func() {
		r.Client = newMarketplaceCSIDriverClient(
			fake.NewFakeClient(driver),
			"create",
			"ClusterRoleBinding",
			"csi-rhm-cos-csi-cluster-role-binding",
		)
		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			},
		}

		_, err := r.Reconcile(req)
		Expect(err).To(HaveOccurred(), "Reconcile should return error for install")

		key := client.ObjectKey{
			Name:      name,
			Namespace: namespace,
		}
		reason := getMarketplaceCSIDriverStatusReason(ctx, key, r.Client, common.ConditionTypeInstall)
		Expect(reason).To(Equal("InstallFailed"))
	})

	It("CSI Driver install error - Role", func() {
		r.Client = newMarketplaceCSIDriverClient(
			fake.NewFakeClient(driver),
			"create",
			"Role",
			"csi-rhm-cos-csi-role",
		)
		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			},
		}

		_, err := r.Reconcile(req)
		Expect(err).To(HaveOccurred(), "Reconcile should return error for install")

		key := client.ObjectKey{
			Name:      name,
			Namespace: namespace,
		}
		reason := getMarketplaceCSIDriverStatusReason(ctx, key, r.Client, common.ConditionTypeInstall)
		Expect(reason).To(Equal("InstallFailed"))
	})

	It("CSI Driver install error - RoleBinding", func() {
		r.Client = newMarketplaceCSIDriverClient(
			fake.NewFakeClient(driver),
			"create",
			"RoleBinding",
			"csi-rhm-cos-csi-role-binding",
		)
		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			},
		}

		_, err := r.Reconcile(req)
		Expect(err).To(HaveOccurred(), "Reconcile should return error for install")

		key := client.ObjectKey{
			Name:      name,
			Namespace: namespace,
		}
		reason := getMarketplaceCSIDriverStatusReason(ctx, key, r.Client, common.ConditionTypeInstall)
		Expect(reason).To(Equal("InstallFailed"))
	})

	It("CSI Driver install error - ServiceAccount", func() {
		r.Client = newMarketplaceCSIDriverClient(
			fake.NewFakeClient(driver),
			"create",
			"ServiceAccount",
			"csi-rhm-cos",
		)
		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			},
		}

		_, err := r.Reconcile(req)
		Expect(err).To(HaveOccurred(), "Reconcile should return error for install")

		key := client.ObjectKey{
			Name:      name,
			Namespace: namespace,
		}
		reason := getMarketplaceCSIDriverStatusReason(ctx, key, r.Client, common.ConditionTypeInstall)
		Expect(reason).To(Equal("InstallFailed"))
	})

	It("CSI Driver install error - DaemonSet ds", func() {
		r.Client = newMarketplaceCSIDriverClient(
			fake.NewFakeClient(driver),
			"create",
			"DaemonSet",
			"csi-rhm-cos-ds",
		)
		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			},
		}

		_, err := r.Reconcile(req)
		Expect(err).To(HaveOccurred(), "Reconcile should return error for install")

		key := client.ObjectKey{
			Name:      name,
			Namespace: namespace,
		}
		reason := getMarketplaceCSIDriverStatusReason(ctx, key, r.Client, common.ConditionTypeInstall)
		Expect(reason).To(Equal("InstallFailed"))
	})

	It("CSI Driver install error - DaemonSet ctrl", func() {
		r.Client = newMarketplaceCSIDriverClient(
			fake.NewFakeClient(driver),
			"create",
			"DaemonSet",
			"csi-rhm-cos-ctrl",
		)
		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			},
		}

		_, err := r.Reconcile(req)
		Expect(err).To(HaveOccurred(), "Reconcile should return error for install")

		key := client.ObjectKey{
			Name:      name,
			Namespace: namespace,
		}
		reason := getMarketplaceCSIDriverStatusReason(ctx, key, r.Client, common.ConditionTypeInstall)
		Expect(reason).To(Equal("InstallFailed"))
	})

	It("CSI Driver install error - StatefulSet", func() {
		r.Client = newMarketplaceCSIDriverClient(
			fake.NewFakeClient(driver),
			"create",
			"StatefulSet",
			"csi-rhm-cos-ss",
		)
		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			},
		}

		_, err := r.Reconcile(req)
		Expect(err).To(HaveOccurred(), "Reconcile should return error for install")

		key := client.ObjectKey{
			Name:      name,
			Namespace: namespace,
		}
		reason := getMarketplaceCSIDriverStatusReason(ctx, key, r.Client, common.ConditionTypeInstall)
		Expect(reason).To(Equal("InstallFailed"))
	})

	It("CSI Driver install error - Service", func() {
		r.Client = newMarketplaceCSIDriverClient(
			fake.NewFakeClient(driver),
			"create",
			"Service",
			"csi-rhm-cos-ss",
		)
		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			},
		}

		_, err := r.Reconcile(req)
		Expect(err).To(HaveOccurred(), "Reconcile should return error for install")

		key := client.ObjectKey{
			Name:      name,
			Namespace: namespace,
		}
		reason := getMarketplaceCSIDriverStatusReason(ctx, key, r.Client, common.ConditionTypeInstall)
		Expect(reason).To(Equal("InstallFailed"))
	})

	It("CSI Driver install error - pod list", func() {
		crobj, err := ra.GetResource(common.AssetPathRbacCsiClusterRole)
		Expect(err).NotTo(HaveOccurred(), "Test set up failed")
		cr := crobj.(*rbacv1.ClusterRole)

		crbobj, err := ra.GetResource(common.AssetPathRbacCsiClusterRoleBinding)
		Expect(err).NotTo(HaveOccurred(), "Test set up failed")
		crb := crbobj.(*rbacv1.ClusterRoleBinding)

		cl := newMarketplaceCSIDriverClient(
			createMarketplaceCSIDriverClientForUpdate(ra, driver),
			"list",
			"PodList",
			"",
		)
		cl.addRuntimeObject(cr)
		cl.addRuntimeObject(crb)

		r.Client = cl
		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			},
		}

		_, err = r.Reconcile(req)
		Expect(err).NotTo(HaveOccurred(), "Reconcile returned error for install")

		key := client.ObjectKey{
			Name:      name,
			Namespace: namespace,
		}
		reason := getMarketplaceCSIDriverStatusReason(ctx, key, r.Client, common.ConditionTypeInstall)
		Expect(reason).To(Equal("InstallComplete"))
		reason = getMarketplaceCSIDriverStatusReason(ctx, key, r.Client, common.ConditionTypeDriver)
		Expect(reason).To(Equal("DriverHealthCheckFailed"))
	})

	It("CSI Driver install error - csidriver no change", func() {
		crobj, err := ra.GetResource(common.AssetPathRbacCsiClusterRole)
		Expect(err).NotTo(HaveOccurred(), "Test set up failed")
		cr := crobj.(*rbacv1.ClusterRole)

		crbobj, err := ra.GetResource(common.AssetPathRbacCsiClusterRoleBinding)
		Expect(err).NotTo(HaveOccurred(), "Test set up failed")
		crb := crbobj.(*rbacv1.ClusterRoleBinding)

		csobj, err := ra.GetResource(common.AssetPathCsiDriver)
		Expect(err).NotTo(HaveOccurred(), "Test set up failed")
		cs := csobj.(*storagev1beta1.CSIDriver)
		cs.OwnerReferences = []metav1.OwnerReference{ra.Owner}

		cl := newMarketplaceCSIDriverClient(createMarketplaceCSIDriverClientForUpdate(ra, driver), "", "", "")
		cl.addRuntimeObject(cs)
		cl.addRuntimeObject(cr)
		cl.addRuntimeObject(crb)
		r.Client = cl
		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			},
		}

		_, err = r.Reconcile(req)
		Expect(err).NotTo(HaveOccurred(), "Reconcile returned error for install")

		key := client.ObjectKey{
			Name:      name,
			Namespace: namespace,
		}
		reason := getMarketplaceCSIDriverStatusReason(ctx, key, r.Client, common.ConditionTypeInstall)
		Expect(reason).To(Equal("InstallComplete"))
	})

	It("CSI Driver install error - storageclass no change", func() {
		crobj, err := ra.GetResource(common.AssetPathRbacCsiClusterRole)
		Expect(err).NotTo(HaveOccurred(), "Test set up failed")
		cr := crobj.(*rbacv1.ClusterRole)

		crbobj, err := ra.GetResource(common.AssetPathRbacCsiClusterRoleBinding)
		Expect(err).NotTo(HaveOccurred(), "Test set up failed")
		crb := crbobj.(*rbacv1.ClusterRoleBinding)

		scobj, err := ra.GetResource(common.AssetPathStorageClass)
		Expect(err).NotTo(HaveOccurred(), "Test set up failed")
		sc := scobj.(*storagev1beta1.StorageClass)
		sc.OwnerReferences = []metav1.OwnerReference{ra.Owner}

		cl := newMarketplaceCSIDriverClient(createMarketplaceCSIDriverClientForUpdate(ra, driver), "", "", "")
		cl.addRuntimeObject(sc)
		cl.addRuntimeObject(cr)
		cl.addRuntimeObject(crb)
		r.Client = cl
		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			},
		}

		_, err = r.Reconcile(req)
		Expect(err).NotTo(HaveOccurred(), "Reconcile returned error for install")

		key := client.ObjectKey{
			Name:      name,
			Namespace: namespace,
		}
		reason := getMarketplaceCSIDriverStatusReason(ctx, key, r.Client, common.ConditionTypeInstall)
		Expect(reason).To(Equal("InstallComplete"))
	})

	It("CSI Driver install error - statefulset no change", func() {
		crobj, err := ra.GetResource(common.AssetPathRbacCsiClusterRole)
		Expect(err).NotTo(HaveOccurred(), "Test set up failed")
		cr := crobj.(*rbacv1.ClusterRole)

		crbobj, err := ra.GetResource(common.AssetPathRbacCsiClusterRoleBinding)
		Expect(err).NotTo(HaveOccurred(), "Test set up failed")
		crb := crbobj.(*rbacv1.ClusterRoleBinding)

		scobj, err := ra.GetResource(common.AssetPathCsiControllerService)
		Expect(err).NotTo(HaveOccurred(), "Test set up failed")
		sc := scobj.(*appsv1.StatefulSet)
		sc.OwnerReferences = []metav1.OwnerReference{ra.Owner}
		sc.Status = appsv1.StatefulSetStatus{
			ReadyReplicas: 2,
		}

		cl := newMarketplaceCSIDriverClient(createMarketplaceCSIDriverClientForUpdate(ra, driver), "", "", "")
		cl.addRuntimeObject(sc)
		cl.addRuntimeObject(cr)
		cl.addRuntimeObject(crb)
		r.Client = cl
		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			},
		}

		_, err = r.Reconcile(req)
		Expect(err).NotTo(HaveOccurred(), "Reconcile returned error for install")

		key := client.ObjectKey{
			Name:      name,
			Namespace: namespace,
		}
		reason := getMarketplaceCSIDriverStatusReason(ctx, key, r.Client, common.ConditionTypeInstall)
		Expect(reason).To(Equal("InstallComplete"))
	})

	It("CSI Driver install error - service no change", func() {
		crobj, err := ra.GetResource(common.AssetPathRbacCsiClusterRole)
		Expect(err).NotTo(HaveOccurred(), "Test set up failed")
		cr := crobj.(*rbacv1.ClusterRole)

		crbobj, err := ra.GetResource(common.AssetPathRbacCsiClusterRoleBinding)
		Expect(err).NotTo(HaveOccurred(), "Test set up failed")
		crb := crbobj.(*rbacv1.ClusterRoleBinding)

		scobj, err := ra.GetResource(common.AssetPathCsiService)
		Expect(err).NotTo(HaveOccurred(), "Test set up failed")
		sc := scobj.(*corev1.Service)
		sc.OwnerReferences = []metav1.OwnerReference{ra.Owner}

		cl := newMarketplaceCSIDriverClient(createMarketplaceCSIDriverClientForUpdate(ra, driver), "", "", "")
		cl.addRuntimeObject(sc)
		cl.addRuntimeObject(cr)
		cl.addRuntimeObject(crb)
		r.Client = cl
		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			},
		}

		_, err = r.Reconcile(req)
		Expect(err).NotTo(HaveOccurred(), "Reconcile returned error for install")

		key := client.ObjectKey{
			Name:      name,
			Namespace: namespace,
		}
		reason := getMarketplaceCSIDriverStatusReason(ctx, key, r.Client, common.ConditionTypeInstall)
		Expect(reason).To(Equal("InstallComplete"))
	})

	It("CSI Driver install error - serviceaccount no change", func() {
		crobj, err := ra.GetResource(common.AssetPathRbacCsiClusterRole)
		Expect(err).NotTo(HaveOccurred(), "Test set up failed")
		cr := crobj.(*rbacv1.ClusterRole)

		crbobj, err := ra.GetResource(common.AssetPathRbacCsiClusterRoleBinding)
		Expect(err).NotTo(HaveOccurred(), "Test set up failed")
		crb := crbobj.(*rbacv1.ClusterRoleBinding)

		scobj, err := ra.GetResource(common.AssetPathRbacServiceAccount)
		Expect(err).NotTo(HaveOccurred(), "Test set up failed")
		sc := scobj.(*corev1.ServiceAccount)
		sc.OwnerReferences = []metav1.OwnerReference{ra.Owner}

		cl := newMarketplaceCSIDriverClient(createMarketplaceCSIDriverClientForUpdate(ra, driver), "", "", "")
		cl.addRuntimeObject(sc)
		cl.addRuntimeObject(cr)
		cl.addRuntimeObject(crb)
		r.Client = cl
		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			},
		}

		_, err = r.Reconcile(req)
		Expect(err).NotTo(HaveOccurred(), "Reconcile returned error for install")

		key := client.ObjectKey{
			Name:      name,
			Namespace: namespace,
		}
		reason := getMarketplaceCSIDriverStatusReason(ctx, key, r.Client, common.ConditionTypeInstall)
		Expect(reason).To(Equal("InstallComplete"))
	})

	It("CSI Driver install error - serviceaccount ownerref change", func() {
		crobj, err := ra.GetResource(common.AssetPathRbacCsiClusterRole)
		Expect(err).NotTo(HaveOccurred(), "Test set up failed")
		cr := crobj.(*rbacv1.ClusterRole)

		crbobj, err := ra.GetResource(common.AssetPathRbacCsiClusterRoleBinding)
		Expect(err).NotTo(HaveOccurred(), "Test set up failed")
		crb := crbobj.(*rbacv1.ClusterRoleBinding)

		scobj, err := ra.GetResource(common.AssetPathRbacServiceAccount)
		Expect(err).NotTo(HaveOccurred(), "Test set up failed")
		sc := scobj.(*corev1.ServiceAccount)
		falseVar := false
		sc.OwnerReferences = []metav1.OwnerReference{
			{
				APIVersion:         "v1",
				Kind:               "Pod",
				Name:               "test",
				Controller:         &falseVar,
				BlockOwnerDeletion: &falseVar,
			},
		}

		cl := newMarketplaceCSIDriverClient(createMarketplaceCSIDriverClientForUpdate(ra, driver), "", "", "")
		cl.addRuntimeObject(sc)
		cl.addRuntimeObject(cr)
		cl.addRuntimeObject(crb)
		r.Client = cl
		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			},
		}

		_, err = r.Reconcile(req)
		Expect(err).NotTo(HaveOccurred(), "Reconcile returned error for install")

		key := client.ObjectKey{
			Name:      name,
			Namespace: namespace,
		}
		reason := getMarketplaceCSIDriverStatusReason(ctx, key, r.Client, common.ConditionTypeInstall)
		Expect(reason).To(Equal("InstallComplete"))
	})

	It("CSI Driver install error - rbac no change", func() {
		crobj, err := ra.GetResource(common.AssetPathRbacCsiClusterRole)
		Expect(err).NotTo(HaveOccurred(), "Test set up failed")
		cr := crobj.(*rbacv1.ClusterRole)
		cr.OwnerReferences = []metav1.OwnerReference{ra.Owner}

		crbobj, err := ra.GetResource(common.AssetPathRbacCsiClusterRoleBinding)
		Expect(err).NotTo(HaveOccurred(), "Test set up failed")
		crb := crbobj.(*rbacv1.ClusterRoleBinding)
		crb.OwnerReferences = []metav1.OwnerReference{ra.Owner}

		robj, err := ra.GetResource(common.AssetPathRbacCsiRole)
		Expect(err).NotTo(HaveOccurred(), "Test set up failed")
		ro := robj.(*rbacv1.Role)
		ro.OwnerReferences = []metav1.OwnerReference{ra.Owner}

		rbobj, err := ra.GetResource(common.AssetPathRbacCsiRoleBinding)
		Expect(err).NotTo(HaveOccurred(), "Test set up failed")
		rb := rbobj.(*rbacv1.RoleBinding)
		rb.OwnerReferences = []metav1.OwnerReference{ra.Owner}

		cl := newMarketplaceCSIDriverClient(createMarketplaceCSIDriverClientForUpdate(ra, driver), "", "", "")
		cl.addRuntimeObject(ro)
		cl.addRuntimeObject(rb)
		cl.addRuntimeObject(cr)
		cl.addRuntimeObject(crb)
		r.Client = cl
		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			},
		}

		_, err = r.Reconcile(req)
		Expect(err).NotTo(HaveOccurred(), "Reconcile returned error for install")

		key := client.ObjectKey{
			Name:      name,
			Namespace: namespace,
		}
		reason := getMarketplaceCSIDriverStatusReason(ctx, key, r.Client, common.ConditionTypeInstall)
		Expect(reason).To(Equal("InstallComplete"))
	})

	It("CSI Driver install error - olm rbac", func() {
		cl := newMarketplaceCSIDriverClient(fake.NewFakeClient(driver), "create", "ClusterRole", "olm-marketplace-dataset-cluster-role")
		ra.Client = cl
		ra.ApplyOLMRbac()

		cl = newMarketplaceCSIDriverClient(fake.NewFakeClient(driver), "create", "ClusterRoleBinding", "olm-marketplace-dataset-cluster-role-binding")
		ra.Client = cl
		ra.ApplyOLMRbac()

		cl = newMarketplaceCSIDriverClient(fake.NewFakeClient(driver), "create", "Role", "olm-marketplace-dataset-leader-election-role")
		ra.Client = cl
		ra.ApplyOLMRbac()

		cl = newMarketplaceCSIDriverClient(fake.NewFakeClient(driver), "create", "RoleBinding", "olm-marketplace-dataset-leader-election-role-binding")
		ra.Client = cl
		ra.ApplyOLMRbac()
	})

	It("CSI Driver install error - driver configmap", func() {
		r.Client = newMarketplaceCSIDriverClient(
			fake.NewFakeClient(driver),
			"create",
			"ConfigMap",
			"csi-rhm-cos-env",
		)
		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			},
		}

		_, err := r.Reconcile(req)
		Expect(err).To(HaveOccurred(), "Reconcile should return error for install")

		key := client.ObjectKey{
			Name:      name,
			Namespace: namespace,
		}
		reason := getMarketplaceCSIDriverStatusReason(ctx, key, r.Client, common.ConditionTypeInstall)
		Expect(reason).To(Equal("InstallFailed"))
	})

	It("CSI Driver - configmap change", func() {
		cmobj, err := ra.GetResource(common.AssetPathDriverEnvironment)
		Expect(err).NotTo(HaveOccurred(), "Test set up failed")
		cm := cmobj.(*corev1.ConfigMap)
		cm.Data["test1"] = "value1"

		crobj, err := ra.GetResource(common.AssetPathRbacCsiClusterRole)
		Expect(err).NotTo(HaveOccurred(), "Test set up failed")
		cr := crobj.(*rbacv1.ClusterRole)

		crbobj, err := ra.GetResource(common.AssetPathRbacCsiClusterRoleBinding)
		Expect(err).NotTo(HaveOccurred(), "Test set up failed")
		crb := crbobj.(*rbacv1.ClusterRoleBinding)

		cl := newMarketplaceCSIDriverClient(createMarketplaceCSIDriverClientForUpdate(ra, driver), "", "", "")
		cl.addRuntimeObject(cm)
		cl.addRuntimeObject(cr)
		cl.addRuntimeObject(crb)
		r.Client = cl
		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			},
		}

		_, err = r.Reconcile(req)
		Expect(err).NotTo(HaveOccurred(), "Reconcile returned error for install")

		key := client.ObjectKey{
			Name:      name,
			Namespace: namespace,
		}
		reason := getMarketplaceCSIDriverStatusReason(ctx, key, r.Client, common.ConditionTypeInstall)
		Expect(reason).To(Equal("InstallComplete"))
	})

	It("Resource apply - unsupported resource", func() {
		_, err := ra.Apply("assets/csi-unit-test.yaml")
		Expect(err).To(HaveOccurred(), "Expected error for bad resource")

		_, err = ra.Apply("assets/csi-unit-test-2.yaml")
		Expect(err).To(HaveOccurred(), "Expected error for bad resource")
	})

	It("Label helper tests", func() {
		lbls := make(map[string]string)
		val := resourceapply.GetLabelValue(lbls, "key1")
		Expect(val).To(Equal(""), "expected empty string")

		lbls["key1"] = "value1"
		val = resourceapply.GetLabelValue(lbls, "key1")
		Expect(val).To(Equal("value1"), "expected value1")

		val = resourceapply.GetLabelValue(lbls, "key2")
		Expect(val).To(Equal(""), "expected empty string")
	})
})

func getMarketplaceCSIDriverStatusReason(ctx context.Context, key client.ObjectKey, cl client.Client, t status.ConditionType) string {
	obj := &marketplacev1alpha1.MarketplaceCSIDriver{}
	err := cl.Get(ctx, key, obj)
	if err != nil {
		return fmt.Sprintf("%s", err)
	}
	c := obj.Status.Conditions.GetCondition(t)
	if c == nil {
		return ""
	}
	return string(c.Reason)
}

func createMarketplaceCSIDriverClientForUpdate(ra resourceapply.ResourceApply, driver *marketplacev1alpha1.MarketplaceCSIDriver) client.Client {
	ssobj, err := ra.GetResource(common.AssetPathCsiControllerService)
	Expect(err).NotTo(HaveOccurred(), "Test set up failed")
	ss := ssobj.(*appsv1.StatefulSet)
	ss.Status = appsv1.StatefulSetStatus{
		ReadyReplicas: 2,
	}
	ss.Spec.Template.Labels["csi.rhm.cos.ibm.com/driverVersion"] = "999.99.99"

	dmobj1, err := ra.GetResource(common.AssetPathCsiNodeService)
	Expect(err).NotTo(HaveOccurred(), "Test set up failed")
	dm1 := dmobj1.(*appsv1.DaemonSet)
	dm1.Spec.Template.Labels["csi.rhm.cos.ibm.com/driverVersion"] = "999.99.99"

	dmobj2, err := ra.GetResource(common.AssetPathCsiReconcileService)
	Expect(err).NotTo(HaveOccurred(), "Test set up failed")
	dm2 := dmobj2.(*appsv1.DaemonSet)
	dm2.OwnerReferences = []metav1.OwnerReference{ra.Owner}

	csobj, err := ra.GetResource(common.AssetPathCsiDriver)
	Expect(err).NotTo(HaveOccurred(), "Test set up failed")
	cs := csobj.(*storagev1beta1.CSIDriver)

	scobj, err := ra.GetResource(common.AssetPathStorageClass)
	Expect(err).NotTo(HaveOccurred(), "Test set up failed")
	sc := scobj.(*storagev1beta1.StorageClass)
	sc.Provisioner = "test"

	saobj, err := ra.GetResource(common.AssetPathRbacServiceAccount)
	Expect(err).NotTo(HaveOccurred(), "Test set up failed")
	sa := saobj.(*corev1.ServiceAccount)

	svobj, err := ra.GetResource(common.AssetPathCsiService)
	Expect(err).NotTo(HaveOccurred(), "Test set up failed")
	sv := svobj.(*corev1.Service)

	rrobj, err := ra.GetResource(common.AssetPathRbacCsiRole)
	Expect(err).NotTo(HaveOccurred(), "Test set up failed")
	rr := rrobj.(*rbacv1.Role)

	rrbobj, err := ra.GetResource(common.AssetPathRbacCsiRoleBinding)
	Expect(err).NotTo(HaveOccurred(), "Test set up failed")
	rrb := rrbobj.(*rbacv1.RoleBinding)

	crobj, err := ra.GetResource(common.AssetPathRbacCsiClusterRole)
	Expect(err).NotTo(HaveOccurred(), "Test set up failed")
	cr := crobj.(*rbacv1.ClusterRole)

	crbobj, err := ra.GetResource(common.AssetPathRbacCsiClusterRoleBinding)
	Expect(err).NotTo(HaveOccurred(), "Test set up failed")
	crb := crbobj.(*rbacv1.ClusterRoleBinding)

	data := make(map[string][]byte)
	data["PULL_SECRET"] = []byte("test")
	sec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: driver.Namespace,
			Name:      "redhat-marketplace-pull-secret",
		},
		Type: corev1.SecretTypeOpaque,
		Data: data,
	}

	cmobj, err := ra.GetResource(common.AssetPathDriverEnvironment)
	Expect(err).NotTo(HaveOccurred(), "Test set up failed")
	cm := cmobj.(*corev1.ConfigMap)
	cm.Data["test1"] = "value1"
	cm.OwnerReferences = []metav1.OwnerReference{ra.Owner}

	return fake.NewFakeClient(driver, ss, sec, dm1, dm2, cs, sc, sa,
		sv, rr, rrb, cr, crb, cm,
		createMarketplaceCSIDriverPod("csi-rhm-cos-ss-1", "csi-rhm-cos-ss"),
		createMarketplaceCSIDriverPod("csi-rhm-cos-ds-1", "csi-rhm-cos-ds"),
		createMarketplaceCSIDriverPod("csi-rhm-cos-ctrl-1", "csi-rhm-cos-ctrl"))
}

func createMarketplaceCSIDriverPod(name, lbl string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: common.MarketplaceNamespace,
			Labels:    map[string]string{common.LabelApp: lbl},
		},
		Spec: corev1.PodSpec{
			NodeName: "test-node",
		},
	}
}
