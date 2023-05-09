package controller

import (
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/eks"
	eksv1 "github.com/rancher/eks-operator/pkg/apis/eks.cattle.io/v1"
	awsservices "github.com/rancher/eks-operator/pkg/eks"
	"github.com/rancher/eks-operator/pkg/eks/services"
	"github.com/rancher/eks-operator/utils"
	"github.com/sirupsen/logrus"
)

func newLaunchTemplateVersionIfNeeded(config *eksv1.EKSClusterConfig, upstreamNg, ng eksv1.NodeGroup, ec2Service services.EC2ServiceInterface) (*eksv1.LaunchTemplate, error) {
	if aws.StringValue(upstreamNg.UserData) != aws.StringValue(ng.UserData) ||
		aws.StringValue(upstreamNg.Ec2SshKey) != aws.StringValue(ng.Ec2SshKey) ||
		aws.Int64Value(upstreamNg.DiskSize) != aws.Int64Value(ng.DiskSize) ||
		aws.StringValue(upstreamNg.ImageID) != aws.StringValue(ng.ImageID) ||
		(!aws.BoolValue(upstreamNg.RequestSpotInstances) && aws.StringValue(upstreamNg.InstanceType) != aws.StringValue(ng.InstanceType)) ||
		!utils.CompareStringMaps(aws.StringValueMap(upstreamNg.ResourceTags), aws.StringValueMap(ng.ResourceTags)) {
		lt, err := awsservices.CreateNewLaunchTemplateVersion(ec2Service, config.Status.ManagedLaunchTemplateID, ng)
		if err != nil {
			return nil, err
		}

		return lt, nil
	}

	return nil, nil
}

func deleteLaunchTemplate(templateID string, ec2Service services.EC2ServiceInterface) {
	var err error
	for i := 0; i < 5; i++ {
		_, err = ec2Service.DeleteLaunchTemplate(&ec2.DeleteLaunchTemplateInput{
			LaunchTemplateId: aws.String(templateID),
		})

		if err == nil || doesNotExist(err) {
			return
		}

		time.Sleep(10 * time.Second)
	}

	logrus.Warnf("could not delete launch template [%s]: %v, will not retry",
		templateID,
		err,
	)
}

func deleteNodeGroups(config *eksv1.EKSClusterConfig, nodeGroups []eksv1.NodeGroup, eksService services.EKSServiceInterface) (bool, error) {
	var waitingForNodegroupDeletion bool
	for _, ng := range nodeGroups {
		_, deleteInProgress, err := deleteNodeGroup(config, ng, eksService)
		if err != nil {
			return false, err
		}
		waitingForNodegroupDeletion = waitingForNodegroupDeletion || deleteInProgress
	}

	return waitingForNodegroupDeletion, nil
}

func deleteNodeGroup(config *eksv1.EKSClusterConfig, ng eksv1.NodeGroup, eksService services.EKSServiceInterface) (*string, bool, error) {
	var templateVersionToDelete *string
	ngState, err := eksService.DescribeNodegroup(
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

	if aws.StringValue(ngState.Nodegroup.Status) != eks.NodegroupStatusDeleting {
		_, err = eksService.DeleteNodegroup(
			&eks.DeleteNodegroupInput{
				ClusterName:   aws.String(config.Spec.DisplayName),
				NodegroupName: ng.NodegroupName,
			})
		if err != nil {
			return templateVersionToDelete, false, err
		}

		if ngState.Nodegroup.LaunchTemplate != nil &&
			aws.StringValue(ngState.Nodegroup.LaunchTemplate.Id) == config.Status.ManagedLaunchTemplateID {
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
		ScalingConfig: &eks.NodegroupScalingConfig{},
	}
	var sendUpdateNodegroupConfig bool

	if ng.Labels != nil {
		unlabels := utils.GetKeysToDelete(aws.StringValueMap(ng.Labels), aws.StringValueMap(upstreamNg.Labels))
		labels := utils.GetKeyValuesToUpdate(aws.StringValueMap(ng.Labels), aws.StringValueMap(upstreamNg.Labels))

		if unlabels != nil || labels != nil {
			sendUpdateNodegroupConfig = true
			nodegroupConfig.Labels = &eks.UpdateLabelsPayload{
				RemoveLabels:      unlabels,
				AddOrUpdateLabels: labels,
			}
		}
	}

	if ng.DesiredSize != nil {
		nodegroupConfig.ScalingConfig.DesiredSize = ng.DesiredSize
		if aws.Int64Value(upstreamNg.DesiredSize) != aws.Int64Value(ng.DesiredSize) {
			sendUpdateNodegroupConfig = true
		}
	}

	if ng.MinSize != nil {
		nodegroupConfig.ScalingConfig.MinSize = ng.MinSize
		if aws.Int64Value(upstreamNg.MinSize) != aws.Int64Value(ng.MinSize) {
			sendUpdateNodegroupConfig = true
		}
	}

	if ng.MaxSize != nil {
		nodegroupConfig.ScalingConfig.MaxSize = ng.MaxSize
		if aws.Int64Value(upstreamNg.MaxSize) != aws.Int64Value(ng.MaxSize) {
			sendUpdateNodegroupConfig = true
		}
	}

	return nodegroupConfig, sendUpdateNodegroupConfig
}
