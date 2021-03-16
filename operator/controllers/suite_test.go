/*


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
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	marketplacev1alpha1 "github.com/redhat-marketplace/marketplace-csi-driver/operator/api/v1alpha1"
	"github.com/redhat-marketplace/marketplace-csi-driver/operator/pkg/common"
	// +kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var cfg *rest.Config
var k8sClient client.Client
var testEnv *envtest.Environment

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t,
		"Controller Suite",
		[]Reporter{printer.NewlineReporter{}})
}

var _ = BeforeSuite(func(done Done) {
	logf.SetLogger(zap.LoggerTo(GinkgoWriter, true))

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "config", "crd", "bases")},
	}

	var err error
	cfg, err = testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	err = marketplacev1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = marketplacev1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = marketplacev1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	// +kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).ToNot(HaveOccurred())
	Expect(k8sClient).ToNot(BeNil())

	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
	})
	Expect(err).ToNot(HaveOccurred())

	err = (&MarketplaceCSIDriverReconciler{
		Client: k8sManager.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("MarketplaceCSIDriver"),
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = (&MarketplaceDatasetReconciler{
		Client: k8sManager.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("MarketplaceDataset"),
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = (&MarketplaceDriverHealthCheckReconciler{
		Client: k8sManager.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("MarketplaceDriverHealthCheck"),
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	close(done)
}, 60)

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
})

type marketplaceCSIDriverClient struct {
	c                   client.Client
	errorOnAction       string
	errorOnKind         string
	errorOnObjectName   string
	errorOnStatusAction string
	objectMap           map[string]runtime.Object
}

type marketplaceCSIDriverClientStatus struct {
	c             client.Client
	errorOnAction string
}

func newMarketplaceCSIDriverClient(c client.Client, action, kind, name string) *marketplaceCSIDriverClient {
	return &marketplaceCSIDriverClient{
		c,
		action,
		kind,
		name,
		"",
		make(map[string]runtime.Object),
	}
}

func (m *marketplaceCSIDriverClient) addRuntimeObject(obj runtime.Object) {
	objVal := reflect.ValueOf(obj).Elem()
	name := objVal.FieldByName("ObjectMeta").FieldByName("Name").String()
	kind := objVal.FieldByName("TypeMeta").FieldByName("Kind").String()
	m.objectMap[fmt.Sprintf("%s-%s", kind, name)] = obj
}

func (m *marketplaceCSIDriverClient) setStatusErrorAction(action string) {
	m.errorOnStatusAction = action
}

func (m *marketplaceCSIDriverClient) getRuntimeObject(key types.NamespacedName, obj runtime.Object) runtime.Object {
	typeStr := fmt.Sprintf("%T", obj)
	typearr := strings.Split(typeStr, ".")
	v, ok := m.objectMap[fmt.Sprintf("%s-%s", typearr[1], key.Name)]
	if !ok {
		return nil
	}
	return v
}

func (m *marketplaceCSIDriverClient) Create(ctx context.Context, obj runtime.Object, opts ...client.CreateOption) error {
	if m.isErrorMatch(obj, "create") {
		return fmt.Errorf("Fake error")
	}
	return m.c.Create(ctx, obj, opts...)
}

func (m *marketplaceCSIDriverClient) Update(ctx context.Context, obj runtime.Object, opts ...client.UpdateOption) error {
	if m.isErrorMatch(obj, "update") {
		return fmt.Errorf("Fake error")
	}
	return m.c.Update(ctx, obj, opts...)
}

func (m *marketplaceCSIDriverClient) Delete(ctx context.Context, obj runtime.Object, opts ...client.DeleteOption) error {
	if m.isErrorMatch(obj, "delete") {
		return fmt.Errorf("Fake error")
	}
	return m.c.Delete(ctx, obj, opts...)
}

func (m *marketplaceCSIDriverClient) DeleteAllOf(ctx context.Context, obj runtime.Object, opts ...client.DeleteAllOfOption) error {
	return m.c.DeleteAllOf(ctx, obj, opts...)
}

func (m *marketplaceCSIDriverClient) Get(ctx context.Context, key types.NamespacedName, obj runtime.Object) error {
	if m.isGetErrorMatch(key, obj) {
		return fmt.Errorf("Fake error")
	}
	val := m.getRuntimeObject(key, obj)
	if val != nil {
		data, err := json.Marshal(val)
		if err != nil {
			fmt.Printf("Error serializeing to json: %v", err)
			return nil
		}
		runtime.DecodeInto(scheme.Codecs.UniversalDecoder(), data, obj)
		return nil
	}
	return m.c.Get(ctx, key, obj)
}

func (m *marketplaceCSIDriverClient) List(ctx context.Context, obj runtime.Object, opts ...client.ListOption) error {
	typeStr := fmt.Sprintf("%T", obj)
	typearr := strings.Split(typeStr, ".")
	kind := typearr[1]
	if m.errorOnAction == "list" && m.errorOnKind == kind {
		return fmt.Errorf("Fake error")
	}
	return m.c.List(ctx, obj, opts...)
}

func (m *marketplaceCSIDriverClient) Patch(ctx context.Context, obj runtime.Object, patch client.Patch, opts ...client.PatchOption) error {
	return m.c.Patch(ctx, obj, patch, opts...)
}

func (m *marketplaceCSIDriverClient) Status() client.StatusWriter {
	return &marketplaceCSIDriverClientStatus{
		m.c,
		m.errorOnStatusAction,
	}
}

func (ms *marketplaceCSIDriverClientStatus) Patch(ctx context.Context, obj runtime.Object, p client.Patch, opts ...client.PatchOption) error {
	if ms.errorOnAction == "patch" {
		return fmt.Errorf("patch error")
	}
	return ms.c.Status().Patch(ctx, obj, p, opts...)
}

func (ms *marketplaceCSIDriverClientStatus) Update(ctx context.Context, obj runtime.Object, opts ...client.UpdateOption) error {
	if ms.errorOnAction == "update" {
		return fmt.Errorf("patch error")
	}
	return ms.c.Status().Update(ctx, obj, opts...)
}

func (m *marketplaceCSIDriverClient) isErrorMatch(obj runtime.Object, action string) bool {
	if action != m.errorOnAction {
		return false
	}
	typeStr := fmt.Sprintf("%T", obj)
	typearr := strings.Split(typeStr, ".")
	kind := typearr[1]

	objVal := reflect.ValueOf(obj).Elem()
	name := objVal.FieldByName("ObjectMeta").FieldByName("Name").String()
	name2 := objVal.FieldByName("ObjectMeta").FieldByName("GenerateName").String()

	if m.errorOnKind == kind {
		return strings.HasPrefix(name, m.errorOnObjectName) || strings.HasPrefix(name2, m.errorOnObjectName)
	}
	return false
}

func (m *marketplaceCSIDriverClient) isGetErrorMatch(key types.NamespacedName, obj runtime.Object) bool {
	if m.errorOnAction != "get" {
		return false
	}
	typeStr := fmt.Sprintf("%T", obj)
	typearr := strings.Split(typeStr, ".")
	kind := typearr[1]
	if m.errorOnKind == kind || kind == "Unstructured" {
		return key.Name == m.errorOnObjectName
	}
	return false
}

type FakeControllerHelper struct {
	errorAssetPath   string
	replaceAssetPath string
	marshalErrorType string
	helper           common.ControllerHelperInterface
}

func NewFakeControllerHelper(errorAssetPath, replaceAssetPath, marshalErrorType string) *FakeControllerHelper {
	return &FakeControllerHelper{
		errorAssetPath,
		replaceAssetPath,
		marshalErrorType,
		common.NewControllerHelper(),
	}
}

func (ch *FakeControllerHelper) Asset(name string) ([]byte, error) {
	if name == ch.errorAssetPath {
		if ch.replaceAssetPath != "" {
			return ch.Asset(ch.replaceAssetPath)
		}
		return nil, fmt.Errorf("Fake error")
	}
	return ch.helper.Asset(name)
}

func (ch *FakeControllerHelper) GetCloudProviderType(c client.Client, log logr.Logger) (string, error) {
	return ch.helper.GetCloudProviderType(c, log)
}

func (ch *FakeControllerHelper) Marshal(v interface{}) ([]byte, error) {
	if ch.marshalErrorType != "" && fmt.Sprintf("%T", v) == ch.marshalErrorType {
		return nil, fmt.Errorf("Fake error")
	}

	return ch.helper.Marshal(v)
}

func (ch *FakeControllerHelper) DeepEqual(r1, r2 interface{}) bool {
	return ch.helper.DeepEqual(r1, r2)
}
