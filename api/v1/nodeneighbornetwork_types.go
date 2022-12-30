/*
Copyright 2022.

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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// NodeNeighborNetworkSpec defines the desired state of NodeNeighborNetwork
type NodeNeighborNetworkSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of NodeNeighborNetwork. Edit nodeneighbornetwork_types.go to remove/update
	Foo string `json:"foo,omitempty"`
}

// NodeNeighborNetworkStatus defines the observed state of NodeNeighborNetwork
type NodeNeighborNetworkStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Interfaces []InterfaceStatus `json:"interfaces,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// NodeNeighborNetwork is the Schema for the nodeneighbornetworks API
type NodeNeighborNetwork struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NodeNeighborNetworkSpec   `json:"spec,omitempty"`
	Status NodeNeighborNetworkStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// NodeNeighborNetworkList contains a list of NodeNeighborNetwork
type NodeNeighborNetworkList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NodeNeighborNetwork `json:"items"`
}

type InterfaceStatus struct {
	Name         string     `json:"name"`
	HardwareAddr string     `json:"hardwareAddr,omitempty"`
	Neighbors    []Neighbor `json:"neighbors,omitempty"`
}

type Neighbor struct {
	NodeName      string `json:"nodeName"`
	InterfaceName string `json:"interfaceName"`
	HardwareAddr  string `json:"hardwareAddr,omitempty"`
}

func init() {
	SchemeBuilder.Register(&NodeNeighborNetwork{}, &NodeNeighborNetworkList{})
}
