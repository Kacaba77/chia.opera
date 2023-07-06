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
	// farmerPort defines the port for farmer instances
	farmerPort = 8447

	// farmerRPCPort defines the port for the farmer RPC
	farmerRPCPort = 8559
)

// ChiaFarmerReconciler reconciles a ChiaFarmer object
type ChiaFarmerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=k8s.chia.net,resources=chiafarmers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=k8s.chia.net,resources=chiafarmers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=k8s.chia.net,resources=chiafarmers/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch
//+kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch

// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.4/pkg/reconcile
func (r *ChiaFarmerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	resourceReconciler := reconciler.NewReconcilerWith(r.Client, reconciler.WithLog(log))
	log.Info(fmt.Sprintf("ChiaFarmerReconciler ChiaFarmer=%s", req.NamespacedName.String()))

	// Get the custom resource
	var farmer k8schianetv1.ChiaFarmer
	err := r.Get(ctx, req.NamespacedName, &farmer)
	if err != nil && errors.IsNotFound(err) {
		// Return here, this can happen if the CR was deleted
		return ctrl.Result{}, nil
	}
	if err != nil {
		log.Error(err, fmt.Sprintf("ChiaFarmerReconciler ChiaFarmer=%s unable to fetch ChiaFarmer resource", req.NamespacedName))
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Reconcile ChiaFarmer owned objects
	res, err := r.reconcileBaseService(ctx, resourceReconciler, farmer)
	if err != nil {
		if res == nil {
			res = &reconcile.Result{}
		}
		return *res, fmt.Errorf("ChiaFarmerReconciler ChiaFarmer=%s encountered error reconciling farmer Service: %v", req.NamespacedName, err)
	}

	res, err = r.reconcileChiaExporterService(ctx, resourceReconciler, farmer)
	if err != nil {
		if res == nil {
			res = &reconcile.Result{}
		}
		return *res, fmt.Errorf("ChiaFarmerReconciler ChiaFarmer=%s encountered error reconciling farmer chia-exporter Service: %v", req.NamespacedName, err)
	}

	res, err = r.reconcileDeployment(ctx, resourceReconciler, farmer)
	if err != nil {
		if res == nil {
			res = &reconcile.Result{}
		}
		return *res, fmt.Errorf("ChiaFarmerReconciler ChiaFarmer=%s encountered error reconciling farmer Deployment: %v", req.NamespacedName, err)
	}

	// Update CR status
	farmer.Status.Ready = true
	err = r.Status().Update(ctx, &farmer)
	if err != nil {
		log.Error(err, fmt.Sprintf("ChiaFarmerReconciler ChiaFarmer=%s unable to update ChiaFarmer status", req.NamespacedName))
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ChiaFarmerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&k8schianetv1.ChiaFarmer{}).
		Complete(r)
}

// reconcileBaseService reconciles the main Service resource for a Chiafarmer CR
func (r *ChiaFarmerReconciler) reconcileBaseService(ctx context.Context, rec reconciler.ResourceReconciler, farmer k8schianetv1.ChiaFarmer) (*reconcile.Result, error) {
	var service corev1.Service = corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:            fmt.Sprintf("%s-farmer", farmer.Name),
			Namespace:       farmer.Namespace,
			Labels:          getCommonLabels(ctx, "chiafarmer", farmer.Name, farmer.Spec.AdditionalMetadata.Labels),
			Annotations:     farmer.Spec.AdditionalMetadata.Annotations,
			OwnerReferences: r.getOwnerReference(ctx, farmer),
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceType(farmer.Spec.ServiceType),
			Ports: []corev1.ServicePort{
				{
					Port:       daemonPort,
					TargetPort: intstr.FromString("daemon"),
					Protocol:   "TCP",
					Name:       "daemon",
				},
				{
					Port:       farmerPort,
					TargetPort: intstr.FromString("peers"),
					Protocol:   "TCP",
					Name:       "peers",
				},
				{
					Port:       farmerRPCPort,
					TargetPort: intstr.FromString("rpc"),
					Protocol:   "TCP",
					Name:       "rpc",
				},
			},
			Selector: getCommonLabels(ctx, "chiafarmer", farmer.Name, farmer.Spec.AdditionalMetadata.Labels),
		},
	}

	return rec.ReconcileResource(&service, reconciler.StatePresent)
}

