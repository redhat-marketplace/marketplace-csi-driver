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
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/redhat-marketplace/marketplace-csi-driver/driver/pkg/common"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	fakedyn "k8s.io/client-go/dynamic/fake"
	fake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/testing"
)

var _ = Describe("CSI Driver Node tests", func() {

	var (
		ctx        context.Context
		count      = 0
		targetPath string
		fHelper    *fakeDriverHelper
		driver     *COSDriver
		npvr       *csi.NodePublishVolumeRequest

		kclient   *fake.Clientset
		dynclient *fakedyn.FakeDynamicClient
	)
	BeforeEach(func() {
		count++
		ctx = context.Background()
		socket := "/tmp/csi-dataset.sock"
		csiEndpoint := "unix://" + socket
		timeSuffix := time.Now().Format("20060102030405PM")
		tempdir := os.TempDir()
		if strings.HasSuffix(tempdir, "/") {
			targetPath = os.TempDir() + "csi-dataset-target-ntest" + timeSuffix + fmt.Sprintf("%d", count)
		} else {
			targetPath = os.TempDir() + "/csi-dataset-target-ntest" + timeSuffix + fmt.Sprintf("%d", count)
		}

		os.MkdirAll(targetPath, 0777)
		driver = &COSDriver{
			name:     "test-dataset-driver",
			nodeName: "test-node",
			version:  GetVersionJSON(),
			endpoint: csiEndpoint,
		}

		npvr = &csi.NodePublishVolumeRequest{
			VolumeId:   "123",
			TargetPath: targetPath,
			VolumeCapability: &csi.VolumeCapability{
				AccessMode: &csi.VolumeCapability_AccessMode{
					Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY,
				},
			},
			VolumeContext: map[string]string{
				"csi.storage.k8s.io/pod.name":      "testpod",
				"csi.storage.k8s.io/pod.namespace": "default",
			},
		}
		kclient = fake.NewSimpleClientset(getTestPodResource(), getTestPullSecret())
		dynclient = getTestFakeDynamicClient(getTestCSIDriverResource(), getTestDatasetResource())
		fHelper = &fakeDriverHelper{
			Clientset: kclient,
			Dynclient: dynclient,
			validator: &fakeValidator{},
		}
		driver.helper = fHelper
	})

	It("Should not mount dataset when podselectors do not match", func() {
		driver.helper = &fakeDriverHelper{
			Clientset: fake.NewSimpleClientset(getTestPodResource(), getTestPullSecret()),
			Dynclient: getTestFakeDynamicClient(getTestCSIDriverResource(), getTestDatasetWithSelectors()),
			validator: &fakeValidator{},
		}
		_, err := driver.NodePublishVolume(ctx, npvr)
		Expect(err).NotTo(HaveOccurred(), "should mount datasets without error")
		dsReason := getTestPodStatus(driver, ctx)
		Expect(dsReason).To(Equal("NoAction"), "expected pod dataset status to be noaction")
		testNodeUnpublish(driver, ctx, targetPath)
	})

	It("Should mount dataset when podselectors match", func() {
		podIn := getTestPodResource()
		podIn.Labels = map[string]string{
			"l1": "v1",
			"l2": "v3",
			"l3": "l10",
			"l4": "test",
		}
		driver.helper = &fakeDriverHelper{
			Clientset: fake.NewSimpleClientset(podIn, getTestPullSecret()),
			Dynclient: getTestFakeDynamicClient(getTestCSIDriverResource(), getTestDatasetWithSelectors()),
			validator: &fakeValidator{},
		}
		_, err := driver.NodePublishVolume(ctx, npvr)
		Expect(err).NotTo(HaveOccurred(), "should mount datasets without error")
		dsReason := getTestPodStatus(driver, ctx)
		Expect(dsReason).To(Equal("Complete"), "expected pod dataset status to be complete")
		testNodeUnpublish(driver, ctx, targetPath)
	})

	It("Should not mount dataset when podselector has errors", func() {
		podIn := getTestPodResource()
		podIn.Labels = map[string]string{
			"l1": "v1",
			"l2": "v3",
			"l3": "l10",
			"l4": "test",
		}
		driver.helper = &fakeDriverHelper{
			Clientset: fake.NewSimpleClientset(podIn, getTestPullSecret()),
			Dynclient: getTestFakeDynamicClient(getTestCSIDriverResource(), getTestDatasetWithSelectorErrors()),
			validator: &fakeValidator{},
		}
		_, err := driver.NodePublishVolume(ctx, npvr)
		Expect(err).NotTo(HaveOccurred(), "should mount datasets without error")
		dsReason := getTestPodStatus(driver, ctx)
		Expect(dsReason).To(Equal("Error"), "expected pod dataset status to be error")
		testNodeUnpublish(driver, ctx, targetPath)
	})

	It("Should not mount dataset when podselector is invalid", func() {
		podIn := getTestPodResource()
		podIn.Labels = map[string]string{
			"l1": "v1",
			"l2": "v3",
			"l3": "l10",
			"l4": "test",
		}
		driver.helper = &fakeDriverHelper{
			Clientset: fake.NewSimpleClientset(podIn, getTestPullSecret()),
			Dynclient: getTestFakeDynamicClient(getTestCSIDriverResource(), getTestDatasetWithBadSelector()),
			validator: &fakeValidator{},
		}
		_, err := driver.NodePublishVolume(ctx, npvr)
		Expect(err).NotTo(HaveOccurred(), "should mount datasets without error")
		dsReason := getTestPodStatus(driver, ctx)
		Expect(dsReason).To(Equal("NoAction"), "expected pod dataset status to be NoAction")
		testNodeUnpublish(driver, ctx, targetPath)
	})

	It("Should mount datasets without selectors", func() {
		_, err := driver.NodePublishVolume(ctx, npvr)
		Expect(err).NotTo(HaveOccurred(), "should mount datasets without error")
		dsReason := getTestPodStatus(driver, ctx)
		Expect(dsReason).To(Equal("Complete"), "expected pod dataset status to be complete")
		testNodeUnpublish(driver, ctx, targetPath)
	})

	It("Should mount datasets using pull secret hmac", func() {
		drv := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "marketplace.redhat.com/v1alpha1",
				"kind":       "MarketplaceCSIDriver",
				"metadata": map[string]interface{}{
					"namespace": common.MarketplaceNamespace,
					"name":      common.ResourceNameS3Driver,
				},
				"spec": map[string]interface{}{
					"endpoint":      "a.b.com",
					"mountRootPath": "/var/redhat-marketplace/datasets",
					"credential": map[string]interface{}{
						"name": "redhat-marketplace-pull-secret",
						"type": "rhm-pull-secret-hmac",
					},
					"mountOptions": []interface{}{"foo", "bar"},
				},
			},
		}
		driver.helper = &fakeDriverHelper{
			Clientset: fake.NewSimpleClientset(getTestPodResource(), getTestPullSecret()),
			Dynclient: getTestFakeDynamicClient(drv, getTestDatasetResource()),
			validator: &fakeValidator{},
		}
		_, err := driver.NodePublishVolume(ctx, npvr)
		Expect(err).NotTo(HaveOccurred(), "should mount datasets without error")
		dsReason := getTestPodStatus(driver, ctx)
		Expect(dsReason).To(Equal("Complete"), "expected pod dataset status to be complete")
		testNodeUnpublish(driver, ctx, targetPath)
	})

	It("Should mount datasets using regular secret hmac", func() {
		drv := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "marketplace.redhat.com/v1alpha1",
				"kind":       "MarketplaceCSIDriver",
				"metadata": map[string]interface{}{
					"namespace": common.MarketplaceNamespace,
					"name":      common.ResourceNameS3Driver,
				},
				"spec": map[string]interface{}{
					"endpoint":      "a.b.com",
					"mountRootPath": "/var/redhat-marketplace/datasets",
					"credential": map[string]interface{}{
						"name": "test",
						"type": "hmac",
					},
					"mountOptions": []interface{}{"foo", "bar"},
				},
			},
		}
		data := make(map[string][]byte)
		data["AccessKeyID"] = []byte("test")
		data["SecretAccessKey"] = []byte("test")
		sec := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: common.MarketplaceNamespace,
				Name:      "test",
			},
			Type: corev1.SecretTypeOpaque,
			Data: data,
		}
		driver.helper = &fakeDriverHelper{
			Clientset: fake.NewSimpleClientset(getTestPodResource(), sec),
			Dynclient: getTestFakeDynamicClient(drv, getTestDatasetResource()),
			validator: &fakeValidator{},
		}
		_, err := driver.NodePublishVolume(ctx, npvr)
		Expect(err).NotTo(HaveOccurred(), "should mount datasets without error")
		dsReason := getTestPodStatus(driver, ctx)
		Expect(dsReason).To(Equal("Complete"), "expected pod dataset status to be complete")
		testNodeUnpublish(driver, ctx, targetPath)
	})

	It("Should handle hmac without access key", func() {
		drv := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "marketplace.redhat.com/v1alpha1",
				"kind":       "MarketplaceCSIDriver",
				"metadata": map[string]interface{}{
					"namespace": common.MarketplaceNamespace,
					"name":      common.ResourceNameS3Driver,
				},
				"spec": map[string]interface{}{
					"endpoint":      "a.b.com",
					"mountRootPath": "/var/redhat-marketplace/datasets",
					"credential": map[string]interface{}{
						"name": "test",
						"type": "hmac",
					},
					"mountOptions": []interface{}{"foo", "bar"},
				},
			},
		}
		data := make(map[string][]byte)
		data["AccessKeyID2"] = []byte("test")
		data["SecretAccessKey"] = []byte("test")
		sec := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: common.MarketplaceNamespace,
				Name:      "test",
			},
			Type: corev1.SecretTypeOpaque,
			Data: data,
		}
		driver.helper = &fakeDriverHelper{
			Clientset: fake.NewSimpleClientset(getTestPodResource(), sec),
			Dynclient: getTestFakeDynamicClient(drv, getTestDatasetResource()),
			validator: &fakeValidator{},
		}
		_, err := driver.NodePublishVolume(ctx, npvr)
		Expect(err).NotTo(HaveOccurred(), "should mount datasets without error")
		dsReason := getTestPodStatus(driver, ctx)
		Expect(dsReason).To(Equal("Error"), "expected pod dataset status to be error")
		testNodeUnpublish(driver, ctx, targetPath)
	})

	It("Should handle hmac without secret key", func() {
		drv := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "marketplace.redhat.com/v1alpha1",
				"kind":       "MarketplaceCSIDriver",
				"metadata": map[string]interface{}{
					"namespace": common.MarketplaceNamespace,
					"name":      common.ResourceNameS3Driver,
				},
				"spec": map[string]interface{}{
					"endpoint":      "a.b.com",
					"mountRootPath": "/var/redhat-marketplace/datasets",
					"credential": map[string]interface{}{
						"name": "test",
						"type": "hmac",
					},
					"mountOptions": []interface{}{"foo", "bar"},
				},
			},
		}
		data := make(map[string][]byte)
		data["AccessKeyID"] = []byte("test")
		data["SecretAccessKey"] = []byte("")
		sec := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: common.MarketplaceNamespace,
				Name:      "test",
			},
			Type: corev1.SecretTypeOpaque,
			Data: data,
		}
		driver.helper = &fakeDriverHelper{
			Clientset: fake.NewSimpleClientset(getTestPodResource(), sec),
			Dynclient: getTestFakeDynamicClient(drv, getTestDatasetResource()),
			validator: &fakeValidator{},
		}
		_, err := driver.NodePublishVolume(ctx, npvr)
		Expect(err).NotTo(HaveOccurred(), "should mount datasets without error")
		dsReason := getTestPodStatus(driver, ctx)
		Expect(dsReason).To(Equal("Error"), "expected pod dataset status to be error")
		testNodeUnpublish(driver, ctx, targetPath)
	})

	It("Should handle mount error", func() {
		fHelper.errorOnAction = "mkdirall"
		_, err := driver.NodePublishVolume(ctx, npvr)
		Expect(err).NotTo(HaveOccurred(), "should mount datasets without error")
		dsReason := getTestPodStatus(driver, ctx)
		Expect(dsReason).To(Equal("Error"), "expected pod dataset status to be Error")
		testNodeUnpublish(driver, ctx, targetPath)
	})

	It("Should handle wait for mount error", func() {
		fHelper.errorOnAction = "waitformount"
		_, err := driver.NodePublishVolume(ctx, npvr)
		Expect(err).NotTo(HaveOccurred(), "should mount datasets without error")
		dsReason := getTestPodStatus(driver, ctx)
		Expect(dsReason).To(Equal("Error"), "expected pod dataset status to be Error")
		testNodeUnpublish(driver, ctx, targetPath)
	})

	It("Should handle missing secret", func() {
		driver.helper = &fakeDriverHelper{
			Clientset: fake.NewSimpleClientset(getTestPodResource()),
			Dynclient: getTestFakeDynamicClient(getTestCSIDriverResource(), getTestDatasetResource()),
			validator: &fakeValidator{},
		}
		_, err := driver.NodePublishVolume(ctx, npvr)
		Expect(err).NotTo(HaveOccurred(), "should mount datasets without error")
		dsReason := getTestPodStatus(driver, ctx)
		Expect(dsReason).To(Equal("Error"), "expected pod dataset status to be Error")
		testNodeUnpublish(driver, ctx, targetPath)
	})

	It("Should handle missing datasets", func() {
		driver.helper = &fakeDriverHelper{
			Clientset: fake.NewSimpleClientset(getTestPodResource(), getTestPullSecret()),
			Dynclient: getTestFakeDynamicClient(getTestCSIDriverResource()),
			validator: &fakeValidator{},
		}
		_, err := driver.NodePublishVolume(ctx, npvr)
		Expect(err).NotTo(HaveOccurred(), "should mount datasets without error")
		dsReason := getTestPodStatus(driver, ctx)
		Expect(dsReason).To(Equal("NoAction"), "expected pod dataset status to be NoAction")
		testNodeUnpublish(driver, ctx, targetPath)
	})

	It("Should handle missing driver config", func() {
		driver.helper = &fakeDriverHelper{
			Clientset: fake.NewSimpleClientset(getTestPodResource(), getTestPullSecret()),
			Dynclient: getTestFakeDynamicClient(getTestDatasetResource()),
			validator: &fakeValidator{},
		}
		_, err := driver.NodePublishVolume(ctx, npvr)
		Expect(err).NotTo(HaveOccurred(), "should mount datasets without error")
		dsReason := getTestPodStatus(driver, ctx)
		Expect(dsReason).To(Equal("Error"), "expected pod dataset status to be Error")
		testNodeUnpublish(driver, ctx, targetPath)
	})

	It("Should handle bad pull without iam", func() {
		data := make(map[string][]byte)
		data["PULL_SECRET"] = []byte("eyJhbGciOiJIUzI1NiJ9.e30.ZRrHA1JJJW8opsbCGfG_HACGpVUMN_a9IV7pAx_Zmeo")
		sec := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: common.MarketplaceNamespace,
				Name:      "redhat-marketplace-pull-secret",
			},
			Type: corev1.SecretTypeOpaque,
			Data: data,
		}
		driver.helper = &fakeDriverHelper{
			Clientset: fake.NewSimpleClientset(getTestPodResource(), sec),
			Dynclient: getTestFakeDynamicClient(getTestCSIDriverResource(), getTestDatasetResource()),
			validator: &fakeValidator{},
		}
		_, err := driver.NodePublishVolume(ctx, npvr)
		Expect(err).NotTo(HaveOccurred(), "should mount datasets without error")
		dsReason := getTestPodStatus(driver, ctx)
		Expect(dsReason).To(Equal("Error"), "expected pod dataset status to be Error")
		testNodeUnpublish(driver, ctx, targetPath)
	})

	It("Should handle bad pull secret hmac without keys", func() {
		data := make(map[string][]byte)
		data["PULL_SECRET"] = []byte("eyJhbGciOiJIUzI1NiJ9.e30.ZRrHA1JJJW8opsbCGfG_HACGpVUMN_a9IV7pAx_Zmeo")
		sec := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: common.MarketplaceNamespace,
				Name:      "redhat-marketplace-pull-secret",
			},
			Type: corev1.SecretTypeOpaque,
			Data: data,
		}
		drv := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "marketplace.redhat.com/v1alpha1",
				"kind":       "MarketplaceCSIDriver",
				"metadata": map[string]interface{}{
					"namespace": common.MarketplaceNamespace,
					"name":      common.ResourceNameS3Driver,
				},
				"spec": map[string]interface{}{
					"endpoint":      "a.b.com",
					"mountRootPath": "/var/redhat-marketplace/datasets",
					"credential": map[string]interface{}{
						"name": "redhat-marketplace-pull-secret",
						"type": "rhm-pull-secret-hmac",
					},
					"mountOptions": []interface{}{"foo", "bar"},
				},
			},
		}
		driver.helper = &fakeDriverHelper{
			Clientset: fake.NewSimpleClientset(getTestPodResource(), sec),
			Dynclient: getTestFakeDynamicClient(drv, getTestDatasetResource()),
			validator: &fakeValidator{},
		}
		_, err := driver.NodePublishVolume(ctx, npvr)
		Expect(err).NotTo(HaveOccurred(), "should mount datasets without error")
		dsReason := getTestPodStatus(driver, ctx)
		Expect(dsReason).To(Equal("Error"), "expected pod dataset status to be Error")
		testNodeUnpublish(driver, ctx, targetPath)
	})

	It("Should handle bad pull secret hmac without secret key", func() {
		data := make(map[string][]byte)
		data["PULL_SECRET"] = []byte("eyJhbGciOiJIUzI1NiJ9.eyJpYW1fYXBpa2V5IjoiaWFta2V5IiwicmhtQWNjb3VudElkIjoiMTIzNDUiLCJlbnRpdGxlbWVudC5zdG9yYWdlLmhtYWMuYWNjZXNzX2tleV9pZCI6ImhtYWNrZXkiLCJpc3MiOiJJQk0gTWFya2V0cGxhY2UiLCJpYXQiOjEyMywianRpIjoiYWJjIn0.n6YmwGPbrufvtSueeEIAstyCHTzsLUvsYbC6BO9MRQE")
		sec := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: common.MarketplaceNamespace,
				Name:      "redhat-marketplace-pull-secret",
			},
			Type: corev1.SecretTypeOpaque,
			Data: data,
		}
		drv := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "marketplace.redhat.com/v1alpha1",
				"kind":       "MarketplaceCSIDriver",
				"metadata": map[string]interface{}{
					"namespace": common.MarketplaceNamespace,
					"name":      common.ResourceNameS3Driver,
				},
				"spec": map[string]interface{}{
					"endpoint":      "a.b.com",
					"mountRootPath": "/var/redhat-marketplace/datasets",
					"credential": map[string]interface{}{
						"name": "redhat-marketplace-pull-secret",
						"type": "rhm-pull-secret-hmac",
					},
					"mountOptions": []interface{}{"foo", "bar"},
				},
			},
		}
		driver.helper = &fakeDriverHelper{
			Clientset: fake.NewSimpleClientset(getTestPodResource(), sec),
			Dynclient: getTestFakeDynamicClient(drv, getTestDatasetResource()),
			validator: &fakeValidator{},
		}
		_, err := driver.NodePublishVolume(ctx, npvr)
		Expect(err).NotTo(HaveOccurred(), "should mount datasets without error")
		dsReason := getTestPodStatus(driver, ctx)
		Expect(dsReason).To(Equal("Error"), "expected pod dataset status to be Error")
		testNodeUnpublish(driver, ctx, targetPath)
	})

	It("Should handle bad pull secret", func() {
		data := make(map[string][]byte)
		data["PULL_SECRET"] = []byte("test.test.test")
		sec := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: common.MarketplaceNamespace,
				Name:      "redhat-marketplace-pull-secret",
			},
			Type: corev1.SecretTypeOpaque,
			Data: data,
		}
		driver.helper = &fakeDriverHelper{
			Clientset: fake.NewSimpleClientset(getTestPodResource(), sec),
			Dynclient: getTestFakeDynamicClient(getTestCSIDriverResource(), getTestDatasetResource()),
			validator: &fakeValidator{},
		}
		_, err := driver.NodePublishVolume(ctx, npvr)
		Expect(err).NotTo(HaveOccurred(), "should mount datasets without error")
		dsReason := getTestPodStatus(driver, ctx)
		Expect(dsReason).To(Equal("Error"), "expected pod dataset status to be Error")
		testNodeUnpublish(driver, ctx, targetPath)
	})

	It("Should handle bad pull secret contd.", func() {
		data := make(map[string][]byte)
		data["PULL_SECRET_TEST"] = []byte("")
		sec := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: common.MarketplaceNamespace,
				Name:      "redhat-marketplace-pull-secret",
			},
			Type: corev1.SecretTypeOpaque,
			Data: data,
		}
		driver.helper = &fakeDriverHelper{
			Clientset: fake.NewSimpleClientset(getTestPodResource(), sec),
			Dynclient: getTestFakeDynamicClient(getTestCSIDriverResource(), getTestDatasetResource()),
			validator: &fakeValidator{},
		}
		_, err := driver.NodePublishVolume(ctx, npvr)
		Expect(err).NotTo(HaveOccurred(), "should mount datasets without error")
		dsReason := getTestPodStatus(driver, ctx)
		Expect(dsReason).To(Equal("Error"), "expected pod dataset status to be Error")
		testNodeUnpublish(driver, ctx, targetPath)
	})

	It("Should handle not mount point", func() {
		driver.helper = &fakeDriverHelper{
			Clientset:     fake.NewSimpleClientset(getTestPodResource(), getTestPullSecret()),
			Dynclient:     getTestFakeDynamicClient(getTestCSIDriverResource(), getTestDatasetResource()),
			errorOnAction: "checkmount",
		}
		_, err := driver.NodePublishVolume(ctx, npvr)
		Expect(err).NotTo(HaveOccurred(), "should handle edge case without error")
	})

	It("Should handle mount point error", func() {
		driver, err := NewCOSDriver(driver.name, driver.nodeName, "v1", driver.endpoint)
		_, err = driver.NodePublishVolume(ctx, npvr)
		Expect(err).NotTo(HaveOccurred(), "should handle edge case without error")
	})

	It("Should handle client set error", func() {
		driver.helper = &fakeDriverHelper{
			errorOnAction: "getclientset",
			Clientset:     fake.NewSimpleClientset(getTestPodResource(), getTestPullSecret()),
			Dynclient:     getTestFakeDynamicClient(getTestCSIDriverResource(), getTestDatasetResource()),
		}
		_, err := driver.NodePublishVolume(ctx, npvr)
		Expect(err).NotTo(HaveOccurred(), "should handle edge case without error")
	})

	It("Should handle get vol context error", func() {
		badnvpr := &csi.NodePublishVolumeRequest{
			VolumeId:   "123",
			TargetPath: targetPath,
			VolumeCapability: &csi.VolumeCapability{
				AccessMode: &csi.VolumeCapability_AccessMode{
					Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY,
				},
			},
			VolumeContext: map[string]string{},
		}

		driver.helper = &fakeDriverHelper{
			Clientset: fake.NewSimpleClientset(getTestPodResource(), getTestPullSecret()),
			Dynclient: getTestFakeDynamicClient(getTestCSIDriverResource(), getTestDatasetResource()),
		}
		_, err := driver.NodePublishVolume(ctx, badnvpr)
		Expect(err).NotTo(HaveOccurred(), "should handle edge case without error")
	})

	It("Should fail for unsupported feature", func() {
		_, err := driver.NodeGetVolumeStats(ctx, &csi.NodeGetVolumeStatsRequest{})
		Expect(err).To(HaveOccurred(), "error expected for unsupported feature")
		Expect(status.Code(err)).To(Equal(codes.Unimplemented), "expected unimplemented response code")

		_, err = driver.NodeExpandVolume(ctx, &csi.NodeExpandVolumeRequest{})
		Expect(err).To(HaveOccurred(), "error expected for unsupported feature")
		Expect(status.Code(err)).To(Equal(codes.Unimplemented), "expected unimplemented response code")
	})

	It("Should handle cleanup error during unmount", func() {
		_, err := driver.NodePublishVolume(ctx, npvr)
		Expect(err).NotTo(HaveOccurred(), "should mount datasets without error")
		dsReason := getTestPodStatus(driver, ctx)
		Expect(dsReason).To(Equal("Complete"), "expected pod dataset status to be complete")

		fHelper.errorOnAction = "cleanmountpoint"
		testNodeUnpublish(driver, ctx, targetPath)
	})

	It("Should handle list dataset error", func() {
		dynclient.PrependReactor("*", "*", func(action testing.Action) (handled bool, ret runtime.Object, err error) {
			if action.GetVerb() == "list" && action.GetResource().Resource == common.ResourceNameDatasets {
				return true, nil, fmt.Errorf("fake error")
			}
			return false, &unstructured.Unstructured{}, nil
		})
		_, err := driver.NodePublishVolume(ctx, npvr)
		Expect(err).NotTo(HaveOccurred(), "should mount datasets without error")
		dsReason := getTestPodStatus(driver, ctx)
		Expect(dsReason).To(Equal("Error"), "expected pod dataset status to be Error")
		testNodeUnpublish(driver, ctx, targetPath)
	})

	It("Should ignore mount options error", func() {
		drvcfg := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "marketplace.redhat.com/v1alpha1",
				"kind":       "MarketplaceCSIDriver",
				"metadata": map[string]interface{}{
					"namespace": common.MarketplaceNamespace,
					"name":      common.ResourceNameS3Driver,
				},
				"spec": map[string]interface{}{
					"endpoint":      "a.b.com",
					"mountRootPath": "/var/redhat-marketplace/datasets",
					"credential": map[string]interface{}{
						"name": "redhat-marketplace-pull-secret",
						"type": "rhm-pull-secret",
					},
					"mountOptions": false,
				},
			},
		}
		dynclient = getTestFakeDynamicClient(drvcfg, getTestDatasetResource())
		driver.helper = &fakeDriverHelper{
			Clientset: kclient,
			Dynclient: dynclient,
			validator: &fakeValidator{},
		}

		_, err := driver.NodePublishVolume(ctx, npvr)
		Expect(err).NotTo(HaveOccurred(), "should mount datasets without error")
		dsReason := getTestPodStatus(driver, ctx)
		Expect(dsReason).To(Equal("Complete"), "expected pod dataset status to be Error")
		testNodeUnpublish(driver, ctx, targetPath)
	})
})

