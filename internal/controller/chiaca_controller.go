/*
Copyright 2023 Chia Network Inc.
*/

package controller

import (
	"context"
	"fmt"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	k8schianetv1 "github.com/chia-network/chia-operator/api/v1"
	"github.com/cisco-open/operator-tools/pkg/reconciler"
)

const caGeneratorNameSuffix = "-chiaca-generator"

var caControllerOwner = true

// ChiaCAReconciler reconciles a ChiaCA object
type ChiaCAReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=k8s.chia.net,resources=chiacas,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=k8s.chia.net,resources=chiacas/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=k8s.chia.net,resources=chiacas/finalizers,verbs=update
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles,verbs=get;list;watch;create;update;patch
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch
//+kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete

// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.4/pkg/reconcile
func (r *ChiaCAReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	resourceReconciler := reconciler.NewReconcilerWith(r.Client, reconciler.WithLog(log))
	log.Info(fmt.Sprintf("ChiaCAReconciler ChiaCA=%s", req.NamespacedName.String()))

	// Get the custom resource
	var ca k8schianetv1.ChiaCA
	err := r.Get(ctx, req.NamespacedName, &ca)
	if err != nil && errors.IsNotFound(err) {
		// Return here, this can happen if the CR was deleted
		return ctrl.Result{}, nil
	}
	if err != nil {
		log.Error(err, fmt.Sprintf("ChiaCAReconciler ChiaCA=%s unable to fetch ChiaCA resource", req.NamespacedName))
		return ctrl.Result{}, err
	}

	// Reconcile resources, creating them if they don't exist
	sa := r.assembleCAServiceAccount(ctx, ca)
	res, err := reconcileServiceAccount(ctx, resourceReconciler, sa)
	if err != nil {
		if res == nil {
			res = &reconcile.Result{}
		}
		return *res, fmt.Errorf("ChiaCAReconciler ChiaCA=%s encountered error reconciling CA generator ServiceAccount: %v", req.NamespacedName, err)
	}

	role := r.assembleCARole(ctx, ca)
	res, err = reconcileRole(ctx, resourceReconciler, role)
	if err != nil {
		if res == nil {
			res = &reconcile.Result{}
		}
		return *res, fmt.Errorf("ChiaCAReconciler ChiaCA=%s encountered error reconciling CA generator Role: %v", req.NamespacedName, err)
	}

	rb := r.assembleCARoleBinding(ctx, ca)
	res, err = reconcileRoleBinding(ctx, resourceReconciler, rb)
	if err != nil {
		if res == nil {
			res = &reconcile.Result{}
		}
		return *res, fmt.Errorf("ChiaCAReconciler ChiaCA=%s encountered error reconciling CA generator RoleBinding: %v", req.NamespacedName, err)
	}

	// Query CA Secret
	_, notFound, err := r.getCASecret(ctx, ca)
	if err != nil {
		log.Error(err, fmt.Sprintf("ChiaCAReconciler ChiaCA=%s unable to query for ChiaCA secret", req.NamespacedName))
		return ctrl.Result{}, err
	}
	// Create CA generating Job if Secret does not already exist
	if notFound {
		job := r.assembleCAJob(ctx, ca)
		res, err = reconcileJob(ctx, resourceReconciler, job)
		if err != nil {
			if res == nil {
				res = &reconcile.Result{}
			}
			return *res, fmt.Errorf("ChiaCAReconciler ChiaCA=%s encountered error reconciling CA generator Job: %v", req.NamespacedName, err)
		}

		// Loop to determine if Secret was made, set to Ready once done
		for i := 1; i <= 100; i++ {
			log.Info(fmt.Sprintf("ChiaCAReconciler ChiaCA=%s waiting for ChiaCA Job to create CA Secret, iteration %d...", req.NamespacedName.String(), i))

			_, notFound, err := r.getCASecret(ctx, ca)
			if err != nil {
				log.Error(err, fmt.Sprintf("ChiaCAReconciler ChiaCA=%s unable to query for ChiaCA secret", req.NamespacedName))
				return ctrl.Result{}, err
			}

			if !notFound {
				ca.Status.Ready = true
				err = r.Status().Update(ctx, &ca)
				if err != nil {
					log.Error(err, fmt.Sprintf("ChiaCAReconciler ChiaCA=%s unable to update ChiaCA status", req.NamespacedName))
					return ctrl.Result{}, err
				}

				break
			}

			time.Sleep(10 * time.Second)
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ChiaCAReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&k8schianetv1.ChiaCA{}).
		Complete(r)
}

// assembleCAJob assembles the Job resource for a ChiaCA CR
func (r *ChiaCAReconciler) assembleCAJob(ctx context.Context, ca k8schianetv1.ChiaCA) batchv1.Job {
	var job batchv1.Job = batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:            ca.Name,
			Namespace:       ca.Namespace,
			Labels:          r.getChiaCACommonLabels(ctx, ca),
			OwnerReferences: r.getChiaCAOwnerReference(ctx, ca),
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy:      "Never",
					ServiceAccountName: fmt.Sprintf("%s%s", ca.Name, caGeneratorNameSuffix),
					Containers: []corev1.Container{
						{
							Name:  "chiaca-generator",
							Image: ca.Spec.Image,
							Env: []corev1.EnvVar{
								{
									Name:  "NAMESPACE",
									Value: ca.Namespace,
								},
								{
									Name:  "SECRET_NAME",
									Value: ca.Spec.Secret,
								},
							},
						},
					},
				},
			},
		},
	}
	if ca.Spec.ImagePullSecret != "" {
		job.Spec.Template.Spec.ImagePullSecrets = []corev1.LocalObjectReference{
			{
				Name: ca.Spec.ImagePullSecret,
			},
		}
	}

	return job
}

