/*
Copyright 2023 Chia Network Inc.
*/

package controller

import (
	"context"
	"fmt"
	"strconv"

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
	// harvesterPort defines the port for harvester instances
	harvesterPort = 8448

	// harvesterRPCPort defines the port for the harvester RPC
	harvesterRPCPort = 8560
)

// ChiaHarvesterReconciler reconciles a ChiaHarvester object
type ChiaHarvesterReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=k8s.chia.net,resources=ChiaHarvesters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=k8s.chia.net,resources=ChiaHarvesters/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=k8s.chia.net,resources=ChiaHarvesters/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch
//+kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch

// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.4/pkg/reconcile
func (r *ChiaHarvesterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	resourceReconciler := reconciler.NewReconcilerWith(r.Client, reconciler.WithLog(log))
	log.Info(fmt.Sprintf("ChiaHarvesterReconciler ChiaHarvester=%s", req.NamespacedName.String()))

	// Get the custom resource
	var harvester k8schianetv1.ChiaHarvester
	err := r.Get(ctx, req.NamespacedName, &harvester)
	if err != nil && errors.IsNotFound(err) {
		// Return here, this can happen if the CR was deleted
		return ctrl.Result{}, nil
	}
	if err != nil {
		log.Error(err, fmt.Sprintf("ChiaHarvesterReconciler ChiaHarvester=%s unable to fetch ChiaHarvester resource", req.NamespacedName))
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Reconcile ChiaHarvester owned objects
	res, err := r.reconcileBaseService(ctx, resourceReconciler, harvester)
	if err != nil {
		if res == nil {
			res = &reconcile.Result{}
		}
		return *res, fmt.Errorf("ChiaHarvesterReconciler ChiaHarvester=%s encountered error reconciling harvester Service: %v", req.NamespacedName, err)
	}

	res, err = r.reconcileChiaExporterService(ctx, resourceReconciler, harvester)
	if err != nil {
		if res == nil {
			res = &reconcile.Result{}
		}
		return *res, fmt.Errorf("ChiaHarvesterReconciler ChiaHarvester=%s encountered error reconciling harvester chia-exporter Service: %v", req.NamespacedName, err)
	}

	res, err = r.reconcileDeployment(ctx, resourceReconciler, harvester)
	if err != nil {
		if res == nil {
			res = &reconcile.Result{}
		}
		return *res, fmt.Errorf("ChiaHarvesterReconciler ChiaHarvester=%s encountered error reconciling harvester Deployment: %v", req.NamespacedName, err)
	}

	// Update CR status
	harvester.Status.Ready = true
	err = r.Status().Update(ctx, &harvester)
	if err != nil {
		log.Error(err, fmt.Sprintf("ChiaHarvesterReconciler ChiaHarvester=%s unable to update ChiaHarvester status", req.NamespacedName))
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ChiaHarvesterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&k8schianetv1.ChiaHarvester{}).
		Complete(r)
}

// reconcileBaseService reconciles the main Service resource for a ChiaHarvester CR
func (r *ChiaHarvesterReconciler) reconcileBaseService(ctx context.Context, rec reconciler.ResourceReconciler, harvester k8schianetv1.ChiaHarvester) (*reconcile.Result, error) {
	var service corev1.Service = corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:            fmt.Sprintf("%s-harvester", harvester.Name),
			Namespace:       harvester.Namespace,
			Labels:          r.getCommonLabels(ctx, harvester, harvester.Spec.AdditionalMetadata.Labels),
			Annotations:     harvester.Spec.AdditionalMetadata.Annotations,
			OwnerReferences: r.getOwnerReference(ctx, harvester),
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceType(harvester.Spec.ServiceType),
			Ports: []corev1.ServicePort{
				{
					Port:       daemonPort,
					TargetPort: intstr.FromString("daemon"),
					Protocol:   "TCP",
					Name:       "daemon",
				},
				{
					Port:       harvesterPort,
					TargetPort: intstr.FromString("peers"),
					Protocol:   "TCP",
					Name:       "peers",
				},
				{
					Port:       harvesterRPCPort,
					TargetPort: intstr.FromString("rpc"),
					Protocol:   "TCP",
					Name:       "rpc",
				},
			},
			Selector: r.getCommonLabels(ctx, harvester, harvester.Spec.AdditionalMetadata.Labels),
		},
	}

	return rec.ReconcileResource(&service, reconciler.StatePresent)
}