func getTestDatasetWithSelectors() *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "marketplace.redhat.com/v1alpha1",
			"kind":       "MarketplaceDataset",
			"metadata": map[string]interface{}{
				"namespace": "default",
				"name":      "dataset-1",
			},
			"spec": map[string]interface{}{
				"bucket": "rhmccp-123-456",
				"podSelectors": []interface{}{
					map[string]interface{}{
						"matchLabels": map[string]interface{}{
							"l1": "v1",
						},
						"matchExpressions": []interface{}{
							map[string]interface{}{
								"key":      "l2",
								"operator": "In",
								"values":   []interface{}{"v2", "v3"},
							},
							map[string]interface{}{
								"key":      "l3",
								"operator": "NotIn",
								"values":   []interface{}{"v2", "v3"},
							},
							map[string]interface{}{
								"key":      "l4",
								"operator": "Exists",
							},
							map[string]interface{}{
								"key":      "l5",
								"operator": "DoesNotExist",
							},
						},
					},
				},
			},
		},
	}
}

func getTestDatasetWithSelectorErrors() *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "marketplace.redhat.com/v1alpha1",
			"kind":       "MarketplaceDataset",
			"metadata": map[string]interface{}{
				"namespace": "default",
				"name":      "dataset-1",
			},
			"spec": map[string]interface{}{
				"bucket": "rhmccp-123-456",
				"podSelectors": []interface{}{
					map[string]interface{}{
						"matchLabels": map[string]interface{}{
							"P3k9jjApsEe2kUfcM7L6aySPpsMzGQKXuSp7JgtjxFcrZ5sPgGmndRtrDPxprBrg4c": "",
						},
						"matchExpressions": []interface{}{
							map[string]interface{}{
								"key":      "l2",
								"operator": "In",
							},
							map[string]interface{}{
								"key":      "l3",
								"operator": "NotIn",
								"values":   []interface{}{"v2", "v3"},
							},
							map[string]interface{}{
								"key":      "l4",
								"operator": "IsExists",
							},
							map[string]interface{}{
								"key":      "l5",
								"operator": "DoesNotExist",
							},
						},
					},
				},
			},
		},
	}
}

func getTestDatasetWithBadSelector() *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "marketplace.redhat.com/v1alpha1",
			"kind":       "MarketplaceDataset",
			"metadata": map[string]interface{}{
				"namespace": "default",
				"name":      "dataset-1",
			},
			"spec": map[string]interface{}{
				"bucket": "rhmccp-123-456",
				"podSelectors": []interface{}{
					map[string]interface{}{
						"matchExpressions": []interface{}{},
					},
				},
			},
		},
	}
}

func testNodeUnpublish(driver *COSDriver, ctx context.Context, targetPath string) {
	_, err := driver.NodeUnpublishVolume(ctx, &csi.NodeUnpublishVolumeRequest{
		TargetPath: targetPath,
	})
	Expect(err).NotTo(HaveOccurred(), "Expected no errors for node unpublish")
}
