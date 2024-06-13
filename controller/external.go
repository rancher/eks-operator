package controller

import (
	"context"
	"encoding/base64"
	"fmt"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	eksv1 "github.com/rancher/eks-operator/pkg/apis/eks.cattle.io/v1"
	awsservices "github.com/rancher/eks-operator/pkg/eks"
	"github.com/rancher/eks-operator/pkg/eks/services"
	"github.com/rancher/eks-operator/utils"
	wranglerv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
)

// StartEC2Service initializes and returns an instance of the EC2ServiceInterface
// interface, which provides methods for interacting with the EC2 service in AWS.
func StartEC2Service(ctx context.Context, secretClient wranglerv1.SecretClient, spec eksv1.EKSClusterConfigSpec) (services.EC2ServiceInterface, error) {
	cfg, err := newAWSConfigV2(ctx, secretClient, spec)
	if err != nil {
		return nil, err
	}

	return services.NewEC2Service(cfg), err
}

// StartEKSService initializes and returns an instance of the EKSServiceInterface
// interface, which provides methods for interacting with the EKS service in AWS.
func StartEKSService(ctx context.Context, secretClient wranglerv1.SecretClient, spec eksv1.EKSClusterConfigSpec) (services.EKSServiceInterface, error) {
	cfg, err := newAWSConfigV2(ctx, secretClient, spec)
	if err != nil {
		return nil, err
	}

	return services.NewEKSService(cfg), err
}

// NodeGroupIssueIsUpdatable checks to see the node group can be updated with the given issue code.
func NodeGroupIssueIsUpdatable(code string) bool {
	return code == string(ekstypes.NodegroupIssueCodeAsgInstanceLaunchFailures) ||
		code == string(ekstypes.NodegroupIssueCodeInstanceLimitExceeded) ||
		code == string(ekstypes.NodegroupIssueCodeInsufficientFreeAddresses) ||
		code == string(ekstypes.NodegroupIssueCodeClusterUnreachable)
}

