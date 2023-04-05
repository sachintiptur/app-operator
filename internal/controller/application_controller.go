/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"

	simpleappv1 "github.com/sachintiptur/app-operator/api/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	controllerName = "application-controller"
	finalizerName  = "simpleapp.github.com/finalizer"
	backendApp     = "backend"
	frontendApp    = "frontend"
)

// ApplicationReconciler reconciles an Application object
type ApplicationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=simpleapp.github.com,resources=applications,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=simpleapp.github.com,resources=applications/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=simpleapp.github.com,resources=applications/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=deployments/status,verbs=get
//+kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=services/status,verbs=get

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.

func (r *ApplicationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx).WithValues(controllerName, req.NamespacedName)

	log.Info("Reconciling application")

	application := &simpleappv1.Application{}
	if err := r.Get(ctx, req.NamespacedName, application); err != nil {
		if errors.IsNotFound(err) {
			// object not found, could have been deleted after
			// reconcile request, hence don't requeue
			return ctrl.Result{}, nil
		}
		log.Error(err, "application not found")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// examine DeletionTimestamp to determine if object is under deletion
	if application.ObjectMeta.DeletionTimestamp.IsZero() {
		if !controllerutil.ContainsFinalizer(application, finalizerName) {
			controllerutil.AddFinalizer(application, finalizerName)
			if err := r.Update(ctx, application); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		// The object is being deleted
		if controllerutil.ContainsFinalizer(application, finalizerName) {
			// remove our finalizer from the list and update it.
			controllerutil.RemoveFinalizer(application, finalizerName)
			if err := r.Update(ctx, application); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
	}

	if err := r.reconcile(ctx, application); err != nil {
		log.Error(err, "reconciling failed")
	}

	return ctrl.Result{}, nil
}

func (r *ApplicationReconciler) reconcile(ctx context.Context, application *simpleappv1.Application) error {
	err := r.ensureApplication(ctx, application)
	if err != nil {
		log.Log.Error(err, "application reconciliation failed")
		return err
	}
	return nil
}

// ensureApplication ensures that application is created/updated and in-sync with application spec
func (r *ApplicationReconciler) ensureApplication(ctx context.Context, application *simpleappv1.Application) error {
	if err := r.ensureDeployment(ctx, application); err != nil {
		return err
	}
	if err := r.ensureService(ctx, application); err != nil {
		return err
	}
	return nil
}

// ensureDeployment ensures that deployment is created/updated and in-sync with application spec
func (r *ApplicationReconciler) ensureDeployment(ctx context.Context, application *simpleappv1.Application) error {
	foundDeployment := &appsv1.Deployment{}
	updateDeployment := false

	for _, service := range application.Spec.Services {
		err := r.Get(ctx, types.NamespacedName{
			Name:      service.Name,
			Namespace: application.Namespace,
		}, foundDeployment)

		if err != nil && errors.IsNotFound(err) {
			log.Log.Info("Creating a new Deployment", service.Name, application.Namespace)

			appDeployment, err := r.createDeploymentConfig(service, application)
			if err != nil {
				return err
			}
			// set the controller reference to handle deployment as part of application object
			if err := controllerutil.SetControllerReference(application, appDeployment, r.Scheme); err != nil {
				return err
			}
			if err := r.Create(ctx, appDeployment); err != nil {
				log.Log.Error(err, "creating deployment failed")
				return err
			}

		} else {
			if foundDeployment.Spec.Replicas != service.NumberOfEndpoints {
				foundDeployment.Spec.Replicas = service.NumberOfEndpoints
				updateDeployment = true
			}
			if foundDeployment.Spec.Template.Spec.Containers[0].Image != service.Image+":"+service.Version {
				foundDeployment.Spec.Template.Spec.Containers[0].Image = service.Image + ":" + service.Version
				updateDeployment = true
			}
			if updateDeployment {
				if err := r.Update(ctx, foundDeployment); err != nil {
					return err
				}
			}
		}

	}
	return nil
}

// ensureService ensures that service is created with given application spec
func (r *ApplicationReconciler) ensureService(ctx context.Context, application *simpleappv1.Application) error {
	foundService := &corev1.Service{}

	for _, service := range application.Spec.Services {
		err := r.Get(ctx, types.NamespacedName{
			Name:      service.Name,
			Namespace: application.Namespace,
		}, foundService)

		if err != nil && errors.IsNotFound(err) {
			log.Log.Info("Creating a new Service", service.Name, application.Namespace)
			serviceConfig, err := r.createServiceConfig(service, application)
			if err != nil {
				return err
			}

			// set the controller reference to handle service as part of application object
			if err := controllerutil.SetControllerReference(application, serviceConfig, r.Scheme); err != nil {
				return err
			}
			if err := r.Create(ctx, serviceConfig); err != nil {
				log.Log.Error(err, "creating service failed")
				return err
			}

		}
	}
	return nil
}

// createDeploymentConfig creates and returns the deployment config
func (r *ApplicationReconciler) createDeploymentConfig(service simpleappv1.ApplicationService, application *simpleappv1.Application) (*appsv1.Deployment, error) {
	labels := map[string]string{
		"app": service.Name,
	}

	appDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      service.Name,
			Namespace: application.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: application.APIVersion,
					UID:        application.UID,
					Kind:       application.Kind,
					Name:       application.Name,
				},
			},
		},

		Spec: appsv1.DeploymentSpec{
			Replicas: service.NumberOfEndpoints,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Image:           service.Image + ":" + service.Version,
							ImagePullPolicy: corev1.PullAlways,
							Name:            service.Name,
							Ports: []corev1.ContainerPort{{
								ContainerPort: service.Port,
								Name:          service.Name,
							}},
						},
					},
				},
			},
		},
	}
	// configure GRPC_SERVER env for frontend service
	if service.Type == frontendApp {
		appDeployment.Spec.Template.Spec.Containers[0].Env = []corev1.EnvVar{{Name: "GRPC_SERVER", Value: application.Spec.GrpcServer}}
	}

	return appDeployment, nil
}

// createServiceConfig creates and returns the service config
func (r *ApplicationReconciler) createServiceConfig(service simpleappv1.ApplicationService, application *simpleappv1.Application) (*corev1.Service, error) {
	labels := map[string]string{
		"app": service.Name,
	}

	serviceConfig := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      service.Name,
			Namespace: application.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: application.APIVersion,
					UID:        application.UID,
					Kind:       application.Kind,
					Name:       application.Name,
				},
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: labels,
			Ports: []corev1.ServicePort{{
				Protocol:   corev1.ProtocolTCP,
				Port:       service.Port,
				TargetPort: intstr.FromInt(int(service.Port)),
			}},
		},
	}

	if service.Type == backendApp {
		serviceConfig.Spec.Type = corev1.ServiceTypeClusterIP
	} else if service.Type == frontendApp {
		serviceConfig.Spec.Type = corev1.ServiceTypeLoadBalancer
	} else {
		return nil, fmt.Errorf("unsupported type in application service spec")
	}

	return serviceConfig, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ApplicationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&simpleappv1.Application{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Complete(r)
}
