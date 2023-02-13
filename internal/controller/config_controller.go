/*
Copyright 2023.

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

package controller

import (
	"context"
	"fmt"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	clustergardenerv1 "cluster.gardener/config/api/v1"
	"cluster.gardener/config/gardener"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ConfigReconciler reconciles a Config object
type ConfigReconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	SecretGenerator gardener.SecretGenerator
}

//+kubebuilder:rbac:groups=cluster.gardener,resources=configs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cluster.gardener,resources=configs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=cluster.gardener,resources=configs/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete

// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *ConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqLogger := log.FromContext(ctx)
	reqLogger.Info("Reconciling ArgoCDCluster Secret")

	//Get CRD config object
	argoConfig := &clustergardenerv1.Config{}

	err := r.Client.Get(ctx, req.NamespacedName, argoConfig)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	reqLogger.Info("Generate baseline ArgoCDCluster Config from CR")
	newConfig, err := r.SecretGenerator.GenerateSecret(&gardener.Input{
		S: argoConfig,
	})
	if err != nil {
		reqLogger.Error(err, "not able to generate secret")
		return ctrl.Result{}, err
	}

	// Finalizer
	finalizerName := "configs.cluster.gardener/finalizer"

	// examine DeletionTimestamp to determine if object is under deletion
	if argoConfig.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// registering our finalizer.
		if !controllerutil.ContainsFinalizer(argoConfig, finalizerName) {
			controllerutil.AddFinalizer(argoConfig, finalizerName)
			if err := r.Client.Update(ctx, argoConfig); err != nil {
				reqLogger.Info("Update Finalizer", req.NamespacedName)
				return ctrl.Result{}, client.IgnoreNotFound(err)
			}
		}
	} else {
		// The object is being deleted
		if controllerutil.ContainsFinalizer(argoConfig, finalizerName) {
			// our finalizer is present, so lets handle any external dependency
			if err := r.Client.Delete(ctx, newConfig); err != nil {
				reqLogger.Info("Delete Secret", req.NamespacedName)
				// if fail to delete the external dependency here, return with error
				// so that it can be retried
				return ctrl.Result{}, err
			}

			// remove finalizer from the list and update it.
			controllerutil.RemoveFinalizer(argoConfig, finalizerName)
			if err := r.Client.Update(ctx, argoConfig); err != nil {
				return ctrl.Result{}, err
			}
		}
		// Stop reconciliation as the item is being deleted
		return ctrl.Result{}, nil
	}

	secret := &v1.Secret{}
	var message string

	// Generate a new secret
	// Logic: if client.get produce error no secret is present
	// if the error is "not found" create a secret
	if err = r.Client.Get(ctx, types.NamespacedName{Namespace: req.Namespace, Name: argoConfig.Spec.Shoot}, secret); err != nil {
		if errors.IsNotFound(err) {
			message = fmt.Sprintf("Generate new ArgoCDCluster secret %s/%s", req.Namespace, argoConfig.Spec.Shoot)
			reqLogger.Info(message)
			if err = r.Client.Create(ctx, newConfig); err != nil {
				reqLogger.Info("unable to Create secret - try reconciling")
				return ctrl.Result{}, err
			}
			argoConfig.Status.Phase = "Created"
		} else {
			return ctrl.Result{}, err
		}
	} else {
		calculatedTime := ((argoConfig.Status.LastUpdatedTime).Add(argoConfig.Spec.Frequency.Duration)).Add(-1 * time.Minute)
		if calculatedTime.After(time.Now()) {
			reqLogger.Info(fmt.Sprintln(calculatedTime))
			reqLogger.Info(fmt.Sprintln(time.Now()))
			// update the secret
			message = fmt.Sprintf("Update config %s/%s", req.Namespace, argoConfig.Spec.Shoot)
			reqLogger.Info(message)
			secret.Data = newConfig.Data
			if err = r.Client.Update(ctx, secret); err != nil {
				return ctrl.Result{}, err
			}
			argoConfig.Status.Phase = "Updated"
			argoConfig.Status.LastUpdatedTime = &metav1.Time{Time: time.Now()}
		}
	}

	if err := r.Client.Status().Update(ctx, argoConfig); err != nil {
		if errors.IsConflict(err) {
			err := r.Client.Get(ctx, types.NamespacedName{Namespace: req.Namespace, Name: argoConfig.Spec.Shoot}, secret)
			if err != nil {
				reqLogger.Info("unable to get latest secret data - try reconciling")
				return ctrl.Result{}, err
			}
			r.Client.Status().Update(ctx, argoConfig)
		} else {
			reqLogger.Info("unable to update ArgoCDCluster secret status - try reconciling")
			return ctrl.Result{}, err
		}
	}
	message = fmt.Sprintf("RequeueAfter: %s", argoConfig.Spec.Frequency.Duration)
	reqLogger.Info(message)
	return ctrl.Result{RequeueAfter: argoConfig.Spec.Frequency.Duration}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&clustergardenerv1.Config{}).
		Complete(r)
}
