/*
Copyright 2023 Chia Network Inc.
*/

package controller

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	k8schianetv1 "github.com/chia-network/chia-operator/api/v1"
	"github.com/cisco-open/operator-tools/pkg/reconciler"
)

const (
	// timelordPort defines the port for timelord
	timelordPort = 8446

	// timelordRPCPort defines the port for the timelord RPC
	timelordRPCPort = 8557
)

// ChiaTimelordReconciler reconciles a ChiaTimelord object
type ChiaTimelordReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=k8s.chia.net,resources=chiatimelords,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=k8s.chia.net,resources=chiatimelords/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=k8s.chia.net,resources=chiatimelords/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch
//+kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch

func (r *ChiaTimelordReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	resourceReconciler := reconciler.NewReconcilerWith(r.Client, reconciler.WithLog(log))
	log.Info(fmt.Sprintf("ChiaTimelordController ChiaTimelord=%s", req.NamespacedName.String()))

	// Get the custom resource
	var tl k8schianetv1.ChiaTimelord
	err := r.Get(ctx, req.NamespacedName, &tl)
	if err != nil && errors.IsNotFound(err) {
		// Return here, this can happen if the CR was deleted
		return ctrl.Result{}, nil
	}
	if err != nil {
		log.Error(err, fmt.Sprintf("ChiaTimelordController ChiaTimelord=%s unable to fetch ChiaTimelord resource", req.NamespacedName))
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Reconcile ChiaTimelord owned objects
	srv := r.assembleBaseService(ctx, tl)
	res, err := reconcileService(ctx, resourceReconciler, srv)
	if err != nil {
		if res == nil {
			res = &reconcile.Result{}
		}
		return *res, fmt.Errorf("ChiaTimelordController ChiaTimelord=%s encountered error reconciling node Service: %v", req.NamespacedName, err)
	}

	srv = r.assembleChiaExporterService(ctx, tl)
	res, err = reconcileService(ctx, resourceReconciler, srv)
	if err != nil {
		if res == nil {
			res = &reconcile.Result{}
		}
		return *res, fmt.Errorf("ChiaTimelordController ChiaTimelord=%s encountered error reconciling node chia-exporter Service: %v", req.NamespacedName, err)
	}

	deploy := r.assembleDeployment(ctx, tl)
	res, err = reconcileDeployment(ctx, resourceReconciler, deploy)
	if err != nil {
		if res == nil {
			res = &reconcile.Result{}
		}
		return *res, fmt.Errorf("ChiaTimelordController ChiaTimelord=%s encountered error reconciling node StatefulSet: %v", req.NamespacedName, err)
	}

	// Update CR status
	tl.Status.Ready = true
	err = r.Status().Update(ctx, &tl)
	if err != nil {
		log.Error(err, fmt.Sprintf("ChiaTimelordController ChiaTimelord=%s unable to update ChiaNode status", req.NamespacedName))
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ChiaTimelordReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&k8schianetv1.ChiaTimelord{}).
		Complete(r)
}

// assembleBaseService assembles the main Service resource for a Chiatl CR
func (r *ChiaTimelordReconciler) assembleBaseService(ctx context.Context, tl k8schianetv1.ChiaTimelord) corev1.Service {
	return corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:            fmt.Sprintf("%s-timelord", tl.Name),
			Namespace:       tl.Namespace,
			Labels:          r.getCommonLabels(ctx, tl, tl.Spec.AdditionalMetadata.Labels),
			Annotations:     tl.Spec.AdditionalMetadata.Annotations,
			OwnerReferences: r.getOwnerReference(ctx, tl),
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceType(tl.Spec.ServiceType),
			Ports: []corev1.ServicePort{
				{
					Port:       daemonPort,
					TargetPort: intstr.FromString("daemon"),
					Protocol:   "TCP",
					Name:       "daemon",
				},
				{
					Port:       timelordPort,
					TargetPort: intstr.FromString("peers"),
					Protocol:   "TCP",
					Name:       "peers",
				},
				{
					Port:       timelordRPCPort,
					TargetPort: intstr.FromString("rpc"),
					Protocol:   "TCP",
					Name:       "rpc",
				},
			},
			Selector: r.getCommonLabels(ctx, tl, tl.Spec.AdditionalMetadata.Labels),
		},
	}
}

