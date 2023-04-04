package controller

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/eks"
	eksv1 "github.com/rancher/eks-operator/pkg/apis/eks.cattle.io/v1"
	awsservices "github.com/rancher/eks-operator/pkg/eks"
	"github.com/rancher/eks-operator/pkg/eks/services"
	"github.com/rancher/eks-operator/templates"
	"github.com/rancher/eks-operator/utils"
	"github.com/sirupsen/logrus"
)

const (
	launchTemplateNameFormat = "rancher-managed-lt-%s"
	launchTemplateTagKey     = "rancher-managed-template"
	launchTemplateTagValue   = "do-not-modify-or-delete"
	defaultStorageDeviceName = "/dev/xvda"
)

func createLaunchTemplate(clusterDisplayName string, ec2Service services.EC2ServiceInterface) (*eksv1.LaunchTemplate, error) {
	// The first version of the rancher-managed launch template will be the default version.
	// Since the default version cannot be deleted until the launch template is deleted, it will not be used for any node group.
	// Also, launch templates cannot be created blank, so fake userdata is added to the first version.
	launchTemplateCreateInput := &ec2.CreateLaunchTemplateInput{
		LaunchTemplateData: &ec2.RequestLaunchTemplateData{UserData: aws.String("cGxhY2Vob2xkZXIK")},
		LaunchTemplateName: aws.String(fmt.Sprintf(launchTemplateNameFormat, clusterDisplayName)),
		TagSpecifications: []*ec2.TagSpecification{
			{
				ResourceType: aws.String(ec2.ResourceTypeLaunchTemplate),
				Tags: []*ec2.Tag{
					{
						Key:   aws.String(launchTemplateTagKey),
						Value: aws.String(launchTemplateTagValue),
					},
				},
			},
		},
	}

	awsLaunchTemplateOutput, err := ec2Service.CreateLaunchTemplate(launchTemplateCreateInput)
	if err != nil {
		return nil, err
	}

	return &eksv1.LaunchTemplate{
		Name:    awsLaunchTemplateOutput.LaunchTemplate.LaunchTemplateName,
		ID:      awsLaunchTemplateOutput.LaunchTemplate.LaunchTemplateId,
		Version: awsLaunchTemplateOutput.LaunchTemplate.LatestVersionNumber,
	}, nil
}

func createNewLaunchTemplateVersion(launchTemplateID string, group eksv1.NodeGroup, ec2Service services.EC2ServiceInterface) (*eksv1.LaunchTemplate, error) {
	launchTemplate, err := buildLaunchTemplateData(group, ec2Service)
	if err != nil {
		return nil, err
	}

	launchTemplateVersionInput := &ec2.CreateLaunchTemplateVersionInput{
		LaunchTemplateData: launchTemplate,
		LaunchTemplateId:   aws.String(launchTemplateID),
	}

	awsLaunchTemplateOutput, err := ec2Service.CreateLaunchTemplateVersion(launchTemplateVersionInput)
	if err != nil {
		return nil, err
	}

	return &eksv1.LaunchTemplate{
		Name:    awsLaunchTemplateOutput.LaunchTemplateVersion.LaunchTemplateName,
		ID:      awsLaunchTemplateOutput.LaunchTemplateVersion.LaunchTemplateId,
		Version: awsLaunchTemplateOutput.LaunchTemplateVersion.VersionNumber,
	}, nil
}

