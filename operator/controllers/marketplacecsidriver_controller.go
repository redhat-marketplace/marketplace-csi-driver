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

	"github.com/redhat-marketplace/marketplace-csi-driver/operator/pkg/common"
	"github.com/redhat-marketplace/marketplace-csi-driver/operator/pkg/resourceapply"
	"github.com/redhat-marketplace/marketplace-csi-driver/operator/pkg/status"
	admbeta1 "k8s.io/api/admissionregistration/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	marketplacev1alpha1 "github.com/redhat-marketplace/marketplace-csi-driver/operator/api/v1alpha1"
)

// MarketplaceCSIDriverReconciler reconciles a MarketplaceCSIDriver object
type MarketplaceCSIDriverReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
	Helper common.ControllerHelperInterface
}

var lastHealthCheckTime time.Time = time.Now()

// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,resources=daemonsets;statefulsets,verbs=get;list;watch;create;update
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles;clusterrolebindings;roles;rolebindings,verbs=get;list;watch;create;update
// +kubebuilder:rbac:groups=core,resources=events,verbs=list;watch;create;update;patch
// +kubebuilder:rbac:groups=core,resources=nodes;configmaps;namespaces,verbs=get;list;watch
// +kubebuilder:rbac:urls=/metrics,verbs=get
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;patch
// +kubebuilder:rbac:groups=core,resources=pods/status,verbs=update
// +kubebuilder:rbac:groups=core,resources=persistentvolumes,verbs=get;list;watch;patch;create;update;delete
// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch;update
// +kubebuilder:rbac:groups=marketplace.redhat.com,resources=marketplacecsidrivers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=marketplace.redhat.com,resources=marketplacecsidrivers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=marketplace.redhat.com,resources=marketplacedatasets,verbs=get;watch;list
// +kubebuilder:rbac:groups=marketplace.redhat.com,resources=marketplacedriverhealthchecks,verbs=create
// +kubebuilder:rbac:groups=security.openshift.io,resources=securitycontextconstraints,verbs=use
// +kubebuilder:rbac:groups=storage.k8s.io,resources=volumeattachments,verbs=get;list;watch;patch
// +kubebuilder:rbac:groups=storage.k8s.io,resources=csinodes,verbs=get;list;watch
// +kubebuilder:rbac:groups=storage.k8s.io,resources=csidrivers;storageclasses,verbs=get;list;watch;create;update
// +kubebuilder:rbac:groups=core,resources=services;serviceaccounts,verbs=get;list;watch;create;update;delete
// +kubebuilder:rbac:groups=admissionregistration.k8s.io,resources=mutatingwebhookconfigurations,verbs=get;list;watch;create;update
// +kubebuilder:rbac:groups=config.openshift.io,resources=infrastructures,verbs=get

func (r *MarketplaceCSIDriverReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("Request.Namespace", req.Namespace, "Request.Name", req.Name)
	log.Info("Starting reconcile for MarketplaceCSIDriver")

	s3driver := &marketplacev1alpha1.MarketplaceCSIDriver{}
	err := r.Get(ctx, req.NamespacedName, s3driver)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			log.Info("MarketplaceCSIDriver resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get MarketplaceCSIDriver")
		return ctrl.Result{}, err
	}

	if req.Namespace != common.MarketplaceNamespace {
		err := errors.NewBadRequest(fmt.Sprintf("%s is only supported in namespace %s", common.ResourceNameS3Driver, common.MarketplaceNamespace))
		r.setStatusInstallError(&req, s3driver, err)
		return ctrl.Result{Requeue: false}, err
	}

	if s3driver.Name != common.ResourceNameS3Driver {
		err := errors.NewBadRequest(fmt.Sprintf("Resource name should be %s", common.ResourceNameS3Driver))
		r.setStatusInstallError(&req, s3driver, err)
		return ctrl.Result{Requeue: false}, err
	}

	r.setUpAssetPlaceHolders(&req)
	ra := resourceapply.ResourceApply{
		Client:  r.Client,
		Context: ctx,
		Log:     log,
		Helper:  r.Helper,
		Owner:   asOwner(s3driver),
	}

	log.Info("Reconcile csidriver")
	_, err = ra.Apply(common.AssetPathCsiDriver)
	if err != nil {
		r.setStatusInstallError(&req, s3driver, err)
		return ctrl.Result{Requeue: false}, err
	}

	_, err = ra.Apply(common.AssetPathStorageClass)
	if err != nil {
		r.setStatusInstallError(&req, s3driver, err)
		return ctrl.Result{Requeue: false}, err
	}

	log.Info("Reconcile serviceaccount")
	_, err = ra.Apply(common.AssetPathRbacServiceAccount)
	if err != nil {
		r.setStatusInstallError(&req, s3driver, err)
		return ctrl.Result{Requeue: false}, err
	}

	log.Info("Reconcile csi clusterrole")
	_, err = ra.Apply(common.AssetPathRbacCsiClusterRole)
	if err != nil {
		r.setStatusInstallError(&req, s3driver, err)
		return ctrl.Result{Requeue: false}, err
	}

	log.Info("Reconcile csi clusterrolebinding")
	_, err = ra.Apply(common.AssetPathRbacCsiClusterRoleBinding)
	if err != nil {
		r.setStatusInstallError(&req, s3driver, err)
		return ctrl.Result{Requeue: false}, err
	}

	log.Info("Reconcile csi role")
	_, err = ra.Apply(common.AssetPathRbacCsiRole)
	if err != nil {
		r.setStatusInstallError(&req, s3driver, err)
		return ctrl.Result{Requeue: false}, err
	}

	log.Info("Reconcile csi rolebinding")
	_, err = ra.Apply(common.AssetPathRbacCsiRoleBinding)
	if err != nil {
		r.setStatusInstallError(&req, s3driver, err)
		return ctrl.Result{Requeue: false}, err
	}

	log.Info("Reconcile service")
	_, err = ra.Apply(common.AssetPathCsiService)
	if err != nil {
		r.setStatusInstallError(&req, s3driver, err)
		return ctrl.Result{Requeue: false}, err
	}

	log.Info("Reconcile csi node service daemonset")
	dmObj, err := ra.Apply(common.AssetPathCsiNodeService)
	if err != nil {
		r.setStatusInstallError(&req, s3driver, err)
		return ctrl.Result{}, err
	}
	daemonset := dmObj.(*appsv1.DaemonSet)

	log.Info("Reconcile csi reconcile service daemonset")
	dmObj2, err := ra.Apply(common.AssetPathCsiReconcileService)
	if err != nil {
		r.setStatusInstallError(&req, s3driver, err)
		return ctrl.Result{}, err
	}
	daemonset2 := dmObj2.(*appsv1.DaemonSet)

	log.Info("Reconcile csi controller service statefulset")
	ssObj, err := ra.Apply(common.AssetPathCsiControllerService)
	if err != nil {
		r.setStatusInstallError(&req, s3driver, err)
		return ctrl.Result{}, err
	}
	statefulset := ssObj.(*appsv1.StatefulSet)

	if daemonset.Status.NumberUnavailable > 0 || daemonset2.Status.NumberUnavailable > 0 || statefulset.Status.ReadyReplicas == 0 {
		change := s3driver.Status.Conditions.SetCondition(status.Condition{
			Type:    common.ConditionTypeInstall,
			Reason:  common.ReasonInstalling,
			Status:  corev1.ConditionFalse,
			Message: "Driver install in progress",
		})
		if change {
			err = r.Client.Status().Update(ctx, s3driver)
			if err != nil {
				log.Error(err, "failed to update status")
			}
		}
		return ctrl.Result{RequeueAfter: time.Second * 30}, nil
	}

	var isUpdateNeeded = false
	isUpdateNeeded = s3driver.Status.Conditions.SetCondition(status.Condition{
		Type:    common.ConditionTypeInstall,
		Reason:  common.ReasonInstallComplete,
		Status:  corev1.ConditionTrue,
		Message: "Driver install complete",
	})

	//validate credential
	credError := r.isCredentialValid(&req, s3driver)

	//driver pods version check
	change, hcInitiated, hcErr := r.checkDriverPods(&req, s3driver, credError == nil)
	isUpdateNeeded = isUpdateNeeded || change
	isUpdateNeeded = setDriverStatus(&s3driver.Status.Conditions, hcInitiated, credError, hcErr) || isUpdateNeeded

	if isUpdateNeeded {
		err = r.Client.Status().Update(ctx, s3driver)
		if err != nil {
			log.Error(err, "failed to update status")
		}
	}

	err = r.runPeriodicHealthCheck(&req, s3driver)
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: getResyncDuration(s3driver)}, nil
}

func (r *MarketplaceCSIDriverReconciler) SetupWithManager(mgr ctrl.Manager) error {
	ownerHandler := &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &marketplacev1alpha1.MarketplaceCSIDriver{},
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&marketplacev1alpha1.MarketplaceCSIDriver{}).
		Watches(&source.Kind{Type: &storagev1.CSIDriver{}}, ownerHandler).
		Watches(&source.Kind{Type: &storagev1.StorageClass{}}, ownerHandler).
		Watches(&source.Kind{Type: &corev1.Service{}}, ownerHandler).
		Watches(&source.Kind{Type: &corev1.ServiceAccount{}}, ownerHandler).
		Watches(&source.Kind{Type: &rbacv1.ClusterRole{}}, ownerHandler).
		Watches(&source.Kind{Type: &rbacv1.ClusterRoleBinding{}}, ownerHandler).
		Watches(&source.Kind{Type: &rbacv1.Role{}}, ownerHandler).
		Watches(&source.Kind{Type: &rbacv1.RoleBinding{}}, ownerHandler).
		Watches(&source.Kind{Type: &appsv1.DaemonSet{}}, ownerHandler).
		Watches(&source.Kind{Type: &appsv1.StatefulSet{}}, ownerHandler).
		Watches(&source.Kind{Type: &admbeta1.MutatingWebhookConfiguration{}}, ownerHandler).
		Complete(r)
}

