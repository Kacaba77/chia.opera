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

var _ = Describe("ChiaFarmer controller", func() {
	const (
		chiaFarmerName      = "test-chiafarmer"
		chiaFarmerNamespace = "default"

		timeout  = time.Second * 10
		duration = time.Second * 10
		interval = time.Millisecond * 250
	)
	var (
		caSecretName  = "test-secret"
		testnet       = true
		timezone      = "UTC"
		logLevel      = "INFO"
		fullNodePeer  = "node.default.svc.cluster.local:58444"
		secretKeyName = "testkeys"
		secretKeyKey  = "key.txt"
	)

	Context("When updating ChiaFarmer Status", func() {
		It("Should update ChiaFarmer Status.Ready to true when deployment is created", func() {
			By("By creating a new ChiaFarmer")
			ctx := context.Background()
			farmer := &apiv1.ChiaFarmer{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "k8s.chia.net/v1",
					Kind:       "ChiaFarmer",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      chiaFarmerName,
					Namespace: chiaFarmerNamespace,
				},
				Spec: apiv1.ChiaFarmerSpec{
					ChiaConfig: apiv1.ChiaFarmerConfigSpec{
						CASecretName: caSecretName,
						Testnet:      &testnet,
						Timezone:     &timezone,
						LogLevel:     &logLevel,
						FullNodePeer: fullNodePeer,
						SecretKeySpec: apiv1.ChiaKeysSpec{
							Name: secretKeyName,
							Key:  secretKeyKey,
						},
					},
					ChiaExporterConfig: apiv1.ChiaExporterConfigSpec{
						ServiceLabels: map[string]string{
							"key": "value",
						},
					},
				},
			}

			// Create ChiaFarmer
			Expect(k8sClient.Create(ctx, farmer)).Should(Succeed())

			// Look up the created ChiaFarmer
			cronjobLookupKey := types.NamespacedName{Name: chiaFarmerName, Namespace: chiaFarmerNamespace}
			createdChiaFarmer := &apiv1.ChiaFarmer{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, cronjobLookupKey, createdChiaFarmer)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			// Ensure the ChiaFarmer's spec.chia.timezone was set to the expected timezone
			Expect(*createdChiaFarmer.Spec.ChiaConfig.Timezone).Should(Equal(timezone))
		})
	})
})
