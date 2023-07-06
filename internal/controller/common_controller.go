/*
Copyright 2023 Chia Network Inc.
*/

package controller

import (
	"context"
)

const (
	// daemonPort defines the port for the Chia daemon
	daemonPort = 55400

	// chiaExporterPort defines the port for Chia Exporter instances
	chiaExporterPort = 9914
)

const (
	// defaultChiaExporterImage is the default image name and tag of the chia-exporter image
	defaultChiaExporterImage = "ghcr.io/chia-network/chia-exporter:latest"
)

// controllerOwner tells k8s objects that the CR that created it is its controller owner
var controllerOwner = true

// getCommonLabels gives some common labels for chia-operator related objects
func getCommonLabels(ctx context.Context, labels map[string]string) map[string]string {
	labels["app.kubernetes.io/name"] = "chia"
	labels["app.kubernetes.io/managed-by"] = "chia-operator"
	return labels
}
