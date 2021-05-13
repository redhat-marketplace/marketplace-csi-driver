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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	marketplacev1alpha1 "github.com/redhat-marketplace/marketplace-csi-driver/operator/api/v1alpha1"
	"github.com/redhat-marketplace/marketplace-csi-driver/operator/pkg/common"
	"github.com/redhat-marketplace/marketplace-csi-driver/operator/pkg/resourceapply"
	"github.com/redhat-marketplace/marketplace-csi-driver/operator/pkg/status"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Context("MarketplaceDataset Controller", func() {
	var (
		ctx       = context.TODO()
		namespace = "default"
		name      = "us-labor-stats"
		dr        *MarketplaceDatasetReconciler
		dataset   *marketplacev1alpha1.MarketplaceDataset
		ra        resourceapply.ResourceApply
	)

	BeforeEach(func() {
		dataset = &marketplacev1alpha1.MarketplaceDataset{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Spec: marketplacev1alpha1.MarketplaceDatasetSpec{
				Bucket: "rhmccp-1234-5678",
			},
		}
		dr = &MarketplaceDatasetReconciler{
			Scheme: scheme.Scheme,
			Log:    logf.Log.WithName("controllers").WithName("MarketplaceDataset"),
			Helper: common.NewControllerHelper(),
		}
		trueVar := true
		falseVar := false
		ra = resourceapply.ResourceApply{
			Context: context.Background(),
			Helper:  dr.Helper,
			Log:     dr.Log,
			Client:  fake.NewFakeClient(),
			Owner: metav1.OwnerReference{
				APIVersion:         "v1alpha1",
				Kind:               "MarketplaceDataset",
				Name:               name,
				Controller:         &trueVar,
				BlockOwnerDeletion: &falseVar,
			},
			Namespace: namespace,
		}
	})

	It("Create dataset", func() {
		dr.Client = fake.NewFakeClient(dataset)

		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			},
		}

		_, err := dr.Reconcile(req)
		Expect(err).NotTo(HaveOccurred(), "Reconcile returned error for dataset create")
		key := client.ObjectKey{
			Name:      name,
			Namespace: namespace,
		}
		reason := getMarketplaceDatasetStatusReason(ctx, key, dr.Client, common.ConditionTypeInstall)
		Expect(reason).To(Equal("InstallComplete"))
	})

	It("Create dataset status error", func() {
		cl := newMarketplaceCSIDriverClient(fake.NewFakeClient(dataset), "", "", "")
		cl.setStatusErrorAction("update")
		dr.Client = cl

		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			},
		}

		_, err := dr.Reconcile(req)
		Expect(err).NotTo(HaveOccurred(), "Reconcile returned error for dataset create")
		key := client.ObjectKey{
			Name:      name,
			Namespace: namespace,
		}
		reason := getMarketplaceDatasetStatusReason(ctx, key, dr.Client, common.ConditionTypeInstall)
		Expect(reason).To(Equal(""))
	})

	It("Delete dataset", func() {
		dr.Client = fake.NewFakeClient()

		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			},
		}

		_, err := dr.Reconcile(req)
		Expect(err).NotTo(HaveOccurred(), "Reconcile returned error for dataset create")
	})

	It("Create dataset with bad podSelector", func() {
		dataset.Spec.PodSelectors = []marketplacev1alpha1.Selector{
			{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "k1",
						Operator: "Bad",
						Values:   []string{"v1", "v2"},
					},
				},
			},
		}
		dr.Client = fake.NewFakeClient(dataset)

		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			},
		}

		_, err := dr.Reconcile(req)
		Expect(err).NotTo(HaveOccurred(), "Reconcile returned error for dataset create")
		key := client.ObjectKey{
			Name:      name,
			Namespace: namespace,
		}
		reason := getMarketplaceDatasetStatusReason(ctx, key, dr.Client, common.ConditionTypeInstall)
		Expect(reason).To(Equal("ValidationFailed"))
	})

	It("Create dataset with podSelector", func() {
		dataset.Spec.PodSelectors = []marketplacev1alpha1.Selector{
			{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "k1",
						Operator: "In",
						Values:   []string{"v1", "v2"},
					},
				},
			},
		}
		dr.Client = fake.NewFakeClient(dataset)

		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			},
		}

		_, err := dr.Reconcile(req)
		Expect(err).NotTo(HaveOccurred(), "Reconcile returned error for dataset create")
		key := client.ObjectKey{
			Name:      name,
			Namespace: namespace,
		}
		reason := getMarketplaceDatasetStatusReason(ctx, key, dr.Client, common.ConditionTypeInstall)
		Expect(reason).To(Equal("InstallComplete"))
	})

	It("Uninstall dataset", func() {
		obj, err := ra.GetResource(common.AssetPathDatasetPVC)
		Expect(err).NotTo(HaveOccurred(), "Test set up failed")
		pvc := obj.(*corev1.PersistentVolumeClaim)
		pvc.Namespace = namespace

		dataset.Spec.PodSelectors = []marketplacev1alpha1.Selector{
			{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "k1",
						Operator: "In",
						Values:   []string{"v1", "v2"},
					},
				},
			},
		}
		dataset.Labels = map[string]string{common.LabelDatasetID: "test", common.LabelSelectorHash: "test", common.LabelUninstall: "true"}
		dataset.Annotations = map[string]string{"csi.marketplace.redhat.com/buckets": "test"}
		dr.Client = fake.NewFakeClient(dataset, pvc)

		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			},
		}

		_, err = dr.Reconcile(req)
		Expect(err).NotTo(HaveOccurred(), "Reconcile returned error for dataset create")
		key := client.ObjectKey{
			Name:      name,
			Namespace: namespace,
		}
		err = dr.Client.Get(ctx, key, dataset)
		Expect(err).To(HaveOccurred(), "Dataset not deleted")
		Expect(errors.IsNotFound(err)).To(BeTrue(), "Invalid error returned after delete")
	})

	It("Uninstall dataset error", func() {
		obj, err := ra.GetResource(common.AssetPathDatasetPVC)
		Expect(err).NotTo(HaveOccurred(), "Test set up failed")
		pvc := obj.(*corev1.PersistentVolumeClaim)
		pvc.Namespace = namespace

		dataset.Spec.PodSelectors = []marketplacev1alpha1.Selector{
			{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "k1",
						Operator: "In",
						Values:   []string{"v1", "v2"},
					},
				},
			},
		}
		dataset.Labels = map[string]string{common.LabelDatasetID: "test", common.LabelSelectorHash: "test", common.LabelUninstall: "true"}
		dataset.Annotations = map[string]string{"csi.marketplace.redhat.com/buckets": "test"}
		dr.Client = newMarketplaceCSIDriverClient(
			fake.NewFakeClient(dataset, pvc),
			"delete",
			"MarketplaceDataset",
			name,
		)

		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			},
		}

		_, err = dr.Reconcile(req)
		Expect(err).To(HaveOccurred(), "Reconcile did not return error as expected")
	})

	It("Update dataset", func() {
		obj, err := ra.GetResource(common.AssetPathDatasetPVC)
		Expect(err).NotTo(HaveOccurred(), "Test set up failed")
		pvc := obj.(*corev1.PersistentVolumeClaim)
		pvc.Namespace = namespace

		dataset.Spec.PodSelectors = []marketplacev1alpha1.Selector{
			{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "k1",
						Operator: "In",
						Values:   []string{"v1", "v2"},
					},
					{
						Key:      "k2",
						Operator: "NotIn",
						Values:   []string{"v1", "v2"},
					},
					{
						Key:      "k3",
						Operator: "Exists",
					},
					{
						Key:      "k4",
						Operator: "DoesNotExist",
					},
				},
			},
		}
		dataset.Labels = map[string]string{common.LabelDatasetID: "test", common.LabelSelectorHash: "test"}
		dataset.Annotations = map[string]string{"csi.marketplace.redhat.com/buckets": "test"}
		dr.Client = fake.NewFakeClient(dataset, pvc)

		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			},
		}

		_, err = dr.Reconcile(req)
		Expect(err).NotTo(HaveOccurred(), "Reconcile returned error for dataset create")
		key := client.ObjectKey{
			Name:      name,
			Namespace: namespace,
		}
		reason := getMarketplaceDatasetStatusReason(ctx, key, dr.Client, common.ConditionTypeInstall)
		Expect(reason).To(Equal("InstallComplete"))
	})

	It("Update dataset selector error", func() {
		obj, err := ra.GetResource(common.AssetPathDatasetPVC)
		Expect(err).NotTo(HaveOccurred(), "Test set up failed")
		pvc := obj.(*corev1.PersistentVolumeClaim)
		pvc.Namespace = namespace

		dataset.Spec.PodSelectors = []marketplacev1alpha1.Selector{
			{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "k1",
						Operator: "In",
						Values:   []string{},
					},
					{
						Key:      "k2",
						Operator: "NotIn",
						Values:   []string{"v1", "v2"},
					},
					{
						Key:      "k3",
						Operator: "Exists",
					},
					{
						Key:      "k4",
						Operator: "DoesNotExist",
					},
				},
			},
		}
		dataset.Labels = map[string]string{common.LabelDatasetID: "test", common.LabelSelectorHash: "test"}
		dataset.Annotations = map[string]string{"csi.marketplace.redhat.com/buckets": "test"}
		dr.Client = fake.NewFakeClient(dataset, pvc)

		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			},
		}

		_, err = dr.Reconcile(req)
		Expect(err).NotTo(HaveOccurred(), "Reconcile returned error for dataset create")
		key := client.ObjectKey{
			Name:      name,
			Namespace: namespace,
		}
		reason := getMarketplaceDatasetStatusReason(ctx, key, dr.Client, common.ConditionTypeInstall)
		Expect(reason).To(Equal("ValidationFailed"))
	})

	It("Update dataset no change", func() {
		obj, err := ra.GetResource(common.AssetPathDatasetPVC)
		Expect(err).NotTo(HaveOccurred(), "Test set up failed")
		pvc := obj.(*corev1.PersistentVolumeClaim)
		pvc.Namespace = namespace
		pvc.OwnerReferences = []metav1.OwnerReference{ra.Owner}

		dataset.Spec.PodSelectors = []marketplacev1alpha1.Selector{
			{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "k1",
						Operator: "In",
						Values:   []string{"v1", "v2"},
					},
				},
			},
		}
		dataset.Labels = map[string]string{common.LabelDatasetID: "test", common.LabelSelectorHash: "test"}
		dataset.Annotations = map[string]string{"csi.marketplace.redhat.com/buckets": "test"}
		dr.Client = fake.NewFakeClient(dataset, pvc)

		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			},
		}

		_, err = dr.Reconcile(req)
		Expect(err).NotTo(HaveOccurred(), "Reconcile returned error for dataset create")
		key := client.ObjectKey{
			Name:      name,
			Namespace: namespace,
		}
		reason := getMarketplaceDatasetStatusReason(ctx, key, dr.Client, common.ConditionTypeInstall)
		Expect(reason).To(Equal("InstallComplete"))
	})

	It("Update dataset variation", func() {
		obj, err := ra.GetResource(common.AssetPathDatasetPVC)
		Expect(err).NotTo(HaveOccurred(), "Test set up failed")
		pvc := obj.(*corev1.PersistentVolumeClaim)
		pvc.Namespace = namespace
		pvc.Annotations = map[string]string{}

		dataset.Spec.PodSelectors = []marketplacev1alpha1.Selector{
			{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "k1",
						Operator: "In",
						Values:   []string{"v1", "v2"},
					},
				},
			},
		}
		dataset.Labels = map[string]string{common.LabelDatasetID: "test", common.LabelSelectorHash: "test"}
		dataset.Annotations = map[string]string{}
		falseVar := false
		pvc.OwnerReferences = []metav1.OwnerReference{
			{
				Kind:               "MarketplaceDataset",
				Name:               name,
				BlockOwnerDeletion: &falseVar,
			},
		}
		dr.Client = fake.NewFakeClient(dataset, pvc)

		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			},
		}

		_, err = dr.Reconcile(req)
		Expect(err).NotTo(HaveOccurred(), "Reconcile returned error for dataset create")
		key := client.ObjectKey{
			Name:      name,
			Namespace: namespace,
		}
		reason := getMarketplaceDatasetStatusReason(ctx, key, dr.Client, common.ConditionTypeInstall)
		Expect(reason).To(Equal("InstallComplete"))
	})

	It("Update dataset error", func() {
		obj, err := ra.GetResource(common.AssetPathDatasetPVC)
		Expect(err).NotTo(HaveOccurred(), "Test set up failed")
		pvc := obj.(*corev1.PersistentVolumeClaim)
		pvc.Namespace = namespace

		dataset.Spec.PodSelectors = []marketplacev1alpha1.Selector{
			{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "k1",
						Operator: "In",
						Values:   []string{"v1", "v2"},
					},
				},
			},
		}
		dataset.Labels = map[string]string{common.LabelDatasetID: "test", common.LabelSelectorHash: "test"}
		dataset.Annotations = map[string]string{"csi.marketplace.redhat.com/buckets": "test"}
		dr.Client = newMarketplaceCSIDriverClient(
			fake.NewFakeClient(dataset, pvc),
			"update",
			"PersistentVolumeClaim",
			"csi-rhm-cos-pvc",
		)

		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			},
		}

		_, err = dr.Reconcile(req)
		Expect(err).To(HaveOccurred(), "Reconcile returned error for dataset create")
		key := client.ObjectKey{
			Name:      name,
			Namespace: namespace,
		}
		reason := getMarketplaceDatasetStatusReason(ctx, key, dr.Client, common.ConditionTypeInstall)
		Expect(reason).To(Equal("InstallFailed"))
	})

	It("Create dataset error", func() {
		dr.Client = newMarketplaceCSIDriverClient(
			fake.NewFakeClient(dataset),
			"create",
			"PersistentVolumeClaim",
			"csi-rhm-cos-pvc",
		)

		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			},
		}

		_, err := dr.Reconcile(req)
		Expect(err).To(HaveOccurred(), "Reconcile returned error for dataset create")
		key := client.ObjectKey{
			Name:      name,
			Namespace: namespace,
		}
		reason := getMarketplaceDatasetStatusReason(ctx, key, dr.Client, common.ConditionTypeInstall)
		Expect(reason).To(Equal("InstallFailed"))
	})

	It("Create dataset error status update", func() {
		cl := newMarketplaceCSIDriverClient(
			fake.NewFakeClient(dataset),
			"create",
			"PersistentVolumeClaim",
			"csi-rhm-cos-pvc",
		)
		cl.setStatusErrorAction("update")
		dr.Client = cl
		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			},
		}

		_, err := dr.Reconcile(req)
		Expect(err).To(HaveOccurred(), "Reconcile returned error for dataset create")
		key := client.ObjectKey{
			Name:      name,
			Namespace: namespace,
		}
		reason := getMarketplaceDatasetStatusReason(ctx, key, dr.Client, common.ConditionTypeInstall)
		Expect(reason).To(Equal(""))
	})

	It("Create dataset error asset read", func() {
		dr.Client = fake.NewFakeClient(dataset)
		dr.Helper = NewFakeControllerHelper(common.AssetPathDatasetPVC, "", "")

		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			},
		}

		_, err := dr.Reconcile(req)
		Expect(err).To(HaveOccurred(), "Reconcile returned error for dataset create")
		key := client.ObjectKey{
			Name:      name,
			Namespace: namespace,
		}
		reason := getMarketplaceDatasetStatusReason(ctx, key, dr.Client, common.ConditionTypeInstall)
		Expect(reason).To(Equal("InstallFailed"))
	})

	It("Create dataset error selector", func() {
		dataset.Spec.PodSelectors = []marketplacev1alpha1.Selector{
			{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "k1",
						Operator: "In",
						Values:   []string{"v1", "v2"},
					},
				},
			},
		}
		dr.Client = fake.NewFakeClient(dataset)
		dr.Helper = NewFakeControllerHelper("", "", "[]v1alpha1.Selector")

		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			},
		}
		_, err := dr.Reconcile(req)
		Expect(err).NotTo(HaveOccurred(), "Reconcile returned error for dataset create")
		key := client.ObjectKey{
			Name:      name,
			Namespace: namespace,
		}
		reason := getMarketplaceDatasetStatusReason(ctx, key, dr.Client, common.ConditionTypeInstall)
		Expect(reason).To(Equal("InstallComplete"))
	})

	It("Update dataset error on pvc get", func() {
		obj, err := ra.GetResource(common.AssetPathDatasetPVC)
		Expect(err).NotTo(HaveOccurred(), "Test set up failed")
		pvc := obj.(*corev1.PersistentVolumeClaim)
		pvc.Namespace = namespace

		dataset.Spec.PodSelectors = []marketplacev1alpha1.Selector{
			{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "k1",
						Operator: "In",
						Values:   []string{"v1", "v2"},
					},
				},
			},
		}
		dataset.Labels = map[string]string{common.LabelDatasetID: "test", common.LabelSelectorHash: "test"}
		dataset.Annotations = map[string]string{"csi.marketplace.redhat.com/buckets": "test"}
		dr.Client = newMarketplaceCSIDriverClient(
			fake.NewFakeClient(dataset, pvc),
			"get",
			"PersistentVolumeClaim",
			"csi-rhm-cos-pvc",
		)

		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			},
		}

		_, err = dr.Reconcile(req)
		Expect(err).To(HaveOccurred(), "Reconcile returned error for dataset create")
		key := client.ObjectKey{
			Name:      name,
			Namespace: namespace,
		}
		reason := getMarketplaceDatasetStatusReason(ctx, key, dr.Client, common.ConditionTypeInstall)
		Expect(reason).To(Equal("InstallFailed"))
	})

	It("Update dataset error on label", func() {
		obj, err := ra.GetResource(common.AssetPathDatasetPVC)
		Expect(err).NotTo(HaveOccurred(), "Test set up failed")
		pvc := obj.(*corev1.PersistentVolumeClaim)
		pvc.Namespace = namespace

		dataset.Spec.PodSelectors = []marketplacev1alpha1.Selector{
			{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "k1",
						Operator: "In",
						Values:   []string{"v1", "v2"},
					},
				},
			},
		}
		dataset.Labels = map[string]string{common.LabelDatasetID: "test", common.LabelSelectorHash: "test"}
		dataset.Annotations = map[string]string{"csi.marketplace.redhat.com/buckets": "test"}
		dr.Client = newMarketplaceCSIDriverClient(
			fake.NewFakeClient(dataset, pvc),
			"update",
			"MarketplaceDataset",
			name,
		)

		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			},
		}

		_, err = dr.Reconcile(req)
		Expect(err).To(HaveOccurred(), "Reconcile returned error for dataset create")
		key := client.ObjectKey{
			Name:      name,
			Namespace: namespace,
		}
		reason := getMarketplaceDatasetStatusReason(ctx, key, dr.Client, common.ConditionTypeInstall)
		Expect(reason).To(Equal("InstallFailed"))
	})

	It("Update dataset error on health check", func() {
		obj, err := ra.GetResource(common.AssetPathDatasetPVC)
		Expect(err).NotTo(HaveOccurred(), "Test set up failed")
		pvc := obj.(*corev1.PersistentVolumeClaim)
		pvc.Namespace = namespace

		dataset.Spec.PodSelectors = []marketplacev1alpha1.Selector{
			{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "k1",
						Operator: "In",
						Values:   []string{"v1", "v2"},
					},
				},
			},
		}
		dataset.Labels = map[string]string{common.LabelDatasetID: "test", common.LabelSelectorHash: "test"}
		dataset.Annotations = map[string]string{"csi.marketplace.redhat.com/buckets": "test"}
		dr.Client = newMarketplaceCSIDriverClient(
			fake.NewFakeClient(dataset, pvc),
			"create",
			"MarketplaceDriverHealthCheck",
			common.NameNodeHealthCheck,
		)

		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			},
		}

		_, err = dr.Reconcile(req)
		Expect(err).NotTo(HaveOccurred(), "Reconcile returned error for dataset create")
		key := client.ObjectKey{
			Name:      name,
			Namespace: namespace,
		}
		reason := getMarketplaceDatasetStatusReason(ctx, key, dr.Client, common.ConditionTypeInstall)
		Expect(reason).To(Equal("InstallComplete"))
	})

	It("Reconcile dataset error", func() {
		dr.Client = newMarketplaceCSIDriverClient(
			fake.NewFakeClient(dataset),
			"get",
			"MarketplaceDataset",
			name,
		)

		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			},
		}

		_, err := dr.Reconcile(req)
		Expect(err).To(HaveOccurred(), "Reconcile returned error for dataset create")
		key := client.ObjectKey{
			Name:      name,
			Namespace: namespace,
		}
		reason := getMarketplaceDatasetStatusReason(ctx, key, dr.Client, common.ConditionTypeInstall)
		Expect(reason).To(Equal("Fake error"))
	})
})

func getMarketplaceDatasetStatusReason(ctx context.Context, key client.ObjectKey, cl client.Client, t status.ConditionType) string {
	obj := &marketplacev1alpha1.MarketplaceDataset{}
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
