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
	"github.com/dgrijalva/jwt-go"
	"github.com/kubernetes-csi/csi-test/pkg/sanity"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/redhat-marketplace/marketplace-csi-driver/driver/pkg/client"
	"github.com/redhat-marketplace/marketplace-csi-driver/driver/pkg/common"
	"github.com/redhat-marketplace/marketplace-csi-driver/driver/pkg/helper"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	fakedyn "k8s.io/client-go/dynamic/fake"
	fake "k8s.io/client-go/kubernetes/fake"
	fakecorev1 "k8s.io/client-go/kubernetes/typed/core/v1/fake"
	"k8s.io/client-go/testing"
)

var _ = Describe("CSI Driver basic test", func() {

	Describe("CSI sanity", func() {
		socket := "/tmp/csi-dataset.sock"
		csiEndpoint := "unix://" + socket
		timeSuffix := time.Now().Format("20060102030405PM")
		tempdir := os.TempDir()
		if !strings.HasSuffix(tempdir, "/") {
			tempdir = tempdir + "/"
		}
		targetPath := tempdir + "csi-dataset-target-sanity-" + timeSuffix
		stagingPath := tempdir + "csi-dataset-staging-sanity-" + timeSuffix
		driver := &COSDriver{
			name:     "test-dataset-driver",
			nodeName: "test-node",
			version:  GetVersionJSON(),
			endpoint: csiEndpoint,
			helper: &fakeDriverHelper{
				Clientset: fake.NewSimpleClientset(getTestPodResource(), getTestPullSecret()),
				Dynclient: getTestFakeDynamicClient(getTestCSIDriverResource(), getTestDatasetResource()),
				validator: &fakeValidator{},
			},
		}
		go driver.Run()
		sanityCfg := &sanity.Config{
			TargetPath:  targetPath,
			StagingPath: stagingPath,
			Address:     csiEndpoint,
			TestVolumeParameters: map[string]string{
				"test":                             "value",
				"csi.storage.k8s.io/pod.name":      "testpod",
				"csi.storage.k8s.io/pod.namespace": "default",
			},
		}
		sanity.GinkgoTest(sanityCfg)
	})

	Describe("Initialization tests", func() {
		It("Should return error for bad endpoint", func() {
			d := &COSDriver{
				name:     "test-dataset-driver-2",
				nodeName: "test-node",
				version:  GetVersionJSON(),
				endpoint: "%^&*()!`~{",
			}
			err := d.Run()
			Expect(err).To(HaveOccurred(), "should return error")
		})

		It("Should return error for unsupported protocol", func() {
			d := &COSDriver{
				name:     "test-dataset-driver-2",
				nodeName: "test-node",
				version:  GetVersionJSON(),
				endpoint: "junkprotocol://a.b.c",
			}
			err := d.Run()
			Expect(err).To(HaveOccurred(), "should return error")
		})

		It("Should return error for endpoint remove error", func() {
			socket := "/tmp/csi-dataset-e.sock"
			csiEndpoint := "unix://" + socket

			d := &COSDriver{
				name:     "test-dataset-driver-2",
				nodeName: "test-node",
				version:  GetVersionJSON(),
				endpoint: csiEndpoint,
				helper:   &fakeDriverHelper{errorOnAction: "removefile"},
			}
			err := d.Run()
			Expect(err).To(HaveOccurred(), "should return error")
		})

		It("Should return error for endpoint listen error", func() {
			socket := "/tmp/csi-dataset-e.sock"
			csiEndpoint := "unix://" + socket

			os.Create(socket)
			d := &COSDriver{
				name:     "test-dataset-driver-2",
				nodeName: "test-node",
				version:  GetVersionJSON(),
				endpoint: csiEndpoint,
				helper:   &fakeDriverHelper{errorOnAction: "removefileignore"},
			}
			err := d.Run()
			Expect(err).To(HaveOccurred(), "should return error")
		})
	})

	Describe("Client coverage tests", func() {
		var kc *client.KubeClient
		var npvr *csi.NodePublishVolumeRequest
		var fHelper *fakeDriverHelper
		var driver *COSDriver
		var volMountDetails common.VolumeMountDetails
		var pod *corev1.Pod
		var fc *fake.Clientset
		var fcd *fakedyn.FakeDynamicClient
		var hcreq *unstructured.Unstructured

		BeforeEach(func() {
			hcreq = &unstructured.Unstructured{
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
			pod = getTestPodResource()
			fc = fake.NewSimpleClientset(pod)
			fcd = getTestFakeDynamicClient(hcreq)
			fHelper = &fakeDriverHelper{Clientset: fc, Dynclient: fcd}
			kc = client.NewKubeClient(context.Background(), fc, fcd, fHelper)
			npvr = &csi.NodePublishVolumeRequest{
				VolumeId:   "123",
				TargetPath: "targetPath",
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
			volMountDetails = common.VolumeMountDetails{
				Datasets: []common.Dataset{},
				Pod:      pod,
			}
			driver = &COSDriver{
				name:     "name",
				nodeName: "node",
				version:  "version",
				endpoint: "endpoint",
				helper:   fHelper,
			}
		})

		It("getPodDetails handle missing pod name", func() {
			npvr.VolumeContext = map[string]string{
				"csi.storage.k8s.io/pod.namespace": "default",
			}
			_, err := kc.GetPodDetails(npvr)
			Expect(err).To(HaveOccurred(), "error expected")
		})

		It("getPodDetails handle pod not found", func() {
			npvr.VolumeContext = map[string]string{
				"csi.storage.k8s.io/pod.name":      "testpod22222",
				"csi.storage.k8s.io/pod.namespace": "default",
			}
			_, err := kc.GetPodDetails(npvr)
			Expect(err).To(HaveOccurred(), "error expected")
		})

		It("GetStringValue handle parse error", func() {
			ds := &unstructured.Unstructured{
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
			v := client.GetStringValue(ds, "test", "spec", "bucket")
			Expect(v).To(Equal("test"), "incorrect return value")
		})

		It("GetStringSliceValue handle parse error", func() {
			ds := &unstructured.Unstructured{
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
			v := client.GetStringSliceValue(ds, []string{"test"}, "spec", "bucket")
			Expect(len(v)).To(Equal(1), "incorrect return value")
			Expect(v[0]).To(Equal("test"), "incorrect return value")
		})

		It("ParseDatasetSpec handle bad spec", func() {
			ds := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "marketplace.redhat.com/v1alpha1",
					"kind":       "MarketplaceDataset",
					"metadata": map[string]interface{}{
						"namespace": "default",
						"name":      "dataset-1",
					},
					"spec": false,
				},
			}
			_, err := kc.ParseDatasetSpec(ds)
			Expect(err).To(HaveOccurred(), "error expected")
		})

		It("ParseDatasetSpec handle missing spec", func() {

			ds := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "marketplace.redhat.com/v1alpha1",
					"kind":       "MarketplaceDataset",
					"metadata": map[string]interface{}{
						"namespace": "default",
						"name":      "dataset-1",
					},
					"spec2": map[string]interface{}{
						"bucket": false,
					},
				},
			}
			_, err := kc.ParseDatasetSpec(ds)
			Expect(err).To(HaveOccurred(), "error expected")
		})

		It("ParseDatasetSpec handle missing spec", func() {
			fHelper = &fakeDriverHelper{errorOnAction: "marshaljson"}
			kc = client.NewKubeClient(context.Background(), fake.NewSimpleClientset(), getTestFakeDynamicClient(), fHelper)
			ds := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "marketplace.redhat.com/v1alpha1",
					"kind":       "MarketplaceDataset",
					"metadata": map[string]interface{}{
						"namespace": "default",
						"name":      "dataset-1",
					},
					"spec": map[string]interface{}{
						"bucket": "rhmccp-1234",
					},
				},
			}
			_, err := kc.ParseDatasetSpec(ds)
			Expect(err).To(HaveOccurred(), "error expected")
		})

		It("UpdatePodOnNodePublishVolume handle successful mount with some errors", func() {
			kc.UpdatePodOnNodePublishVolume(npvr, volMountDetails, 2, fmt.Errorf("fake error"))
			cond := getTestPodCondition(driver, context.Background())
			Expect(cond.Reason).To(Equal("Warning"), "expected reason Warning ")
		})

		It("UpdatePodOnNodePublishVolume handle get pod error", func() {
			var count = 0
			fc.CoreV1().(*fakecorev1.FakeCoreV1).PrependReactor("get", "pods", func(action testing.Action) (handled bool, ret runtime.Object, err error) {
				count++
				if count == 2 {
					return true, &corev1.Pod{}, fmt.Errorf("fake error")
				}
				return false, &corev1.Pod{}, nil
			})
			kc.UpdatePodOnNodePublishVolume(npvr, volMountDetails, 2, fmt.Errorf("fake error"))
			cond := getTestPodCondition(driver, context.Background())
			Expect(cond.Reason).To(Equal(""), "expected no dataset condition ")
		})

		It("UpdatePodOnNodePublishVolume handle update pod error", func() {
			fc.CoreV1().(*fakecorev1.FakeCoreV1).PrependReactor("*", "pods", func(action testing.Action) (handled bool, ret runtime.Object, err error) {
				if action.GetVerb() == "update" && action.GetSubresource() == "status" {
					return true, &corev1.Pod{}, fmt.Errorf("fake error")
				}
				return false, nil, nil
			})
			kc.UpdatePodOnNodePublishVolume(npvr, volMountDetails, 2, fmt.Errorf("fake error"))
			cond := getTestPodCondition(driver, context.Background())
			Expect(cond.Reason).To(Equal(""), "expected no dataset condition ")
		})

		It("UpdatePodStatusOnReconcile handle successful mount with some errors", func() {
			kc.UpdatePodStatusOnReconcile(volMountDetails, 2, fmt.Errorf("fake error"))
			cond := getTestPodCondition(driver, context.Background())
			Expect(cond.Reason).To(Equal("Warning"), "expected reason Warning ")
		})

		It("UpdatePodStatusOnReconcile handle get pod error", func() {
			var count = 0
			fc.CoreV1().(*fakecorev1.FakeCoreV1).PrependReactor("get", "pods", func(action testing.Action) (handled bool, ret runtime.Object, err error) {
				count++
				if count == 1 {
					return true, &corev1.Pod{}, fmt.Errorf("fake error")
				}
				return false, &corev1.Pod{}, nil
			})
			kc.UpdatePodStatusOnReconcile(volMountDetails, 2, fmt.Errorf("fake error"))
			cond := getTestPodCondition(driver, context.Background())
			Expect(cond.Reason).To(Equal(""), "expected no dataset condition ")
		})

		It("UpdatePodStatusOnReconcile handle update pod error", func() {
			fc.CoreV1().(*fakecorev1.FakeCoreV1).PrependReactor("*", "pods", func(action testing.Action) (handled bool, ret runtime.Object, err error) {
				if action.GetVerb() == "update" && action.GetSubresource() == "status" {
					return true, &corev1.Pod{}, fmt.Errorf("fake error")
				}
				return false, nil, nil
			})
			kc.UpdatePodStatusOnReconcile(volMountDetails, 2, fmt.Errorf("fake error"))
			cond := getTestPodCondition(driver, context.Background())
			Expect(cond.Reason).To(Equal(""), "expected no dataset condition ")
		})

		It("TagPodForTracking handle get pod error", func() {
			var count = 0
			fc.CoreV1().(*fakecorev1.FakeCoreV1).PrependReactor("get", "pods", func(action testing.Action) (handled bool, ret runtime.Object, err error) {
				count++
				if count < 2 {
					return true, &corev1.Pod{}, fmt.Errorf("fake error")
				}
				return false, &corev1.Pod{}, nil
			})
			kc.UpdatePodOnNodePublishVolume(npvr, volMountDetails, 2, fmt.Errorf("fake error"))
			p, err := fc.CoreV1().Pods("default").Get(context.Background(), "testpod", metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred(), "error unexpected")
			lbl := ""
			if len(p.Labels) > 0 {
				v, ok := p.Labels[common.PodLabelWatchKey]
				if ok {
					lbl = v
				}
			}
			Expect(lbl).To(Equal(""), "expected no labels set ")
		})

		It("TagPodForTracking handle patch pod error", func() {
			var count = 0
			fc.CoreV1().(*fakecorev1.FakeCoreV1).PrependReactor("patch", "pods", func(action testing.Action) (handled bool, ret runtime.Object, err error) {
				count++
				if count < 2 {
					return true, &corev1.Pod{}, fmt.Errorf("fake error")
				}
				return false, &corev1.Pod{}, nil
			})
			kc.UpdatePodOnNodePublishVolume(npvr, volMountDetails, 2, fmt.Errorf("fake error"))
			p, err := fc.CoreV1().Pods("default").Get(context.Background(), "testpod", metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred(), "error unexpected")
			lbl := ""
			if len(p.Labels) > 0 {
				v, ok := p.Labels[common.PodLabelWatchKey]
				if ok {
					lbl = v
				}
			}
			Expect(lbl).To(Equal(""), "expected no labels set ")
		})

		It("TagPodForTracking handle marshal error", func() {
			fHelper = &fakeDriverHelper{Clientset: fc, Dynclient: fcd, errorOnAction: "marshaljson"}
			kc = client.NewKubeClient(context.Background(), fc, fcd, fHelper)
			kc.UpdatePodOnNodePublishVolume(npvr, volMountDetails, 2, fmt.Errorf("fake error"))
			p, err := fc.CoreV1().Pods("default").Get(context.Background(), "testpod", metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred(), "error unexpected")
			lbl := ""
			if len(p.Labels) > 0 {
				v, ok := p.Labels[common.PodLabelWatchKey]
				if ok {
					lbl = v
				}
			}
			Expect(lbl).To(Equal(""), "expected no labels set ")
		})

		It("UpdateHealthCheckStatus handle get hcr error", func() {
			var count = 0
			fcd.PrependReactor("get", "*", func(action testing.Action) (handled bool, ret runtime.Object, err error) {
				if count < 1 && action.GetResource().Resource == common.ResourceNameDriverCheck {
					count++
					return true, nil, fmt.Errorf("fake error")
				}
				return false, &unstructured.Unstructured{}, nil
			})
			status := common.HealthCheckStatus{
				DriverResponse: "Complete",
				PodCount:       2,
				Message:        "Repair complete",
			}
			kc.UpdateHealthCheckStatus("testnode", "default", "health-check-cr1", status)
			hc, err := fcd.Resource(kc.GetHealthCheckGVR()).Namespace("default").Get(context.Background(), "health-check-cr1", metav1.GetOptions{}, "status")
			Expect(err).NotTo(HaveOccurred(), "error unexpected")
			_, exists, err := unstructured.NestedMap(hc.Object, "status", "driverHealthCheckStatus")
			Expect(err).NotTo(HaveOccurred(), "error unexpected")
			Expect(exists).To(BeFalse(), "status should not exist")
		})

		It("UpdateHealthCheckStatus handle update hcr error", func() {
			var count = 0
			fcd.PrependReactor("update", "*", func(action testing.Action) (handled bool, ret runtime.Object, err error) {
				if count < 1 && action.GetResource().Resource == common.ResourceNameDriverCheck {
					count++
					return true, nil, fmt.Errorf("fake error")
				}
				return false, &unstructured.Unstructured{}, nil
			})
			status := common.HealthCheckStatus{
				DriverResponse: "Complete",
				PodCount:       2,
				Message:        "Repair complete",
			}
			kc.UpdateHealthCheckStatus("testnode", "default", "health-check-cr1", status)
			hc, err := fcd.Resource(kc.GetHealthCheckGVR()).Namespace("default").Get(context.Background(), "health-check-cr1", metav1.GetOptions{}, "status")
			Expect(err).NotTo(HaveOccurred(), "error unexpected")
			_, exists, err := unstructured.NestedMap(hc.Object, "status", "driverHealthCheckStatus")
			Expect(err).NotTo(HaveOccurred(), "error unexpected")
			Expect(exists).To(BeFalse(), "status should not exist")
		})

		It("UpdateHealthCheckStatus handle bad status data", func() {
			hcreq = &unstructured.Unstructured{
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
					"status": map[string]interface{}{
						"driverHealthCheckStatus": []interface{}{""},
					},
				},
			}
			fcd = getTestFakeDynamicClient(hcreq)
			fHelper = &fakeDriverHelper{Clientset: fc, Dynclient: fcd}
			kc = client.NewKubeClient(context.Background(), fc, fcd, fHelper)

			status := common.HealthCheckStatus{
				DriverResponse: "Complete",
				PodCount:       2,
				Message:        "Repair complete",
			}
			kc.UpdateHealthCheckStatus("testnode", "default", "health-check-cr1", status)
			hc, err := fcd.Resource(kc.GetHealthCheckGVR()).Namespace("default").Get(context.Background(), "health-check-cr1", metav1.GetOptions{}, "status")
			Expect(err).NotTo(HaveOccurred(), "error unexpected")
			_, _, err = unstructured.NestedMap(hc.Object, "status", "driverHealthCheckStatus")
			Expect(err).To(HaveOccurred(), "error expected")
		})

		It("GetPodCondition handle nil status", func() {
			_, cnd := client.GetPodCondition(nil, common.PodConditionType)
			Expect(cnd).To(BeNil(), "condition should be nil")
		})

		It("GetPodConditionFromList handle no match", func() {
			conditions := []corev1.PodCondition{
				{
					Type:    "Running",
					Reason:  "reason",
					Message: "message",
				},
			}
			_, cnd := client.GetPodConditionFromList(conditions, common.PodConditionType)
			Expect(cnd).To(BeNil(), "condition should be nil")
		})
	})

	Describe("Helper coverage tests", func() {
		helper := helper.NewDriverHelper()

		It("GetClaimValue should handle bad claim data", func() {
			claims := jwt.MapClaims{
				"test": 1,
			}
			_, err := helper.GetClaimValue(claims, "test")
			Expect(err).To(HaveOccurred(), "error expected")
		})

	})

})
