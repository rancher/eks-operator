package controller

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	eksv1 "github.com/rancher/eks-operator/pkg/apis/eks.cattle.io/v1"
	awsservices "github.com/rancher/eks-operator/pkg/eks"
	"github.com/rancher/eks-operator/pkg/eks/services"
	"github.com/rancher/eks-operator/utils"
	"github.com/sirupsen/logrus"
)

func newLaunchTemplateVersionIfNeeded(ctx context.Context, config *eksv1.EKSClusterConfig, upstreamNg, ng eksv1.NodeGroup, ec2Service services.EC2ServiceInterface) (*eksv1.LaunchTemplate, error) {
	if aws.ToString(upstreamNg.UserData) != aws.ToString(ng.UserData) ||
		aws.ToString(upstreamNg.Ec2SshKey) != aws.ToString(ng.Ec2SshKey) ||
		aws.ToInt32(upstreamNg.DiskSize) != aws.ToInt32(ng.DiskSize) ||
		aws.ToString(upstreamNg.ImageID) != aws.ToString(ng.ImageID) ||
		(!aws.ToBool(upstreamNg.RequestSpotInstances) && upstreamNg.InstanceType != ng.InstanceType) ||
		!utils.CompareStringMaps(upstreamNg.ResourceTags, ng.ResourceTags) {
		lt, err := awsservices.CreateNewLaunchTemplateVersion(ctx, ec2Service, config.Status.ManagedLaunchTemplateID, ng)
		if err != nil {
			return nil, err
		}

		return lt, nil
	}

	return nil, nil
}

func deleteLaunchTemplate(ctx context.Context, templateID string, ec2Service services.EC2ServiceInterface) {
	var err error
	for i := 0; i < 5; i++ {
		_, err = ec2Service.DeleteLaunchTemplate(ctx, &ec2.DeleteLaunchTemplateInput{
			LaunchTemplateId: aws.String(templateID),
		})

		if err == nil || doesNotExist(err) {
			return
		}

		time.Sleep(10 * time.Second)
	}

	logrus.Warnf("Could not delete launch template [%s]: %v, will not retry",
		templateID,
		err,
	)
}

func deleteNodeGroups(ctx context.Context, config *eksv1.EKSClusterConfig, nodeGroups []eksv1.NodeGroup, eksService services.EKSServiceInterface) (bool, error) {
	var waitingForNodegroupDeletion bool
	for _, ng := range nodeGroups {
		_, deleteInProgress, err := deleteNodeGroup(ctx, config, ng, eksService)
		if err != nil {
			return false, err
		}
		waitingForNodegroupDeletion = waitingForNodegroupDeletion || deleteInProgress
	}

	return waitingForNodegroupDeletion, nil
}

func deleteNodeGroup(ctx context.Context, config *eksv1.EKSClusterConfig, ng eksv1.NodeGroup, eksService services.EKSServiceInterface) (*string, bool, error) {
	var templateVersionToDelete *string
	ngState, err := eksService.DescribeNodegroup(ctx,
		&eks.DescribeNodegroupInput{
			ClusterName:   aws.String(config.Spec.DisplayName),
			NodegroupName: ng.NodegroupName,
		})
	if err != nil {
		if notFound(err) {
			return templateVersionToDelete, false, nil
		}
		return templateVersionToDelete, false, err
	}

	if ngState.Nodegroup.Status != ekstypes.NodegroupStatusDeleting {
		_, err = eksService.DeleteNodegroup(ctx,
			&eks.DeleteNodegroupInput{
				ClusterName:   aws.String(config.Spec.DisplayName),
				NodegroupName: ng.NodegroupName,
			})
		if err != nil {
			return templateVersionToDelete, false, err
		}

		if ngState.Nodegroup.LaunchTemplate != nil &&
			aws.ToString(ngState.Nodegroup.LaunchTemplate.Id) == config.Status.ManagedLaunchTemplateID {
			templateVersionToDelete = ngState.Nodegroup.LaunchTemplate.Version
		}
	}

	return templateVersionToDelete, true, err
}

// getNodegroupConfigUpdate returns an UpdateNodegroupConfigInput that represents desired state and a bool
// indicating whether an update needs to take place to achieve the desired state.
func getNodegroupConfigUpdate(clusterName string, ng eksv1.NodeGroup, upstreamNg eksv1.NodeGroup) (eks.UpdateNodegroupConfigInput, bool) {
	nodegroupConfig := eks.UpdateNodegroupConfigInput{
		ClusterName:   aws.String(clusterName),
		NodegroupName: ng.NodegroupName,
		ScalingConfig: &ekstypes.NodegroupScalingConfig{},
	}
	var sendUpdateNodegroupConfig bool

	if ng.Labels != nil {
		unlabels := utils.GetKeysToDelete(aws.ToStringMap(ng.Labels), aws.ToStringMap(upstreamNg.Labels))
		labels := utils.GetKeyValuesToUpdate(aws.ToStringMap(ng.Labels), aws.ToStringMap(upstreamNg.Labels))

		if unlabels != nil || labels != nil {
			sendUpdateNodegroupConfig = true
			nodegroupConfig.Labels = &ekstypes.UpdateLabelsPayload{
				RemoveLabels:      unlabels,
				AddOrUpdateLabels: labels,
			}
		}
	}

	if ng.DesiredSize != nil {
		nodegroupConfig.ScalingConfig.DesiredSize = ng.DesiredSize
		if aws.ToInt32(upstreamNg.DesiredSize) != aws.ToInt32(ng.DesiredSize) {
			sendUpdateNodegroupConfig = true
		}
	}

	if ng.MinSize != nil {
		nodegroupConfig.ScalingConfig.MinSize = ng.MinSize
		if aws.ToInt32(upstreamNg.MinSize) != aws.ToInt32(ng.MinSize) {
			sendUpdateNodegroupConfig = true
		}
	}

	if ng.MaxSize != nil {
		nodegroupConfig.ScalingConfig.MaxSize = ng.MaxSize
		if aws.ToInt32(upstreamNg.MaxSize) != aws.ToInt32(ng.MaxSize) {
			sendUpdateNodegroupConfig = true
		}
	}

	return nodegroupConfig, sendUpdateNodegroupConfig
}