// reconcileChiaExporterService reconciles the chia-exporter Service resource for a ChiaHarvester CR
func (r *ChiaHarvesterReconciler) reconcileChiaExporterService(ctx context.Context, rec reconciler.ResourceReconciler, harvester k8schianetv1.ChiaHarvester) (*reconcile.Result, error) {
	var service corev1.Service = corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:            fmt.Sprintf("%s-harvester-metrics", harvester.Name),
			Namespace:       harvester.Namespace,
			Labels:          r.getCommonLabels(ctx, harvester, harvester.Spec.AdditionalMetadata.Labels, harvester.Spec.ChiaExporterConfig.ServiceLabels),
			Annotations:     harvester.Spec.AdditionalMetadata.Annotations,
			OwnerReferences: r.getOwnerReference(ctx, harvester),
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
			Selector: r.getCommonLabels(ctx, harvester, harvester.Spec.AdditionalMetadata.Labels),
		},
	}

	return rec.ReconcileResource(&service, reconciler.StatePresent)
}

// reconcileDeployment reconciles the harvester Deployment resource for a ChiaHarvester CR
func (r *ChiaHarvesterReconciler) reconcileDeployment(ctx context.Context, rec reconciler.ResourceReconciler, harvester k8schianetv1.ChiaHarvester) (*reconcile.Result, error) {
	var chiaSecContext *corev1.SecurityContext
	if harvester.Spec.ChiaConfig.SecurityContext != nil {
		chiaSecContext = harvester.Spec.ChiaConfig.SecurityContext
	}

	var chiaLivenessProbe *corev1.Probe
	if harvester.Spec.ChiaConfig.LivenessProbe != nil {
		chiaLivenessProbe = harvester.Spec.ChiaConfig.LivenessProbe
	}

	var chiaReadinessProbe *corev1.Probe
	if harvester.Spec.ChiaConfig.ReadinessProbe != nil {
		chiaReadinessProbe = harvester.Spec.ChiaConfig.ReadinessProbe
	}

	var chiaStartupProbe *corev1.Probe
	if harvester.Spec.ChiaConfig.StartupProbe != nil {
		chiaStartupProbe = harvester.Spec.ChiaConfig.StartupProbe
	}

	var chiaResources corev1.ResourceRequirements
	if harvester.Spec.ChiaConfig.Resources != nil {
		chiaResources = *harvester.Spec.ChiaConfig.Resources
	}

	var imagePullPolicy corev1.PullPolicy
	if harvester.Spec.ImagePullPolicy != nil {
		imagePullPolicy = *harvester.Spec.ImagePullPolicy
	}

	var chiaExporterImage = harvester.Spec.ChiaExporterConfig.Image
	if chiaExporterImage == "" {
		chiaExporterImage = "ghcr.io/chia-network/chia-exporter:latest"
	}

	var deploy appsv1.Deployment = appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:            fmt.Sprintf("%s-harvester", harvester.Name),
			Namespace:       harvester.Namespace,
			Labels:          r.getCommonLabels(ctx, harvester, harvester.Spec.AdditionalMetadata.Labels),
			Annotations:     harvester.Spec.AdditionalMetadata.Annotations,
			OwnerReferences: r.getOwnerReference(ctx, harvester),
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: r.getCommonLabels(ctx, harvester),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      r.getCommonLabels(ctx, harvester, harvester.Spec.AdditionalMetadata.Labels),
					Annotations: harvester.Spec.AdditionalMetadata.Annotations,
				},
				Spec: corev1.PodSpec{
					// TODO add: imagePullSecret, serviceAccountName config
					Containers: []corev1.Container{
						{
							Name:            "chia",
							SecurityContext: chiaSecContext,
							Image:           harvester.Spec.ChiaConfig.Image,
							ImagePullPolicy: imagePullPolicy,
							Env:             r.getChiaEnv(ctx, harvester),
							Ports: []corev1.ContainerPort{
								{
									Name:          "daemon",
									ContainerPort: daemonPort,
									Protocol:      "TCP",
								},
								{
									Name:          "peers",
									ContainerPort: harvesterPort,
									Protocol:      "TCP",
								},
								{
									Name:          "rpc",
									ContainerPort: harvesterRPCPort,
									Protocol:      "TCP",
								},
							},
							LivenessProbe:  chiaLivenessProbe,
							ReadinessProbe: chiaReadinessProbe,
							StartupProbe:   chiaStartupProbe,
							Resources:      chiaResources,
							VolumeMounts:   r.getChiaVolumeMounts(ctx, harvester),
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
					NodeSelector: harvester.Spec.NodeSelector,
					Volumes:      r.getChiaVolumes(ctx, harvester),
				},
			},
		},
	}

	if harvester.Spec.PodSecurityContext != nil {
		deploy.Spec.Template.Spec.SecurityContext = harvester.Spec.PodSecurityContext
	}

	// TODO add pod affinity, tolerations

	return rec.ReconcileResource(&deploy, reconciler.StatePresent)
}