// reconcileChiaExporterService reconciles the chia-exporter Service resource for a ChiaFarmer CR
func (r *ChiaFarmerReconciler) reconcileChiaExporterService(ctx context.Context, rec reconciler.ResourceReconciler, farmer k8schianetv1.ChiaFarmer) (*reconcile.Result, error) {
	var service corev1.Service = corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:            fmt.Sprintf("%s-farmer-metrics", farmer.Name),
			Namespace:       farmer.Namespace,
			Labels:          getCommonLabels(ctx, "chiafarmer", farmer.Name, farmer.Spec.AdditionalMetadata.Labels, farmer.Spec.ChiaExporterConfig.ServiceLabels),
			Annotations:     farmer.Spec.AdditionalMetadata.Annotations,
			OwnerReferences: r.getOwnerReference(ctx, farmer),
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
			Selector: getCommonLabels(ctx, "chiafarmer", farmer.Name, farmer.Spec.AdditionalMetadata.Labels),
		},
	}

	return rec.ReconcileResource(&service, reconciler.StatePresent)
}

// reconcileDeployment reconciles the farmer Deployment resource for a ChiaFarmer CR
func (r *ChiaFarmerReconciler) reconcileDeployment(ctx context.Context, rec reconciler.ResourceReconciler, farmer k8schianetv1.ChiaFarmer) (*reconcile.Result, error) {
	var chiaSecContext *corev1.SecurityContext
	if farmer.Spec.ChiaConfig.SecurityContext != nil {
		chiaSecContext = farmer.Spec.ChiaConfig.SecurityContext
	}

	var chiaLivenessProbe *corev1.Probe
	if farmer.Spec.ChiaConfig.LivenessProbe != nil {
		chiaLivenessProbe = farmer.Spec.ChiaConfig.LivenessProbe
	}

	var chiaReadinessProbe *corev1.Probe
	if farmer.Spec.ChiaConfig.ReadinessProbe != nil {
		chiaReadinessProbe = farmer.Spec.ChiaConfig.ReadinessProbe
	}

	var chiaStartupProbe *corev1.Probe
	if farmer.Spec.ChiaConfig.StartupProbe != nil {
		chiaStartupProbe = farmer.Spec.ChiaConfig.StartupProbe
	}

	var chiaResources corev1.ResourceRequirements
	if farmer.Spec.ChiaConfig.Resources != nil {
		chiaResources = *farmer.Spec.ChiaConfig.Resources
	}

	var imagePullPolicy corev1.PullPolicy
	if farmer.Spec.ImagePullPolicy != nil {
		imagePullPolicy = *farmer.Spec.ImagePullPolicy
	}

	var chiaExporterImage = farmer.Spec.ChiaExporterConfig.Image
	if chiaExporterImage == "" {
		chiaExporterImage = "ghcr.io/chia-network/chia-exporter:latest"
	}

	var deploy appsv1.Deployment = appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:            fmt.Sprintf("%s-farmer", farmer.Name),
			Namespace:       farmer.Namespace,
			Labels:          getCommonLabels(ctx, "chiafarmer", farmer.Name, farmer.Spec.AdditionalMetadata.Labels),
			Annotations:     farmer.Spec.AdditionalMetadata.Annotations,
			OwnerReferences: r.getOwnerReference(ctx, farmer),
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: getCommonLabels(ctx, "chiafarmer", farmer.Name),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      getCommonLabels(ctx, "chiafarmer", farmer.Name, farmer.Spec.AdditionalMetadata.Labels),
					Annotations: farmer.Spec.AdditionalMetadata.Annotations,
				},
				Spec: corev1.PodSpec{
					// TODO add: imagePullSecret, serviceAccountName config
					Containers: []corev1.Container{
						{
							Name:            "chia",
							SecurityContext: chiaSecContext,
							Image:           farmer.Spec.ChiaConfig.Image,
							ImagePullPolicy: imagePullPolicy,
							Env:             r.getChiaEnv(ctx, farmer),
							Ports: []corev1.ContainerPort{
								{
									Name:          "daemon",
									ContainerPort: daemonPort,
									Protocol:      "TCP",
								},
								{
									Name:          "peers",
									ContainerPort: farmerPort,
									Protocol:      "TCP",
								},
								{
									Name:          "rpc",
									ContainerPort: farmerRPCPort,
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
									Name:      "key",
									MountPath: "/key",
								},
								{
									Name:      "chiaroot",
									MountPath: "/chia-data",
								},
							},
						},
						{
							Name:            "chia-exporter",
							SecurityContext: chiaSecContext,
							Image:           chiaExporterImage,
							ImagePullPolicy: imagePullPolicy,
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
							Resources: chiaResources,
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "chiaroot",
									MountPath: "/chia-data",
								},
							},
						},
					},
					NodeSelector: farmer.Spec.NodeSelector,
					Volumes:      r.getChiaVolumes(ctx, farmer),
				},
			},
		},
	}

	if farmer.Spec.PodSecurityContext != nil {
		deploy.Spec.Template.Spec.SecurityContext = farmer.Spec.PodSecurityContext
	}

	// TODO add pod affinity, tolerations

	return rec.ReconcileResource(&deploy, reconciler.StatePresent)
}