func buildLaunchTemplateData(group eksv1.NodeGroup, ec2Service services.EC2ServiceInterface) (*ec2.RequestLaunchTemplateData, error) {
	var imageID *string
	if aws.StringValue(group.ImageID) != "" {
		imageID = group.ImageID
	}

	userdata := group.UserData
	if aws.StringValue(userdata) != "" {
		if !strings.Contains(*userdata, "Content-Type: multipart/mixed") {
			return nil, fmt.Errorf("userdata for nodegroup [%s] is not of mime time multipart/mixed", aws.StringValue(group.NodegroupName))
		}
		*userdata = base64.StdEncoding.EncodeToString([]byte(*userdata))
	}

	deviceName := aws.String(defaultStorageDeviceName)
	if aws.StringValue(group.ImageID) != "" {
		if rootDeviceName, err := getImageRootDeviceName(group.ImageID, ec2Service); err != nil {
			return nil, err
		} else if rootDeviceName != nil {
			deviceName = rootDeviceName
		}
	}

	launchTemplateData := &ec2.RequestLaunchTemplateData{
		ImageId:  imageID,
		KeyName:  group.Ec2SshKey,
		UserData: userdata,
		BlockDeviceMappings: []*ec2.LaunchTemplateBlockDeviceMappingRequest{
			{
				DeviceName: deviceName,
				Ebs: &ec2.LaunchTemplateEbsBlockDeviceRequest{
					VolumeSize: group.DiskSize,
				},
			},
		},
		TagSpecifications: utils.CreateTagSpecs(group.ResourceTags),
	}
	if !aws.BoolValue(group.RequestSpotInstances) {
		launchTemplateData.InstanceType = group.InstanceType
	}

	return launchTemplateData, nil
}

