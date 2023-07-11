/*
Copyright 2023 Chia Network Inc.
*/

package controller

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
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

// getChiaExporterContainer assembles a chia-exporter container spec
func getChiaExporterContainer(ctx context.Context, image string, secContext *corev1.SecurityContext, pullPolicy corev1.PullPolicy, resReq corev1.ResourceRequirements) corev1.Container {
	return corev1.Container{
		Name:            "chia-exporter",
		SecurityContext: secContext,
		Image:           image,
		ImagePullPolicy: pullPolicy,
		Env: []corev1.EnvVar{
			{
				Name:  "CHIA_ROOT",
				Value: "/chia-data",
			},
		},
		Ports: []corev1.ContainerPort{
			{
				Name:          "metrics",
				ContainerPort: chiaExporterPort,
				Protocol:      "TCP",
			},
		},
		LivenessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/healthz",
					Port: intstr.FromInt(chiaExporterPort),
				},
			},
		},
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/healthz",
					Port: intstr.FromInt(chiaExporterPort),
				},
			},
		},
		StartupProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/healthz",
					Port: intstr.FromInt(chiaExporterPort),
				},
			},
			FailureThreshold: 30,
			PeriodSeconds:    10,
		},
		Resources: resReq,
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "chiaroot",
				MountPath: "/chia-data",
			},
		},
	}
}