// getChiaVolumes retrieves the requisite volumes from the Chia config struct
func (r *ChiaFarmerReconciler) getChiaVolumes(ctx context.Context, farmer k8schianetv1.ChiaFarmer) []corev1.Volume {
	var v []corev1.Volume

	// secret ca volume
	v = append(v, corev1.Volume{
		Name: "secret-ca",
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: farmer.Spec.ChiaConfig.CASecretName,
			},
		},
	})

	// mnemonic key volume
	v = append(v, corev1.Volume{
		Name: "key",
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: farmer.Spec.ChiaConfig.SecretKeySpec.Name,
			},
		},
	})

	// CHIA_ROOT volume -- PVC is respected first if both it and hostpath are specified, falls back to hostPath if specified
	// If both are empty, fall back to emptyDir so chia-exporter can mount CHIA_ROOT
	var chiaRootAdded bool = false
	if farmer.Spec.Storage != nil && farmer.Spec.Storage.ChiaRoot != nil {
		if farmer.Spec.Storage.ChiaRoot.PersistentVolumeClaim != nil {
			v = append(v, corev1.Volume{
				Name: "chiaroot",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: farmer.Spec.Storage.ChiaRoot.HostPathVolume.Path,
					},
				},
			})
			chiaRootAdded = true
		} else if farmer.Spec.Storage.ChiaRoot.HostPathVolume != nil {
			v = append(v, corev1.Volume{
				Name: "chiaroot",
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: farmer.Spec.Storage.ChiaRoot.HostPathVolume.Path,
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
func (r *ChiaFarmerReconciler) getChiaEnv(ctx context.Context, farmer k8schianetv1.ChiaFarmer) []corev1.EnvVar {
	var env []corev1.EnvVar

	// service env var
	env = append(env, corev1.EnvVar{
		Name:  "service",
		Value: "farmer-only",
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
	if farmer.Spec.ChiaConfig.Testnet != nil && *farmer.Spec.ChiaConfig.Testnet {
		env = append(env, corev1.EnvVar{
			Name:  "testnet",
			Value: "true",
		})
	}

	// TZ env var
	if farmer.Spec.ChiaConfig.Timezone != nil {
		env = append(env, corev1.EnvVar{
			Name:  "TZ",
			Value: *farmer.Spec.ChiaConfig.Timezone,
		})
	}

	// keys env var
	env = append(env, corev1.EnvVar{
		Name:  "keys",
		Value: fmt.Sprintf("/key/%s", farmer.Spec.ChiaConfig.SecretKeySpec.Key),
	})

	// node peer env var
	env = append(env, corev1.EnvVar{
		Name:  "full_node_peer",
		Value: farmer.Spec.ChiaConfig.FullNodePeer,
	})

	return env
}

// getOwnerReference gives the common owner reference spec for ChiaFarmer related objects
func (r *ChiaFarmerReconciler) getOwnerReference(ctx context.Context, farmer k8schianetv1.ChiaFarmer) []metav1.OwnerReference {
	return []metav1.OwnerReference{
		{
			APIVersion: farmer.APIVersion,
			Kind:       farmer.Kind,
			Name:       farmer.Name,
			UID:        farmer.UID,
			Controller: &controllerOwner,
		},
	}
}
