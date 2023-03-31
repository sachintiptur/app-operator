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
	"github.com/go-logr/logr"
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
)

// ApplicationReconciler reconciles a Application object
type ApplicationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
}

//+kubebuilder:rbac:groups=simpleapp.github.com,resources=applications,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=simpleapp.github.com,resources=applications/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=simpleapp.github.com,resources=applications/finalizers,verbs=update
//+kubebuilder:rbac:groups=simpleapp.github.com,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=simpleapp.github.com,resources=deployments/status,verbs=get

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.

func (r *ApplicationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

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

	// handling finalizer
	finalizerName := "simpleapp.github.com/finalizer"
	//examine DeletionTimestamp to determine if object is under deletion
	if application.ObjectMeta.DeletionTimestamp.IsZero() {
		if !controllerutil.ContainsFinalizer(application, finalizerName) {
			fmt.Println("Adding finalizers")
			controllerutil.AddFinalizer(application, finalizerName)
			if err := r.Update(ctx, application); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		// The object is being deleted
		if controllerutil.ContainsFinalizer(application, finalizerName) {
			// our finalizer is present, so lets handle any external dependency
			fmt.Println("in Deleting phase")
			if err := r.deleteExternalResources(ctx, application); err != nil {
				// if fail to delete the external dependency here, return with error
				// so that it can be retried
				return ctrl.Result{}, err
			}
			// remove our finalizer from the list and update it.
			controllerutil.RemoveFinalizer(application, finalizerName)
			if err := r.Update(ctx, application); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
	}

	if err := r.reconcile(ctx, &log, application); err != nil {
		log.Error(err, "reconciling failed")
	}

	return ctrl.Result{}, nil
}

func (r *ApplicationReconciler) reconcile(ctx context.Context, log *logr.Logger, application *simpleappv1.Application) error {

	r.ensureApplication(ctx, application)

	return nil

}

func (r *ApplicationReconciler) createDeployment(ctx context.Context, service simpleappv1.ApplicationService) error {

	labels := map[string]string{
		"app": service.Name}

	appDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      service.Name,
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &service.NumberOfEndpoints,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Image:           service.Image + ":" + service.Version,
						ImagePullPolicy: corev1.PullAlways,
						Name:            service.Name,
						Ports: []corev1.ContainerPort{{
							ContainerPort: service.Port,
							Name:          service.Name,
						}},
					}},
				},
			},
		},
	}
	log.Log.Info("creating deployment")
	if err := r.Create(ctx, appDeployment); err != nil {
		log.Log.Error(err, "creating deployment failed")
		return err
	}

	return nil
}

func (r *ApplicationReconciler) createService(ctx context.Context, service simpleappv1.ApplicationService) error {
	labels := map[string]string{
		"app": service.Name}

	srv := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      service.Name,
			Namespace: "default",
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

	if service.Type == "backend" {
		srv.Spec.Type = corev1.ServiceTypeClusterIP
	} else {
		srv.Spec.Type = corev1.ServiceTypeNodePort
	}

	log.Log.Info("creating service")
	if err := r.Create(ctx, srv); err != nil {
		log.Log.Error(err, "creating service failed")
		return err
	}

	return nil

}

func updateApplicationDeployment(ctx context.Context, application *simpleappv1.Application) error {
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ApplicationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&simpleappv1.Application{}).
		Complete(r)
}

func (r *ApplicationReconciler) ensureApplication(ctx context.Context, application *simpleappv1.Application) error {

	found := &appsv1.Deployment{}
	var ownerReferences metav1.OwnerReference

	for _, service := range application.Spec.Services {
		err := r.Get(ctx, types.NamespacedName{
			Name:      service.Name,
			Namespace: "default"}, found)

		if err != nil && errors.IsNotFound(err) {
			// Create the application
			log.Log.Info("Creating a new Application", "Application.Namespace", application.Namespace, "Deployment.Name", application.Name)
			err = r.createDeployment(ctx, service)
			if err != nil {
				return err
			}

		} else {
			ownerReferences = metav1.OwnerReference{
				APIVersion: application.APIVersion,
				UID:        application.UID,
				Kind:       "Application",
				Name:       application.Name,
			}
			found.OwnerReferences = append(found.OwnerReferences, ownerReferences)
			if err = r.Update(ctx, found); err != nil {
				log.Log.Error(err, "updating object failed")
				return err
			}
		}
		found := &corev1.Service{}
		err = r.Get(ctx, types.NamespacedName{
			Name:      service.Name,
			Namespace: "default"}, found)

		if err != nil && errors.IsNotFound(err) {
			err = r.createService(ctx, service)
			if err != nil {
				return err
			}
		} else {
			ownerReferences = metav1.OwnerReference{
				APIVersion: application.APIVersion,
				UID:        application.UID,
				Kind:       "Application",
				Name:       application.Name,
			}
			found.OwnerReferences = append(found.OwnerReferences, ownerReferences)
			if err = r.Update(ctx, found); err != nil {
				log.Log.Error(err, "deleting object failed")
				return err
			}

		}
	}
	return nil
}

func (r *ApplicationReconciler) deleteExternalResources(ctx context.Context, application *simpleappv1.Application) error {

	return nil
}
