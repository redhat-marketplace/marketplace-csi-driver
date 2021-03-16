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
	"crypto/md5"
	"fmt"

	"github.com/redhat-marketplace/marketplace-csi-driver/operator/pkg/common"
	"github.com/redhat-marketplace/marketplace-csi-driver/operator/pkg/resourceapply"
	"github.com/redhat-marketplace/marketplace-csi-driver/operator/pkg/selector"

	"github.com/go-logr/logr"
	"github.com/redhat-marketplace/marketplace-csi-driver/operator/pkg/status"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	marketplacev1alpha1 "github.com/redhat-marketplace/marketplace-csi-driver/operator/api/v1alpha1"
)

// MarketplaceDatasetReconciler reconciles a MarketplaceDataset object
type MarketplaceDatasetReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
	Helper common.ControllerHelperInterface
}

// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch;create;update;delete
// +kubebuilder:rbac:groups=marketplace.redhat.com,resources=marketplacedatasets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=marketplace.redhat.com,resources=marketplacedatasets/status,verbs=get;update;patch

// Reconcile ...
func (r *MarketplaceDatasetReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("Request.Namespace", req.Namespace, "Request.Name", req.Name)
	log.Info("Starting reconcile for MarketplaceDataset")

	dataset := &marketplacev1alpha1.MarketplaceDataset{}
	err := r.Get(ctx, req.NamespacedName, dataset)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			log.Info("MarketplaceDataset resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get MarketplaceDataset")
		return ctrl.Result{}, err
	}

	if r.isUninstallRequest(dataset) {
		log.Info("MarketplaceDataset uninstall requested, deleting dataset")
		err = r.Delete(ctx, dataset)
		if err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	err = r.updateLabel(&req, dataset)
	if err != nil {
		r.setStatusInstallError(&req, dataset, err)
		return ctrl.Result{}, err
	}

	ra := resourceapply.ResourceApply{
		Client:    r.Client,
		Context:   ctx,
		Log:       log,
		Helper:    r.Helper,
		Owner:     datasetAsOwner(dataset),
		Namespace: req.Namespace,
	}
	_, err = ra.Apply(common.AssetPathDatasetPVC)
	if err != nil {
		r.setStatusInstallError(&req, dataset, err)
		return ctrl.Result{}, err
	}

	r.setStatusInstallComplete(&req, dataset)
	return ctrl.Result{}, nil
}

// SetupWithManager ...
func (r *MarketplaceDatasetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	ownerHandler := &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &marketplacev1alpha1.MarketplaceDataset{},
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&marketplacev1alpha1.MarketplaceDataset{}).
		Watches(&source.Kind{Type: &corev1.PersistentVolumeClaim{}}, ownerHandler).
		Complete(r)
}

func (r *MarketplaceDatasetReconciler) updateLabel(req *ctrl.Request, dataset *marketplacev1alpha1.MarketplaceDataset) error {
	log := r.Log.WithValues("Request.Namespace", req.Namespace, "Request.Name", req.Name)
	ctx := context.Background()

	selectorHash := r.getSelectorHash(dataset, log)
	updateNeeded := false

	if len(dataset.Labels) == 0 {
		dataset.Labels = make(map[string]string)
		updateNeeded = true
	}
	value, ok := dataset.Labels[common.LabelDatasetID]
	if !ok || value != dataset.Spec.Bucket {
		log.Info("setting UID label for dataset")
		dataset.Labels[common.LabelDatasetID] = dataset.Spec.Bucket
		updateNeeded = true
	}
	prevHash, ok := dataset.Labels[common.LabelSelectorHash]
	selectorChanged := ok && prevHash != selectorHash
	if !ok || selectorChanged {
		log.Info("setting UID label for dataset")
		dataset.Labels[common.LabelSelectorHash] = selectorHash
		updateNeeded = true
	}
	if updateNeeded {
		r.Update(ctx, dataset)
		err := r.Client.Update(ctx, dataset)
		if err != nil {
			log.Error(err, "Unable to add UID label to dataset", "Dataset", dataset.Name, "Bucket", dataset.Spec.Bucket)
			return err
		}
		err = r.createHealthCheckCR(req, dataset)
		if err != nil {
			log.Error(err, "Unable to create health check CR for dataset", "Dataset", dataset.Name, "Bucket", dataset.Spec.Bucket)
		}
	}
	return nil
}