func (r *MarketplaceCSIDriverReconciler) checkDriverPods(req *ctrl.Request, s3driver *marketplacev1alpha1.MarketplaceCSIDriver, isCredValid bool) (bool, bool, error) {
	log := r.Log.WithValues("Request.Namespace", req.Namespace, "Request.Name", req.Name)
	ctx := context.Background()

	selector, _ := labels.Parse(fmt.Sprintf("%s in (%s, %s, %s)", common.LabelApp, common.LabelAppNode,
		common.LabelAppReconcile, common.LabelAppRegister))

	podList := &corev1.PodList{}
	err := r.Client.List(ctx, podList, &client.ListOptions{
		Namespace:     common.MarketplaceNamespace,
		LabelSelector: selector,
	})
	if err != nil {
		log.Error(err, "Unable to get driver pods for health check")
		return false, false, err
	}

	nodesToFixMap := make(map[string]struct{})
	driverPods := make(map[string]marketplacev1alpha1.DriverPod)
	for _, pod := range podList.Items {
		created := pod.CreationTimestamp.Format(time.RFC3339)
		driverPods[pod.Name] = marketplacev1alpha1.DriverPod{
			NodeName:   pod.Spec.NodeName,
			CreateTime: created,
			Version:    pod.ResourceVersion,
		}
		val, ok := s3driver.Status.DriverPods[pod.Name]
		if !ok || val.CreateTime != created || val.Version != pod.ResourceVersion {
			nodesToFixMap[pod.Spec.NodeName] = struct{}{}
		}
	}

	isInitialCheck := len(s3driver.Status.DriverPods) == 0
	if isInitialCheck {
		ra := resourceapply.ResourceApply{
			Client:  r.Client,
			Context: ctx,
			Log:     log,
			Helper:  r.Helper,
			Owner:   metav1.OwnerReference{},
		}
		ra.ApplyOLMRbac()
	}
	repairNeeded := len(nodesToFixMap) > 0
	updateNeeded := repairNeeded || len(driverPods) != len(s3driver.Status.DriverPods)
	log.Info(fmt.Sprintf("Nodes to fix: %d, first check? %t, update needed?%t", len(nodesToFixMap), isInitialCheck, updateNeeded))
	s3driver.Status.DriverPods = driverPods
	if !isCredValid || !repairNeeded {
		return updateNeeded, false, nil
	}

	log.Info("Create driver check CR")
	nodesToFix := make([]string, 0, len(nodesToFixMap))
	for k := range nodesToFixMap {
		nodesToFix = append(nodesToFix, k)
	}

	err = r.createHealthCheckCR(ctx, s3driver, nodesToFix)
	if err != nil {
		log.Error(err, "Failed to create health check cr", "Namespace", s3driver.Namespace, "Name", common.NameNodeHealthCheck)
		return updateNeeded, false, err
	}
	log.Info("Created csi health check cr")
	return updateNeeded, true, nil
}

func (r *MarketplaceCSIDriverReconciler) isCredentialValid(req *ctrl.Request, s3driver *marketplacev1alpha1.MarketplaceCSIDriver) error {
	log := r.Log.WithValues("Request.Namespace", req.Namespace, "Request.Name", req.Name)
	ctx := context.Background()

	found := &corev1.Secret{}
	err := r.Client.Get(ctx, client.ObjectKey{Name: s3driver.Spec.Credential.Name, Namespace: common.MarketplaceNamespace}, found)
	if err != nil {
		credError := fmt.Errorf("Failed to retrieve secret %s/%s, error: %s", common.MarketplaceNamespace, s3driver.Spec.Credential.Name, err)
		log.Error(err, credError.Error())
		return credError
	}
	if checkCredentialFields(s3driver.Spec.Credential.Type, found) {
		return nil
	} else {
		return fmt.Errorf("Secret %s/%s is not valid", common.MarketplaceNamespace, s3driver.Spec.Credential.Name)
	}
}

