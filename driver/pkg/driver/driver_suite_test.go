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
	"testing"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/dgrijalva/jwt-go"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/redhat-marketplace/marketplace-csi-driver/driver/pkg/common"
	"github.com/redhat-marketplace/marketplace-csi-driver/driver/pkg/helper"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	fakedyn "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes"
)

func TestDriver(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Driver Suite")
}

type fakeValidator struct {
	createdVolumes map[string]int64
}

func (dv *fakeValidator) CheckCreateVolumeAllowed(req *csi.CreateVolumeRequest) error {
	if len(req.GetName()) == 0 {
		return status.Error(codes.InvalidArgument, "missing name")
	}
	if req.GetVolumeCapabilities() == nil {
		return status.Error(codes.InvalidArgument, "missing volume capabilities")
	}

	volumeID := strings.ToUpper(req.GetName())
	val, ok := dv.getVolume(volumeID)
	if !ok {
		dv.addVolume(volumeID, req.CapacityRange.GetRequiredBytes())
		return nil
	}
	if val != req.CapacityRange.GetRequiredBytes() {
		return status.Error(codes.AlreadyExists, "Volume exists")
	}
	dv.addVolume(volumeID, req.CapacityRange.GetRequiredBytes())
	return nil
}

func (dv *fakeValidator) CheckDeleteVolumeAllowed(req *csi.DeleteVolumeRequest) error {
	if len(req.GetVolumeId()) == 0 {
		return status.Error(codes.InvalidArgument, "missing volume ID")
	}
	return nil
}

func (dv *fakeValidator) CheckValidateVolumeCapabilitiesAllowed(req *csi.ValidateVolumeCapabilitiesRequest) error {
	if len(req.GetVolumeId()) == 0 {
		return status.Error(codes.InvalidArgument, "missing volume id")
	}
	if req.GetVolumeCapabilities() == nil {
		return status.Error(codes.InvalidArgument, "missing volume capabilities")
	}
	volumeID := strings.ToUpper(req.GetVolumeId())
	if len(dv.createdVolumes) > 0 {
		_, ok := dv.getVolume(volumeID)
		if !ok {
			return status.Error(codes.NotFound, "invalid volume id")
		}
	}
	return nil
}

func (dv *fakeValidator) addVolume(volumeID string, cap int64) {
	if len(dv.createdVolumes) == 0 {
		dv.createdVolumes = make(map[string]int64)
	}
	dv.createdVolumes[volumeID] = cap
}

func (dv *fakeValidator) getVolume(volumeID string) (int64, bool) {
	if len(dv.createdVolumes) == 0 {
		return 0, false
	}
	v, ok := dv.createdVolumes[volumeID]
	return v, ok
}

var origDriverHelper = helper.NewDriverHelper()

type fakeDriverHelper struct {
	Clientset     kubernetes.Interface
	Dynclient     dynamic.Interface
	errorOnAction string
	validator     helper.Validator
	command       string
}

func (dv *fakeDriverHelper) GetClientSet() (kubernetes.Interface, dynamic.Interface, error) {
	if dv.errorOnAction == "getclientset" {
		return nil, nil, fmt.Errorf("fake error")
	}
	return dv.Clientset, dv.Dynclient, nil
}

func (dv *fakeDriverHelper) GetMountCommand() string {
	if dv.command != "" {
		return dv.command
	}
	return "echo"
}

func (dv *fakeDriverHelper) GetValidator() helper.Validator {
	return dv.validator
}

func (dv *fakeDriverHelper) CheckMount(path string) (bool, error) {
	if dv.errorOnAction == "checkmount" {
		return false, nil
	}
	if dv.errorOnAction == "checkmounterror" {
		return false, fmt.Errorf("fake error")
	}
	return true, nil
}

func (dv *fakeDriverHelper) WaitForMount(path string, timeout time.Duration) error {
	if dv.errorOnAction == "waitformount" {
		return fmt.Errorf("fake error")
	}
	return nil
}

func (dv *fakeDriverHelper) GetDatasetDirectoryNames(targetPath string) []string {
	return origDriverHelper.GetDatasetDirectoryNames(targetPath)
}

func (dv *fakeDriverHelper) CleanMountPoint(targetPath string) error {
	if dv.errorOnAction == "cleanmountpoint" {
		return fmt.Errorf("fake error")
	}
	return os.Remove(targetPath)
}

func (dv *fakeDriverHelper) MkdirAll(path string, perm os.FileMode) error {
	if dv.errorOnAction == "mkdirall" {
		return fmt.Errorf("fake error")
	}
	return origDriverHelper.MkdirAll(path, perm)
}

func (dv *fakeDriverHelper) WriteFile(name, content string, flag int, perm os.FileMode) error {
	if dv.errorOnAction == "Writefile" {
		return fmt.Errorf("fake error")
	}
	return origDriverHelper.WriteFile(name, content, flag, perm)
}

func (dv *fakeDriverHelper) FileStat(path string) (os.FileInfo, error) {
	if dv.errorOnAction == "filestat" {
		return nil, fmt.Errorf("fake error")
	}
	return origDriverHelper.FileStat(path)
}

