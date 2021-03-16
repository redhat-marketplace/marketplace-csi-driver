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
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	marketplacev1alpha1 "github.com/redhat-marketplace/marketplace-csi-driver/operator/api/v1alpha1"
	"github.com/redhat-marketplace/marketplace-csi-driver/operator/pkg/common"
	"github.com/redhat-marketplace/marketplace-csi-driver/operator/pkg/status"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Context("MarketplaceDriverHealthCheck Controller", func() {
	var (
		ctx       = context.TODO()
		namespace = "default"
		name      = "healthcheck"
		hcr       *MarketplaceDriverHealthCheckReconciler
		hc        *marketplacev1alpha1.MarketplaceDriverHealthCheck
	)

	BeforeEach(func() {
		hc = &marketplacev1alpha1.MarketplaceDriverHealthCheck{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Spec: marketplacev1alpha1.MarketplaceDriverHealthCheckSpec{
				AffectedNodes: []string{"test-node-1", "test-node-2"},
			},
		}
		hcr = &MarketplaceDriverHealthCheckReconciler{
			Scheme: scheme.Scheme,
			Log:    logf.Log.WithName("controllers").WithName("MarketplaceDriverHealthCheck"),
		}
	})

	It("Create healthcheck", func() {
		hc.CreationTimestamp = metav1.Time{Time: time.Now()}
		hcr.Client = fake.NewFakeClient(hc)

		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			},
		}

		_, err := hcr.Reconcile(req)
		Expect(err).NotTo(HaveOccurred(), "Reconcile returned error for HC create")
		key := client.ObjectKey{
			Name:      name,
			Namespace: namespace,
		}

		reason := getMarketplaceDriverHealthCheckStatusReason(ctx, key, hcr.Client, common.ConditionTypeHealthCheck)
		Expect(reason).To(Equal("RepairInProgress"))
	})

	It("update healthcheck complete", func() {
		hc.CreationTimestamp = metav1.Time{Time: time.Now()}
		hc.Status.DriverHealthCheckStatus = map[string]marketplacev1alpha1.HealthCheckStatus{
			"test-node-1": {
				PodCount:       1,
				DriverResponse: "RepairComplete",
			},
			"test-node-2": {
				PodCount:       0,
				DriverResponse: "NoEligibePods",
			},
		}
		hcr.Client = fake.NewFakeClient(hc)

		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			},
		}

		_, err := hcr.Reconcile(req)
		Expect(err).NotTo(HaveOccurred(), "Reconcile returned error for HC update")
		key := client.ObjectKey{
			Name:      name,
			Namespace: namespace,
		}

		reason := getMarketplaceDriverHealthCheckStatusReason(ctx, key, hcr.Client, common.ConditionTypeHealthCheck)
		Expect(reason).To(Equal("RepairComplete"))
	})

	It("update healthcheck inprogress", func() {
		hc.CreationTimestamp = metav1.Time{Time: time.Now().Add(time.Minute * -65)}
		hc.Status.DriverHealthCheckStatus = map[string]marketplacev1alpha1.HealthCheckStatus{
			"test-node-1": {
				PodCount:       1,
				DriverResponse: "RepairComplete",
			},
		}
		hcr.Client = fake.NewFakeClient(hc)

		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			},
		}

		_, err := hcr.Reconcile(req)
		Expect(err).NotTo(HaveOccurred(), "Reconcile returned error for HC update")
		key := client.ObjectKey{
			Name:      name,
			Namespace: namespace,
		}

		reason := getMarketplaceDriverHealthCheckStatusReason(ctx, key, hcr.Client, common.ConditionTypeHealthCheck)
		Expect(reason).To(Equal("RepairInProgress"))
	})

	It("Delete healthcheck", func() {
		hcr.Client = fake.NewFakeClient()
		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			},
		}

		_, err := hcr.Reconcile(req)
		Expect(err).NotTo(HaveOccurred(), "Reconcile returned error for HC create")
	})

	It("Expired healthcheck", func() {
		hc.CreationTimestamp = metav1.Time{Time: time.Now().Add(time.Hour * -48)}
		hcr.Client = fake.NewFakeClient(hc)

		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			},
		}

		_, err := hcr.Reconcile(req)
		Expect(err).NotTo(HaveOccurred(), "Reconcile returned error for HC create")
		key := client.ObjectKey{
			Name:      name,
			Namespace: namespace,
		}

		err = hcr.Client.Get(ctx, key, hc)
		Expect(err).To(HaveOccurred(), "expired HC not deleted")
		Expect(errors.IsNotFound(err)).To(BeTrue(), "expected not found error after delete")
	})

	It("update healthcheck failed", func() {
		hc.CreationTimestamp = metav1.Time{Time: time.Now()}
		hc.Status.DriverHealthCheckStatus = map[string]marketplacev1alpha1.HealthCheckStatus{
			"test-node-1": {
				PodCount:       1,
				DriverResponse: "RepairComplete",
			},
			"test-node-2": {
				PodCount:       0,
				DriverResponse: "Error",
				Message:        "something bad happened",
			},
		}
		hcr.Client = fake.NewFakeClient(hc)

		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			},
		}

		_, err := hcr.Reconcile(req)
		Expect(err).NotTo(HaveOccurred(), "Reconcile returned error for HC update")
		key := client.ObjectKey{
			Name:      name,
			Namespace: namespace,
		}

		reason := getMarketplaceDriverHealthCheckStatusReason(ctx, key, hcr.Client, common.ConditionTypeHealthCheck)
		Expect(reason).To(Equal("RepairFailed"))
	})

	It("Create healthcheck error", func() {
		hc.CreationTimestamp = metav1.Time{Time: time.Now()}
		cl := newMarketplaceCSIDriverClient(fake.NewFakeClient(hc), "get", "MarketplaceDriverHealthCheck", name)
		hcr.Client = cl

		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			},
		}

		_, err := hcr.Reconcile(req)
		Expect(err).To(HaveOccurred(), "Reconcile returned error for HC create")
	})

	It("Delete healthcheck error", func() {
		cl := newMarketplaceCSIDriverClient(fake.NewFakeClient(hc), "delete", "MarketplaceDriverHealthCheck", name)
		hcr.Client = cl
		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			},
		}

		_, err := hcr.Reconcile(req)
		Expect(err).To(HaveOccurred(), "Reconcile returned error for HC create")
	})

	It("Create healthcheck status update error", func() {
		hc.CreationTimestamp = metav1.Time{Time: time.Now()}
		cl := newMarketplaceCSIDriverClient(fake.NewFakeClient(hc), "", "", "")
		cl.setStatusErrorAction("update")
		hcr.Client = cl

		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			},
		}

		_, err := hcr.Reconcile(req)
		Expect(err).NotTo(HaveOccurred(), "Reconcile returned error for HC create")
		key := client.ObjectKey{
			Name:      name,
			Namespace: namespace,
		}

		reason := getMarketplaceDriverHealthCheckStatusReason(ctx, key, hcr.Client, common.ConditionTypeHealthCheck)
		Expect(reason).To(Equal(""))
	})
})

func getMarketplaceDriverHealthCheckStatusReason(ctx context.Context, key client.ObjectKey, cl client.Client, t status.ConditionType) string {
	obj := &marketplacev1alpha1.MarketplaceDriverHealthCheck{}
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