// assembleChiaExporterService assembles the chia-exporter Service resource for a ChiaTimelord CR
func (r *ChiaTimelordReconciler) assembleChiaExporterService(ctx context.Context, tl k8schianetv1.ChiaTimelord) corev1.Service {
	return corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:            fmt.Sprintf("%s-timelord-metrics", tl.Name),
			Namespace:       tl.Namespace,
			Labels:          r.getCommonLabels(ctx, tl, tl.Spec.AdditionalMetadata.Labels, tl.Spec.ChiaExporterConfig.ServiceLabels),
			Annotations:     tl.Spec.AdditionalMetadata.Annotations,
			OwnerReferences: r.getOwnerReference(ctx, tl),
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceType("ClusterIP"),
			Ports: []corev1.ServicePort{
				{
					Port:       chiaExporterPort,
					TargetPort: intstr.FromString("metrics"),
					Protocol:   "TCP",
					Name:       "metrics",
				},
			},
			Selector: r.getCommonLabels(ctx, tl, tl.Spec.AdditionalMetadata.Labels),
		},
	}
}

// assembleDeployment assembles the tl Deployment resource for a ChiaTimelord CR
func (r *ChiaTimelordReconciler) assembleDeployment(ctx context.Context, tl k8schianetv1.ChiaTimelord) appsv1.Deployment {
	var chiaSecContext *corev1.SecurityContext
	if tl.Spec.ChiaConfig.SecurityContext != nil {
		chiaSecContext = tl.Spec.ChiaConfig.SecurityContext
	}

	var chiaLivenessProbe *corev1.Probe
	if tl.Spec.ChiaConfig.LivenessProbe != nil {
		chiaLivenessProbe = tl.Spec.ChiaConfig.LivenessProbe
	}

	var chiaReadinessProbe *corev1.Probe
	if tl.Spec.ChiaConfig.ReadinessProbe != nil {
		chiaReadinessProbe = tl.Spec.ChiaConfig.ReadinessProbe
	}

	var chiaStartupProbe *corev1.Probe
	if tl.Spec.ChiaConfig.StartupProbe != nil {
		chiaStartupProbe = tl.Spec.ChiaConfig.StartupProbe
	}

	var chiaResources corev1.ResourceRequirements
	if tl.Spec.ChiaConfig.Resources != nil {
		chiaResources = *tl.Spec.ChiaConfig.Resources
	}

	var imagePullPolicy corev1.PullPolicy
	if tl.Spec.ImagePullPolicy != nil {
		imagePullPolicy = *tl.Spec.ImagePullPolicy
	}

	var chiaExporterImage = tl.Spec.ChiaExporterConfig.Image
	if chiaExporterImage == "" {
		chiaExporterImage = defaultChiaExporterImage
	}

	var deploy appsv1.Deployment = appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:            fmt.Sprintf("%s-timelord", tl.Name),
			Namespace:       tl.Namespace,
			Labels:          r.getCommonLabels(ctx, tl, tl.Spec.AdditionalMetadata.Labels),
			Annotations:     tl.Spec.AdditionalMetadata.Annotations,
			OwnerReferences: r.getOwnerReference(ctx, tl),
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: r.getCommonLabels(ctx, tl),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      r.getCommonLabels(ctx, tl, tl.Spec.AdditionalMetadata.Labels),
					Annotations: tl.Spec.AdditionalMetadata.Annotations,
				},
				Spec: corev1.PodSpec{
					// TODO add: imagePullSecret, serviceAccountName config
					Containers: []corev1.Container{
						{
							Name:            "chia",
							SecurityContext: chiaSecContext,
							Image:           tl.Spec.ChiaConfig.Image,
							ImagePullPolicy: imagePullPolicy,
							Env:             r.getChiaEnv(ctx, tl),
							Ports: []corev1.ContainerPort{
								{
									Name:          "daemon",
									ContainerPort: daemonPort,
									Protocol:      "TCP",
								},
								{
									Name:          "peers",
									ContainerPort: timelordPort,
									Protocol:      "TCP",
								},
								{
									Name:          "rpc",
									ContainerPort: timelordRPCPort,
									Protocol:      "TCP",
								},
							},
							LivenessProbe:  chiaLivenessProbe,
							ReadinessProbe: chiaReadinessProbe,
							StartupProbe:   chiaStartupProbe,
							Resources:      chiaResources,
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "secret-ca",
									MountPath: "/chia-ca",
								},
								{
									Name:      "chiaroot",
									MountPath: "/chia-data",
								},
							},
						},
					},
					NodeSelector: tl.Spec.NodeSelector,
					Volumes:      r.getChiaVolumes(ctx, tl),
				},
			},
		},
	}

	exporterContainer := getChiaExporterContainer(ctx, chiaExporterImage, chiaSecContext, imagePullPolicy, chiaResources)
	deploy.Spec.Template.Spec.Containers = append(deploy.Spec.Template.Spec.Containers, exporterContainer)

	if tl.Spec.PodSecurityContext != nil {
		deploy.Spec.Template.Spec.SecurityContext = tl.Spec.PodSecurityContext
	}

	// TODO add pod affinity, tolerations

	return deploy
}