func (dv *fakeDriverHelper) ReadDir(path string) ([]os.FileInfo, error) {
	if dv.errorOnAction == "readdir" {
		return nil, fmt.Errorf("fake error")
	}
	return origDriverHelper.ReadDir(path)
}

func (dv *fakeDriverHelper) RemoveFile(path string) error {
	if dv.errorOnAction == "removefileignore" {
		return nil
	}
	if dv.errorOnAction == "removefile" {
		return fmt.Errorf("fake error")
	}
	return origDriverHelper.RemoveFile(path)
}

func (dv *fakeDriverHelper) ParseJWTclaims(jwtString string) (jwt.MapClaims, error) {
	return origDriverHelper.ParseJWTclaims(jwtString)
}

func (dv *fakeDriverHelper) GetClaimValue(claims jwt.MapClaims, key string) (string, error) {
	return origDriverHelper.GetClaimValue(claims, key)
}

func (dv *fakeDriverHelper) MarshaljSON(v interface{}) ([]byte, error) {
	if dv.errorOnAction == "marshaljson" {
		return nil, fmt.Errorf("fake error")
	}
	return origDriverHelper.MarshaljSON(v)
}

func (dv *fakeDriverHelper) UnMarshaljSON(data []byte, v interface{}) error {
	if dv.errorOnAction == "unmarshaljson" {
		return fmt.Errorf("fake error")
	}
	return origDriverHelper.UnMarshaljSON(data, v)
}

func getTestPodResource() *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "testpod",
		},
		Spec: corev1.PodSpec{
			NodeName: "test-node",
			Containers: []corev1.Container{
				{
					Name:  "test",
					Image: "nginx",
				},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
		},
	}
}

func getTestPullSecret() *corev1.Secret {
	data := make(map[string][]byte)
	fakePS := "eyJhbGciOiJIUzI1NiJ9.eyJpYW1fYXBpa2V5IjoiaWFta2V5IiwicmhtQWNjb3VudElkIjoiMTIzNDUiLCJlbnRpdGxlbWVudC5zdG9yYWdlLmhtYWMuYWNjZXNzX2tleV9pZCI6ImhtYWNrZXkiLCJlbnRpdGxlbWVudC5zdG9yYWdlLmhtYWMuc2VjcmV0X2FjY2Vzc19rZXkiOiJhY2Nlc3NrZXkiLCJpc3MiOiJJQk0gTWFya2V0cGxhY2UiLCJpYXQiOjEyMywianRpIjoiYWJjIn0.dJOBqdEpMzs4PCmnzTUT9ITGO2G_ZwiemjJbFtb3lmQ"
	data["PULL_SECRET"] = []byte(fakePS)
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: common.MarketplaceNamespace,
			Name:      "redhat-marketplace-pull-secret",
		},
		Type: corev1.SecretTypeOpaque,
		Data: data,
	}
}

func getTestCSIDriverResource() runtime.Object {
	return &unstructured.Unstructured{
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
				"mountOptions": []interface{}{"foo", "bar"},
			},
		},
	}
}

func getTestDatasetResource() runtime.Object {
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
			},
		},
	}
}

func getTestFakeDynamicClient(res ...runtime.Object) *fakedyn.FakeDynamicClient {
	scheme := runtime.NewScheme()
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{Group: common.CustomResourceGroup, Version: common.CustomResourceVersion, Kind: "MarketplaceDatasetList"}, &unstructured.UnstructuredList{})
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{Group: common.CustomResourceGroup, Version: common.CustomResourceVersion, Kind: "MarketplaceCSIDriverList"}, &unstructured.UnstructuredList{})
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{Group: common.CustomResourceGroup, Version: common.CustomResourceVersion, Kind: "MarketplaceDriverHealthCheckList"}, &unstructured.UnstructuredList{})
	return fakedyn.NewSimpleDynamicClient(scheme, res...)
}

func getTestPodStatus(driver *COSDriver, ctx context.Context) string {
	cs, _, err := driver.helper.GetClientSet()
	Expect(err).NotTo(HaveOccurred(), "unexpected error")
	pod, err := cs.CoreV1().Pods("default").Get(ctx, "testpod", metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred(), "unexpected error")
	dsReason := ""
	for _, cnd := range pod.Status.Conditions {
		if cnd.Type == common.PodConditionType {
			dsReason = cnd.Reason
		}
	}
	return dsReason
}

func getTestPodCondition(driver *COSDriver, ctx context.Context) *corev1.PodCondition {
	cs, _, err := driver.helper.GetClientSet()
	Expect(err).NotTo(HaveOccurred(), "unexpected error")
	pod, err := cs.CoreV1().Pods("default").Get(ctx, "testpod", metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred(), "unexpected error")
	var dsReason corev1.PodCondition
	for _, cnd := range pod.Status.Conditions {
		if cnd.Type == common.PodConditionType {
			dsReason = cnd
		}
	}
	return &dsReason
}