func (r *MarketplaceCSIDriverReconciler) setStatusInstallError(req *ctrl.Request, s3driver *marketplacev1alpha1.MarketplaceCSIDriver, err error) {
	log := r.Log.WithValues("Request.Namespace", req.Namespace, "Request.Name", req.Name)
	ctx := context.Background()
	s3driver.Status.Conditions.SetCondition(status.Condition{
		Type:    common.ConditionTypeInstall,
		Reason:  common.ReasonInstallFailed,
		Status:  corev1.ConditionFalse,
		Message: err.Error(),
	})
	updError := r.Client.Status().Update(ctx, s3driver)
	if updError != nil {
		log.Error(updError, "failed to update status")
	}
}

func (r *MarketplaceCSIDriverReconciler) runPeriodicHealthCheck(req *ctrl.Request, s3driver *marketplacev1alpha1.MarketplaceCSIDriver) error {
	log := r.Log.WithValues("Request.Namespace", req.Namespace, "Request.Name", req.Name)
	if s3driver.Spec.HealthCheckIntervalInMinutes < 1 || time.Now().Sub(lastHealthCheckTime).Minutes() < float64(s3driver.Spec.HealthCheckIntervalInMinutes) {
		return nil
	}
	log.Info("Start health check")
	return r.createHealthCheckCR(context.Background(), s3driver, []string{""})
}

func (r *MarketplaceCSIDriverReconciler) createHealthCheckCR(ctx context.Context, s3driver *marketplacev1alpha1.MarketplaceCSIDriver, nodesToFix []string) error {
	dsCheck := &marketplacev1alpha1.MarketplaceDriverHealthCheck{}
	dsCheck.Namespace = common.MarketplaceNamespace
	dsCheck.GenerateName = common.NameNodeHealthCheck
	dsCheck.Spec.AffectedNodes = nodesToFix
	resourceapply.AddOwnerRefToObject(dsCheck, asOwner(s3driver))
	err := r.Create(ctx, dsCheck)
	if err == nil {
		lastHealthCheckTime = time.Now()
	}
	return err
}

func (r *MarketplaceCSIDriverReconciler) setUpAssetPlaceHolders(req *ctrl.Request) {
	log := r.Log.WithValues("Request.Namespace", req.Namespace, "Request.Name", req.Name)
	platformType, _ := r.Helper.GetCloudProviderType(r.Client, log)
	log.Info(fmt.Sprintf("Setting up asset place holders for platfrom: %s", platformType))
	if platformType == "ibmcloud" {
		resourceapply.AddAssetPlaceHolder("${KUBELET_ROOT}", "/var/data")
	} else {
		resourceapply.AddAssetPlaceHolder("${KUBELET_ROOT}", "/var/lib")
	}
}

func checkCredentialFields(credType string, secret *corev1.Secret) bool {
	if credType == "hmac" {
		_, ok1 := secret.Data[common.CredentialAccessKey]
		_, ok2 := secret.Data[common.CredentialSecretAccessKey]
		return ok1 && ok2
	}
	//rhm-pull-secret
	_, ok := secret.Data[common.CredentialPullSecret]
	return ok
}

func asOwner(v *marketplacev1alpha1.MarketplaceCSIDriver) metav1.OwnerReference {
	trueVar := true
	falseVar := false
	return metav1.OwnerReference{
		APIVersion:         marketplacev1alpha1.GroupVersion.String(),
		Kind:               v.Kind,
		Name:               v.Name,
		UID:                v.UID,
		Controller:         &trueVar,
		BlockOwnerDeletion: &falseVar,
	}
}

func setDriverStatus(conditions *status.Conditions, hcInitiated bool, credError, hcError error) bool {
	if credError != nil {
		return conditions.SetCondition(status.Condition{
			Type:    common.ConditionTypeDriver,
			Reason:  common.ReasonCredentialError,
			Status:  corev1.ConditionFalse,
			Message: credError.Error(),
		})
	}
	if hcError != nil {
		return conditions.SetCondition(status.Condition{
			Type:    common.ConditionTypeDriver,
			Reason:  common.ReasonHealthCheckFailed,
			Status:  corev1.ConditionFalse,
			Message: hcError.Error(),
		})
	}
	if hcInitiated {
		return conditions.SetCondition(status.Condition{
			Type:    common.ConditionTypeDriver,
			Reason:  common.ReasonRepairInitiated,
			Status:  corev1.ConditionTrue,
			Message: "Mounted volumes check and repair initiated",
		})
	}
	return conditions.SetCondition(status.Condition{
		Type:    common.ConditionTypeDriver,
		Reason:  common.ReasonOperational,
		Status:  corev1.ConditionTrue,
		Message: "Driver is operational",
	})
}

func getResyncDuration(s3driver *marketplacev1alpha1.MarketplaceCSIDriver) time.Duration {
	duration := time.Hour
	if s3driver.Spec.HealthCheckIntervalInMinutes > 0 && s3driver.Spec.HealthCheckIntervalInMinutes < 60 {
		duration = time.Duration(s3driver.Spec.HealthCheckIntervalInMinutes) * time.Minute
	}
	return duration
}
