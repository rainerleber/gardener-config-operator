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
	argoCrConfig := &clustergardenerv1.Config{}

	err := r.Client.Get(ctx, req.NamespacedName, argoCrConfig)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	reqLogger.Info("Generate baseline ArgoCDCluster Config from CR")

	referenceSecret := &v1.Secret{}
	var message string

	// Generate a new secret
	// Logic: if client.get produce error no secret is present
	// if the error is "not found" create a secret
	if err = r.Client.Get(ctx, types.NamespacedName{Namespace: req.Namespace, Name: argoCrConfig.Spec.Shoot}, referenceSecret); err != nil {
		if errors.IsNotFound(err) {

			// Generate new Secret with Token
			newSecret, err := r.SecretGenerator.GenerateSecret(&gardener.Input{
				S: argoCrConfig,
			})
			if err != nil {
				reqLogger.Error(err, "not able to generate secret")
				return ctrl.Result{}, err
			}

			message = fmt.Sprintf("Generate new ArgoCDCluster secret %s/%s", req.Namespace, argoCrConfig.Spec.Shoot)
			reqLogger.Info(message)
			if err = r.Client.Create(ctx, newSecret); err != nil {
				reqLogger.Info("unable to Create secret - try reconciling")
				return ctrl.Result{}, err
			}
			argoCrConfig.Status.Phase = "Created"
			argoCrConfig.Status.LastUpdatedTime = &metav1.Time{Time: time.Now()}
		} else {
			return ctrl.Result{}, err
		}
	} else {
		// update the secret
		timeNow := &metav1.Time{Time: time.Now().Add(time.Duration(+1) * time.Minute)}
		nextReconiling := argoCrConfig.Status.LastUpdatedTime.Add(argoCrConfig.Spec.Frequency.Duration)
		if timeNow.After(nextReconiling) {
			message = fmt.Sprintf("Update config %s/%s", req.Namespace, argoCrConfig.Spec.Shoot)
			reqLogger.Info(message)

			// Generate new Secret with Token
			newSecret, err := r.SecretGenerator.GenerateSecret(&gardener.Input{
				S: argoCrConfig,
			})
			if err != nil {
				reqLogger.Error(err, "not able to generate secret")
				return ctrl.Result{}, err
			}

			referenceSecret.Data = newSecret.Data
			if err = r.Client.Update(ctx, referenceSecret); err != nil {
				return ctrl.Result{}, err
			}
			argoCrConfig.Status.Phase = "Updated"
			argoCrConfig.Status.LastUpdatedTime = timeNow
		}
	}

	// Finalizer
	finalizerName := "configs.cluster.gardener/finalizer"

	// examine DeletionTimestamp to determine if object is under deletion
	if argoCrConfig.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// registering our finalizer.
		if !controllerutil.ContainsFinalizer(argoCrConfig, finalizerName) {
			controllerutil.AddFinalizer(argoCrConfig, finalizerName)
			if err := r.Client.Update(ctx, argoCrConfig); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else if controllerutil.ContainsFinalizer(argoCrConfig, finalizerName) {
		// The object is being deleted
		// our finalizer is present, so lets handle any external dependency
		err := r.Client.Delete(ctx, referenceSecret)
		if err != nil && !errors.IsNotFound(err) {
			// if it fail because an other reason then not present to delete the external
			// dependency here, return with error so that it can be retried
			return ctrl.Result{}, err
		}

		// remove finalizer from the list and update it.
		controllerutil.RemoveFinalizer(argoCrConfig, finalizerName)
		if err := r.Client.Update(ctx, argoCrConfig); err != nil {
			return ctrl.Result{}, err
		}
	}

	if err := r.Client.Status().Update(ctx, argoCrConfig); err != nil {
		reqLogger.Info("unable to update ArgoCDCluster secret status - try reconciling")
		return ctrl.Result{}, err
	}

	message = fmt.Sprintf("RequeueAfter: %s", argoCrConfig.Spec.Frequency.Duration)
	reqLogger.Info(message)
	return ctrl.Result{RequeueAfter: argoCrConfig.Spec.Frequency.Duration}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&clustergardenerv1.Config{}).
		Complete(r)
}
