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
	// walletPort defines the port for wallet instances
	walletPort = 8449

	// walletRPCPort defines the port for the wallet RPC
	walletRPCPort = 9256
)

// ChiaWalletReconciler reconciles a ChiaWallet object
type ChiaWalletReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=k8s.chia.net,resources=chiawallets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=k8s.chia.net,resources=chiawallets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=k8s.chia.net,resources=chiawallets/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch
//+kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch

// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.15.0/pkg/reconcile
func (r *ChiaWalletReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	resourceReconciler := reconciler.NewReconcilerWith(r.Client, reconciler.WithLog(log))
	log.Info(fmt.Sprintf("ChiaWalletReconciler ChiaWallet=%s", req.NamespacedName.String()))

	// Get the custom resource
	var wallet k8schianetv1.ChiaWallet
	err := r.Get(ctx, req.NamespacedName, &wallet)
	if err != nil && errors.IsNotFound(err) {
		// Return here, this can happen if the CR was deleted
		return ctrl.Result{}, nil
	}
	if err != nil {
		log.Error(err, fmt.Sprintf("ChiaWalletReconciler ChiaWallet=%s unable to fetch ChiaWallet resource", req.NamespacedName))
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Reconcile ChiaWallet owned objects
	service := r.assembleBaseService(ctx, wallet)
	res, err := reconcileService(ctx, resourceReconciler, service)
	if err != nil {
		if res == nil {
			res = &reconcile.Result{}
		}
		return *res, fmt.Errorf("ChiaWalletReconciler ChiaWallet=%s encountered error reconciling wallet Service: %v", req.NamespacedName, err)
	}

	service = r.assembleChiaExporterService(ctx, wallet)
	res, err = reconcileService(ctx, resourceReconciler, service)
	if err != nil {
		if res == nil {
			res = &reconcile.Result{}
		}
		return *res, fmt.Errorf("ChiaWalletReconciler ChiaWallet=%s encountered error reconciling wallet chia-exporter Service: %v", req.NamespacedName, err)
	}

	deploy := r.assembleDeployment(ctx, wallet)
	res, err = reconcileDeployment(ctx, resourceReconciler, deploy)
	if err != nil {
		if res == nil {
			res = &reconcile.Result{}
		}
		return *res, fmt.Errorf("ChiaWalletReconciler ChiaWallet=%s encountered error reconciling wallet Deployment: %v", req.NamespacedName, err)
	}

	// Update CR status
	wallet.Status.Ready = true
	err = r.Status().Update(ctx, &wallet)
	if err != nil {
		log.Error(err, fmt.Sprintf("ChiaWalletReconciler ChiaWallet=%s unable to update ChiaWallet status", req.NamespacedName))
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ChiaWalletReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&k8schianetv1.ChiaWallet{}).
		Complete(r)
}

// reconcileBaseService reconciles the main Service resource for a ChiaWallet CR
func (r *ChiaWalletReconciler) assembleBaseService(ctx context.Context, wallet k8schianetv1.ChiaWallet) corev1.Service {
	return corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:            fmt.Sprintf("%s-wallet", wallet.Name),
			Namespace:       wallet.Namespace,
			Labels:          r.getCommonLabels(ctx, wallet, wallet.Spec.AdditionalMetadata.Labels),
			Annotations:     wallet.Spec.AdditionalMetadata.Annotations,
			OwnerReferences: r.getOwnerReference(ctx, wallet),
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceType(wallet.Spec.ServiceType),
			Ports: []corev1.ServicePort{
				{
					Port:       daemonPort,
					TargetPort: intstr.FromString("daemon"),
					Protocol:   "TCP",
					Name:       "daemon",
				},
				{
					Port:       walletPort,
					TargetPort: intstr.FromString("peers"),
					Protocol:   "TCP",
					Name:       "peers",
				},
				{
					Port:       walletRPCPort,
					TargetPort: intstr.FromString("rpc"),
					Protocol:   "TCP",
					Name:       "rpc",
				},
			},
			Selector: r.getCommonLabels(ctx, wallet, wallet.Spec.AdditionalMetadata.Labels),
		},
	}
}

