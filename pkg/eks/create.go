package eks

import (
	"context"
	"crypto/sha1"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/aws-sdk-go/aws/endpoints"

	eksv1 "github.com/rancher/eks-operator/pkg/apis/eks.cattle.io/v1"
	"github.com/rancher/eks-operator/pkg/eks/services"
	"github.com/rancher/eks-operator/templates"
	"github.com/rancher/eks-operator/utils"
)

const (
	// CloudFormation stack statuses
	createInProgressStatus   = "CREATE_IN_PROGRESS"
	createCompleteStatus     = "CREATE_COMPLETE"
	createFailedStatus       = "CREATE_FAILED"
	rollbackInProgressStatus = "ROLLBACK_IN_PROGRESS"

	LaunchTemplateNameFormat = "rancher-managed-lt-%s"
	launchTemplateTagKey     = "rancher-managed-template"
	launchTemplateTagValue   = "do-not-modify-or-delete"
	defaultStorageDeviceName = "/dev/xvda"

	defaultAudienceOpenIDConnect = "sts.amazonaws.com"
	ebsCSIAddonName              = "aws-ebs-csi-driver"
)

type CreateClusterOptions struct {
	EKSService services.EKSServiceInterface
	Config     *eksv1.EKSClusterConfig
	RoleARN    string
}

func CreateCluster(ctx context.Context, opts *CreateClusterOptions) error {
	createClusterInput := newClusterInput(opts.Config, opts.RoleARN)

	_, err := opts.EKSService.CreateCluster(ctx, createClusterInput)
	return err
}

func newClusterInput(config *eksv1.EKSClusterConfig, roleARN string) *eks.CreateClusterInput {
	createClusterInput := &eks.CreateClusterInput{
		Name:    aws.String(config.Spec.DisplayName),
		RoleArn: aws.String(roleARN),
		ResourcesVpcConfig: &ekstypes.VpcConfigRequest{
			EndpointPrivateAccess: config.Spec.PrivateAccess,
			EndpointPublicAccess:  config.Spec.PublicAccess,
			SecurityGroupIds:      config.Status.SecurityGroups,
			SubnetIds:             config.Status.Subnets,
			PublicAccessCidrs:     getPublicAccessCidrs(config.Spec.PublicAccessSources),
		},
		Tags:    getTags(config.Spec.Tags),
		Logging: getLogging(config.Spec.LoggingTypes),
		Version: config.Spec.KubernetesVersion,
	}

	if aws.ToBool(config.Spec.SecretsEncryption) {
		createClusterInput.EncryptionConfig = []ekstypes.EncryptionConfig{
			{
				Provider: &ekstypes.Provider{
					KeyArn: config.Spec.KmsKey,
				},
				Resources: []string{"secrets"},
			},
		}
	}

	return createClusterInput
}

type CreateStackOptions struct {
	CloudFormationService services.CloudFormationServiceInterface
	StackName             string
	DisplayName           string
	TemplateBody          string
	Capabilities          []cftypes.Capability
	Parameters            []cftypes.Parameter
}

