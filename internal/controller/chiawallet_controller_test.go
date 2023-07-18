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

var _ = Describe("ChiaWallet controller", func() {
	const (
		chiaWalletName      = "test-chiawallet"
		chiaWalletNamespace = "default"

		timeout  = time.Second * 10
		duration = time.Second * 10
		interval = time.Millisecond * 250
	)
	var (
		testnet       = true
		timezone      = "UTC"
		logLevel      = "INFO"
		fullNodePeer  = "node.default.svc.cluster.local:58444"
		secretKeyName = "testkeys"
		secretKeyKey  = "key.txt"
	)

	Context("When updating ChiaWallet Status", func() {
		It("Should update ChiaWallet Status.Ready to true when deployment is created", func() {
			By("By creating a new ChiaWallet")
			ctx := context.Background()
			wallet := &apiv1.ChiaWallet{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "k8s.chia.net/v1",
					Kind:       "ChiaWallet",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      chiaWalletName,
					Namespace: chiaWalletNamespace,
				},
				Spec: apiv1.ChiaWalletSpec{
					ChiaConfig: apiv1.ChiaWalletConfigSpec{
						CASecretName: "test-secret",
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

			// Create ChiaWallet
			Expect(k8sClient.Create(ctx, wallet)).Should(Succeed())

			// Look up the created ChiaWallet
			cronjobLookupKey := types.NamespacedName{Name: chiaWalletName, Namespace: chiaWalletNamespace}
			createdChiaWallet := &apiv1.ChiaWallet{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, cronjobLookupKey, createdChiaWallet)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			// Ensure the ChiaWallet's spec.chia.fullNodePeer was set to the expected fullNodePeer
			Expect(createdChiaWallet.Spec.ChiaConfig.FullNodePeer).Should(Equal(fullNodePeer))
		})
	})

})