func newLaunchTemplateVersionIfNeeded(config *eksv1.EKSClusterConfig, upstreamNg, ng eksv1.NodeGroup, ec2Service services.EC2ServiceInterface) (*eksv1.LaunchTemplate, error) {
	if aws.StringValue(upstreamNg.UserData) != aws.StringValue(ng.UserData) ||
		aws.StringValue(upstreamNg.Ec2SshKey) != aws.StringValue(ng.Ec2SshKey) ||
		aws.Int64Value(upstreamNg.DiskSize) != aws.Int64Value(ng.DiskSize) ||
		aws.StringValue(upstreamNg.ImageID) != aws.StringValue(ng.ImageID) ||
		(!aws.BoolValue(upstreamNg.RequestSpotInstances) && aws.StringValue(upstreamNg.InstanceType) != aws.StringValue(ng.InstanceType)) ||
		!utils.CompareStringMaps(aws.StringValueMap(upstreamNg.ResourceTags), aws.StringValueMap(ng.ResourceTags)) {
		lt, err := createNewLaunchTemplateVersion(config.Status.ManagedLaunchTemplateID, ng, ec2Service)
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

func deleteLaunchTemplateVersions(templateID string, templateVersions []*string, ec2Service services.EC2ServiceInterface) {
	launchTemplateDeleteVersionInput := &ec2.DeleteLaunchTemplateVersionsInput{
		LaunchTemplateId: aws.String(templateID),
		Versions:         templateVersions,
	}

	var err error
	var deleteVersionsOutput *ec2.DeleteLaunchTemplateVersionsOutput
	for i := 0; i < 5; i++ {
		deleteVersionsOutput, err = ec2Service.DeleteLaunchTemplateVersions(launchTemplateDeleteVersionInput)

		if deleteVersionsOutput != nil {
			templateVersions = templateVersions[:0]
			for _, version := range deleteVersionsOutput.UnsuccessfullyDeletedLaunchTemplateVersions {
				if !launchTemplateVersionDoesNotExist(aws.StringValue(version.ResponseError.Code)) {
					templateVersions = append(templateVersions, aws.String(strconv.Itoa(int(*version.VersionNumber))))
				}
			}
		}

		if err == nil || len(templateVersions) == 0 {
			return
		}

		launchTemplateDeleteVersionInput.Versions = templateVersions
		time.Sleep(10 * time.Second)
	}

	logrus.Warnf("could not delete versions [%v] of launch template [%s]: %v, will not retry",
		aws.StringValueSlice(templateVersions),
		*launchTemplateDeleteVersionInput.LaunchTemplateId,
		err,
	)
}

func createNodeGroup(config *eksv1.EKSClusterConfig, group eksv1.NodeGroup, eksService services.EKSServiceInterface,
	ec2Service services.EC2ServiceInterface, svc services.CloudFormationServiceInterface) (string, string, error) {
	var err error
	capacityType := eks.CapacityTypesOnDemand
	if aws.BoolValue(group.RequestSpotInstances) {
		capacityType = eks.CapacityTypesSpot
	}
	nodeGroupCreateInput := &eks.CreateNodegroupInput{
		ClusterName:   aws.String(config.Spec.DisplayName),
		NodegroupName: group.NodegroupName,
		Labels:        group.Labels,
		ScalingConfig: &eks.NodegroupScalingConfig{
			DesiredSize: group.DesiredSize,
			MaxSize:     group.MaxSize,
			MinSize:     group.MinSize,
		},
		CapacityType: aws.String(capacityType),
	}

	lt := group.LaunchTemplate

	if lt == nil {
		// In this case, the user has not specified their own launch template.
		// If the cluster doesn't have a launch template associated with it, then we create one.
		lt, err = createNewLaunchTemplateVersion(config.Status.ManagedLaunchTemplateID, group, ec2Service)
		if err != nil {
			return "", "", err
		}
	}

	var launchTemplateVersion *string
	if aws.Int64Value(lt.Version) != 0 {
		launchTemplateVersion = aws.String(strconv.FormatInt(*lt.Version, 10))
	}

	nodeGroupCreateInput.LaunchTemplate = &eks.LaunchTemplateSpecification{
		Id:      lt.ID,
		Version: launchTemplateVersion,
	}

	if aws.BoolValue(group.RequestSpotInstances) {
		nodeGroupCreateInput.InstanceTypes = group.SpotInstanceTypes
	}

	if aws.StringValue(group.ImageID) == "" {
		if gpu := group.Gpu; aws.BoolValue(gpu) {
			nodeGroupCreateInput.AmiType = aws.String(eks.AMITypesAl2X8664Gpu)
		} else {
			nodeGroupCreateInput.AmiType = aws.String(eks.AMITypesAl2X8664)
		}
	}

	if len(group.Subnets) != 0 {
		nodeGroupCreateInput.Subnets = aws.StringSlice(group.Subnets)
	} else {
		nodeGroupCreateInput.Subnets = aws.StringSlice(config.Status.Subnets)
	}

	generatedNodeRole := config.Status.GeneratedNodeRole

	if aws.StringValue(group.NodeRole) == "" {
		if config.Status.GeneratedNodeRole == "" {
			finalTemplate := fmt.Sprintf(templates.NodeInstanceRoleTemplate, getEC2ServiceEndpoint(config.Spec.Region))
			output, err := awsservices.CreateStack(awsservices.CreateStackOptions{
				CloudFormationService: svc,
				StackName:             fmt.Sprintf("%s-node-instance-role", config.Spec.DisplayName),
				DisplayName:           config.Spec.DisplayName,
				TemplateBody:          finalTemplate,
				Capabilities:          []string{cloudformation.CapabilityCapabilityIam},
				Parameters:            []*cloudformation.Parameter{},
			})
			if err != nil {
				// If there was an error creating the node role stack, return an empty launch template
				// version and the error.
				return "", "", err
			}
			generatedNodeRole = getParameterValueFromOutput("NodeInstanceRole", output.Stacks[0].Outputs)
		}
		nodeGroupCreateInput.NodeRole = aws.String(generatedNodeRole)
	} else {
		nodeGroupCreateInput.NodeRole = group.NodeRole
	}

	_, err = eksService.CreateNodegroup(nodeGroupCreateInput)
	if err != nil {
		// If there was an error creating the node group, then the template version should be deleted
		// to prevent many launch template versions from being created before the issue is fixed.
		deleteLaunchTemplateVersions(*lt.ID, []*string{launchTemplateVersion}, ec2Service)
	}

	// Return the launch template version and generated node role to the calling function so they can
	// be set on the Status.
	return aws.StringValue(launchTemplateVersion), generatedNodeRole, err
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

func getImageRootDeviceName(imageID *string, ec2Service services.EC2ServiceInterface) (*string, error) {
	describeOutput, err := ec2Service.DescribeImages(&ec2.DescribeImagesInput{ImageIds: []*string{imageID}})
	if err != nil {
		return nil, err
	}
	if len(describeOutput.Images) == 0 {
		return nil, fmt.Errorf("no images returned for id %v", aws.StringValue(imageID))
	}

	return describeOutput.Images[0].RootDeviceName, nil
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
