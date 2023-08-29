package controller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	simpleappv1 "github.com/sachintiptur/app-operator/api/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Application controller", func() {
	const (
		ApplicationName      = "test-application"
		ApplicationNamespace = "default"
		BackendName          = "test-backend"
		FrontendName         = "test-frontend"
	)

	var (
		BackendReplicas  int32 = 2
		FrontendReplicas int32 = 1
	)

	Context("When creating an Application", func() {
		It("Should create Deployments and Services for Application components", func() {
			By("Creating a Application custom resource")
			ctx := context.Background()
			application := simpleappv1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      ApplicationName,
					Namespace: "default",
				},
				Spec: simpleappv1.ApplicationSpec{
					Name: ApplicationName,
					Components: []simpleappv1.ApplicationComponent{
						{
							Name:              BackendName,
							Port:              9000,
							Type:              backendApp,
							NumberOfEndpoints: &BackendReplicas,
							Image:             "ghcr.io/sachintiptur/backend",
							Version:           "latest",
						},
						{
							Name:              "test-frontend",
							Port:              8080,
							Type:              frontendApp,
							NumberOfEndpoints: &FrontendReplicas,
							Image:             "ghcr.io/sachintiptur/frontend",
							Version:           "latest",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, &application)).Should(Succeed())
			// Check for component deployments
			for _, component := range application.Spec.Components {
				By("Checking component deployments")
				foundDeployment := &appsv1.Deployment{}

				Eventually(func() bool {
					if err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      component.Name,
						Namespace: application.Namespace,
					}, foundDeployment); err != nil {
						return false
					}
					return true
				}).Should(BeTrue())

				Expect(component.NumberOfEndpoints).Should(Equal(foundDeployment.Spec.Replicas))
			}

			// Check for component services
			foundService := &corev1.Service{}
			for _, component := range application.Spec.Components {
				Eventually(func() bool {
					if err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      component.Name,
						Namespace: application.Namespace,
					}, foundService); err != nil {
						return false
					}
					return true
				}).Should(BeTrue())
			}
		})
	})

	Context("When an Application component's replica is modified ", func() {
		It("Should  modify Deployment replicas for Application components", func() {
			By("Updating an Application custom resource")
			ctx := context.Background()

			foundApplication := &simpleappv1.Application{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      ApplicationName,
				Namespace: "default",
			}, foundApplication)).Should(Succeed())

			for _, component := range foundApplication.Spec.Components {
				*component.NumberOfEndpoints = *(component.NumberOfEndpoints) + 1
			}
			Expect(k8sClient.Update(ctx, foundApplication)).Should(Succeed())

			// Check for component deployments
			for _, component := range foundApplication.Spec.Components {
				By("Checking component deployments")
				foundDeployment := &appsv1.Deployment{}

				Eventually(func() int32 {
					if err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      component.Name,
						Namespace: foundApplication.Namespace,
					}, foundDeployment); err != nil {
						return 0
					}

					return *foundDeployment.Spec.Replicas
				}).Should(Equal(*component.NumberOfEndpoints))

			}
		})
	})

	Context("When deleting an Application", func() {
		It("Should delete the application", func() {
			By("Deleting an Application custom resource")
			ctx := context.Background()

			foundApplication := &simpleappv1.Application{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      ApplicationName,
				Namespace: "default",
			}, foundApplication)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, foundApplication)).Should(Succeed())
		})
	})
})