// getChiaVolumes retrieves the requisite volumes from the Chia config struct
func (r *ChiaHarvesterReconciler) getChiaVolumes(ctx context.Context, harvester k8schianetv1.ChiaHarvester) []corev1.Volume {
	var v []corev1.Volume

	// secret ca volume
	v = append(v, corev1.Volume{
		Name: "secret-ca",
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: harvester.Spec.ChiaConfig.CASecretName,
			},
		},
	})

	// CHIA_ROOT volume -- PVC is respected first if both it and hostpath are specified, falls back to hostPath if specified
	// If both are empty, fall back to emptyDir so chia-exporter can mount CHIA_ROOT
	var chiaRootAdded bool = false
	if harvester.Spec.Storage != nil && harvester.Spec.Storage.ChiaRoot != nil {
		if harvester.Spec.Storage.ChiaRoot.PersistentVolumeClaim != nil {
			v = append(v, corev1.Volume{
				Name: "chiaroot",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: harvester.Spec.Storage.ChiaRoot.HostPathVolume.Path,
					},
				},
			})
			chiaRootAdded = true
		} else if harvester.Spec.Storage.ChiaRoot.HostPathVolume != nil {
			v = append(v, corev1.Volume{
				Name: "chiaroot",
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: harvester.Spec.Storage.ChiaRoot.HostPathVolume.Path,
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

	// hostPath and PVC plot volumes
	if harvester.Spec.Storage != nil {
		if harvester.Spec.Storage.Plots != nil {
			// PVC plot volumes
			if harvester.Spec.Storage.Plots.PersistentVolumeClaim != nil {
				for i, vol := range harvester.Spec.Storage.Plots.PersistentVolumeClaim {
					if vol != nil {
						v = append(v, corev1.Volume{
							Name: fmt.Sprintf("pvc-plots-%d", i),
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: vol.ClaimName,
								},
							},
						})
					}
				}
			}

			// hostPath plot volumes
			if harvester.Spec.Storage.Plots.HostPathVolume != nil {
				for i, vol := range harvester.Spec.Storage.Plots.HostPathVolume {
					if vol != nil {
						v = append(v, corev1.Volume{
							Name: fmt.Sprintf("hostpath-plots-%d", i),
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: vol.Path,
								},
							},
						})
					}
				}
			}
		}
	}

	return v
}