// getChiaVolumes retrieves the requisite volumes from the Chia config struct
func (r *ChiaTimelordReconciler) getChiaVolumes(ctx context.Context, tl k8schianetv1.ChiaTimelord) []corev1.Volume {
	var v []corev1.Volume

	// secret ca volume
	v = append(v, corev1.Volume{
		Name: "secret-ca",
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: tl.Spec.ChiaConfig.CASecretName,
			},
		},
	})

	// CHIA_ROOT volume -- PVC is respected first if both it and hostpath are specified, falls back to hostPath if specified
	// If both are empty, fall back to emptyDir so chia-exporter can mount CHIA_ROOT
	var chiaRootAdded bool = false
	if tl.Spec.Storage != nil && tl.Spec.Storage.ChiaRoot != nil {
		if tl.Spec.Storage.ChiaRoot.PersistentVolumeClaim != nil {
			v = append(v, corev1.Volume{
				Name: "chiaroot",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: tl.Spec.Storage.ChiaRoot.HostPathVolume.Path,
					},
				},
			})
			chiaRootAdded = true
		} else if tl.Spec.Storage.ChiaRoot.HostPathVolume != nil {
			v = append(v, corev1.Volume{
				Name: "chiaroot",
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: tl.Spec.Storage.ChiaRoot.HostPathVolume.Path,
					},
				},
			})
			chiaRootAdded = true
		}
	}
	if !chiaRootAdded {
		v = append(v, corev1.Volume{
			Name: "chiaroot",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		})
	}

	return v
}

// getChiaEnv retrieves the environment variables from the Chia config struct
func (r *ChiaTimelordReconciler) getChiaEnv(ctx context.Context, tl k8schianetv1.ChiaTimelord) []corev1.EnvVar {
	var env []corev1.EnvVar

	// service env var
	env = append(env, corev1.EnvVar{
		Name:  "service",
		Value: "timelord-only timelord-launcher-only",
	})

	// CHIA_ROOT env var
	env = append(env, corev1.EnvVar{
		Name:  "CHIA_ROOT",
		Value: "/chia-data",
	})

	// ca env var
	env = append(env, corev1.EnvVar{
		Name:  "ca",
		Value: "/chia-ca",
	})

	// testnet env var
	if tl.Spec.ChiaConfig.Testnet != nil && *tl.Spec.ChiaConfig.Testnet {
		env = append(env, corev1.EnvVar{
			Name:  "testnet",
			Value: "true",
		})
	}

	// TZ env var
	if tl.Spec.ChiaConfig.Timezone != nil {
		env = append(env, corev1.EnvVar{
			Name:  "TZ",
			Value: *tl.Spec.ChiaConfig.Timezone,
		})
	}

	// log_level env var
	if tl.Spec.ChiaConfig.LogLevel != nil {
		env = append(env, corev1.EnvVar{
			Name:  "log_level",
			Value: *tl.Spec.ChiaConfig.LogLevel,
		})
	}

	// node peer env var
	env = append(env, corev1.EnvVar{
		Name:  "full_node_peer",
		Value: tl.Spec.ChiaConfig.FullNodePeer,
	})

	return env
}

// getCommonLabels gives some common labels for ChiaTimelord related objects
func (r *ChiaTimelordReconciler) getCommonLabels(ctx context.Context, tl k8schianetv1.ChiaTimelord, additionalLabels ...map[string]string) map[string]string {
	var labels = make(map[string]string)
	for _, addition := range additionalLabels {
		for k, v := range addition {
			labels[k] = v
		}
	}
	labels["app.kubernetes.io/instance"] = tl.Name
	labels["chiatimelord-owner"] = tl.Name
	labels = getCommonLabels(ctx, labels)
	return labels
}

// getOwnerReference gives the common owner reference spec for ChiaTimelord related objects
func (r *ChiaTimelordReconciler) getOwnerReference(ctx context.Context, tl k8schianetv1.ChiaTimelord) []metav1.OwnerReference {
	return []metav1.OwnerReference{
		{
			APIVersion: tl.APIVersion,
			Kind:       tl.Kind,
			Name:       tl.Name,
			UID:        tl.UID,
			Controller: &controllerOwner,
		},
	}
}
