package controller

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

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"sigs.k8s.io/controller-runtime/pkg/log"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	secretsv1 "github.com/envsecrets/k8/api/v1"
)

const (
	kubeSecretVersionAnnotation           = "k8.envsecrets.com/version"
	kubeSecretProcessorsVersionAnnotation = "k8.envsecrets.com/processor-version"
	kubeSecretFormatVersionAnnotation     = "k8.envsecrets.com/format"
	kubeSecretServiceTokenKey             = "envToken"
)

type Header string

const (

	//	Standard HTTP headers
	AuthorizationHeader Header = "Authorization"
	ContentTypeHeader   Header = "Content-Type"
	TokenHeader         Header = "x-envsecrets-token"
)

type APICredentials struct {
	Host      string
	APIKey    string
	VerifyTLS bool
}

type GetValuesOptions struct {
	EnvID   string
	Token   string
	Key     *string
	Version *int
}

type GetResponse struct {
	Data    map[string]Payload `json:"data"`
	Version *int               `json:"version,omitempty"`
}

type Type string

const (
	Plaintext  Type = "plaintext"
	Ciphertext Type = "ciphertext"
)

type Payload struct {

	//	Base 64 encoded value.
	Value interface{} `json:"value,omitempty"`

	//	Plaintext or Ciphertext
	Type Type `json:"type,omitempty"`
}

func (p *Payload) Map() map[string]interface{} {
	return map[string]interface{}{
		"value": p.Value,
		"type":  p.Type,
	}
}

type APIResponse struct {
	HTTPResponse *http.Response
	Body         []byte
}

type APIError struct {
	Err     error
	Message string
}

// GetAPICredentials generates an APICredentials from a Manager
func GetAPICredentials(manager secretsv1.Manager, token string) *APICredentials {
	return &APICredentials{
		Host: manager.Spec.Host,
		//VerifyTLS: Manager.Spec.VerifyTLS,
		APIKey: token,
	}
}

// GetReferencedSecret gets a Kubernetes secret from a SecretReference
func (r *ManagerReconciler) GetReferencedSecret(ctx context.Context, ref secretsv1.SecretReference) (*corev1.Secret, error) {
	kubeSecretNamespacedName := types.NamespacedName{
		Namespace: ref.Namespace,
		Name:      ref.Name,
	}
	existingKubeSecret := &corev1.Secret{}
	err := r.Client.Get(ctx, kubeSecretNamespacedName, existingKubeSecret)
	if err != nil {
		existingKubeSecret = nil
	}
	return existingKubeSecret, err
}

// GetEnviromentToken gets the envsecrets enviroment token referenced by the Manager
func (r *ManagerReconciler) GetEnviromentToken(ctx context.Context, Manager secretsv1.Manager) (string, error) {
	tokenSecret, err := r.GetReferencedSecret(ctx, Manager.Spec.EnvTokenRef)
	if err != nil {
		return "", fmt.Errorf("Failed to fetch token secret reference: %w", err)
	}
	token := tokenSecret.Data[kubeSecretServiceTokenKey]
	if token == nil {
		return "", fmt.Errorf("Could not find secret key %s.%s", Manager.Spec.EnvTokenRef.Name, kubeSecretServiceTokenKey)
	}
	return string(token), nil
}

// GetKubeSecretData generates Kube secret data from a Doppler API secrets result
func GetKubeSecretData(data map[string]Payload) (result map[string][]byte, err error) {
	for key, payload := range data {

		//	Base64 decode the value
		value, err := base64.StdEncoding.DecodeString(payload.Value.(string))
		if err != nil {
			return nil, err
		}
		result[key] = value
	}
	return
}

// CreateManagedSecret creates a managed Kubernetes secret
func (r *ManagerReconciler) CreateManagedSecret(ctx context.Context, Manager secretsv1.Manager, result map[string]Payload) error {
	secretData, dataErr := GetKubeSecretData(result)
	if dataErr != nil {
		return fmt.Errorf("Failed to build Kubernetes secret data: %w", dataErr)
	}

	newKubeSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      Manager.Spec.ManagedSecretRef.Name,
			Namespace: Manager.Spec.ManagedSecretRef.Namespace,
			Labels: map[string]string{
				"k8.envsecrets.com/subtype": "Manager",
			},
		},
		Type: "Opaque",
		Data: secretData,
	}
	err := r.Client.Create(ctx, newKubeSecret)
	if err != nil {
		return fmt.Errorf("Failed to create Kubernetes secret: %w", err)
	}
	r.Log.Info("[/] Successfully created new Kubernetes secret")
	return nil
}

// UpdateManagedSecret updates a managed Kubernetes secret
func (r *ManagerReconciler) UpdateManagedSecret(ctx context.Context, secret corev1.Secret, Manager secretsv1.Manager, result map[string]Payload) error {
	secretData, dataErr := GetKubeSecretData(result)
	if dataErr != nil {
		return fmt.Errorf("Failed to build Kubernetes secret data: %w", dataErr)
	}
	secret.Data = secretData
	err := r.Client.Update(ctx, &secret)
	if err != nil {
		return fmt.Errorf("Failed to update Kubernetes secret: %w", err)
	}
	r.Log.Info("[/] Successfully updated existing Kubernetes secret")
	return nil
}

// UpdateSecret updates a Kubernetes secret using the configuration specified in a Manager
func (r *ManagerReconciler) UpdateSecret(ctx context.Context, Manager secretsv1.Manager) error {
	log := log.FromContext(ctx)

	log.Info("Updating Kubernetes secret")
	if Manager.Spec.ManagedSecretRef.Namespace == "" {
		Manager.Spec.ManagedSecretRef.Namespace = Manager.Namespace
	}
	if Manager.Spec.EnvTokenRef.Namespace == "" {
		Manager.Spec.EnvTokenRef.Namespace = Manager.Namespace
	}

	token, err := r.GetEnviromentToken(ctx, Manager)
	if err != nil {
		return fmt.Errorf("Failed to load envsecrets enviroment Token: %w", err)
	}

	existingKubeSecret, err := r.GetReferencedSecret(ctx, Manager.Spec.ManagedSecretRef)
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("Failed to fetch managed secret reference: %w", err)
	}

	log.Info("Fetching secrets from server")

	result, apiErr := GetValues(ctx, GetAPICredentials(Manager, token), &GetValuesOptions{})
	if apiErr != nil {
		return apiErr
	}

	log.Info("[/] Secrets have been updated with version ", result.Version)

	if existingKubeSecret == nil {
		return r.CreateManagedSecret(ctx, Manager, result.Data)
	} else {
		return r.UpdateManagedSecret(ctx, *existingKubeSecret, Manager, result.Data)
	}
}

func GetValues(ctx context.Context, credentials *APICredentials, options *GetValuesOptions) (*GetResponse, error) {

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, credentials.Host+"/v1/secrets/values", nil)
	if err != nil {
		return nil, err
	}

	//	Initialize the query values.
	query := req.URL.Query()

	if options.Key != nil {
		query.Set("key", *options.Key)
	}

	if options.Version != nil {
		query.Set("version", fmt.Sprint(*options.Key))
	}

	//	If the environment token is passed,
	//	set it in the request header.
	if options.Token != "" {
		req.Header.Set(string(TokenHeader), options.Token)
	} else {

		//	Set the environment ID in query.
		query.Set("env_id", options.EnvID)
	}

	req.URL.RawQuery = query.Encode()

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result GetResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	return &result, nil
}