// getChiaVolumeMounts retrieves the requisite volume mounts from the Chia config struct
func (r *ChiaHarvesterReconciler) getChiaVolumeMounts(ctx context.Context, harvester k8schianetv1.ChiaHarvester) []corev1.VolumeMount {
	var v []corev1.VolumeMount

	// secret ca volume
	v = append(v, corev1.VolumeMount{
		Name:      "secret-ca",
		MountPath: "/chia-ca",
	})

	// CHIA_ROOT volume
	v = append(v, corev1.VolumeMount{
		Name:      "chiaroot",
		MountPath: "/chia-data",
	})

	// hostPath and PVC plot volumemounts
	if harvester.Spec.Storage != nil {
		if harvester.Spec.Storage.Plots != nil {
			// PVC plot volume mounts
			if harvester.Spec.Storage.Plots.PersistentVolumeClaim != nil {
				for i, vol := range harvester.Spec.Storage.Plots.PersistentVolumeClaim {
					if vol != nil {
						v = append(v, corev1.VolumeMount{
							Name:      fmt.Sprintf("pvc-plots-%d", i),
							ReadOnly:  true,
							MountPath: fmt.Sprintf("/plots/pvc-plots-%d", i),
						})
					}
				}
			}

			// hostPath plot volume mounts
			if harvester.Spec.Storage.Plots.HostPathVolume != nil {
				for i, vol := range harvester.Spec.Storage.Plots.HostPathVolume {
					if vol != nil {
						v = append(v, corev1.VolumeMount{
							Name:      fmt.Sprintf("hostpath-plots-%d", i),
							ReadOnly:  true,
							MountPath: fmt.Sprintf("/plots/hostpath-plots-%d", i),
						})
					}
				}
			}
		}
	}

	return v
}

// getChiaEnv retrieves the environment variables from the Chia config struct
func (r *ChiaHarvesterReconciler) getChiaEnv(ctx context.Context, harvester k8schianetv1.ChiaHarvester) []corev1.EnvVar {
	var env []corev1.EnvVar

	// service env var
	env = append(env, corev1.EnvVar{
		Name:  "service",
		Value: "harvester",
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
	if harvester.Spec.ChiaConfig.Testnet != nil && *harvester.Spec.ChiaConfig.Testnet {
		env = append(env, corev1.EnvVar{
			Name:  "testnet",
			Value: "true",
		})
	}

	// TZ env var
	if harvester.Spec.ChiaConfig.Timezone != nil {
		env = append(env, corev1.EnvVar{
			Name:  "TZ",
			Value: *harvester.Spec.ChiaConfig.Timezone,
		})
	}

	// recursive_plot_scan env var -- needed because all plot drives are just mounted as subdirs under `/plots`.
	// TODO make plot mount paths configurable -- make this var optional
	env = append(env, corev1.EnvVar{
		Name:  "recursive_plot_scan",
		Value: "true",
	})

	// farmer peer env vars
	env = append(env, corev1.EnvVar{
		Name:  "farmer_address",
		Value: harvester.Spec.ChiaConfig.FarmerAddress,
	})
	env = append(env, corev1.EnvVar{
		Name:  "farmer_port",
		Value: strconv.Itoa(farmerPort),
	})

	return env
}

// getCommonLabels gives some common labels for ChiaHarvester related objects
func (r *ChiaHarvesterReconciler) getCommonLabels(ctx context.Context, harvester k8schianetv1.ChiaHarvester, additionalLabels ...map[string]string) map[string]string {
	var labels = make(map[string]string)
	for _, addition := range additionalLabels {
		for k, v := range addition {
			labels[k] = v
		}
	}
	labels["app.kubernetes.io/name"] = "chia"
	labels["app.kubernetes.io/managed-by"] = "chia-operator"
	labels["app.kubernetes.io/instance"] = harvester.Name
	labels["ChiaHarvester-owner"] = harvester.Name
	return labels
}

// getOwnerReference gives the common owner reference spec for ChiaHarvester related objects
func (r *ChiaHarvesterReconciler) getOwnerReference(ctx context.Context, harvester k8schianetv1.ChiaHarvester) []metav1.OwnerReference {
	return []metav1.OwnerReference{
		{
			APIVersion: harvester.APIVersion,
			Kind:       harvester.Kind,
			Name:       harvester.Name,
			UID:        harvester.UID,
			Controller: &controllerOwner,
		},
	}
}
