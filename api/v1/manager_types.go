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

package v1

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ManagerSpec defines the desired state of EnvSecretsManager
type ManagerSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// The Kubernetes secret containing the envsecrets environment token
	EnvTokenRef SecretReference `json:"envToken,omitempty"`

	// The Kubernetes secret where the operator will store and sync the fetched secrets
	ManagedSecretRef SecretReference `json:"managedSecret,omitempty"`

	// The envsecrets API host
	// +kubebuilder:default="https://envsecrets-3dizc5e3rq-el.a.run.app"
	Host string `json:"host,omitempty"`

	// The number of seconds to wait before the next fetch request
	// +kubebuilder:default=60
	DelaySeconds int64 `json:"delaySeconds,omitempty"`
}

// A reference to a Kubernetes secret
type SecretReference struct {
	// The name of the Secret resource
	Name string `json:"name"`

	// Namespace of the resource being referred to. Ignored if not cluster scoped
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// ManagerStatus defines the observed state of EnvSecretsManager
type ManagerStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Information when was the last time the job was successfully scheduled.
	// +optional
	LastRun *metav1.Time `json:"lastRun,omitempty"`

	Conditions []metav1.Condition `json:"conditions"`
}

// SecretStatus defines the observed state of our secrets EnvSecretsManager
type SecretStatus struct {
	Conditions []metav1.Condition `json:"conditions"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// EnvSecretsManager is the Schema for the managers API
type EnvSecretsManager struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ManagerSpec   `json:"spec,omitempty"`
	Status ManagerStatus `json:"status,omitempty"`
}

func (m EnvSecretsManager) GetNamespacedName() string {
	return fmt.Sprintf("%s/%s", m.Namespace, m.Name)
}

//+kubebuilder:object:root=true

// ManagerList contains a list of EnvSecretsManager
type ManagerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []EnvSecretsManager `json:"items"`
}

func init() {
	SchemeBuilder.Register(&EnvSecretsManager{}, &ManagerList{})
}
