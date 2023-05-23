package controller

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
)

func GetOwnNamespace() (string, error) {
	// Adapted from https://github.com/kubernetes/kubernetes/pull/63707

	// This way assumes you've set the POD_NAMESPACE environment variable using the downward API.
	// This check has to be done first for backwards compatibility with the way InClusterConfig was originally set up
	if ns, ok := os.LookupEnv("POD_NAMESPACE"); ok {
		return ns, nil
	}

	// Fall back to the namespace associated with the service account token, if available
	if data, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace"); err == nil {
		if ns := strings.TrimSpace(string(data)); len(ns) > 0 {
			return ns, nil
		}
		return "", fmt.Errorf("Failed to find current namespace for the operator: %w", err)
	} else {
		return "", err
	}
}