func CreateStack(ctx context.Context, opts *CreateStackOptions) (*cloudformation.DescribeStacksOutput, error) {
	_, err := opts.CloudFormationService.CreateStack(ctx, &cloudformation.CreateStackInput{
		StackName:    aws.String(opts.StackName),
		TemplateBody: aws.String(opts.TemplateBody),
		Capabilities: opts.Capabilities,
		Parameters:   opts.Parameters,
		Tags: []cftypes.Tag{
			{
				Key:   aws.String("displayName"),
				Value: aws.String(opts.DisplayName),
			},
		},
	})
	if err != nil && !alreadyExistsInCloudFormationError(err) {
		return nil, fmt.Errorf("error creating master: %v", err)
	}

	var stack *cloudformation.DescribeStacksOutput
	status := createInProgressStatus

	for status == createInProgressStatus {
		time.Sleep(time.Second * 5)
		stack, err = opts.CloudFormationService.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{
			StackName: aws.String(opts.StackName),
		})
		if err != nil {
			return nil, fmt.Errorf("error polling stack info: %v", err)
		}

		if stack == nil || stack.Stacks == nil || len(stack.Stacks) == 0 {
			return nil, fmt.Errorf("stack did not have output: %v", err)
		}

		status = string(stack.Stacks[0].StackStatus)
	}

	if status != createCompleteStatus {
		reason := "reason unknown"
		events, err := opts.CloudFormationService.DescribeStackEvents(ctx, &cloudformation.DescribeStackEventsInput{
			StackName: aws.String(opts.StackName),
		})
		if err == nil {
			for _, event := range events.StackEvents {
				// guard against nil pointer dereference
				if event.LogicalResourceId == nil || event.ResourceStatusReason == nil {
					continue
				}

				if event.ResourceStatus == cftypes.ResourceStatusCreateFailed {
					reason = *event.ResourceStatusReason
					break
				}

				if event.ResourceStatus == cftypes.ResourceStatusRollbackInProgress {
					reason = *event.ResourceStatusReason
					// do not break so that CREATE_FAILED takes priority
				}
			}
		}
		return nil, fmt.Errorf("stack failed to create: %v", reason)
	}

	return stack, nil
}

type CreateLaunchTemplateOptions struct {
	EC2Service services.EC2ServiceInterface
	Config     *eksv1.EKSClusterConfig
}

func CreateLaunchTemplate(ctx context.Context, opts *CreateLaunchTemplateOptions) error {
	_, err := opts.EC2Service.DescribeLaunchTemplates(ctx, &ec2.DescribeLaunchTemplatesInput{
		LaunchTemplateIds: []string{opts.Config.Status.ManagedLaunchTemplateID},
	})
	if opts.Config.Status.ManagedLaunchTemplateID == "" || doesNotExist(err) {
		lt, err := createLaunchTemplate(ctx, opts.EC2Service, opts.Config.Spec.DisplayName)
		if err != nil {
			return fmt.Errorf("error creating launch template: %w", err)
		}
		opts.Config.Status.ManagedLaunchTemplateID = aws.ToString(lt.ID)
	} else if err != nil {
		return fmt.Errorf("error checking for existing launch template: %w", err)
	}

	return nil
}

func createLaunchTemplate(ctx context.Context, ec2Service services.EC2ServiceInterface, clusterDisplayName string) (*eksv1.LaunchTemplate, error) {
	// The first version of the rancher-managed launch template will be the default version.
	// Since the default version cannot be deleted until the launch template is deleted, it will not be used for any node group.
	// Also, launch templates cannot be created blank, so fake userdata is added to the first version.
	launchTemplateCreateInput := &ec2.CreateLaunchTemplateInput{
		LaunchTemplateData: &ec2types.RequestLaunchTemplateData{UserData: aws.String("cGxhY2Vob2xkZXIK")},
		LaunchTemplateName: aws.String(fmt.Sprintf(LaunchTemplateNameFormat, clusterDisplayName)),
		TagSpecifications: []ec2types.TagSpecification{
			{
				ResourceType: ec2types.ResourceTypeLaunchTemplate,
				Tags: []ec2types.Tag{
					{
						Key:   aws.String(launchTemplateTagKey),
						Value: aws.String(launchTemplateTagValue),
					},
				},
			},
		},
	}

	awsLaunchTemplateOutput, err := ec2Service.CreateLaunchTemplate(ctx, launchTemplateCreateInput)
	if err != nil {
		return nil, err
	}

	return &eksv1.LaunchTemplate{
		Name:    awsLaunchTemplateOutput.LaunchTemplate.LaunchTemplateName,
		ID:      awsLaunchTemplateOutput.LaunchTemplate.LaunchTemplateId,
		Version: awsLaunchTemplateOutput.LaunchTemplate.LatestVersionNumber,
	}, nil
}

