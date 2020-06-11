/*
Copyright 2017 The Kubernetes Authors.

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

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type EKSClusterConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EKSClusterConfigSpec   `json:"spec"`
	Status EKSClusterConfigStatus `json:"status"`
}

// EKSClusterConfigSpec is the spec for a EKSClusterConfig resource
type EKSClusterConfigSpec struct {
	KubernetesVersion   string            `json:"kubernetesVersion"`
	Tags                map[string]string `json:"tags"`
	SecretsEncryption   bool              `json:"secretsEncryption"`
	KmsKey              string            `json:"kmsKey"`
	PublicAccess        bool              `json:"publicAccess"`
	PrivateAccess       bool              `json:"privateAccess"`
	PublicAccessSources []string          `json:"publicAccessSources"`
	LoggingTypes        []string          `json:"loggingTypes"`
	CloudCredential     string            `json:"cloudCredential"`
	VirtualNetwork      string            `json:"virtualNetwork"`
	DisplayName         string            `json:"displayName"`
	Subnets             []string          `json:"subnets"`
	SecurityGroups      []string          `json:"securityGroups"`
	ServiceRole         string            `json:"serviceRole"`
	Region              string            `json:"region"`
	Imported            *bool             `json:"imported,omitempty"`
	NodeGroups          []NodeGroup       `json:"nodeGroups"`
}

type EKSClusterConfigStatus struct {
	Phase string `json:"phase"`
}

type NodeGroup struct {
	Gpu bool `json:"gpu"`
	NodegroupName string `json:"nodegroupName"`
	DiskSize *int64 `json:"diskSize"`
	InstanceType *string `json:"instanceType"`
	Labels map[string]*string `json:"labels"`
	Ec2SshKey *string `json:"ec2SshKey"`
	SourceSecurityGroups []*string `json:"sourceSecurityGroups"`
	DesiredSize *int64 `json:"desiredSize"`
	MaxSize *int64 `json:"maxSize"`
	MinSize *int64 `json:"minSize"`
	Subnets []string `json:"subnets"`
	Tags map[string]*string `json:"tags"`
	Version *string `json:"version"`
}