func datasetAsOwner(v *marketplacev1alpha1.MarketplaceDataset) metav1.OwnerReference {
	falseVar := false
	return metav1.OwnerReference{
		APIVersion:         marketplacev1alpha1.GroupVersion.String(),
		Kind:               v.Kind,
		Name:               v.Name,
		UID:                v.UID,
		Controller:         &falseVar,
		BlockOwnerDeletion: &falseVar,
	}
}

func (r *MarketplaceDatasetReconciler) isUninstallRequest(dataset *marketplacev1alpha1.MarketplaceDataset) bool {
	if len(dataset.Labels) == 0 {
		return false
	}
	val, ok := dataset.Labels[common.LabelUninstall]
	return ok && val == "true"
}

func (r *MarketplaceDatasetReconciler) createHealthCheckCR(req *ctrl.Request, dataset *marketplacev1alpha1.MarketplaceDataset) error {
	ctx := context.Background()
	dsCheck := &marketplacev1alpha1.MarketplaceDriverHealthCheck{}
	dsCheck.Namespace = req.Namespace
	dsCheck.GenerateName = common.NameNodeHealthCheck
	dsCheck.Spec.Dataset = dataset.Name
	resourceapply.AddOwnerRefToObject(dsCheck, datasetAsOwner(dataset))
	return r.Create(ctx, dsCheck)
}

func (r *MarketplaceDatasetReconciler) setStatusInstallError(req *ctrl.Request, dataset *marketplacev1alpha1.MarketplaceDataset, err error) {
	log := r.Log.WithValues("Request.Namespace", req.Namespace, "Request.Name", req.Name)
	ctx := context.Background()
	dataset.Status.Conditions.SetCondition(status.Condition{
		Type:    common.ConditionTypeInstall,
		Reason:  common.ReasonInstallFailed,
		Status:  corev1.ConditionFalse,
		Message: err.Error(),
	})
	updError := r.Client.Status().Update(ctx, dataset)
	if updError != nil {
		log.Error(updError, "failed to update status")
	}
}

func (r *MarketplaceDatasetReconciler) setStatusInstallComplete(req *ctrl.Request, dataset *marketplacev1alpha1.MarketplaceDataset) {
	log := r.Log.WithValues("Request.Namespace", req.Namespace, "Request.Name", req.Name)
	ctx := context.Background()
	log.Info("Set complete status")

	icc := status.Condition{
		Type:    common.ConditionTypeInstall,
		Reason:  common.ReasonInstallComplete,
		Status:  corev1.ConditionTrue,
		Message: "Dataset install completed",
	}
	if len(dataset.Spec.PodSelectors) > 0 {
		err := selector.ValidateTerms(dataset.Spec.PodSelectors)
		if err != nil {
			dataset.Status.Conditions.SetCondition(status.Condition{
				Type:    common.ConditionTypeInstall,
				Reason:  common.ReasonValidationError,
				Status:  corev1.ConditionFalse,
				Message: err.Error(),
			})
		} else {
			dataset.Status.Conditions.SetCondition(icc)
		}
	} else {
		dataset.Status.Conditions.SetCondition(icc)
	}
	err := r.Client.Status().Update(ctx, dataset)
	if err != nil {
		log.Error(err, "failed to update status")
	}
}

func (r *MarketplaceDatasetReconciler) getSelectorHash(dataset *marketplacev1alpha1.MarketplaceDataset, log logr.Logger) string {
	selectorHash := ""
	if len(dataset.Spec.PodSelectors) > 0 {
		jsonBytes, err := r.Helper.Marshal(dataset.Spec.PodSelectors)
		if err != nil {
			log.Error(err, fmt.Sprintf("Unable to marshall podSelectors for %s", dataset.Name))
			return selectorHash
		}
		selectorHash = fmt.Sprintf("%x", md5.Sum(jsonBytes))
	}
	return selectorHash
}