type CreateNodeGroupOptions struct {
	EC2Service            services.EC2ServiceInterface
	CloudFormationService services.CloudFormationServiceInterface
	EKSService            services.EKSServiceInterface

	Config    *eksv1.EKSClusterConfig
	NodeGroup eksv1.NodeGroup
}

func CreateNodeGroup(ctx context.Context, opts *CreateNodeGroupOptions) (string, string, error) {
	var err error
	capacityType := ekstypes.CapacityTypesOnDemand
	if aws.ToBool(opts.NodeGroup.RequestSpotInstances) {
		capacityType = ekstypes.CapacityTypesSpot
	}
	nodeGroupCreateInput := &eks.CreateNodegroupInput{
		ClusterName:   aws.String(opts.Config.Spec.DisplayName),
		NodegroupName: opts.NodeGroup.NodegroupName,
		Labels:        aws.ToStringMap(opts.NodeGroup.Labels),
		ScalingConfig: &ekstypes.NodegroupScalingConfig{
			DesiredSize: opts.NodeGroup.DesiredSize,
			MaxSize:     opts.NodeGroup.MaxSize,
			MinSize:     opts.NodeGroup.MinSize,
		},
		CapacityType: capacityType,
	}

	lt := opts.NodeGroup.LaunchTemplate

	if len(opts.NodeGroup.ResourceTags) > 0 {
		nodeGroupCreateInput.Tags = opts.NodeGroup.ResourceTags
	}

	if lt == nil {
		// In this case, the user has not specified their own launch template.
		// If the cluster doesn't have a launch template associated with it, then we create one.
		lt, err = CreateNewLaunchTemplateVersion(ctx, opts.EC2Service, opts.Config.Status.ManagedLaunchTemplateID, opts.NodeGroup)
		if err != nil {
			return "", "", err
		}
	}

	var launchTemplateVersion *string
	if aws.ToInt64(lt.Version) != 0 {
		launchTemplateVersion = aws.String(strconv.FormatInt(*lt.Version, 10))
	}

	nodeGroupCreateInput.LaunchTemplate = &ekstypes.LaunchTemplateSpecification{
		Id:      lt.ID,
		Version: launchTemplateVersion,
	}

	if aws.ToBool(opts.NodeGroup.RequestSpotInstances) {
		nodeGroupCreateInput.InstanceTypes = opts.NodeGroup.SpotInstanceTypes
	}

	if aws.ToString(opts.NodeGroup.ImageID) == "" {
		if opts.NodeGroup.LaunchTemplate != nil {
			nodeGroupCreateInput.AmiType = ekstypes.AMITypesCustom
		} else if arm := opts.NodeGroup.Arm; aws.ToBool(arm) {
			nodeGroupCreateInput.AmiType = ekstypes.AMITypesAl2023Arm64Standard
		} else if gpu := opts.NodeGroup.Gpu; aws.ToBool(gpu) {
			nodeGroupCreateInput.AmiType = ekstypes.AMITypesAl2023X8664Nvidia
		} else {
			nodeGroupCreateInput.AmiType = ekstypes.AMITypesAl2023X8664Standard
		}
	}

	if len(opts.NodeGroup.Subnets) != 0 {
		nodeGroupCreateInput.Subnets = opts.NodeGroup.Subnets
	} else {
		nodeGroupCreateInput.Subnets = opts.Config.Status.Subnets
	}

	generatedNodeRole := opts.Config.Status.GeneratedNodeRole

	if aws.ToString(opts.NodeGroup.NodeRole) == "" {
		if opts.Config.Status.GeneratedNodeRole == "" {
			finalTemplate, err := templates.GetNodeInstanceRoleTemplate(opts.Config.Spec.Region)
			if err != nil {
				return "", "", err
			}

			output, err := CreateStack(ctx, &CreateStackOptions{
				CloudFormationService: opts.CloudFormationService,
				StackName:             fmt.Sprintf("%s-node-instance-role", opts.Config.Spec.DisplayName),
				DisplayName:           opts.Config.Spec.DisplayName,
				TemplateBody:          finalTemplate,
				Capabilities:          []cftypes.Capability{cftypes.CapabilityCapabilityIam},
				Parameters:            []cftypes.Parameter{},
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
		nodeGroupCreateInput.NodeRole = opts.NodeGroup.NodeRole
	}

	_, err = opts.EKSService.CreateNodegroup(ctx, nodeGroupCreateInput)
	if err != nil && lt.ID != nil {
		// If there was an error creating the node group, then the template version should be deleted
		// to prevent many launch template versions from being created before the issue is fixed.
		DeleteLaunchTemplateVersions(ctx, opts.EC2Service, *lt.ID, []*string{launchTemplateVersion})
	}

	// Return the launch template version and generated node role to the calling function so they can
	// be set on the Status.
	return aws.ToString(launchTemplateVersion), generatedNodeRole, err
}

func CreateNewLaunchTemplateVersion(ctx context.Context, ec2Service services.EC2ServiceInterface, launchTemplateID string, group eksv1.NodeGroup) (*eksv1.LaunchTemplate, error) {
	launchTemplate, err := buildLaunchTemplateData(ctx, ec2Service, group)
	if err != nil {
		return nil, err
	}

	launchTemplateVersionInput := &ec2.CreateLaunchTemplateVersionInput{
		LaunchTemplateData: launchTemplate,
		LaunchTemplateId:   aws.String(launchTemplateID),
	}

	awsLaunchTemplateOutput, err := ec2Service.CreateLaunchTemplateVersion(ctx, launchTemplateVersionInput)
	if err != nil {
		return nil, err
	}

	return &eksv1.LaunchTemplate{
		Name:    awsLaunchTemplateOutput.LaunchTemplateVersion.LaunchTemplateName,
		ID:      awsLaunchTemplateOutput.LaunchTemplateVersion.LaunchTemplateId,
		Version: awsLaunchTemplateOutput.LaunchTemplateVersion.VersionNumber,
	}, nil
}

func buildLaunchTemplateData(ctx context.Context, ec2Service services.EC2ServiceInterface, group eksv1.NodeGroup) (*ec2types.RequestLaunchTemplateData, error) {
	var imageID *string
	if aws.ToString(group.ImageID) != "" {
		imageID = group.ImageID
	}

	userdata := group.UserData
	if aws.ToString(userdata) != "" {
		if !strings.Contains(*userdata, "Content-Type: multipart/mixed") {
			return nil, fmt.Errorf("userdata for nodegroup [%s] is not of mime time multipart/mixed", aws.ToString(group.NodegroupName))
		}
		*userdata = base64.StdEncoding.EncodeToString([]byte(*userdata))
	}

	deviceName := aws.String(defaultStorageDeviceName)
	if aws.ToString(group.ImageID) != "" {
		if rootDeviceName, err := getImageRootDeviceName(ctx, ec2Service, group.ImageID); err != nil {
			return nil, err
		} else if rootDeviceName != nil {
			deviceName = rootDeviceName
		}
	}

	launchTemplateData := &ec2types.RequestLaunchTemplateData{
		ImageId:  imageID,
		KeyName:  group.Ec2SshKey,
		UserData: userdata,
		BlockDeviceMappings: []ec2types.LaunchTemplateBlockDeviceMappingRequest{
			{
				DeviceName: deviceName,
				Ebs: &ec2types.LaunchTemplateEbsBlockDeviceRequest{
					VolumeSize: group.DiskSize,
				},
			},
		},
		TagSpecifications: utils.CreateTagSpecs(group.ResourceTags),
	}
	if !aws.ToBool(group.RequestSpotInstances) {
		launchTemplateData.InstanceType = ec2types.InstanceType(group.InstanceType)
	}

	return launchTemplateData, nil
}

func getImageRootDeviceName(ctx context.Context, ec2Service services.EC2ServiceInterface, imageID *string) (*string, error) {
	if imageID == nil {
		return nil, fmt.Errorf("imageID is nil")
	}
	describeOutput, err := ec2Service.DescribeImages(ctx, &ec2.DescribeImagesInput{ImageIds: []string{aws.ToString(imageID)}})
	if err != nil {
		return nil, err
	}
	if len(describeOutput.Images) == 0 {
		return nil, fmt.Errorf("no images returned for id %v", aws.ToString(imageID))
	}

	return describeOutput.Images[0].RootDeviceName, nil
}

func getTags(tags map[string]string) map[string]string {
	if len(tags) == 0 {
		return nil
	}

	return tags
}

func getLogging(loggingTypes []string) *ekstypes.Logging {
	if len(loggingTypes) == 0 {
		return &ekstypes.Logging{
			ClusterLogging: []ekstypes.LogSetup{
				{
					Enabled: aws.Bool(false),
					Types:   []ekstypes.LogType{},
				},
			},
		}
	}
	return &ekstypes.Logging{
		ClusterLogging: []ekstypes.LogSetup{
			{
				Enabled: aws.Bool(true),
				Types:   utils.ConvertToLogTypes(loggingTypes),
			},
		},
	}
}

func getPublicAccessCidrs(publicAccessCidrs []string) []string {
	if len(publicAccessCidrs) == 0 {
		return []string{"0.0.0.0/0"}
	}

	return publicAccessCidrs
}

func alreadyExistsInCloudFormationError(err error) bool {
	var aee *cftypes.AlreadyExistsException
	return errors.As(err, &aee)
}

func doesNotExist(err error) bool {
	// There is no better way of doing this because AWS API does not distinguish between a attempt to delete a stack
	// (or key pair) that does not exist, and, for example, a malformed delete request, so we have to parse the error
	// message
	if err != nil {
		return strings.Contains(err.Error(), "does not exist")
	}

	return false
}

func getEC2ServiceEndpoint(region string) string {
	if p, ok := endpoints.PartitionForRegion(endpoints.DefaultPartitions(), region); ok {
		return fmt.Sprintf("ec2.%s", p.DNSSuffix())
	}
	return "ec2.amazonaws.com"
}

func getParameterValueFromOutput(key string, outputs []cftypes.Output) string {
	for _, output := range outputs {
		if *output.OutputKey == key {
			return *output.OutputValue
		}
	}

	return ""
}

// EnableEBSCSIDriverInput holds the options for enabling the EBS CSI driver
type EnableEBSCSIDriverInput struct {
	EKSService   services.EKSServiceInterface
	IAMService   services.IAMServiceInterface
	CFService    services.CloudFormationServiceInterface
	Config       *eksv1.EKSClusterConfig
	AddonVersion string
}

// EnableEBSCSIDriver manages the installation of the EBS CSI driver for EKS, including the
// creation of the OIDC Provider, the IAM role and the validation and installation of the EKS add-on
func EnableEBSCSIDriver(ctx context.Context, opts *EnableEBSCSIDriverInput) error {
	oidcID, err := configureOIDCProvider(ctx, opts.IAMService, opts.EKSService, opts.Config)
	if err != nil {
		return fmt.Errorf("could not configure oidc provider: %w", err)
	}
	roleArn, err := createEBSCSIDriverRole(ctx, opts.CFService, opts.Config, oidcID)
	if err != nil {
		return fmt.Errorf("could not create ebs csi driver role: %w", err)
	}
	if _, err := installEBSAddon(ctx, opts.EKSService, opts.Config, roleArn, opts.AddonVersion); err != nil {
		return fmt.Errorf("failed to install ebs csi driver addon: %w", err)
	}

	return nil
}

func configureOIDCProvider(ctx context.Context, iamService services.IAMServiceInterface, eksService services.EKSServiceInterface, config *eksv1.EKSClusterConfig) (string, error) {
	output, err := iamService.ListOIDCProviders(ctx, &iam.ListOpenIDConnectProvidersInput{})
	if err != nil {
		return "", err
	}
	clusterOutput, err := eksService.DescribeCluster(ctx, &eks.DescribeClusterInput{
		Name: aws.String(config.Spec.DisplayName),
	})
	if err != nil {
		return "", err
	}
	if clusterOutput == nil {
		return "", fmt.Errorf("could not find cluster [%s (id: %s)]", config.Spec.DisplayName, config.Name)
	}
	id := path.Base(*clusterOutput.Cluster.Identity.Oidc.Issuer)

	for _, prov := range output.OpenIDConnectProviderList {
		if strings.Contains(*prov.Arn, id) {
			return "", nil
		}
	}

	thumbprint, err := getIssuerThumbprint(*clusterOutput.Cluster.Identity.Oidc.Issuer)
	if err != nil {
		return "", err
	}
	input := &iam.CreateOpenIDConnectProviderInput{
		ClientIDList:   []string{string(defaultAudienceOpenIDConnect)},
		ThumbprintList: []string{thumbprint},
		Url:            clusterOutput.Cluster.Identity.Oidc.Issuer,
		Tags:           []iamtypes.Tag{},
	}
	newOIDC, err := iamService.CreateOIDCProvider(ctx, input)
	if err != nil {
		return "", err
	}

	return path.Base(*newOIDC.OpenIDConnectProviderArn), nil
}

func getIssuerThumbprint(issuer string) (string, error) {
	issuerURL, err := url.Parse(issuer)
	if err != nil {
		return "", err
	}
	if issuerURL.Port() == "" {
		issuerURL.Host += ":443"
	}

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
				MinVersion:         tls.VersionTLS12,
			},
			Proxy: http.ProxyFromEnvironment,
		},
	}
	resp, err := client.Get(issuerURL.String())
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.TLS == nil || len(resp.TLS.PeerCertificates) == 0 {
		return "", err
	}

	root := resp.TLS.PeerCertificates[len(resp.TLS.PeerCertificates)-1]

	return fmt.Sprintf("%x", sha1.Sum(root.Raw)), nil
}

