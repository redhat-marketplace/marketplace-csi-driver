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
	"github.com/redhat-marketplace/marketplace-csi-driver/driver/pkg/helper"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	fakedyn "k8s.io/client-go/dynamic/fake"
	fake "k8s.io/client-go/kubernetes/fake"
	fakecorev1 "k8s.io/client-go/kubernetes/typed/core/v1/fake"
	"k8s.io/client-go/testing"
)

var _ = Describe("CSI Driver Reconcile tests", func() {

	var (
		ctx        context.Context
		targetPath string
		driver     *COSDriver
		npvr       *csi.NodePublishVolumeRequest

		kclient       *fake.Clientset
		dynclient     *fakedyn.FakeDynamicClient
		fDriverHelper *fakeDriverHelper
		dsr           schema.GroupVersionResource
		drr           schema.GroupVersionResource
		hcr           schema.GroupVersionResource
		reCtx         *ReconcileContext
		count         = 0
	)

	BeforeEach(func() {
		count++
		ctx = context.Background()
		socket := "/tmp/csi-dataset.sock"
		csiEndpoint := "unix://" + socket

		dsr = schema.GroupVersionResource{Group: common.CustomResourceGroup, Version: common.CustomResourceVersion, Resource: common.ResourceNameDatasets}
		drr = schema.GroupVersionResource{Group: common.CustomResourceGroup, Version: common.CustomResourceVersion, Resource: common.ResourceNameS3Drivers}
		hcr = schema.GroupVersionResource{Group: common.CustomResourceGroup, Version: common.CustomResourceVersion, Resource: common.ResourceNameDriverCheck}
		timeSuffix := time.Now().Format("20060102030405PM")
		tempdir := os.TempDir()
		if strings.HasSuffix(tempdir, "/") {
			targetPath = os.TempDir() + "csi-dataset-target-rctest" + timeSuffix + fmt.Sprintf("%d", count)
		} else {
			targetPath = os.TempDir() + "/csi-dataset-target-rctest" + timeSuffix + fmt.Sprintf("%d", count)
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

		tpod := getTestPodResource()
		tpod.Labels = map[string]string{"t1": "v1"}
		tpod.Annotations = map[string]string{"t1": "v1"}
		kclient = fake.NewSimpleClientset(tpod, getTestPullSecret())
		dynclient = getTestFakeDynamicClient(getTestCSIDriverResource(), getTestDatasetResource())
		fDriverHelper = &fakeDriverHelper{
			Clientset: kclient,
			Dynclient: dynclient,
			validator: &fakeValidator{},
		}
		driver.helper = fDriverHelper
		_, err := driver.NodePublishVolume(ctx, npvr)
		Expect(err).NotTo(HaveOccurred(), "should mount datasets without error")

		reCtx, err = NewReconcileContext(driver)
		Expect(err).NotTo(HaveOccurred(), "initialization failed")
	})

	It("Should repair all datasets in running pod", func() {
		dsu := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "marketplace.redhat.com/v1alpha1",
				"kind":       "MarketplaceDataset",
				"metadata": map[string]interface{}{
					"namespace": "default",
					"name":      "dataset-2",
				},
				"spec": map[string]interface{}{
					"bucket": "rhmccp-123-4567",
				},
			},
		}
		dsu.SetCreationTimestamp(metav1.Now())
		dynclient.Resource(dsr).Namespace("default").Create(ctx, dsu, metav1.CreateOptions{})

		hcu := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "marketplace.redhat.com/v1alpha1",
				"kind":       "MarketplaceDriverHealthCheck",
				"metadata": map[string]interface{}{
					"namespace": "default",
					"name":      "health-check-cr1",
				},
				"spec": map[string]interface{}{
					"affectedNodes": []interface{}{""},
				},
			},
		}

		hcu.SetCreationTimestamp(metav1.Now())
		dynclient.Resource(hcr).Namespace("default").Create(ctx, hcu, metav1.CreateOptions{})
		reCtx.processHealthCheck(hcu)

		cnd := getTestPodCondition(driver, ctx)
		Expect(cnd).NotTo(BeNil(), "expected dataset condition")
		Expect(cnd.Reason).To(Equal("Complete"), "expected pod dataset status to be Complete")
		Expect(cnd.Message).To(Equal("Reconcile mounted 2 datasets"), "condition message incorrect")
	})

	It("Should not repair if datasets are healthy", func() {
		hcu := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "marketplace.redhat.com/v1alpha1",
				"kind":       "MarketplaceDriverHealthCheck",
				"metadata": map[string]interface{}{
					"namespace": "default",
					"name":      "health-check-cr1",
				},
				"spec": map[string]interface{}{
					"affectedNodes": []interface{}{""},
				},
			},
		}

		csvFile := fmt.Sprintf("%s%s%s%s%s", targetPath, string(os.PathSeparator), "dataset-1", string(os.PathSeparator), "test.csv")
		_, err := os.Create(csvFile)
		Expect(err).NotTo(HaveOccurred(), "dataset file setup failed")
		hcu.SetCreationTimestamp(metav1.Now())
		dynclient.Resource(hcr).Namespace("default").Create(ctx, hcu, metav1.CreateOptions{})
		reCtx.processHealthCheck(hcu)

		cnd := getTestPodCondition(driver, ctx)
		Expect(cnd).NotTo(BeNil(), "expected dataset condition")
		Expect(cnd.Reason).To(Equal("Complete"), "expected pod dataset status to be Complete")
		Expect(cnd.Message).To(Equal("Reconcile mounted 0 datasets"), "condition message incorrect")
	})

	It("Should repair specific datasets in running pod", func() {
		dsu := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "marketplace.redhat.com/v1alpha1",
				"kind":       "MarketplaceDataset",
				"metadata": map[string]interface{}{
					"namespace": "default",
					"name":      "dataset-2",
				},
				"spec": map[string]interface{}{
					"bucket": "rhmccp-123-4567",
				},
			},
		}
		dsu.SetCreationTimestamp(metav1.Now())
		dynclient.Resource(dsr).Namespace("default").Create(ctx, dsu, metav1.CreateOptions{})

		hcu := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "marketplace.redhat.com/v1alpha1",
				"kind":       "MarketplaceDriverHealthCheck",
				"metadata": map[string]interface{}{
					"namespace": "default",
					"name":      "health-check-cr1",
				},
				"spec": map[string]interface{}{
					"dataset": "dataset-2",
				},
			},
		}
		hcu.SetCreationTimestamp(metav1.Now())
		dynclient.Resource(hcr).Namespace("default").Create(ctx, hcu, metav1.CreateOptions{})
		reCtx.processHealthCheck(hcu)

		cnd := getTestPodCondition(driver, ctx)
		Expect(cnd).NotTo(BeNil(), "expected dataset condition")
		Expect(cnd.Reason).To(Equal("Complete"), "expected pod dataset status to be Complete")
		Expect(cnd.Message).To(Equal("Reconcile mounted 1 datasets"), "condition message incorrect")
	})

	It("Should not act on old health check requests", func() {
		hcu := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "marketplace.redhat.com/v1alpha1",
				"kind":       "MarketplaceDriverHealthCheck",
				"metadata": map[string]interface{}{
					"namespace": "default",
					"name":      "health-check-cr1",
				},
				"spec": map[string]interface{}{
					"affectedNodes": []interface{}{""},
				},
			},
		}
		hcu.SetCreationTimestamp(metav1.Time{Time: time.Now().Add(time.Hour * -1)})
		dynclient.Resource(hcr).Namespace("default").Create(ctx, hcu, metav1.CreateOptions{})
		reCtx.processHealthCheck(hcu)

		cnd := getTestPodCondition(driver, ctx)
		Expect(cnd).NotTo(BeNil(), "expected dataset condition")
		Expect(cnd.Reason).To(Equal("Complete"), "expected pod dataset status to be Complete")
		Expect(cnd.Message).To(Equal("Mounted 1 datasets"), "condition message incorrect")
	})

	It("Should not act on health check requests for different node", func() {
		hcu := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "marketplace.redhat.com/v1alpha1",
				"kind":       "MarketplaceDriverHealthCheck",
				"metadata": map[string]interface{}{
					"namespace": "default",
					"name":      "health-check-cr1",
				},
				"spec": map[string]interface{}{
					"affectedNodes": []interface{}{"another-node"},
				},
			},
		}
		hcu.SetCreationTimestamp(metav1.Now())
		dynclient.Resource(hcr).Namespace("default").Create(ctx, hcu, metav1.CreateOptions{})
		reCtx.processHealthCheck(hcu)

		cnd := getTestPodCondition(driver, ctx)
		Expect(cnd).NotTo(BeNil(), "expected dataset condition")
		Expect(cnd.Reason).To(Equal("Complete"), "expected pod dataset status to be Complete")
		Expect(cnd.Message).To(Equal("Mounted 1 datasets"), "condition message incorrect")
	})

	It("Should handle missing dataset on reconcile", func() {
		hcu := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "marketplace.redhat.com/v1alpha1",
				"kind":       "MarketplaceDriverHealthCheck",
				"metadata": map[string]interface{}{
					"namespace": "default",
					"name":      "health-check-cr1",
				},
				"spec": map[string]interface{}{
					"dataset": "dataset-2",
				},
			},
		}
		hcu.SetCreationTimestamp(metav1.Now())
		dynclient.Resource(hcr).Namespace("default").Create(ctx, hcu, metav1.CreateOptions{})
		reCtx.processHealthCheck(hcu)

		cnd := getTestPodCondition(driver, ctx)
		Expect(cnd).NotTo(BeNil(), "expected dataset condition")
		Expect(cnd.Reason).To(Equal("Complete"), "expected pod dataset status to be Complete")
		Expect(cnd.Message).To(Equal("Mounted 1 datasets"), "condition message incorrect")
	})

	It("Should handle no datasets on reconcile", func() {

		dynclient.Resource(dsr).Namespace("default").Delete(ctx, "dataset-1", metav1.DeleteOptions{})

		hcu := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "marketplace.redhat.com/v1alpha1",
				"kind":       "MarketplaceDriverHealthCheck",
				"metadata": map[string]interface{}{
					"namespace": "default",
					"name":      "health-check-cr1",
				},
				"spec": map[string]interface{}{
					"affectedNodes": []interface{}{""},
				},
			},
		}
		hcu.SetCreationTimestamp(metav1.Now())
		dynclient.Resource(hcr).Namespace("default").Create(ctx, hcu, metav1.CreateOptions{})
		reCtx.processHealthCheck(hcu)

		cnd := getTestPodCondition(driver, ctx)
		Expect(cnd).NotTo(BeNil(), "expected dataset condition")
		Expect(cnd.Reason).To(Equal("Complete"), "expected pod dataset status to be Complete")
		Expect(cnd.Message).To(Equal("Mounted 1 datasets"), "condition message incorrect")
	})

	It("Should handle missing driver config on reconcile", func() {
		err := dynclient.Resource(drr).Namespace(common.MarketplaceNamespace).Delete(ctx, common.ResourceNameS3Driver, metav1.DeleteOptions{})
		Expect(err).NotTo(HaveOccurred(), "test set up error")
		hcu := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "marketplace.redhat.com/v1alpha1",
				"kind":       "MarketplaceDriverHealthCheck",
				"metadata": map[string]interface{}{
					"namespace": "default",
					"name":      "health-check-cr1",
				},
				"spec": map[string]interface{}{
					"dataset": "dataset-1",
				},
			},
		}
		hcu.SetCreationTimestamp(metav1.Now())
		dynclient.Resource(hcr).Namespace("default").Create(ctx, hcu, metav1.CreateOptions{})
		reCtx.processHealthCheck(hcu)

		hcu = &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "marketplace.redhat.com/v1alpha1",
				"kind":       "MarketplaceDriverHealthCheck",
				"metadata": map[string]interface{}{
					"namespace": "default",
					"name":      "health-check-cr2",
				},
				"spec": map[string]interface{}{
					"affectedNodes": []interface{}{""},
				},
			},
		}
		hcu.SetCreationTimestamp(metav1.Now())
		dynclient.Resource(hcr).Namespace("default").Create(ctx, hcu, metav1.CreateOptions{})
		reCtx.processHealthCheck(hcu)

		cnd := getTestPodCondition(driver, ctx)
		Expect(cnd).NotTo(BeNil(), "expected dataset condition")
		Expect(cnd.Reason).To(Equal("Complete"), "expected pod dataset status to be Complete")
		Expect(cnd.Message).To(Equal("Mounted 1 datasets"), "condition message incorrect")
	})

	It("Should handle no pods to fix on reconcile", func() {
		p, err := kclient.CoreV1().Pods("default").Get(ctx, "testpod", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred(), "test set up error")
		p.Labels = map[string]string{"test": "value"}
		p, err = kclient.CoreV1().Pods("default").Update(ctx, p, metav1.UpdateOptions{})
		Expect(err).NotTo(HaveOccurred(), "test set up error 2 ")
		hcu := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "marketplace.redhat.com/v1alpha1",
				"kind":       "MarketplaceDriverHealthCheck",
				"metadata": map[string]interface{}{
					"namespace": "default",
					"name":      "health-check-cr1",
				},
				"spec": map[string]interface{}{
					"dataset": "dataset-1",
				},
			},
		}
		hcu.SetCreationTimestamp(metav1.Now())
		dynclient.Resource(hcr).Namespace("default").Create(ctx, hcu, metav1.CreateOptions{})
		reCtx.processHealthCheck(hcu)

		hcu = &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "marketplace.redhat.com/v1alpha1",
				"kind":       "MarketplaceDriverHealthCheck",
				"metadata": map[string]interface{}{
					"namespace": "default",
					"name":      "health-check-cr2",
				},
				"spec": map[string]interface{}{
					"affectedNodes": []interface{}{""},
				},
			},
		}
		hcu.SetCreationTimestamp(metav1.Now())
		dynclient.Resource(hcr).Namespace("default").Create(ctx, hcu, metav1.CreateOptions{})
		reCtx.processHealthCheck(hcu)

		cnd := getTestPodCondition(driver, ctx)
		Expect(cnd).NotTo(BeNil(), "expected dataset condition")
		Expect(cnd.Reason).To(Equal("Complete"), "expected pod dataset status to be Complete")
		Expect(cnd.Message).To(Equal("Mounted 1 datasets"), "condition message incorrect")
	})

	It("Should handle pods missing dspath to fix on reconcile", func() {
		p, err := kclient.CoreV1().Pods("default").Get(ctx, "testpod", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred(), "test set up error")
		p.Annotations = map[string]string{"test": "value"}
		p, err = kclient.CoreV1().Pods("default").Update(ctx, p, metav1.UpdateOptions{})
		Expect(err).NotTo(HaveOccurred(), "test set up error 2 ")
		hcu := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "marketplace.redhat.com/v1alpha1",
				"kind":       "MarketplaceDriverHealthCheck",
				"metadata": map[string]interface{}{
					"namespace": "default",
					"name":      "health-check-cr1",
				},
				"spec": map[string]interface{}{
					"dataset": "dataset-1",
				},
			},
		}
		hcu.SetCreationTimestamp(metav1.Now())
		dynclient.Resource(hcr).Namespace("default").Create(ctx, hcu, metav1.CreateOptions{})
		reCtx.processHealthCheck(hcu)

		hcu = &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "marketplace.redhat.com/v1alpha1",
				"kind":       "MarketplaceDriverHealthCheck",
				"metadata": map[string]interface{}{
					"namespace": "default",
					"name":      "health-check-cr2",
				},
				"spec": map[string]interface{}{
					"affectedNodes": []interface{}{""},
				},
			},
		}
		hcu.SetCreationTimestamp(metav1.Now())
		dynclient.Resource(hcr).Namespace("default").Create(ctx, hcu, metav1.CreateOptions{})
		reCtx.processHealthCheck(hcu)

		cnd := getTestPodCondition(driver, ctx)
		Expect(cnd).NotTo(BeNil(), "expected dataset condition")
		Expect(cnd.Reason).To(Equal("Complete"), "expected pod dataset status to be Complete")
		Expect(cnd.Message).To(Equal("Mounted 1 datasets"), "condition message incorrect")
	})

	It("Should handle terminating pods on reconcile", func() {
		p, err := kclient.CoreV1().Pods("default").Get(ctx, "testpod", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred(), "test set up error")
		p.Status.Phase = "Terminating"
		p, err = kclient.CoreV1().Pods("default").UpdateStatus(ctx, p, metav1.UpdateOptions{})
		Expect(err).NotTo(HaveOccurred(), "test set up error 2 ")
		hcu := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "marketplace.redhat.com/v1alpha1",
				"kind":       "MarketplaceDriverHealthCheck",
				"metadata": map[string]interface{}{
					"namespace": "default",
					"name":      "health-check-cr1",
				},
				"spec": map[string]interface{}{
					"dataset": "dataset-1",
				},
			},
		}
		hcu.SetCreationTimestamp(metav1.Now())
		dynclient.Resource(hcr).Namespace("default").Create(ctx, hcu, metav1.CreateOptions{})
		reCtx.processHealthCheck(hcu)

		hcu = &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "marketplace.redhat.com/v1alpha1",
				"kind":       "MarketplaceDriverHealthCheck",
				"metadata": map[string]interface{}{
					"namespace": "default",
					"name":      "health-check-cr2",
				},
				"spec": map[string]interface{}{
					"affectedNodes": []interface{}{""},
				},
			},
		}
		hcu.SetCreationTimestamp(metav1.Now())
		dynclient.Resource(hcr).Namespace("default").Create(ctx, hcu, metav1.CreateOptions{})
		reCtx.processHealthCheck(hcu)

		cnd := getTestPodCondition(driver, ctx)
		Expect(cnd).NotTo(BeNil(), "expected dataset condition")
		Expect(cnd.Reason).To(Equal("Complete"), "expected pod dataset status to be Complete")
		Expect(cnd.Message).To(Equal("Mounted 1 datasets"), "condition message incorrect")
	})

	It("Should handle pod list error on reconcile", func() {
		kclient.CoreV1().(*fakecorev1.FakeCoreV1).PrependReactor("list", "pods", func(action testing.Action) (handled bool, ret runtime.Object, err error) {
			return true, &corev1.PodList{}, fmt.Errorf("fake error")
		})

		hcu := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "marketplace.redhat.com/v1alpha1",
				"kind":       "MarketplaceDriverHealthCheck",
				"metadata": map[string]interface{}{
					"namespace": "default",
					"name":      "health-check-cr1",
				},
				"spec": map[string]interface{}{
					"dataset": "dataset-1",
				},
			},
		}
		hcu.SetCreationTimestamp(metav1.Now())
		dynclient.Resource(hcr).Namespace("default").Create(ctx, hcu, metav1.CreateOptions{})
		reCtx.processHealthCheck(hcu)

		hcu = &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "marketplace.redhat.com/v1alpha1",
				"kind":       "MarketplaceDriverHealthCheck",
				"metadata": map[string]interface{}{
					"namespace": "default",
					"name":      "health-check-cr2",
				},
				"spec": map[string]interface{}{
					"affectedNodes": []interface{}{""},
				},
			},
		}
		hcu.SetCreationTimestamp(metav1.Now())
		dynclient.Resource(hcr).Namespace("default").Create(ctx, hcu, metav1.CreateOptions{})
		reCtx.processHealthCheck(hcu)

		cnd := getTestPodCondition(driver, ctx)
		Expect(cnd).NotTo(BeNil(), "expected dataset condition")
		Expect(cnd.Reason).To(Equal("Complete"), "expected pod dataset status to be Complete")
		Expect(cnd.Message).To(Equal("Mounted 1 datasets"), "condition message incorrect")
	})

	It("Should handle dataset list error on reconcile", func() {
		dynclient.PrependReactor("*", "*", func(action testing.Action) (handled bool, ret runtime.Object, err error) {
			if action.GetVerb() == "list" && action.GetResource().Resource == common.ResourceNameDatasets {
				return true, nil, fmt.Errorf("fake error")
			}
			if action.GetVerb() == "get" && action.GetResource().Resource == common.ResourceNameDatasets {
				return true, &unstructured.Unstructured{}, fmt.Errorf("fake error")
			}
			return false, &unstructured.Unstructured{}, nil
		})

		hcu := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "marketplace.redhat.com/v1alpha1",
				"kind":       "MarketplaceDriverHealthCheck",
				"metadata": map[string]interface{}{
					"namespace": "default",
					"name":      "health-check-cr2",
				},
				"spec": map[string]interface{}{
					"affectedNodes": []interface{}{""},
				},
			},
		}
		hcu.SetCreationTimestamp(metav1.Now())
		dynclient.Resource(hcr).Namespace("default").Create(ctx, hcu, metav1.CreateOptions{})
		reCtx.processHealthCheck(hcu)

		hcu = &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "marketplace.redhat.com/v1alpha1",
				"kind":       "MarketplaceDriverHealthCheck",
				"metadata": map[string]interface{}{
					"namespace": "default",
					"name":      "health-check-cr1",
				},
				"spec": map[string]interface{}{
					"dataset": "dataset-1",
				},
			},
		}
		hcu.SetCreationTimestamp(metav1.Now())
		_, err := dynclient.Resource(hcr).Namespace("default").Create(ctx, hcu, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred(), "unexpected error")
		reCtx.processHealthCheck(hcu)

		cnd := getTestPodCondition(driver, ctx)
		Expect(cnd).NotTo(BeNil(), "expected dataset condition")
		Expect(cnd.Reason).To(Equal("Complete"), "expected pod dataset status to be Complete")
		Expect(cnd.Message).To(Equal("Mounted 1 datasets"), "condition message incorrect")
	})

	It("Should handle error on reconcile", func() {
		driver.helper = &fakeDriverHelper{
			Clientset: kclient,
			Dynclient: dynclient,
			validator: &fakeValidator{},
			command:   "fakecmdzzz",
		}
		hcu := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "marketplace.redhat.com/v1alpha1",
				"kind":       "MarketplaceDriverHealthCheck",
				"metadata": map[string]interface{}{
					"namespace": "default",
					"name":      "health-check-cr1",
				},
				"spec": map[string]interface{}{
					"dataset": "dataset-1",
				},
			},
		}
		hcu.SetCreationTimestamp(metav1.Now())
		dynclient.Resource(hcr).Namespace("default").Create(ctx, hcu, metav1.CreateOptions{})
		reCtx.processHealthCheck(hcu)

		cnd := getTestPodCondition(driver, ctx)
		Expect(cnd).NotTo(BeNil(), "expected dataset condition")
		Expect(cnd.Reason).To(Equal("Error"), "expected pod dataset status to be error")

		hcu = &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "marketplace.redhat.com/v1alpha1",
				"kind":       "MarketplaceDriverHealthCheck",
				"metadata": map[string]interface{}{
					"namespace": "default",
					"name":      "health-check-cr2",
				},
				"spec": map[string]interface{}{
					"affectedNodes": []interface{}{""},
				},
			},
		}
		hcu.SetCreationTimestamp(metav1.Now())
		dynclient.Resource(hcr).Namespace("default").Create(ctx, hcu, metav1.CreateOptions{})
		reCtx.processHealthCheck(hcu)

		cnd = getTestPodCondition(driver, ctx)
		Expect(cnd).NotTo(BeNil(), "expected dataset condition")
		Expect(cnd.Reason).To(Equal("Error"), "expected pod dataset status to be error")

		cnd = getTestPodCondition(driver, ctx)
		Expect(cnd).NotTo(BeNil(), "expected dataset condition")
		Expect(cnd.Reason).To(Equal("Error"), "expected pod dataset status to be error")
	})

	It("Should handle cleanup error", func() {
		hcu := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "marketplace.redhat.com/v1alpha1",
				"kind":       "MarketplaceDriverHealthCheck",
				"metadata": map[string]interface{}{
					"namespace": "default",
					"name":      "health-check-cr1",
				},
				"spec": map[string]interface{}{
					"affectedNodes": []interface{}{""},
				},
			},
		}

		hcu.SetCreationTimestamp(metav1.Now())
		dynclient.Resource(hcr).Namespace("default").Create(ctx, hcu, metav1.CreateOptions{})
		fDriverHelper.errorOnAction = "cleanmountpoint"
		reCtx.processHealthCheck(hcu)

		cnd := getTestPodCondition(driver, ctx)
		Expect(cnd).NotTo(BeNil(), "expected dataset condition")
		Expect(cnd.Reason).To(Equal("Complete"), "expected pod dataset status to be Complete")
		Expect(cnd.Message).To(Equal("Reconcile mounted 0 datasets"), "condition message incorrect")
	})

	It("Should handle bad dataset folder", func() {
		hcu := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "marketplace.redhat.com/v1alpha1",
				"kind":       "MarketplaceDriverHealthCheck",
				"metadata": map[string]interface{}{
					"namespace": "default",
					"name":      "health-check-cr1",
				},
				"spec": map[string]interface{}{
					"affectedNodes": []interface{}{""},
				},
			},
		}

		fDriverHelper.errorOnAction = "filestat"
		hcu.SetCreationTimestamp(metav1.Now())
		dynclient.Resource(hcr).Namespace("default").Create(ctx, hcu, metav1.CreateOptions{})
		reCtx.processHealthCheck(hcu)

		cnd := getTestPodCondition(driver, ctx)
		Expect(cnd).NotTo(BeNil(), "expected dataset condition")
		Expect(cnd.Reason).To(Equal("Complete"), "expected pod dataset status to be Complete")
		Expect(cnd.Message).To(Equal("Reconcile mounted 1 datasets"), "condition message incorrect")
	})

	It("Should handle readdir error", func() {
		hcu := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "marketplace.redhat.com/v1alpha1",
				"kind":       "MarketplaceDriverHealthCheck",
				"metadata": map[string]interface{}{
					"namespace": "default",
					"name":      "health-check-cr1",
				},
				"spec": map[string]interface{}{
					"affectedNodes": []interface{}{""},
				},
			},
		}

		fDriverHelper.errorOnAction = "readdir"
		hcu.SetCreationTimestamp(metav1.Now())
		dynclient.Resource(hcr).Namespace("default").Create(ctx, hcu, metav1.CreateOptions{})
		reCtx.processHealthCheck(hcu)

		cnd := getTestPodCondition(driver, ctx)
		Expect(cnd).NotTo(BeNil(), "expected dataset condition")
		Expect(cnd.Reason).To(Equal("Complete"), "expected pod dataset status to be Complete")
		Expect(cnd.Message).To(Equal("Reconcile mounted 1 datasets"), "condition message incorrect")
	})

	It("Should handle missing dataset folder", func() {
		hcu := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "marketplace.redhat.com/v1alpha1",
				"kind":       "MarketplaceDriverHealthCheck",
				"metadata": map[string]interface{}{
					"namespace": "default",
					"name":      "health-check-cr1",
				},
				"spec": map[string]interface{}{
					"affectedNodes": []interface{}{""},
				},
			},
		}

		dspath := targetPath + string(os.PathSeparator) + "dataset-1"
		os.Remove(dspath)

		hcu.SetCreationTimestamp(metav1.Now())
		dynclient.Resource(hcr).Namespace("default").Create(ctx, hcu, metav1.CreateOptions{})
		reCtx.processHealthCheck(hcu)

		cnd := getTestPodCondition(driver, ctx)
		Expect(cnd).NotTo(BeNil(), "expected dataset condition")
		Expect(cnd.Reason).To(Equal("Complete"), "expected pod dataset status to be Complete")
		Expect(cnd.Message).To(Equal("Reconcile mounted 1 datasets"), "condition message incorrect")
	})

	It("Should skip datasets with bad spec", func() {
		dsu := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "marketplace.redhat.com/v1alpha1",
				"kind":       "MarketplaceDataset",
				"metadata": map[string]interface{}{
					"namespace": "default",
					"name":      "dataset-2",
				},
				"spec": map[string]interface{}{
					"bucket": false,
				},
			},
		}
		dsu.SetCreationTimestamp(metav1.Now())
		dynclient.Resource(dsr).Namespace("default").Create(ctx, dsu, metav1.CreateOptions{})

		hcu := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "marketplace.redhat.com/v1alpha1",
				"kind":       "MarketplaceDriverHealthCheck",
				"metadata": map[string]interface{}{
					"namespace": "default",
					"name":      "health-check-cr1",
				},
				"spec": map[string]interface{}{
					"dataset": "dataset-2",
				},
			},
		}

		hcu.SetCreationTimestamp(metav1.Now())
		dynclient.Resource(hcr).Namespace("default").Create(ctx, hcu, metav1.CreateOptions{})
		reCtx.processHealthCheck(hcu)

		hcu = &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "marketplace.redhat.com/v1alpha1",
				"kind":       "MarketplaceDriverHealthCheck",
				"metadata": map[string]interface{}{
					"namespace": "default",
					"name":      "health-check-cr1",
				},
				"spec": map[string]interface{}{
					"affectedNodes": []interface{}{""},
				},
			},
		}

		hcu.SetCreationTimestamp(metav1.Now())
		dynclient.Resource(hcr).Namespace("default").Create(ctx, hcu, metav1.CreateOptions{})
		reCtx.processHealthCheck(hcu)

		cnd := getTestPodCondition(driver, ctx)
		Expect(cnd).NotTo(BeNil(), "expected dataset condition")
		Expect(cnd.Reason).To(Equal("Complete"), "expected pod dataset status to be Complete")
		Expect(cnd.Message).To(Equal("Reconcile mounted 1 datasets"), "condition message incorrect")
	})

	It("Should handle dataset with no matching selectors", func() {
		dsu := &unstructured.Unstructured{
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
							"matchExpressions": []interface{}{
								map[string]interface{}{
									"key":      "lll2",
									"operator": "Exists",
								},
							},
						},
					},
				},
			},
		}
		dynclient.Resource(dsr).Namespace("default").Update(ctx, dsu, metav1.UpdateOptions{})

		dspath := targetPath + string(os.PathSeparator) + "dataset-1"
		os.Remove(dspath)

		hcu := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "marketplace.redhat.com/v1alpha1",
				"kind":       "MarketplaceDriverHealthCheck",
				"metadata": map[string]interface{}{
					"namespace": "default",
					"name":      "health-check-cr1",
				},
				"spec": map[string]interface{}{
					"affectedNodes": []interface{}{""},
				},
			},
		}

		hcu.SetCreationTimestamp(metav1.Now())
		dynclient.Resource(hcr).Namespace("default").Create(ctx, hcu, metav1.CreateOptions{})
		reCtx.processHealthCheck(hcu)

		cnd := getTestPodCondition(driver, ctx)
		Expect(cnd).NotTo(BeNil(), "expected dataset condition")
		Expect(cnd.Reason).To(Equal("Complete"), "expected pod dataset status to be complete")
		Expect(cnd.Message).To(Equal("Reconcile mounted 0 datasets"), "condition message incorrect")
	})

	It("Should handle dataset with bad selectors", func() {
		dsu := &unstructured.Unstructured{
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
							"matchExpressions": []interface{}{
								map[string]interface{}{
									"key":      "lll2",
									"operator": "Dummy",
								},
							},
						},
					},
				},
			},
		}
		dynclient.Resource(dsr).Namespace("default").Update(ctx, dsu, metav1.UpdateOptions{})

		dspath := targetPath + string(os.PathSeparator) + "dataset-1"
		os.Remove(dspath)

		hcu := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "marketplace.redhat.com/v1alpha1",
				"kind":       "MarketplaceDriverHealthCheck",
				"metadata": map[string]interface{}{
					"namespace": "default",
					"name":      "health-check-cr1",
				},
				"spec": map[string]interface{}{
					"affectedNodes": []interface{}{""},
				},
			},
		}

		hcu.SetCreationTimestamp(metav1.Now())
		dynclient.Resource(hcr).Namespace("default").Create(ctx, hcu, metav1.CreateOptions{})
		reCtx.processHealthCheck(hcu)

		cnd := getTestPodCondition(driver, ctx)
		Expect(cnd).NotTo(BeNil(), "expected dataset condition")
		Expect(cnd.Reason).To(Equal("Error"), "expected pod dataset status to be complete")
	})

	It("Should show correct message when all datasets have bad spec ", func() {
		dsu := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "marketplace.redhat.com/v1alpha1",
				"kind":       "MarketplaceDataset",
				"metadata": map[string]interface{}{
					"namespace": "default",
					"name":      "dataset-1",
				},
				"spec": map[string]interface{}{
					"bucket": false,
				},
			},
		}
		dynclient = getTestFakeDynamicClient(getTestCSIDriverResource(), dsu)
		driver = &COSDriver{
			name:     "test-dataset-driver",
			nodeName: "test-node",
			version:  GetVersionJSON(),
			endpoint: "test",
			helper: &fakeDriverHelper{
				Clientset: kclient,
				Dynclient: dynclient,
				validator: &fakeValidator{},
			},
		}
		_, err := driver.NodePublishVolume(ctx, npvr)
		Expect(err).NotTo(HaveOccurred(), "initialization failed")

		reCtx, err = NewReconcileContext(driver)
		Expect(err).NotTo(HaveOccurred(), "initialization failed")

		hcu := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "marketplace.redhat.com/v1alpha1",
				"kind":       "MarketplaceDriverHealthCheck",
				"metadata": map[string]interface{}{
					"namespace": "default",
					"name":      "health-check-cr1",
				},
				"spec": map[string]interface{}{
					"affectedNodes": []interface{}{""},
				},
			},
		}

		hcu.SetCreationTimestamp(metav1.Now())
		dynclient.Resource(hcr).Namespace("default").Create(ctx, hcu, metav1.CreateOptions{})
		reCtx.processHealthCheck(hcu)
		cnd := getTestPodCondition(driver, ctx)
		Expect(cnd).NotTo(BeNil(), "expected dataset condition")
		Expect(cnd.Reason).To(Equal("Error"), "expected pod dataset status to be Error")
	})

	It("Should handle client set init error", func() {
		driver.helper = helper.NewDriverHelper()
		_, err := NewReconcileContext(driver)
		Expect(err).To(HaveOccurred(), "expected clientset err")
	})

	It("Should handle bad target path", func() {
		c, err := ReconcileDatasets(driver.helper, common.VolumeMountDetails{}, targetPath+"/badfolder")
		Expect(err).NotTo(HaveOccurred(), "Expected no error for bad path")
		Expect(c).To(Equal(0), "Expected no datasets mounted")
	})

	It("Should handle no datasets", func() {
		c, err := ReconcileDatasets(driver.helper, common.VolumeMountDetails{Datasets: []common.Dataset{}}, targetPath)
		Expect(err).NotTo(HaveOccurred(), "Expected no error ")
		Expect(c).To(Equal(0), "Expected no datasets mounted")
	})

	It("Should handle pwd file create error", func() {
		err := os.Chmod(targetPath, 0200)
		Expect(err).NotTo(HaveOccurred(), "error")
		ds := common.Dataset{
			Bucket: "test",
			Folder: "test",
		}
		c, err := ReconcileDatasets(driver.helper, common.VolumeMountDetails{Datasets: []common.Dataset{ds}}, targetPath)
		Expect(err).To(HaveOccurred(), "Expected error for bad path")
		Expect(c).To(Equal(0), "Expected no datasets mounted")
	})

	It("Should handle pwd file create error", func() {
		//err := os.Chmod(targetPath, 0200)
		//Expect(err).NotTo(HaveOccurred(), "error")
		ds := common.Dataset{
			Bucket: "test",
			Folder: "test",
		}
		c, err := ReconcileDatasets(driver.helper, common.VolumeMountDetails{Datasets: []common.Dataset{ds}}, targetPath)
		Expect(err).NotTo(HaveOccurred(), "Expected no error")
		Expect(c).To(Equal(1), "Expected 1 dataset mounted")
	})

	It("Should start informers and handle events", func() {
		stopCh := make(chan struct{})
		defer close(stopCh)
		go reCtx.StartInformers(stopCh)

		dsu := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "marketplace.redhat.com/v1alpha1",
				"kind":       "MarketplaceDataset",
				"metadata": map[string]interface{}{
					"namespace": "default",
					"name":      "dataset-2",
				},
				"spec": map[string]interface{}{
					"bucket": "rhmccp-123-4567",
				},
			},
		}
		dsu.SetCreationTimestamp(metav1.Now())
		dynclient.Resource(dsr).Namespace("default").Create(ctx, dsu, metav1.CreateOptions{})

		hcu := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "marketplace.redhat.com/v1alpha1",
				"kind":       "MarketplaceDriverHealthCheck",
				"metadata": map[string]interface{}{
					"namespace": "default",
					"name":      "health-check-cr1",
				},
				"spec": map[string]interface{}{
					"affectedNodes": []interface{}{""},
				},
			},
		}
		hcu.SetCreationTimestamp(metav1.Now())
		dynclient.Resource(hcr).Namespace("default").Create(ctx, hcu, metav1.CreateOptions{})

		reCtx.processHealthCheck(hcu)

		cnd := getTestPodCondition(driver, ctx)
		Expect(cnd).NotTo(BeNil(), "expected dataset condition")
		Expect(cnd.Reason).To(Equal("Complete"), "expected pod dataset status to be Complete")
		Expect(cnd.Message).To(Equal("Reconcile mounted 2 datasets"), "condition message incorrect")
	})

	It("Should start informers", func() {
		stopCh := make(chan struct{})
		defer close(stopCh)
		go reCtx.StartInformers(stopCh)
	})
})