// BuildUpstreamClusterState builds the upstream cluster state from the given eks cluster and node group states.
func BuildUpstreamClusterState(ctx context.Context, name, managedTemplateID string, clusterState *eks.DescribeClusterOutput, nodeGroupStates []*eks.DescribeNodegroupOutput, ec2Service services.EC2ServiceInterface, includeManagedLaunchTemplate bool) (*eksv1.EKSClusterConfigSpec, string, error) {
	upstreamSpec := &eksv1.EKSClusterConfigSpec{}

	upstreamSpec.Imported = true
	upstreamSpec.DisplayName = name

	// set kubernetes version
	upstreamVersion := aws.ToString(clusterState.Cluster.Version)
	if upstreamVersion == "" {
		return nil, "", fmt.Errorf("cannot detect cluster [%s] upstream kubernetes version", name)
	}
	upstreamSpec.KubernetesVersion = aws.String(upstreamVersion)

	// set  tags
	upstreamSpec.Tags = make(map[string]string)
	if len(clusterState.Cluster.Tags) != 0 {
		upstreamSpec.Tags = clusterState.Cluster.Tags
	}

	// set public access
	if hasPublicAccess := clusterState.Cluster.ResourcesVpcConfig.EndpointPublicAccess; hasPublicAccess {
		upstreamSpec.PublicAccess = aws.Bool(true)
	} else {
		upstreamSpec.PublicAccess = aws.Bool(false)
	}

	// set private access
	if hasPrivateAccess := clusterState.Cluster.ResourcesVpcConfig.EndpointPrivateAccess; hasPrivateAccess {
		upstreamSpec.PrivateAccess = aws.Bool(true)
	} else {
		upstreamSpec.PrivateAccess = aws.Bool(false)
	}

	// set public access sources
	upstreamSpec.PublicAccessSources = make([]string, 0)
	if publicAccessSources := clusterState.Cluster.ResourcesVpcConfig.PublicAccessCidrs; len(publicAccessSources) > 0 {
		upstreamSpec.PublicAccessSources = publicAccessSources
	}

	// set logging
	upstreamSpec.LoggingTypes = make([]string, 0)
	if clusterState.Cluster.Logging != nil {
		if clusterLogging := clusterState.Cluster.Logging.ClusterLogging; len(clusterLogging) > 0 {
			setup := clusterLogging[0]
			if aws.ToBool(setup.Enabled) {
				upstreamSpec.LoggingTypes = utils.ConvertFromLogTypes(setup.Types)
			}
		}
	}

	// set node groups
	upstreamSpec.NodeGroups = make([]eksv1.NodeGroup, 0, len(nodeGroupStates))
	for _, ng := range nodeGroupStates {
		if ng.Nodegroup.Status == ekstypes.NodegroupStatusDeleting {
			continue
		}
		ngToAdd := eksv1.NodeGroup{
			NodegroupName:        ng.Nodegroup.NodegroupName,
			DiskSize:             ng.Nodegroup.DiskSize,
			Labels:               aws.StringMap(ng.Nodegroup.Labels),
			DesiredSize:          ng.Nodegroup.ScalingConfig.DesiredSize,
			MaxSize:              ng.Nodegroup.ScalingConfig.MaxSize,
			MinSize:              ng.Nodegroup.ScalingConfig.MinSize,
			NodeRole:             ng.Nodegroup.NodeRole,
			Subnets:              ng.Nodegroup.Subnets,
			Tags:                 aws.StringMap(ng.Nodegroup.Tags),
			RequestSpotInstances: aws.Bool(ng.Nodegroup.CapacityType == ekstypes.CapacityTypesSpot),
		}

		if clusterState.Cluster.Version == ng.Nodegroup.Version ||
			ng.Nodegroup.Status != ekstypes.NodegroupStatusUpdating {
			ngToAdd.Version = ng.Nodegroup.Version
		}

		if aws.ToBool(ngToAdd.RequestSpotInstances) {
			ngToAdd.SpotInstanceTypes = ng.Nodegroup.InstanceTypes
		}

		if ng.Nodegroup.LaunchTemplate != nil {
			var version *int64
			versionNumber, err := strconv.ParseInt(aws.ToString(ng.Nodegroup.LaunchTemplate.Version), 10, 64)
			if err == nil {
				version = aws.Int64(versionNumber)
			}

			ngToAdd.LaunchTemplate = &eksv1.LaunchTemplate{
				ID:      ng.Nodegroup.LaunchTemplate.Id,
				Name:    ng.Nodegroup.LaunchTemplate.Name,
				Version: version,
			}

			if managedTemplateID == aws.ToString(ngToAdd.LaunchTemplate.ID) {
				// If this is a rancher-managed launch template, then we move the data from the launch template to the node group.
				launchTemplateRequestOutput, err := awsservices.GetLaunchTemplateVersions(ctx, &awsservices.GetLaunchTemplateVersionsOpts{
					EC2Service:       ec2Service,
					LaunchTemplateID: ngToAdd.LaunchTemplate.ID,
					Versions:         []*string{ng.Nodegroup.LaunchTemplate.Version},
				})
				if err != nil || len(launchTemplateRequestOutput.LaunchTemplateVersions) == 0 {
					if doesNotExist(err) || notFound(err) {
						if includeManagedLaunchTemplate {
							// In this case, we need to continue rather than error so that we can update the launch template for the nodegroup.
							ngToAdd.LaunchTemplate.ID = nil
							upstreamSpec.NodeGroups = append(upstreamSpec.NodeGroups, ngToAdd)
							continue
						}

						return nil, "", fmt.Errorf("rancher-managed launch template for node group [%s] in cluster [%s] not found, must create new node group and destroy existing",
							aws.ToString(ngToAdd.NodegroupName),
							upstreamSpec.DisplayName,
						)
					}
					return nil, "", fmt.Errorf("error getting launch template info for node group [%s] in cluster [%s]", aws.ToString(ngToAdd.NodegroupName), upstreamSpec.DisplayName)
				}
				launchTemplateData := launchTemplateRequestOutput.LaunchTemplateVersions[0].LaunchTemplateData

				if len(launchTemplateData.BlockDeviceMappings) == 0 {
					return nil, "", fmt.Errorf("launch template for node group [%s] in cluster [%s] is malformed", aws.ToString(ngToAdd.NodegroupName), upstreamSpec.DisplayName)
				}
				ngToAdd.DiskSize = launchTemplateData.BlockDeviceMappings[0].Ebs.VolumeSize
				ngToAdd.Ec2SshKey = launchTemplateData.KeyName
				ngToAdd.ImageID = launchTemplateData.ImageId
				ngToAdd.InstanceType = string(launchTemplateData.InstanceType)
				ngToAdd.ResourceTags = utils.GetInstanceTags(launchTemplateData.TagSpecifications)

				userData := aws.ToString(launchTemplateData.UserData)
				if userData != "" {
					decodedUserdata, err := base64.StdEncoding.DecodeString(userData)
					if err == nil {
						ngToAdd.UserData = aws.String(string(decodedUserdata))
					} else {
						logrus.Warnf("Could not decode userdata for nodegroup [%s] in cluster[%s]", aws.ToString(ngToAdd.NodegroupName), name)
					}
				}

				if !includeManagedLaunchTemplate {
					ngToAdd.LaunchTemplate = nil
				}
			}
		} else {
			// If the node group does not have a launch template, then the following must be pulled from the node group config.
			if !aws.ToBool(ngToAdd.RequestSpotInstances) && len(ng.Nodegroup.InstanceTypes) > 0 {
				ngToAdd.InstanceType = ng.Nodegroup.InstanceTypes[0]
			}
			if ng.Nodegroup.RemoteAccess != nil {
				ngToAdd.Ec2SshKey = ng.Nodegroup.RemoteAccess.Ec2SshKey
			}
		}
		// TODO: Update AMITypesAl2X8664Gpu to Amazon Linux 2023 when it is available
		// Issue https://github.com/rancher/eks-operator/issues/568
		if ng.Nodegroup.AmiType == ekstypes.AMITypesAl2X8664Gpu {
			ngToAdd.Gpu = aws.Bool(true)
		} else if ng.Nodegroup.AmiType == ekstypes.AMITypesAl2023X8664Standard {
			ngToAdd.Gpu = aws.Bool(false)
		} else if ng.Nodegroup.AmiType == ekstypes.AMITypesAl2023Arm64Standard {
			ngToAdd.Arm = aws.Bool(true)
		}
		upstreamSpec.NodeGroups = append(upstreamSpec.NodeGroups, ngToAdd)
	}

	// set subnets
	upstreamSpec.Subnets = clusterState.Cluster.ResourcesVpcConfig.SubnetIds
	// set security groups
	upstreamSpec.SecurityGroups = clusterState.Cluster.ResourcesVpcConfig.SecurityGroupIds

	upstreamSpec.SecretsEncryption = aws.Bool(len(clusterState.Cluster.EncryptionConfig) != 0)
	upstreamSpec.KmsKey = aws.String("")
	if len(clusterState.Cluster.EncryptionConfig) > 0 {
		upstreamSpec.KmsKey = clusterState.Cluster.EncryptionConfig[0].Provider.KeyArn
	}

	upstreamSpec.ServiceRole = clusterState.Cluster.RoleArn
	if upstreamSpec.ServiceRole == nil {
		upstreamSpec.ServiceRole = aws.String("")
	}
	return upstreamSpec, aws.ToString(clusterState.Cluster.Arn), nil
}