// assembleChiaExporterService assembles the chia-exporter Service resource for a ChiaWallet CR
func (r *ChiaWalletReconciler) assembleChiaExporterService(ctx context.Context, wallet k8schianetv1.ChiaWallet) corev1.Service {
	return corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:            fmt.Sprintf("%s-wallet-metrics", wallet.Name),
			Namespace:       wallet.Namespace,
			Labels:          r.getCommonLabels(ctx, wallet, wallet.Spec.AdditionalMetadata.Labels, wallet.Spec.ChiaExporterConfig.ServiceLabels),
			Annotations:     wallet.Spec.AdditionalMetadata.Annotations,
			OwnerReferences: r.getOwnerReference(ctx, wallet),
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
			Selector: r.getCommonLabels(ctx, wallet, wallet.Spec.AdditionalMetadata.Labels),
		},
	}
}

// assembleDeployment reconciles the wallet Deployment resource for a ChiaWallet CR
func (r *ChiaWalletReconciler) assembleDeployment(ctx context.Context, wallet k8schianetv1.ChiaWallet) appsv1.Deployment {
	var chiaSecContext *corev1.SecurityContext
	if wallet.Spec.ChiaConfig.SecurityContext != nil {
		chiaSecContext = wallet.Spec.ChiaConfig.SecurityContext
	}

	var chiaLivenessProbe *corev1.Probe
	if wallet.Spec.ChiaConfig.LivenessProbe != nil {
		chiaLivenessProbe = wallet.Spec.ChiaConfig.LivenessProbe
	}

	var chiaReadinessProbe *corev1.Probe
	if wallet.Spec.ChiaConfig.ReadinessProbe != nil {
		chiaReadinessProbe = wallet.Spec.ChiaConfig.ReadinessProbe
	}

	var chiaStartupProbe *corev1.Probe
	if wallet.Spec.ChiaConfig.StartupProbe != nil {
		chiaStartupProbe = wallet.Spec.ChiaConfig.StartupProbe
	}

	var chiaResources corev1.ResourceRequirements
	if wallet.Spec.ChiaConfig.Resources != nil {
		chiaResources = *wallet.Spec.ChiaConfig.Resources
	}

	var imagePullPolicy corev1.PullPolicy
	if wallet.Spec.ImagePullPolicy != nil {
		imagePullPolicy = *wallet.Spec.ImagePullPolicy
	}

	var chiaExporterImage = wallet.Spec.ChiaExporterConfig.Image
	if chiaExporterImage == "" {
		chiaExporterImage = defaultChiaExporterImage
	}

	var deploy appsv1.Deployment = appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:            fmt.Sprintf("%s-wallet", wallet.Name),
			Namespace:       wallet.Namespace,
			Labels:          r.getCommonLabels(ctx, wallet, wallet.Spec.AdditionalMetadata.Labels),
			Annotations:     wallet.Spec.AdditionalMetadata.Annotations,
			OwnerReferences: r.getOwnerReference(ctx, wallet),
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: r.getCommonLabels(ctx, wallet),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      r.getCommonLabels(ctx, wallet, wallet.Spec.AdditionalMetadata.Labels),
					Annotations: wallet.Spec.AdditionalMetadata.Annotations,
				},
				Spec: corev1.PodSpec{
					// TODO add: imagePullSecret, serviceAccountName config
					Containers: []corev1.Container{
						{
							Name:            "chia",
							SecurityContext: chiaSecContext,
							Image:           wallet.Spec.ChiaConfig.Image,
							ImagePullPolicy: imagePullPolicy,
							Env:             r.getChiaEnv(ctx, wallet),
							Ports: []corev1.ContainerPort{
								{
									Name:          "daemon",
									ContainerPort: daemonPort,
									Protocol:      "TCP",
								},
								{
									Name:          "peers",
									ContainerPort: walletPort,
									Protocol:      "TCP",
								},
								{
									Name:          "rpc",
									ContainerPort: walletRPCPort,
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
					},
					NodeSelector: wallet.Spec.NodeSelector,
					Volumes:      r.getChiaVolumes(ctx, wallet),
				},
			},
		},
	}

	exporterContainer := getChiaExporterContainer(ctx, chiaExporterImage, chiaSecContext, imagePullPolicy, chiaResources)
	deploy.Spec.Template.Spec.Containers = append(deploy.Spec.Template.Spec.Containers, exporterContainer)

	if wallet.Spec.PodSecurityContext != nil {
		deploy.Spec.Template.Spec.SecurityContext = wallet.Spec.PodSecurityContext
	}

	// TODO add pod affinity, tolerations

	return deploy
}

