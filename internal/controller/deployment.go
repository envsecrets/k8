/*
Copyright 2021.

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
	"sync"

	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	secretsv1 "github.com/envsecrets/k8/api/v1"
)

const (
	deploymentSecretUpdateAnnotationPrefix = "k8.envsecrets.com/secretsupdate"
	deploymentRestartAnnotation            = "k8.envsecrets.com/reload"
)

// Reconciles deployments marked with the restart annotation and that use the specified EnvSecretsManager.
func (r *ManagerReconciler) ReconcileDeploymentsUsingSecret(ctx context.Context, manager secretsv1.EnvSecretsManager) (int, error) {
	log := log.FromContext(ctx)
	namespace := manager.Namespace
	if manager.Spec.ManagedSecretRef.Namespace != "" {
		namespace = manager.Spec.ManagedSecretRef.Namespace
	}
	deploymentList := &v1.DeploymentList{}
	err := r.Client.List(ctx, deploymentList, &client.ListOptions{Namespace: namespace})
	if err != nil {
		return 0, fmt.Errorf("Unable to fetch deployments: %w", err)
	}
	kubeSecretNamespacedName := types.NamespacedName{
		Namespace: namespace,
		Name:      manager.Spec.ManagedSecretRef.Name,
	}
	kubeSecret := &corev1.Secret{}
	err = r.Client.Get(ctx, kubeSecretNamespacedName, kubeSecret)
	if err != nil {
		return 0, fmt.Errorf("Unable to fetch Kubernetes secret to update deployment: %w", err)
	}
	var wg sync.WaitGroup
	for _, deployment := range deploymentList.Items {
		if deployment.Annotations[deploymentRestartAnnotation] == "true" && r.IsDeploymentUsingSecret(deployment, manager) {
			wg.Add(1)
			go func(deployment v1.Deployment, kubeSecret corev1.Secret, wg *sync.WaitGroup) {
				defer wg.Done()
				err := r.ReconcileDeployment(ctx, deployment, kubeSecret)
				if err != nil {
					// Errors reconciling deployments are logged but not propagated up. Failed deployments will be reconciled on the next run.
					log.Error(err, "Unable to reconcile deployment")
				}
			}(deployment, *kubeSecret, &wg)
		}
	}
	wg.Wait()

	log.Info("Finished reconciling deployments", "numDeployments", len(deploymentList.Items))

	return len(deploymentList.Items), nil
}

// Evaluates whether or not the deployment is using the specified EnvSecretsManager.
// Specifically, a deployment is using a EnvSecretsManager if it references it using `envFrom`, `secretKeyRef` or `volumes`.
func (r *ManagerReconciler) IsDeploymentUsingSecret(deployment v1.Deployment, manager secretsv1.EnvSecretsManager) bool {
	managedSecretName := manager.Spec.ManagedSecretRef.Name
	for _, container := range deployment.Spec.Template.Spec.Containers {
		for _, envFrom := range container.EnvFrom {
			if envFrom.SecretRef != nil && envFrom.SecretRef.LocalObjectReference.Name == managedSecretName {
				return true
			}
		}
		for _, env := range container.Env {
			if env.ValueFrom != nil && env.ValueFrom.SecretKeyRef != nil && env.ValueFrom.SecretKeyRef.LocalObjectReference.Name == managedSecretName {
				return true
			}
		}
	}
	for _, volume := range deployment.Spec.Template.Spec.Volumes {
		if volume.Secret != nil && volume.Secret.SecretName == managedSecretName {
			return true
		}
	}

	return false
}

// Reconciles a deployment with a Kubernetes secret
// Specifically, if the Kubernetes secret version is different from the deployment's secret version annotation,
// the annotation is updated to restart the deployment.
func (r *ManagerReconciler) ReconcileDeployment(ctx context.Context, deployment v1.Deployment, secret corev1.Secret) error {
	log := log.FromContext(ctx)
	annotationKey := fmt.Sprintf("%s.%s", deploymentSecretUpdateAnnotationPrefix, secret.Name)
	annotationValue := secret.Annotations[kubeSecretVersionAnnotation]
	if deployment.Annotations[annotationKey] == annotationValue &&
		deployment.Spec.Template.Annotations[annotationKey] == annotationValue {
		log.Info("[-] Deployment is already running latest version, nothing to do")
		return nil
	}
	deployment.Annotations[annotationKey] = annotationValue
	if deployment.Spec.Template.Annotations == nil {
		deployment.Spec.Template.Annotations = make(map[string]string)
	}
	deployment.Spec.Template.Annotations[annotationKey] = annotationValue
	err := r.Client.Update(ctx, &deployment)
	if err != nil {
		return fmt.Errorf("Failed to update deployment annotation: %w", err)
	}
	log.Info("[/] Updated deployment")
	return nil
}
