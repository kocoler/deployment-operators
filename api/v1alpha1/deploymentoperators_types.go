/*


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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// DeploymentOperatorsSpec defines the desired state of DeploymentOperators
type DeploymentOperatorsSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// selector for deployments
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`

	// customer endpoints, deployment operator will push deployment results to all endpoints
	CustomerEndpoints []CustomerEndpoints `json:"customerEndpoints,omitempty"`
}

// CustomerEndpoints defines customer endpoints
type CustomerEndpoints struct {
	// customer endpoint
	Host string `json:"host,omitempty"`

	// use to encode pushed messages(ase-256)
	Secret string `json:"secret,omitempty"`
}

// DeploymentOperatorsStatus defines the observed state of DeploymentOperators
type DeploymentOperatorsStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Total number of pods collected by this deployment operator.
	CollectedPods int32 `json:"collectedPods,omitempty"`

	// Total number of deployments collected by this deployment operator.
	CollectedDeployments int32 `json:"collectedDeployments,omitempty"`

	// Total number of pushed messages to customerEndpoints
	PushMessages int32 `json:"pushMessages,omitempty"`

	// others ...

	// The number of services(deployment: app=service) monitoring by this deployment operator.
	MonitoringServices int32 `json:"monitoringServices,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster

// DeploymentOperators is the Schema for the deploymentoperators API
type DeploymentOperators struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DeploymentOperatorsSpec   `json:"spec,omitempty"`
	Status DeploymentOperatorsStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// DeploymentOperatorsList contains a list of DeploymentOperators
type DeploymentOperatorsList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DeploymentOperators `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DeploymentOperators{}, &DeploymentOperatorsList{})
}