func createEBSCSIDriverRole(ctx context.Context, cfService services.CloudFormationServiceInterface, config *eksv1.EKSClusterConfig, oidcID string) (string, error) {
	finalTemplate, err := templates.GetEBSCSIDriverTemplate(config.Spec.Region, oidcID)
	if err != nil {
		return "", err
	}

	output, err := CreateStack(ctx, &CreateStackOptions{
		CloudFormationService: cfService,
		StackName:             fmt.Sprintf("%s-ebs-csi-driver-role", config.Spec.DisplayName),
		DisplayName:           config.Spec.DisplayName,
		TemplateBody:          finalTemplate,
		Capabilities:          []cftypes.Capability{cftypes.CapabilityCapabilityIam},
		Parameters:            []cftypes.Parameter{},
	})
	if err != nil {
		return "", err
	}
	createdRoleArn := getParameterValueFromOutput("EBSCSIDriverRole", output.Stacks[0].Outputs)

	return createdRoleArn, nil
}

func installEBSAddon(ctx context.Context, eksService services.EKSServiceInterface, config *eksv1.EKSClusterConfig, roleArn, version string) (string, error) {
	input := eks.CreateAddonInput{
		AddonName:             aws.String(ebsCSIAddonName),
		ClusterName:           aws.String(config.Spec.DisplayName),
		ServiceAccountRoleArn: aws.String(roleArn),
	}
	if version != "latest" {
		input.AddonVersion = aws.String(version)
	}

	addonOutput, err := eksService.CreateAddon(ctx, &input)
	if err != nil {
		return "", err
	}
	if addonOutput == nil {
		return "", fmt.Errorf("could not create addon [%s] for cluster [%s (id: %s)]", ebsCSIAddonName, config.Spec.DisplayName, config.Name)
	}

	return *addonOutput.Addon.AddonArn, nil
}
