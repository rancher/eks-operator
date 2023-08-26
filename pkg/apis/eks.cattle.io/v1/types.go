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
// +kubebuilder:printcolumn:name="DisplayName",type="string",JSONPath=".spec.clusterName"
// +kubebuilder:printcolumn:name="KubernetesVersion",type="string",JSONPath=".spec.kubernetesVersion"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="FailureMessage",type="string",JSONPath=".status.failureMessage"

type EKSClusterConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EKSClusterConfigSpec   `json:"spec"`
	Status EKSClusterConfigStatus `json:"status"`
}

// EKSClusterConfigSpec is the spec for a EKSClusterConfig resource
type EKSClusterConfigSpec struct {
	// AmazonCredentialSecret is the name of the secret containing the Amazon credentials.
	// +kubebuilder:validation:Required
	AmazonCredentialSecret string `json:"amazonCredentialSecret"`
	// DisplayName is the name of the cluster to be displayed in the UI.
	// +kubebuilder:validation:Required
	DisplayName string `json:"displayName" norman:"noupdate"`
	// Region is the AWS region to create the cluster in.
	// +optional
	Region string `json:"region" norman:"noupdate"`
	// Imported is true if the cluster was imported.
	// +optional
	Imported bool `json:"imported" norman:"noupdate"`
	// KubernetesVersion is the version of Kubernetes to use.
	// +optional
	KubernetesVersion *string `json:"kubernetesVersion" norman:"pointer"`
	// Tags is a map of tags to apply to the cluster.
	// +optional
	// +kubebuilder:validation:UniqueItems:=true
	Tags map[string]string `json:"tags"`
	// SecretsEncryption is true if secrets should be encrypted.
	// +optional
	SecretsEncryption *bool `json:"secretsEncryption" norman:"noupdate"`
	// KmsKey is the KMS key to use for encryption.
	// +optional
	KmsKey *string `json:"kmsKey" norman:"noupdate,pointer"`
	// PublicAccess is true if the cluster should be publicly accessible.
	// +kubebuilder:validation:Required
	PublicAccess *bool `json:"publicAccess"`
	// PrivateAccess is true if the cluster should be privately accessible.
	// +kubebuilder:validation:Required
	PrivateAccess *bool `json:"privateAccess"`
	// EbsCSIDriver is true if the EBS CSI driver should be installed.
	// +optional
	EBSCSIDriver *bool `json:"ebsCSIDriver"`
	// PublicAccessSources is a list of CIDRs that can access the cluster.
	// +optional
	PublicAccessSources []string `json:"publicAccessSources"`
	// LoggingTypes is a list of logging types to enable.
	// +optional
	LoggingTypes []string `json:"loggingTypes"`
	// Subnets is a list of subnets to use for the cluster.
	// +kubebuilder:validation:Required
	Subnets []string `json:"subnets" norman:"noupdate"`
	// SecurityGroups is a list of security groups to use for the cluster.
	// +kubebuilder:validation:Required
	SecurityGroups []string `json:"securityGroups" norman:"noupdate"`
	// ServiceRole is the IAM role to use for the cluster.
	// +optional
	ServiceRole *string `json:"serviceRole" norman:"noupdate,pointer"`
	// NodeGroups is a list of node groups to create.
	// +kubebuilder:validation:Required
	NodeGroups []NodeGroup `json:"nodeGroups"`
}

type EKSClusterConfigStatus struct {
	// Phase is the current lifecycle phase of the cluster.
	Phase string `json:"phase"`
	// VirtualNetwork is the ID of the virtual network.
	VirtualNetwork string `json:"virtualNetwork"`
	// Subnets is a list of subnets to use for the cluster.
	Subnets []string `json:"subnets"`
	// SecurityGroups is a list of security groups to use for the cluster.
	SecurityGroups []string `json:"securityGroups"`
	// ManagedLaunchTemplateID is the ID of the managed launch template.
	ManagedLaunchTemplateID string `json:"managedLaunchTemplateID"`
	// ManagedLaunchTemplateVersion is the version of the managed launch template.
	ManagedLaunchTemplateVersions map[string]string `json:"managedLaunchTemplateVersions"`
	// TemplateVersionsToDelete is a list of template versions to delete.
	TemplateVersionsToDelete []string `json:"templateVersionsToDelete"`
	// describes how the above network fields were provided. Valid values are provided and generated
	NetworkFieldsSource string `json:"networkFieldsSource"`
	// FailureMessage is the message from the last failure, if any.
	FailureMessage string `json:"failureMessage"`
	// GeneratedNodeRole is the node role generated by the cluster.
	GeneratedNodeRole string `json:"generatedNodeRole"`
}

type NodeGroup struct {
	// GPU is true if the node group should have GPU instances.
	// +kubebuilder:validation:Required
	// +kubebuilder:default=false
	Gpu *bool `json:"gpu"`
	// ImageID is the AMI to use for the node group.
	// +optional
	ImageID *string `json:"imageId" norman:"pointer"`
	// NodegroupName is the name of the node group.
	// +kubebuilder:validation:Required
	NodegroupName *string `json:"nodegroupName" norman:"required,pointer" wrangler:"required"`
	// DiskSize is the size of the root volume.
	// +optional
	// +kubebuilder:validation:Minimum=1
	DiskSize *int64 `json:"diskSize"`
	// InstanceType is the instance type to use for the node group.
	// +optional
	InstanceType *string `json:"instanceType" norman:"pointer"`
	// Labels is a map of labels to apply to the node group.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:UniqueItems:=true
	Labels map[string]*string `json:"labels"`
	// EC2SshKey is the SSH key to use for the node group.
	// +optional
	Ec2SshKey *string `json:"ec2SshKey" norman:"pointer"`
	// DesiredSize is the desired size of the node group.
	// +optional
	DesiredSize *int64 `json:"desiredSize"`
	// MaxSize is the maximum size of the node group.
	// +kubebuilder:validation:Required
	MaxSize *int64 `json:"maxSize"`
	// MinSize is the minimum size of the node group.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=1
	MinSize *int64 `json:"minSize"`
	// Subnets is a list of subnets to use for the node group.
	// +kubebuilder:validation:Required
	Subnets []string `json:"subnets"`
	// Tags is a map of tags to apply to the node group.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:UniqueItems:=true
	Tags map[string]*string `json:"tags"`
	// ResourceTags is a map of tags to apply to the node group resources.
	// +optional
	ResourceTags map[string]*string `json:"resourceTags"`
	// UserData is the user data to use for the node group.
	// +optional
	UserData *string `json:"userData" norman:"pointer"`
	// Version is the Kubernetes version to use for the node group.
	// +optional
	Version *string `json:"version" norman:"pointer"`
	// LaunchTemplate is the launch template to use for the node group.
	// +optional
	LaunchTemplate *LaunchTemplate `json:"launchTemplate"`
	// RequestSpotInstances is true if the node group should use spot instances.
	// +optional
	RequestSpotInstances *bool `json:"requestSpotInstances"`
	// SpotInstanceTypes is a list of spot instance types to use for the node group.
	// +optional
	SpotInstanceTypes []*string `json:"spotInstanceTypes"`
	// NodeRole is the IAM role to use for the node group.
	// +optional
	NodeRole *string `json:"nodeRole" norman:"pointer"`
}

type LaunchTemplate struct {
	// ID is the ID of the launch template.
	// +optional
	ID *string `json:"id" norman:"pointer"`
	// Name is the name of the launch template.
	// +optional
	Name *string `json:"name" norman:"pointer"`
	// Version is the version of the launch template.
	// +optional
	Version *int64 `json:"version"`
}
