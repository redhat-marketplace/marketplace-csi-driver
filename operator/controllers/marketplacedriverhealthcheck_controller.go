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
	"time"

	marketplacev1alpha1 "github.com/redhat-marketplace/marketplace-csi-driver/operator/api/v1alpha1"
	"github.com/redhat-marketplace/marketplace-csi-driver/operator/pkg/common"
	"github.com/redhat-marketplace/marketplace-csi-driver/operator/pkg/status"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// MarketplaceDriverHealthCheckReconciler reconciles a MarketplaceDriverHealthCheck object
type MarketplaceDriverHealthCheckReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

var keepTimeInMinutes float64 = 1440
var pollingBackOffTimeInMinutes float64 = 60

// +kubebuilder:rbac:groups=marketplace.redhat.com,resources=marketplacedriverhealthchecks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=marketplace.redhat.com,resources=marketplacedriverhealthchecks/status,verbs=get;update;patch

func (r *MarketplaceDriverHealthCheckReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("marketplacedriverhealthcheck", req.NamespacedName)
	log.Info("Starting reconcile for MarketplaceDriverHealthCheck")

	hc := &marketplacev1alpha1.MarketplaceDriverHealthCheck{}
	err := r.Get(ctx, req.NamespacedName, hc)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			log.Info("MarketplaceDriverHealthCheck resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get MarketplaceDriverHealthCheck")
		return ctrl.Result{}, err
	}

	elapsedTime := time.Now().Sub(hc.CreationTimestamp.Time)
	if elapsedTime.Minutes() > keepTimeInMinutes {
		err = r.Delete(ctx, hc)
		if err != nil {
			log.Error(err, "Failed to delete MarketplaceDriverHealthCheck")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	requeue, updateNeeded := r.updateHealthCheckCondition(hc)
	if updateNeeded {
		updError := r.Client.Status().Update(ctx, hc)
		if updError != nil {
			log.Error(updError, "failed to update health check status")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{RequeueAfter: requeue}, nil
}

func (r *MarketplaceDriverHealthCheckReconciler) updateHealthCheckCondition(hc *marketplacev1alpha1.MarketplaceDriverHealthCheck) (time.Duration, bool) {
	requeue := time.Minute * 5
	updateNeeded := false
	if len(hc.Spec.AffectedNodes) > len(hc.Status.DriverHealthCheckStatus) {
		waitTime := time.Now().Sub(hc.CreationTimestamp.Time)
		if waitTime.Minutes() > pollingBackOffTimeInMinutes {
			requeue = time.Minute * 60
		}
		updateNeeded = hc.Status.Conditions.SetCondition(status.Condition{
			Type:    common.ConditionTypeHealthCheck,
			Reason:  common.ReasonRepairInProgress,
			Status:  corev1.ConditionFalse,
			Message: "Repair in progress",
		})
	} else {
		reason := common.ReasonRepairComplete
		message := "Repair completed in all nodes"
		rstatus := corev1.ConditionTrue
		for _, hcStatus := range hc.Status.DriverHealthCheckStatus {
			if hcStatus.DriverResponse == "Error" {
				reason = common.ReasonRepairFailed
				message = hcStatus.Message
				rstatus = corev1.ConditionFalse
				break
			}
		}
		updateNeeded = hc.Status.Conditions.SetCondition(status.Condition{
			Type:    common.ConditionTypeHealthCheck,
			Reason:  reason,
			Status:  rstatus,
			Message: message,
		})
		requeue = time.Minute * time.Duration(keepTimeInMinutes)
	}
	return requeue, updateNeeded
}

func (r *MarketplaceDriverHealthCheckReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&marketplacev1alpha1.MarketplaceDriverHealthCheck{}).
		Complete(r)
}
