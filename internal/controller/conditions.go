package controller

import (
	"context"
	"fmt"

	secretsv1 "github.com/envsecrets/k8/api/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *ManagerReconciler) SetSecretsSyncReadyCondition(ctx context.Context, manager *secretsv1.EnvSecretsManager, updateSecretsError error) {
	log := log.FromContext(ctx)
	if manager.Status.Conditions == nil {
		manager.Status.Conditions = []metav1.Condition{}
	}
	if updateSecretsError == nil {
		meta.SetStatusCondition(&manager.Status.Conditions, metav1.Condition{
			Type:    "k8.envsecrets.com/SecretSyncReady",
			Status:  metav1.ConditionTrue,
			Reason:  "OK",
			Message: "Controller is continuously syncing secrets",
		})
	} else {
		meta.SetStatusCondition(&manager.Status.Conditions, metav1.Condition{
			Type:    "k8.envsecrets.com/SecretSyncReady",
			Status:  metav1.ConditionFalse,
			Reason:  "Error",
			Message: fmt.Sprintf("Secret update failed: %v", updateSecretsError),
		})
		meta.SetStatusCondition(&manager.Status.Conditions, metav1.Condition{
			Type:    "k8.envsecrets.com/DeploymentReloadReady",
			Status:  metav1.ConditionFalse,
			Reason:  "Stopped",
			Message: "Deployment reload has been stopped due to secrets sync failure",
		})
	}
	err := r.Client.Status().Update(ctx, manager)
	if err != nil {
		log.Error(err, "Unable to set update secret condition")
	}
}

func (r *ManagerReconciler) SetDeploymentReloadReadyCondition(ctx context.Context, manager *secretsv1.EnvSecretsManager, numDeployments int, deploymentError error) {
	log := log.FromContext(ctx)
	if manager.Status.Conditions == nil {
		manager.Status.Conditions = []metav1.Condition{}
	}
	if deploymentError == nil {
		meta.SetStatusCondition(&manager.Status.Conditions, metav1.Condition{
			Type:    "k8.envsecrets.com/DeploymentReloadReady",
			Status:  metav1.ConditionTrue,
			Reason:  "OK",
			Message: fmt.Sprintf("Controller is ready to reload deployments. %v found.", numDeployments),
		})
	} else {
		meta.SetStatusCondition(&manager.Status.Conditions, metav1.Condition{
			Type:    "k8.envsecrets.com/DeploymentReloadReady",
			Status:  metav1.ConditionFalse,
			Reason:  "Error",
			Message: fmt.Sprintf("Deployment reconcile failed: %v", deploymentError),
		})
	}
	err := r.Client.Status().Update(ctx, manager)
	if err != nil {
		log.Error(err, "Unable to set reconcile deployments condition")
	}
}