// assembleCAServiceAccount assembles the ServiceAccount resource for a ChiaCA CR
func (r *ChiaCAReconciler) assembleCAServiceAccount(ctx context.Context, ca k8schianetv1.ChiaCA) corev1.ServiceAccount {
	return corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:            fmt.Sprintf("%s%s", ca.Name, caGeneratorNameSuffix),
			Namespace:       ca.Namespace,
			Labels:          r.getChiaCACommonLabels(ctx, ca),
			OwnerReferences: r.getChiaCAOwnerReference(ctx, ca),
		},
	}
}

// assembleCARole assembles the Role resource for a ChiaCA CR
func (r *ChiaCAReconciler) assembleCARole(ctx context.Context, ca k8schianetv1.ChiaCA) rbacv1.Role {
	return rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:            fmt.Sprintf("%s%s", ca.Name, caGeneratorNameSuffix),
			Namespace:       ca.Namespace,
			Labels:          r.getChiaCACommonLabels(ctx, ca),
			OwnerReferences: r.getChiaCAOwnerReference(ctx, ca),
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{
					"",
				},
				Resources: []string{
					"secrets",
				},
				Verbs: []string{
					"create",
				},
			},
		},
	}
}

// assembleCARoleBinding assembles the RoleBinding resource for a ChiaCA CR
func (r *ChiaCAReconciler) assembleCARoleBinding(ctx context.Context, ca k8schianetv1.ChiaCA) rbacv1.RoleBinding {
	return rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:            fmt.Sprintf("%s%s", ca.Name, caGeneratorNameSuffix),
			Namespace:       ca.Namespace,
			Labels:          r.getChiaCACommonLabels(ctx, ca),
			OwnerReferences: r.getChiaCAOwnerReference(ctx, ca),
		},
		Subjects: []rbacv1.Subject{
			{
				Kind: "ServiceAccount",
				Name: fmt.Sprintf("%s%s", ca.Name, caGeneratorNameSuffix),
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind: "Role",
			Name: fmt.Sprintf("%s%s", ca.Name, caGeneratorNameSuffix),
		},
	}
}

// getCASecret fetches the k8s Secret that matches this ChiaCA deployment. Returns Secret, boolean, and error (if any).
// Boolean will be true if there is an error and it was generated by the NewNotFound wrapped error helper.
func (r *ChiaCAReconciler) getCASecret(ctx context.Context, ca k8schianetv1.ChiaCA) (corev1.Secret, bool, error) {
	var caSecret corev1.Secret
	err := r.Get(ctx, types.NamespacedName{
		Namespace: ca.Namespace,
		Name:      ca.Spec.Secret,
	}, &caSecret)
	if err != nil && errors.IsNotFound(err) {
		return caSecret, true, nil
	}
	if err != nil {
		return caSecret, false, err
	}

	return caSecret, false, nil
}

// getChiaCACommonLabels gives some common labels for ChiaCA related objects
func (r *ChiaCAReconciler) getChiaCACommonLabels(ctx context.Context, ca k8schianetv1.ChiaCA) map[string]string {
	var labels map[string]string = make(map[string]string)
	labels = getCommonLabels(ctx, labels)
	labels["app.kubernetes.io/instance"] = ca.Name
	labels["chiaca-owner"] = ca.Name
	return labels
}

// getChiaNodeOwnerReference gives the common owner reference spec for ChiaCA related objects
func (r *ChiaCAReconciler) getChiaCAOwnerReference(ctx context.Context, ca k8schianetv1.ChiaCA) []metav1.OwnerReference {
	return []metav1.OwnerReference{
		{
			APIVersion: ca.APIVersion,
			Kind:       ca.Kind,
			Name:       ca.Name,
			UID:        ca.UID,
			Controller: &caControllerOwner,
		},
	}
}
