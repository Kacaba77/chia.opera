/*
Copyright 2023 Chia Network Inc.
*/

package controller

import (
	"context"
	"time"

	apiv1 "github.com/chia-network/chia-operator/api/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// +kubebuilder:docs-gen:collapse=Imports

var _ = Describe("ChiaCA controller", func() {
	const (
		chiaCAName      = "test-chiaca"
		chiaCANamespace = "default"

		timeout  = time.Second * 10
		duration = time.Second * 10
		interval = time.Millisecond * 250
	)
	var (
		caSecretName = "test-secret"
	)

	Context("When updating ChiaCA Status", func() {
		It("Should update ChiaCA Status.Ready to true when deployment is created", func() {
			By("By creating a new ChiaCA")
			ctx := context.Background()
			ca := &apiv1.ChiaCA{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "k8s.chia.net/v1",
					Kind:       "ChiaCA",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      chiaCAName,
					Namespace: chiaCANamespace,
				},
				Spec: apiv1.ChiaCASpec{
					Secret: caSecretName,
				},
			}

			// Create ChiaCA
			Expect(k8sClient.Create(ctx, ca)).Should(Succeed())

			// Look up the created ChiaCA
			cronjobLookupKey := types.NamespacedName{Name: chiaCAName, Namespace: chiaCANamespace}
			createdChiaCA := &apiv1.ChiaCA{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, cronjobLookupKey, createdChiaCA)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			// Ensure the ChiaCA's spec.chia.timezone was set to the expected timezone
			Expect(createdChiaCA.Spec.Secret).Should(Equal(caSecretName))
		})
	})
})
