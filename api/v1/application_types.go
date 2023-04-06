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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ApplicationComponent struct {
	// Name is a name of the service that needs to be deployed
	Name string `json:"name,omitempty"`

	// Port is server port that application starts on
	Port int32 `json:"port,omitempty"`

	// Type of service is either backend or frontend
	// Valid values are:
	// - backend: configures service type as ClusterIP
	// - frontend: configures service type as NodePort
	Type string `json:"type,omitempty"`

	// NumberOfEndpoints is number of pod replicas
	//+kubebuilder:validation:Minimum=1
	NumberOfEndpoints *int32 `json:"numberofendpoints,omitempty"`

	// Image is the container image to pull
	Image string `json:"image" required:"true"`

	// Version is the application's version that need to be installed
	Version string `json:"version" required:"true"`
}

// ApplicationSpec defines the desired state of Application
type ApplicationSpec struct {
	// Name is a name of the Application that needs to be deployed
	Name string `json:"name,omitempty"`
	// Components is a list of application components need to be deployed
	Components []ApplicationComponent `json:"components,omitempty"`
}

// ApplicationStatus defines the observed state of Application
type ApplicationStatus struct{}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Application is the Schema for the applications API
type Application struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ApplicationSpec   `json:"spec,omitempty"`
	Status ApplicationStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ApplicationList contains a list of Application
type ApplicationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Application `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Application{}, &ApplicationList{})
}