// getChiaVolumes retrieves the requisite volumes from the Chia config struct
func (r *ChiaWalletReconciler) getChiaVolumes(ctx context.Context, wallet k8schianetv1.ChiaWallet) []corev1.Volume {
	var v []corev1.Volume

	// secret ca volume
	v = append(v, corev1.Volume{
		Name: "secret-ca",
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: wallet.Spec.ChiaConfig.CASecretName,
			},
		},
	})

	// mnemonic key volume
	v = append(v, corev1.Volume{
		Name: "key",
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: wallet.Spec.ChiaConfig.SecretKeySpec.Name,
			},
		},
	})

	// CHIA_ROOT volume -- PVC is respected first if both it and hostpath are specified, falls back to hostPath if specified
	// If both are empty, fall back to emptyDir so chia-exporter can mount CHIA_ROOT
	var chiaRootAdded bool = false
	if wallet.Spec.Storage != nil && wallet.Spec.Storage.ChiaRoot != nil {
		if wallet.Spec.Storage.ChiaRoot.PersistentVolumeClaim != nil {
			v = append(v, corev1.Volume{
				Name: "chiaroot",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: wallet.Spec.Storage.ChiaRoot.HostPathVolume.Path,
					},
				},
			})
			chiaRootAdded = true
		} else if wallet.Spec.Storage.ChiaRoot.HostPathVolume != nil {
			v = append(v, corev1.Volume{
				Name: "chiaroot",
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: wallet.Spec.Storage.ChiaRoot.HostPathVolume.Path,
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
func (r *ChiaWalletReconciler) getChiaEnv(ctx context.Context, wallet k8schianetv1.ChiaWallet) []corev1.EnvVar {
	var env []corev1.EnvVar

	// service env var
	env = append(env, corev1.EnvVar{
		Name:  "service",
		Value: "wallet",
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
	if wallet.Spec.ChiaConfig.Testnet != nil && *wallet.Spec.ChiaConfig.Testnet {
		env = append(env, corev1.EnvVar{
			Name:  "testnet",
			Value: "true",
		})
	}

	// TZ env var
	if wallet.Spec.ChiaConfig.Timezone != nil {
		env = append(env, corev1.EnvVar{
			Name:  "TZ",
			Value: *wallet.Spec.ChiaConfig.Timezone,
		})
	}

	// log_level env var
	if wallet.Spec.ChiaConfig.LogLevel != nil {
		env = append(env, corev1.EnvVar{
			Name:  "log_level",
			Value: *wallet.Spec.ChiaConfig.LogLevel,
		})
	}

	// keys env var
	env = append(env, corev1.EnvVar{
		Name:  "keys",
		Value: fmt.Sprintf("/key/%s", wallet.Spec.ChiaConfig.SecretKeySpec.Key),
	})

	// node peer env var
	env = append(env, corev1.EnvVar{
		Name:  "full_node_peer",
		Value: wallet.Spec.ChiaConfig.FullNodePeer,
	})

	return env
}

// getCommonLabels gives some common labels for ChiaWallet related objects
func (r *ChiaWalletReconciler) getCommonLabels(ctx context.Context, wallet k8schianetv1.ChiaWallet, additionalLabels ...map[string]string) map[string]string {
	var labels = make(map[string]string)
	for _, addition := range additionalLabels {
		for k, v := range addition {
			labels[k] = v
		}
	}
	labels["app.kubernetes.io/instance"] = wallet.Name
	labels["chiawallet-owner"] = wallet.Name
	labels = getCommonLabels(ctx, labels)
	return labels
}

// getOwnerReference gives the common owner reference spec for ChiaWallet related objects
func (r *ChiaWalletReconciler) getOwnerReference(ctx context.Context, wallet k8schianetv1.ChiaWallet) []metav1.OwnerReference {
	return []metav1.OwnerReference{
		{
			APIVersion: wallet.APIVersion,
			Kind:       wallet.Kind,
			Name:       wallet.Name,
			UID:        wallet.UID,
			Controller: &controllerOwner,
		},
	}
}
