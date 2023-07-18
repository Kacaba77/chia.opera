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

var _ = Describe("ChiaNode controller", func() {
	const (
		chiaNodeName      = "test-chianode"
		chiaNodeNamespace = "default"

		timeout  = time.Second * 10
		duration = time.Second * 10
		interval = time.Millisecond * 250
	)
	var (
		testnet         = true
		timezone        = "UTC"
		logLevel        = "INFO"
		storageClass    = ""
		resourceRequest = "250Gi"
	)

	Context("When updating ChiaNode Status", func() {
		It("Should update ChiaNode Status.Ready to true when deployment is created", func() {
			By("By creating a new ChiaNode")
			ctx := context.Background()
			node := &apiv1.ChiaNode{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "k8s.chia.net/v1",
					Kind:       "ChiaNode",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      chiaNodeName,
					Namespace: chiaNodeNamespace,
				},
				Spec: apiv1.ChiaNodeSpec{
					ChiaConfig: apiv1.ChiaNodeConfigSpec{
						CASecretName: "test-secret",
						Testnet:      &testnet,
						Timezone:     &timezone,
						LogLevel:     &logLevel,
					},
					Storage: &apiv1.StorageConfig{
						ChiaRoot: &apiv1.ChiaRootConfig{
							PersistentVolumeClaim: &apiv1.PersistentVolumeClaimConfig{
								StorageClass:    storageClass,
								ResourceRequest: resourceRequest,
							},
						},
					},
					ChiaExporterConfig: apiv1.ChiaExporterConfigSpec{
						ServiceLabels: map[string]string{
							"key": "value",
						},
					},
				},
			}

			// Create ChiaNode
			Expect(k8sClient.Create(ctx, node)).Should(Succeed())

			// Look up the created ChiaNode
			cronjobLookupKey := types.NamespacedName{Name: chiaNodeName, Namespace: chiaNodeNamespace}
			createdChiaNode := &apiv1.ChiaNode{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, cronjobLookupKey, createdChiaNode)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			// Ensure the ChiaNode's spec.chia.timezone was set to the expected timezone
			Expect(createdChiaNode.Spec.ChiaConfig.Timezone).Should(Equal(timezone))
		})
	})
})
