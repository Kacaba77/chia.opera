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

var _ = Describe("ChiaHarvester controller", func() {
	const (
		chiaHarvesterName      = "test-chiaharvester"
		chiaHarvesterNamespace = "default"

		timeout  = time.Second * 10
		duration = time.Second * 10
		interval = time.Millisecond * 250
	)
	var (
		caSecretName = "test-secret"
		testnet      = true
		timezone     = "UTC"
		logLevel     = "INFO"
	)

	Context("When updating ChiaHarvester Status", func() {
		It("Should update ChiaHarvester Status.Ready to true when deployment is created", func() {
			By("By creating a new ChiaHarvester")
			ctx := context.Background()
			harvester := &apiv1.ChiaHarvester{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "k8s.chia.net/v1",
					Kind:       "ChiaHarvester",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      chiaHarvesterName,
					Namespace: chiaHarvesterNamespace,
				},
				Spec: apiv1.ChiaHarvesterSpec{
					ChiaConfig: apiv1.ChiaHarvesterConfigSpec{
						CASecretName: caSecretName,
						Testnet:      &testnet,
						Timezone:     &timezone,
						LogLevel:     &logLevel,
					},
					Storage: &apiv1.StorageConfig{
						Plots: &apiv1.PlotsConfig{
							HostPathVolume: []*apiv1.HostPathVolumeConfig{
								{
									Path: "/home/test/plot1",
								},
								{
									Path: "/home/test/plot2",
								},
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

			// Create ChiaHarvester
			Expect(k8sClient.Create(ctx, harvester)).Should(Succeed())

			// Look up the created ChiaHarvester
			cronjobLookupKey := types.NamespacedName{Name: chiaHarvesterName, Namespace: chiaHarvesterNamespace}
			createdChiaHarvester := &apiv1.ChiaHarvester{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, cronjobLookupKey, createdChiaHarvester)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			// Ensure the ChiaHarvester's spec.chia.timezone was set to the expected timezone
			Expect(*createdChiaHarvester.Spec.ChiaConfig.Timezone).Should(Equal(timezone))
		})
	})
})
