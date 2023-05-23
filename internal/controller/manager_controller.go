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

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	batchv1 "github.com/envsecrets/k8/api/v1"
	secretsV1 "github.com/envsecrets/k8/api/v1"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
)

const (
	defaultRequeueDuration = time.Minute
)

// ManagerReconciler reconciles a EnvSecretsManager object
type ManagerReconciler struct {
	client.Client
	Log    logrus.Logger
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=batch.k8.envsecrets.dev,resources=managers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=batch.k8.envsecrets.dev,resources=managers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=batch.k8.envsecrets.dev,resources=managers/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the EnvSecretsManager object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.4/pkg/reconcile
func (r *ManagerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	ownNamespace, namespaceErr := GetOwnNamespace()
	if namespaceErr != nil {
		log.Error(namespaceErr, "Unable to load current namespace")
		return ctrl.Result{
			RequeueAfter: defaultRequeueDuration,
		}, nil
	}

	if ownNamespace != req.Namespace {
		log.Error(fmt.Errorf("cannot reconcile doppler secret (%v) in a namespace different from the operator (%v)", req.NamespacedName, ownNamespace), "")
		return ctrl.Result{}, nil
	}

	log.Info("Reconciling environment secrets")

	manager := secretsV1.EnvSecretsManager{}
	err := r.Client.Get(ctx, req.NamespacedName, &manager)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("[-] manager not found, nothing to do")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Unable to fetch manager")
		return ctrl.Result{
			RequeueAfter: defaultRequeueDuration,
		}, nil
	}

	requeueAfter := defaultRequeueDuration
	if manager.Spec.DelaySeconds != 0 {
		requeueAfter = time.Second * time.Duration(manager.Spec.DelaySeconds)
	}
	log.Info("Requeue duration set", "requeueAfter", requeueAfter)

	if manager.GetDeletionTimestamp() != nil {
		log.Info("manager has been deleted, nothing to do")
		return ctrl.Result{}, nil
	}

	err = r.UpdateSecret(ctx, manager)
	r.SetSecretsSyncReadyCondition(ctx, &manager, err)
	if err != nil {
		log.Error(err, "Unable to update secrets")
		return ctrl.Result{
			RequeueAfter: requeueAfter,
		}, nil
	}

	numDeployments, err := r.ReconcileDeploymentsUsingSecret(ctx, manager)
	r.SetDeploymentReloadReadyCondition(ctx, &manager, numDeployments, err)
	if err != nil {
		log.Error(err, "Failed to update deployments")
		return ctrl.Result{
			RequeueAfter: requeueAfter,
		}, nil
	}

	log.Info("Finished reconciliation")
	return ctrl.Result{
		RequeueAfter: requeueAfter,
	}, nil

}

// SetupWithManager sets up the controller with the EnvSecretsManager.
func (r *ManagerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&batchv1.EnvSecretsManager{}).
		Complete(r)
}
