package eks

import (
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	eksv1 "github.com/rancher/eks-operator/pkg/apis/eks.cattle.io/v1"
	"github.com/rancher/eks-operator/pkg/eks/services/mock_services"
	"github.com/rancher/eks-operator/utils"
)

var _ = Describe("CreateCluster", func() {
	var (
		mockController        *gomock.Controller
		eksServiceMock        *mock_services.MockEKSServiceInterface
		clustercCreateOptions *CreateClusterOptions
	)

	BeforeEach(func() {
		mockController = gomock.NewController(GinkgoT())
		eksServiceMock = mock_services.NewMockEKSServiceInterface(mockController)
		clustercCreateOptions = &CreateClusterOptions{
			EKSService: eksServiceMock,
			RoleARN:    "test",
			Config:     &eksv1.EKSClusterConfig{},
		}
	})

	AfterEach(func() {
		mockController.Finish()
	})

	It("should successfully create a cluster", func() {
		eksServiceMock.EXPECT().CreateCluster(gomock.Any()).Return(nil, nil)
		Expect(CreateCluster(clustercCreateOptions)).To(Succeed())
	})

	It("should fail to create a cluster", func() {
		eksServiceMock.EXPECT().CreateCluster(gomock.Any()).Return(nil, errors.New("error creating cluster"))
		Expect(CreateCluster(clustercCreateOptions)).ToNot(Succeed())
	})
})

var _ = Describe("newClusterInput", func() {
	var (
		roleARN string
		config  *eksv1.EKSClusterConfig
	)

	BeforeEach(func() {
		roleARN = "test"
		config = &eksv1.EKSClusterConfig{
			Spec: eksv1.EKSClusterConfigSpec{
				DisplayName:         "test",
				PrivateAccess:       aws.Bool(true),
				PublicAccess:        aws.Bool(true),
				PublicAccessSources: []string{"test"},
				Tags:                map[string]string{"test": "test"},
				LoggingTypes:        []string{"test"},
				KubernetesVersion:   aws.String("test"),
				SecretsEncryption:   aws.Bool(true),
				KmsKey:              aws.String("test"),
			},
			Status: eksv1.EKSClusterConfigStatus{
				SecurityGroups: []string{"test"},
				Subnets:        []string{"test"},
			},
		}
	})

	It("should successfully create a cluster input", func() {
		clusterInput := newClusterInput(config, roleARN)
		Expect(clusterInput).ToNot(BeNil())

		Expect(clusterInput.Name).To(Equal(aws.String(config.Spec.DisplayName)))
		Expect(clusterInput.RoleArn).To(Equal(aws.String(roleARN)))
		Expect(clusterInput.ResourcesVpcConfig).ToNot(BeNil())
		Expect(clusterInput.ResourcesVpcConfig.SecurityGroupIds).To(Equal(aws.StringSlice(config.Status.SecurityGroups)))
		Expect(clusterInput.ResourcesVpcConfig.SubnetIds).To(Equal(aws.StringSlice(config.Status.Subnets)))
		Expect(clusterInput.ResourcesVpcConfig.EndpointPrivateAccess).To(Equal(config.Spec.PrivateAccess))
		Expect(clusterInput.ResourcesVpcConfig.EndpointPublicAccess).To(Equal(config.Spec.PublicAccess))
		Expect(clusterInput.ResourcesVpcConfig.PublicAccessCidrs).To(Equal(aws.StringSlice(config.Spec.PublicAccessSources)))
		Expect(clusterInput.Tags).To(Equal(aws.StringMap(config.Spec.Tags)))
		Expect(clusterInput.Logging.ClusterLogging).To(HaveLen(1))
		Expect(clusterInput.Logging.ClusterLogging[0].Enabled).To(Equal(aws.Bool(true)))
		Expect(clusterInput.Logging.ClusterLogging[0].Types).To(Equal(aws.StringSlice(config.Spec.LoggingTypes)))
		Expect(clusterInput.Version).To(Equal(config.Spec.KubernetesVersion))
		Expect(clusterInput.EncryptionConfig).To(HaveLen(1))
		Expect(clusterInput.EncryptionConfig[0].Provider.KeyArn).To(Equal(config.Spec.KmsKey))
		Expect(clusterInput.EncryptionConfig[0].Resources).To(Equal(aws.StringSlice([]string{"secrets"})))
	})

	It("should successfully create a cluster input with no public access cidrs set", func() {
		config.Spec.PublicAccessSources = []string{}
		clusterInput := newClusterInput(config, roleARN)
		Expect(clusterInput).ToNot(BeNil())

		Expect(clusterInput.ResourcesVpcConfig.PublicAccessCidrs).ToNot(BeNil())
		Expect(clusterInput.ResourcesVpcConfig.PublicAccessCidrs).To(Equal(aws.StringSlice([]string{"0.0.0.0/0"})))
	})

	It("should successfully create a cluster with no tags set", func() {
		config.Spec.Tags = map[string]string{}
		clusterInput := newClusterInput(config, roleARN)
		Expect(clusterInput).ToNot(BeNil())

		Expect(clusterInput.Tags).To(BeNil())
	})

	It("should successfully create a cluster with no logging types set", func() {
		config.Spec.LoggingTypes = []string{}
		clusterInput := newClusterInput(config, roleARN)
		Expect(clusterInput).ToNot(BeNil())

		Expect(clusterInput.Logging.ClusterLogging).To(HaveLen(1))
		Expect(clusterInput.Logging.ClusterLogging[0].Enabled).To(Equal(aws.Bool(false)))
		Expect(clusterInput.Logging.ClusterLogging[0].Types).To(Equal(aws.StringSlice(config.Spec.LoggingTypes)))
	})

	It("should successfully create a cluster with no secrets encryption set", func() {
		config.Spec.SecretsEncryption = aws.Bool(false)
		clusterInput := newClusterInput(config, roleARN)
		Expect(clusterInput).ToNot(BeNil())

		Expect(clusterInput.EncryptionConfig).To(BeNil())
	})
})

var _ = Describe("CreateStack", func() {
	var (
		mockController            *gomock.Controller
		cloudFormationServiceMock *mock_services.MockCloudFormationServiceInterface
		stackCreationOptions      *CreateStackOptions
	)

	BeforeEach(func() {
		mockController = gomock.NewController(GinkgoT())
		cloudFormationServiceMock = mock_services.NewMockCloudFormationServiceInterface(mockController)
		stackCreationOptions = &CreateStackOptions{
			CloudFormationService: cloudFormationServiceMock,
			StackName:             "test",
			DisplayName:           "test",
			TemplateBody:          "test-body",
			Capabilities:          []string{"test"},
			Parameters:            []*cloudformation.Parameter{{ParameterKey: aws.String("test"), ParameterValue: aws.String("test")}},
		}
	})

	AfterEach(func() {
		mockController.Finish()
	})

	It("should successfully create a stack", func() {
		cloudFormationServiceMock.EXPECT().CreateStack(&cloudformation.CreateStackInput{
			StackName:    &stackCreationOptions.StackName,
			TemplateBody: &stackCreationOptions.TemplateBody,
			Capabilities: aws.StringSlice(stackCreationOptions.Capabilities),
			Parameters:   stackCreationOptions.Parameters,
			Tags: []*cloudformation.Tag{
				{
					Key:   aws.String("displayName"),
					Value: aws.String(stackCreationOptions.DisplayName),
				},
			},
		}).Return(nil, nil)

		cloudFormationServiceMock.EXPECT().DescribeStacks(
			&cloudformation.DescribeStacksInput{
				StackName: &stackCreationOptions.StackName,
			},
		).Return(
			&cloudformation.DescribeStacksOutput{
				Stacks: []*cloudformation.Stack{
					{
						StackStatus: aws.String(createCompleteStatus),
					},
				},
			}, nil)

		describeStacksOutput, err := CreateStack(stackCreationOptions)
		Expect(err).ToNot(HaveOccurred())

		Expect(describeStacksOutput).ToNot(BeNil())
	})

	It("should fail to create a stack if CreateStack returns error", func() {
		cloudFormationServiceMock.EXPECT().CreateStack(gomock.Any()).Return(nil, errors.New("error"))

		_, err := CreateStack(stackCreationOptions)
		Expect(err).To(HaveOccurred())
	})

	It("should fail to create a stack if DescribeStacks returns no stacks", func() {
		cloudFormationServiceMock.EXPECT().CreateStack(gomock.Any()).Return(nil, nil)

		cloudFormationServiceMock.EXPECT().DescribeStacks(
			&cloudformation.DescribeStacksInput{
				StackName: &stackCreationOptions.StackName,
			},
		).Return(&cloudformation.DescribeStacksOutput{}, nil)

		_, err := CreateStack(stackCreationOptions)
		Expect(err).To(HaveOccurred())
	})

	It("should fail to create a stack if stack already exists", func() {
		cloudFormationServiceMock.EXPECT().CreateStack(gomock.Any()).Return(nil, awserr.New(cloudformation.ErrCodeAlreadyExistsException, "", nil))
		cloudFormationServiceMock.EXPECT().DescribeStacks(gomock.Any()).Return(
			&cloudformation.DescribeStacksOutput{
				Stacks: []*cloudformation.Stack{
					{
						StackStatus: aws.String(createCompleteStatus),
					},
				},
			}, nil)

		_, err := CreateStack(stackCreationOptions)
		Expect(err).ToNot(HaveOccurred())
	})

	It("should fail to create a stack if DescribeStack return errors", func() {
		cloudFormationServiceMock.EXPECT().CreateStack(gomock.Any()).Return(nil, nil)
		cloudFormationServiceMock.EXPECT().DescribeStacks(gomock.Any()).Return(nil, errors.New("error"))

		_, err := CreateStack(stackCreationOptions)
		Expect(err).To(HaveOccurred())
	})

	It("should fail to create a stack if stack status is CREATE_FAILED", func() {
		cloudFormationServiceMock.EXPECT().CreateStack(gomock.Any()).Return(nil, nil)
		cloudFormationServiceMock.EXPECT().DescribeStacks(gomock.Any()).Return(
			&cloudformation.DescribeStacksOutput{
				Stacks: []*cloudformation.Stack{
					{
						StackStatus: aws.String(createFailedStatus),
					},
				},
			}, nil)
		cloudFormationServiceMock.EXPECT().DescribeStackEvents(
			&cloudformation.DescribeStackEventsInput{
				StackName: &stackCreationOptions.StackName,
			},
		).Return(
			&cloudformation.DescribeStackEventsOutput{
				StackEvents: []*cloudformation.StackEvent{
					{
						ResourceStatus:       aws.String(createFailedStatus),
						ResourceStatusReason: aws.String(createFailedStatus),
						LogicalResourceId:    aws.String("test"),
					},
				},
			}, nil)

		_, err := CreateStack(stackCreationOptions)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring(createFailedStatus))
	})

	It("should fail to create a stack if stack status is ROLLBACK_IN_PROGRESS", func() {
		cloudFormationServiceMock.EXPECT().CreateStack(gomock.Any()).Return(nil, nil)
		cloudFormationServiceMock.EXPECT().DescribeStacks(gomock.Any()).Return(
			&cloudformation.DescribeStacksOutput{
				Stacks: []*cloudformation.Stack{
					{
						StackStatus: aws.String(rollbackInProgressStatus),
					},
				},
			}, nil)
		cloudFormationServiceMock.EXPECT().DescribeStackEvents(
			&cloudformation.DescribeStackEventsInput{
				StackName: &stackCreationOptions.StackName,
			},
		).Return(
			&cloudformation.DescribeStackEventsOutput{
				StackEvents: []*cloudformation.StackEvent{
					{
						ResourceStatus:       aws.String(rollbackInProgressStatus),
						ResourceStatusReason: aws.String(rollbackInProgressStatus),
						LogicalResourceId:    aws.String("test"),
					},
				},
			}, nil)

		_, err := CreateStack(stackCreationOptions)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring(rollbackInProgressStatus))
	})
})

var _ = Describe("createLaunchTemplate", func() {
	var (
		mockController     *gomock.Controller
		ec2ServiceMock     *mock_services.MockEC2ServiceInterface
		clusterDisplayName = "testName"
	)

	BeforeEach(func() {
		mockController = gomock.NewController(GinkgoT())
		ec2ServiceMock = mock_services.NewMockEC2ServiceInterface(mockController)
	})

	AfterEach(func() {
		mockController.Finish()
	})

	It("should create a launch template", func() {
		expectedOutput := &ec2.CreateLaunchTemplateOutput{
			LaunchTemplate: &ec2.LaunchTemplate{
				LaunchTemplateName:   aws.String("testName"),
				LaunchTemplateId:     aws.String("testID"),
				DefaultVersionNumber: aws.Int64(1),
			},
		}
		ec2ServiceMock.EXPECT().CreateLaunchTemplate(
			&ec2.CreateLaunchTemplateInput{
				LaunchTemplateData: &ec2.RequestLaunchTemplateData{UserData: aws.String("cGxhY2Vob2xkZXIK")},
				LaunchTemplateName: aws.String(fmt.Sprintf(LaunchTemplateNameFormat, clusterDisplayName)),
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
			},
		).Return(expectedOutput, nil)
		launchTemplate, err := createLaunchTemplate(ec2ServiceMock, clusterDisplayName)
		Expect(err).ToNot(HaveOccurred())
		Expect(launchTemplate).ToNot(BeNil())

		Expect(launchTemplate.Name).To(Equal(expectedOutput.LaunchTemplate.LaunchTemplateName))
		Expect(launchTemplate.ID).To(Equal(expectedOutput.LaunchTemplate.LaunchTemplateId))
		Expect(launchTemplate.Version).To(Equal(expectedOutput.LaunchTemplate.LatestVersionNumber))
	})

	It("should fail to create a launch template", func() {
		ec2ServiceMock.EXPECT().CreateLaunchTemplate(gomock.Any()).Return(nil, errors.New("error"))
		_, err := createLaunchTemplate(ec2ServiceMock, clusterDisplayName)
		Expect(err).To(HaveOccurred())
	})
})

var _ = Describe("CreateLaunchTemplate", func() {
	var (
		mockController           *gomock.Controller
		ec2ServiceMock           *mock_services.MockEC2ServiceInterface
		createLaunchTemplateOpts *CreateLaunchTemplateOptions
	)

	BeforeEach(func() {
		mockController = gomock.NewController(GinkgoT())
		ec2ServiceMock = mock_services.NewMockEC2ServiceInterface(mockController)
		createLaunchTemplateOpts = &CreateLaunchTemplateOptions{
			EC2Service: ec2ServiceMock,
			Config: &eksv1.EKSClusterConfig{
				Spec: eksv1.EKSClusterConfigSpec{
					DisplayName: "test",
				},
				Status: eksv1.EKSClusterConfigStatus{
					ManagedLaunchTemplateID: "test",
				},
			},
		}
	})

	AfterEach(func() {
		mockController.Finish()
	})

	It("should create a launch template if managed launch template ID is not set", func() {
		createLaunchTemplateOpts.Config.Status.ManagedLaunchTemplateID = ""
		ec2ServiceMock.EXPECT().CreateLaunchTemplate(gomock.Any()).Return(&ec2.CreateLaunchTemplateOutput{
			LaunchTemplate: &ec2.LaunchTemplate{
				LaunchTemplateName:   aws.String("testName"),
				LaunchTemplateId:     aws.String("testID"),
				DefaultVersionNumber: aws.Int64(1),
			},
		}, nil)

		ec2ServiceMock.EXPECT().DescribeLaunchTemplates(
			&ec2.DescribeLaunchTemplatesInput{
				LaunchTemplateIds: []*string{aws.String(createLaunchTemplateOpts.Config.Status.ManagedLaunchTemplateID)},
			},
		).Return(nil, nil)

		Expect(CreateLaunchTemplate(createLaunchTemplateOpts)).To(Succeed())
		Expect(createLaunchTemplateOpts.Config.Status.ManagedLaunchTemplateID).To(Equal("testID"))
	})

	It("should create a launch template if managed launch template doesn't exist", func() {
		ec2ServiceMock.EXPECT().CreateLaunchTemplate(gomock.Any()).Return(&ec2.CreateLaunchTemplateOutput{
			LaunchTemplate: &ec2.LaunchTemplate{
				LaunchTemplateName:   aws.String("testName"),
				LaunchTemplateId:     aws.String("testID"),
				DefaultVersionNumber: aws.Int64(1),
			},
		}, nil)

		ec2ServiceMock.EXPECT().DescribeLaunchTemplates(
			&ec2.DescribeLaunchTemplatesInput{
				LaunchTemplateIds: []*string{aws.String(createLaunchTemplateOpts.Config.Status.ManagedLaunchTemplateID)},
			},
		).Return(nil, errors.New("does not exist"))

		Expect(CreateLaunchTemplate(createLaunchTemplateOpts)).To(Succeed())
		Expect(createLaunchTemplateOpts.Config.Status.ManagedLaunchTemplateID).To(Equal("testID"))
	})

	It("should not create a launch template if managed launch template exists", func() {
		ec2ServiceMock.EXPECT().DescribeLaunchTemplates(
			&ec2.DescribeLaunchTemplatesInput{
				LaunchTemplateIds: []*string{aws.String(createLaunchTemplateOpts.Config.Status.ManagedLaunchTemplateID)},
			},
		).Return(nil, nil)

		Expect(CreateLaunchTemplate(createLaunchTemplateOpts)).To(Succeed())
	})

	It("should fail to create a launch template if DescribeLaunchTemplates returns error", func() {
		ec2ServiceMock.EXPECT().DescribeLaunchTemplates(gomock.Any()).Return(nil, errors.New("error"))
		Expect(CreateLaunchTemplate(createLaunchTemplateOpts)).ToNot(Succeed())
	})

	It("should fail to create a launch template if CreateLaunchTemplate return error", func() {
		createLaunchTemplateOpts.Config.Status.ManagedLaunchTemplateID = ""
		ec2ServiceMock.EXPECT().DescribeLaunchTemplates(gomock.Any()).Return(nil, nil)

		ec2ServiceMock.EXPECT().CreateLaunchTemplate(gomock.Any()).Return(nil, errors.New("error"))

		Expect(CreateLaunchTemplate(createLaunchTemplateOpts)).ToNot(Succeed())
	})
})

var _ = Describe("getImageRootDeviceName", func() {
	var (
		mockController *gomock.Controller
		ec2ServiceMock *mock_services.MockEC2ServiceInterface
		imageID        = "test-image-id"
	)

	BeforeEach(func() {
		mockController = gomock.NewController(GinkgoT())
		ec2ServiceMock = mock_services.NewMockEC2ServiceInterface(mockController)
	})

	AfterEach(func() {
		mockController.Finish()
	})

	It("should get the root device name", func() {
		exptectedRootDeviceName := "test-root-device-name"
		ec2ServiceMock.EXPECT().DescribeImages(
			&ec2.DescribeImagesInput{
				ImageIds: []*string{&imageID},
			},
		).Return(
			&ec2.DescribeImagesOutput{
				Images: []*ec2.Image{
					{
						RootDeviceName: &exptectedRootDeviceName,
					},
				},
			},
			nil)

		rootDeviceName, err := getImageRootDeviceName(ec2ServiceMock, &imageID)
		Expect(err).ToNot(HaveOccurred())

		Expect(rootDeviceName).To(Equal(&exptectedRootDeviceName))
	})

	It("should fail to get the root device name if image is nil", func() {
		_, err := getImageRootDeviceName(ec2ServiceMock, nil)
		Expect(err).To(HaveOccurred())
	})

	It("should fail to get the root device name if error is return by ec2", func() {
		ec2ServiceMock.EXPECT().DescribeImages(gomock.Any()).Return(nil, errors.New("error"))
		_, err := getImageRootDeviceName(ec2ServiceMock, &imageID)
		Expect(err).To(HaveOccurred())
	})
})

var _ = Describe("buildLaunchTemplateData", func() {
	var (
		mockController *gomock.Controller
		ec2ServiceMock *mock_services.MockEC2ServiceInterface
		group          *eksv1.NodeGroup
	)

	BeforeEach(func() {
		mockController = gomock.NewController(GinkgoT())
		ec2ServiceMock = mock_services.NewMockEC2ServiceInterface(mockController)
		group = &eksv1.NodeGroup{
			ImageID:      aws.String("test-ami"),
			UserData:     aws.String("Content-Type: multipart/mixed ..."),
			DiskSize:     aws.Int64(20),
			ResourceTags: aws.StringMap(map[string]string{"test": "test"}),
			InstanceType: aws.String("test-instance-type"),
			Ec2SshKey:    aws.String("test-ssh-key"),
		}
	})

	AfterEach(func() {
		mockController.Finish()
	})

	It("should build a launch template data", func() {
		exptectedRootDeviceName := "test-root-device-name"
		ec2ServiceMock.EXPECT().DescribeImages(
			&ec2.DescribeImagesInput{
				ImageIds: []*string{group.ImageID},
			},
		).Return(
			&ec2.DescribeImagesOutput{
				Images: []*ec2.Image{
					{
						RootDeviceName: &exptectedRootDeviceName,
					},
				},
			},
			nil)

		launchTemplateData, err := buildLaunchTemplateData(ec2ServiceMock, *group)
		Expect(err).ToNot(HaveOccurred())

		Expect(launchTemplateData).ToNot(BeNil())
		Expect(launchTemplateData.ImageId).To(Equal(group.ImageID))
		Expect(launchTemplateData.KeyName).To(Equal(group.Ec2SshKey))
		Expect(launchTemplateData.UserData).To(Equal(group.UserData))
		Expect(launchTemplateData.BlockDeviceMappings).To(HaveLen(1))
		Expect(launchTemplateData.BlockDeviceMappings[0].DeviceName).To(Equal(&exptectedRootDeviceName))
		Expect(launchTemplateData.BlockDeviceMappings[0].Ebs.VolumeSize).To(Equal(group.DiskSize))
		Expect(launchTemplateData.TagSpecifications).To(Equal(utils.CreateTagSpecs(group.ResourceTags)))
		Expect(launchTemplateData.InstanceType).To(Equal(group.InstanceType))
	})

	It("should fail to build a launch template data if userdata is invalid", func() {
		group.UserData = aws.String("invalid-user-data")
		_, err := buildLaunchTemplateData(ec2ServiceMock, *group)
		Expect(err).To(HaveOccurred())
	})

	It("should fail to build a launch template data if error is return by ec2", func() {
		ec2ServiceMock.EXPECT().DescribeImages(gomock.Any()).Return(nil, errors.New("error"))
		_, err := buildLaunchTemplateData(ec2ServiceMock, *group)
		Expect(err).To(HaveOccurred())
	})
})

var _ = Describe("createNewLaunchTemplateVersion", func() {
	var (
		mockController *gomock.Controller
		ec2ServiceMock *mock_services.MockEC2ServiceInterface
		group          *eksv1.NodeGroup
		templateID     = "test-launch-template"
	)

	BeforeEach(func() {
		mockController = gomock.NewController(GinkgoT())
		ec2ServiceMock = mock_services.NewMockEC2ServiceInterface(mockController)
		group = &eksv1.NodeGroup{
			DiskSize:     aws.Int64(20),
			ResourceTags: aws.StringMap(map[string]string{"test": "test"}),
			InstanceType: aws.String("test-instance-type"),
			Ec2SshKey:    aws.String("test-ssh-key"),
		}
	})

	AfterEach(func() {
		mockController.Finish()
	})

	It("should create a new launch template", func() {
		input, err := buildLaunchTemplateData(ec2ServiceMock, *group)
		Expect(err).ToNot(HaveOccurred())

		output := &ec2.CreateLaunchTemplateVersionOutput{
			LaunchTemplateVersion: &ec2.LaunchTemplateVersion{
				LaunchTemplateName: aws.String("test"),
				LaunchTemplateId:   aws.String("test"),
				VersionNumber:      aws.Int64(1),
			},
		}

		ec2ServiceMock.EXPECT().CreateLaunchTemplateVersion(&ec2.CreateLaunchTemplateVersionInput{
			LaunchTemplateData: input,
			LaunchTemplateId:   aws.String(templateID),
		}).Return(output, nil)

		launchTemplate, err := CreateNewLaunchTemplateVersion(ec2ServiceMock, templateID, *group)
		Expect(err).ToNot(HaveOccurred())

		Expect(launchTemplate.Name).To(Equal(output.LaunchTemplateVersion.LaunchTemplateName))
		Expect(launchTemplate.ID).To(Equal(output.LaunchTemplateVersion.LaunchTemplateId))
		Expect(launchTemplate.Version).To(Equal(output.LaunchTemplateVersion.VersionNumber))
	})

	It("should fail to create a new launch template if error is returned by ec2", func() {
		ec2ServiceMock.EXPECT().CreateLaunchTemplateVersion(gomock.Any()).Return(nil, errors.New("error"))
		_, err := CreateNewLaunchTemplateVersion(ec2ServiceMock, templateID, *group)
		Expect(err).To(HaveOccurred())
	})
})

var _ = Describe("CreateNodeGroup", func() {
	var (
		mockController            *gomock.Controller
		eksServiceMock            *mock_services.MockEKSServiceInterface
		ec2ServiceMock            *mock_services.MockEC2ServiceInterface
		cloudFormationServiceMock *mock_services.MockCloudFormationServiceInterface
		createNodeGroupOpts       *CreateNodeGroupOptions
	)

	BeforeEach(func() {
		mockController = gomock.NewController(GinkgoT())
		eksServiceMock = mock_services.NewMockEKSServiceInterface(mockController)
		ec2ServiceMock = mock_services.NewMockEC2ServiceInterface(mockController)
		cloudFormationServiceMock = mock_services.NewMockCloudFormationServiceInterface(mockController)
		createNodeGroupOpts = &CreateNodeGroupOptions{
			EC2Service:            ec2ServiceMock,
			EKSService:            eksServiceMock,
			CloudFormationService: cloudFormationServiceMock,

			Config: &eksv1.EKSClusterConfig{
				Spec: eksv1.EKSClusterConfigSpec{
					DisplayName: "test",
				},
				Status: eksv1.EKSClusterConfigStatus{
					ManagedLaunchTemplateID: "test",
				},
			},
			NodeGroup: eksv1.NodeGroup{
				RequestSpotInstances: aws.Bool(true),
				NodegroupName:        aws.String("test"),
				Labels:               aws.StringMap(map[string]string{"test": "test"}),
				DesiredSize:          aws.Int64(1),
				MaxSize:              aws.Int64(1),
				MinSize:              aws.Int64(1),
				Subnets:              []string{"test"},
				ImageID:              aws.String("test"),
				Ec2SshKey:            aws.String("test"),
				SpotInstanceTypes:    aws.StringSlice([]string{"test"}),
			},
		}
	})

	AfterEach(func() {
		mockController.Finish()
	})

	It("should create a node group", func() {
		ec2ServiceMock.EXPECT().CreateLaunchTemplateVersion(&ec2.CreateLaunchTemplateVersionInput{
			LaunchTemplateData: &ec2.RequestLaunchTemplateData{
				ImageId: createNodeGroupOpts.NodeGroup.ImageID,
				KeyName: createNodeGroupOpts.NodeGroup.Ec2SshKey,
				BlockDeviceMappings: []*ec2.LaunchTemplateBlockDeviceMappingRequest{
					{
						DeviceName: aws.String("test"),
						Ebs: &ec2.LaunchTemplateEbsBlockDeviceRequest{
							VolumeSize: createNodeGroupOpts.NodeGroup.DiskSize,
						},
					},
				},
				TagSpecifications: utils.CreateTagSpecs(createNodeGroupOpts.NodeGroup.ResourceTags),
			},
			LaunchTemplateId: aws.String(createNodeGroupOpts.Config.Status.ManagedLaunchTemplateID),
		}).Return(&ec2.CreateLaunchTemplateVersionOutput{
			LaunchTemplateVersion: &ec2.LaunchTemplateVersion{
				LaunchTemplateName: aws.String("test"),
				LaunchTemplateId:   aws.String("test"),
				VersionNumber:      aws.Int64(1),
			},
		}, nil)

		ec2ServiceMock.EXPECT().DescribeImages(&ec2.DescribeImagesInput{ImageIds: []*string{createNodeGroupOpts.NodeGroup.ImageID}}).Return(&ec2.DescribeImagesOutput{
			Images: []*ec2.Image{
				{
					RootDeviceName: aws.String("test"),
				},
			},
		}, nil)

		cloudFormationServiceMock.EXPECT().CreateStack(gomock.Any()).Return(nil, nil)

		cloudFormationServiceMock.EXPECT().DescribeStacks(gomock.Any()).Return(
			&cloudformation.DescribeStacksOutput{
				Stacks: []*cloudformation.Stack{
					{
						StackStatus: aws.String(createCompleteStatus),
						Outputs: []*cloudformation.Output{
							{
								OutputKey:   aws.String("NodeInstanceRole"),
								OutputValue: aws.String("test"),
							},
						},
					},
				},
			}, nil)

		eksServiceMock.EXPECT().CreateNodegroup(&eks.CreateNodegroupInput{
			ClusterName:   aws.String(createNodeGroupOpts.Config.Spec.DisplayName),
			NodegroupName: createNodeGroupOpts.NodeGroup.NodegroupName,
			Labels:        createNodeGroupOpts.NodeGroup.Labels,
			ScalingConfig: &eks.NodegroupScalingConfig{
				DesiredSize: createNodeGroupOpts.NodeGroup.DesiredSize,
				MaxSize:     createNodeGroupOpts.NodeGroup.MaxSize,
				MinSize:     createNodeGroupOpts.NodeGroup.MinSize,
			},
			CapacityType: aws.String(eks.CapacityTypesSpot),
			LaunchTemplate: &eks.LaunchTemplateSpecification{
				Id:      aws.String("test"),
				Version: aws.String("1"),
			},
			InstanceTypes: createNodeGroupOpts.NodeGroup.SpotInstanceTypes,
			Subnets:       aws.StringSlice(createNodeGroupOpts.NodeGroup.Subnets),
			NodeRole:      aws.String("test"),
		}).Return(nil, nil)

		launchTemplateVersion, generatedNodeRole, err := CreateNodeGroup(createNodeGroupOpts)
		Expect(err).ToNot(HaveOccurred())

		Expect(launchTemplateVersion).To(Equal("1"))
		Expect(generatedNodeRole).To(Equal("test"))
	})

	It("shouldn't create launch template if it exists", func() {
		createNodeGroupOpts.NodeGroup.LaunchTemplate = &eksv1.LaunchTemplate{
			ID:      aws.String("test"),
			Version: aws.Int64(1),
			Name:    aws.String("test"),
		}

		cloudFormationServiceMock.EXPECT().CreateStack(gomock.Any()).Return(nil, nil)

		cloudFormationServiceMock.EXPECT().DescribeStacks(gomock.Any()).Return(
			&cloudformation.DescribeStacksOutput{
				Stacks: []*cloudformation.Stack{
					{
						StackStatus: aws.String(createCompleteStatus),
						Outputs: []*cloudformation.Output{
							{
								OutputKey:   aws.String("NodeInstanceRole"),
								OutputValue: aws.String("test"),
							},
						},
					},
				},
			}, nil)

		eksServiceMock.EXPECT().CreateNodegroup(gomock.Any()).Return(nil, nil)

		launchTemplateVersion, generatedNodeRole, err := CreateNodeGroup(createNodeGroupOpts)
		Expect(err).ToNot(HaveOccurred())

		Expect(launchTemplateVersion).To(Equal("1"))
		Expect(generatedNodeRole).To(Equal("test"))
	})

	It("shouldn't create node role if it exists", func() {
		createNodeGroupOpts.Config.Status.GeneratedNodeRole = "test"
		ec2ServiceMock.EXPECT().CreateLaunchTemplateVersion(gomock.Any()).Return(&ec2.CreateLaunchTemplateVersionOutput{
			LaunchTemplateVersion: &ec2.LaunchTemplateVersion{
				LaunchTemplateName: aws.String("test"),
				LaunchTemplateId:   aws.String("test"),
				VersionNumber:      aws.Int64(1),
			},
		}, nil)

		ec2ServiceMock.EXPECT().DescribeImages(gomock.Any()).Return(&ec2.DescribeImagesOutput{
			Images: []*ec2.Image{
				{
					RootDeviceName: aws.String("test"),
				},
			},
		}, nil)

		eksServiceMock.EXPECT().CreateNodegroup(gomock.Any()).Return(nil, nil)

		launchTemplateVersion, generatedNodeRole, err := CreateNodeGroup(createNodeGroupOpts)
		Expect(err).ToNot(HaveOccurred())

		Expect(launchTemplateVersion).To(Equal("1"))
		Expect(generatedNodeRole).To(Equal("test"))
	})

	It("delete launch template versions if creating node group fails", func() {
		ec2ServiceMock.EXPECT().CreateLaunchTemplateVersion(gomock.Any()).Return(&ec2.CreateLaunchTemplateVersionOutput{
			LaunchTemplateVersion: &ec2.LaunchTemplateVersion{
				LaunchTemplateName: aws.String("test"),
				LaunchTemplateId:   aws.String("test"),
				VersionNumber:      aws.Int64(1),
			},
		}, nil)
		ec2ServiceMock.EXPECT().DescribeImages(gomock.Any()).Return(&ec2.DescribeImagesOutput{
			Images: []*ec2.Image{
				{
					RootDeviceName: aws.String("test"),
				},
			},
		}, nil)
		cloudFormationServiceMock.EXPECT().CreateStack(gomock.Any()).Return(nil, nil)
		cloudFormationServiceMock.EXPECT().DescribeStacks(gomock.Any()).Return(
			&cloudformation.DescribeStacksOutput{
				Stacks: []*cloudformation.Stack{
					{
						StackStatus: aws.String(createCompleteStatus),
						Outputs: []*cloudformation.Output{
							{
								OutputKey:   aws.String("NodeInstanceRole"),
								OutputValue: aws.String("test"),
							},
						},
					},
				},
			}, nil)
		eksServiceMock.EXPECT().CreateNodegroup(gomock.Any()).Return(nil, errors.New("error"))
		ec2ServiceMock.EXPECT().DeleteLaunchTemplateVersions(gomock.Any()).Return(nil, nil)

		launchTemplateVersion, generatedNodeRole, err := CreateNodeGroup(createNodeGroupOpts)
		Expect(err).To(HaveOccurred())

		Expect(launchTemplateVersion).To(Equal("1"))
		Expect(generatedNodeRole).To(Equal("test"))
	})

	It("should fail to create node group if creating launch template return error", func() {
		ec2ServiceMock.EXPECT().CreateLaunchTemplateVersion(gomock.Any()).Return(nil, errors.New("error"))

		ec2ServiceMock.EXPECT().DescribeImages(gomock.Any()).Return(&ec2.DescribeImagesOutput{
			Images: []*ec2.Image{
				{
					RootDeviceName: aws.String("test"),
				},
			},
		}, nil)

		_, _, err := CreateNodeGroup(createNodeGroupOpts)
		Expect(err).To(HaveOccurred())
	})

	It("get subnets from status if not set", func() {
		createNodeGroupOpts.NodeGroup.Subnets = nil
		createNodeGroupOpts.Config.Status.Subnets = []string{"from", "status"}
		ec2ServiceMock.EXPECT().CreateLaunchTemplateVersion(gomock.Any()).Return(&ec2.CreateLaunchTemplateVersionOutput{
			LaunchTemplateVersion: &ec2.LaunchTemplateVersion{
				LaunchTemplateName: aws.String("test"),
				LaunchTemplateId:   aws.String("test"),
				VersionNumber:      aws.Int64(1),
			},
		}, nil)

		ec2ServiceMock.EXPECT().DescribeImages(gomock.Any()).Return(&ec2.DescribeImagesOutput{
			Images: []*ec2.Image{
				{
					RootDeviceName: aws.String("test"),
				},
			},
		}, nil)

		cloudFormationServiceMock.EXPECT().CreateStack(gomock.Any()).Return(nil, nil)

		cloudFormationServiceMock.EXPECT().DescribeStacks(gomock.Any()).Return(
			&cloudformation.DescribeStacksOutput{
				Stacks: []*cloudformation.Stack{
					{
						StackStatus: aws.String(createCompleteStatus),
						Outputs: []*cloudformation.Output{
							{
								OutputKey:   aws.String("NodeInstanceRole"),
								OutputValue: aws.String("test"),
							},
						},
					},
				},
			}, nil)

		eksServiceMock.EXPECT().CreateNodegroup(&eks.CreateNodegroupInput{
			ClusterName:   aws.String(createNodeGroupOpts.Config.Spec.DisplayName),
			NodegroupName: createNodeGroupOpts.NodeGroup.NodegroupName,
			Labels:        createNodeGroupOpts.NodeGroup.Labels,
			ScalingConfig: &eks.NodegroupScalingConfig{
				DesiredSize: createNodeGroupOpts.NodeGroup.DesiredSize,
				MaxSize:     createNodeGroupOpts.NodeGroup.MaxSize,
				MinSize:     createNodeGroupOpts.NodeGroup.MinSize,
			},
			CapacityType: aws.String(eks.CapacityTypesSpot),
			LaunchTemplate: &eks.LaunchTemplateSpecification{
				Id:      aws.String("test"),
				Version: aws.String("1"),
			},
			InstanceTypes: createNodeGroupOpts.NodeGroup.SpotInstanceTypes,
			Subnets:       aws.StringSlice(createNodeGroupOpts.Config.Status.Subnets),
			NodeRole:      aws.String("test"),
		}).Return(nil, nil)

		launchTemplateVersion, generatedNodeRole, err := CreateNodeGroup(createNodeGroupOpts)
		Expect(err).ToNot(HaveOccurred())

		Expect(launchTemplateVersion).To(Equal("1"))
		Expect(generatedNodeRole).To(Equal("test"))
	})

	It("set gpu ami type", func() {
		createNodeGroupOpts.NodeGroup.Gpu = aws.Bool(true)
		createNodeGroupOpts.NodeGroup.ImageID = nil

		ec2ServiceMock.EXPECT().CreateLaunchTemplateVersion(gomock.Any()).Return(&ec2.CreateLaunchTemplateVersionOutput{
			LaunchTemplateVersion: &ec2.LaunchTemplateVersion{
				LaunchTemplateName: aws.String("test"),
				LaunchTemplateId:   aws.String("test"),
				VersionNumber:      aws.Int64(1),
			},
		}, nil)

		cloudFormationServiceMock.EXPECT().CreateStack(gomock.Any()).Return(nil, nil)

		cloudFormationServiceMock.EXPECT().DescribeStacks(gomock.Any()).Return(
			&cloudformation.DescribeStacksOutput{
				Stacks: []*cloudformation.Stack{
					{
						StackStatus: aws.String(createCompleteStatus),
						Outputs: []*cloudformation.Output{
							{
								OutputKey:   aws.String("NodeInstanceRole"),
								OutputValue: aws.String("test"),
							},
						},
					},
				},
			}, nil)

		eksServiceMock.EXPECT().CreateNodegroup(&eks.CreateNodegroupInput{
			ClusterName:   aws.String(createNodeGroupOpts.Config.Spec.DisplayName),
			NodegroupName: createNodeGroupOpts.NodeGroup.NodegroupName,
			Labels:        createNodeGroupOpts.NodeGroup.Labels,
			ScalingConfig: &eks.NodegroupScalingConfig{
				DesiredSize: createNodeGroupOpts.NodeGroup.DesiredSize,
				MaxSize:     createNodeGroupOpts.NodeGroup.MaxSize,
				MinSize:     createNodeGroupOpts.NodeGroup.MinSize,
			},
			CapacityType: aws.String(eks.CapacityTypesSpot),
			LaunchTemplate: &eks.LaunchTemplateSpecification{
				Id:      aws.String("test"),
				Version: aws.String("1"),
			},
			InstanceTypes: createNodeGroupOpts.NodeGroup.SpotInstanceTypes,
			Subnets:       aws.StringSlice(createNodeGroupOpts.NodeGroup.Subnets),
			NodeRole:      aws.String("test"),
			AmiType:       aws.String(eks.AMITypesAl2X8664Gpu),
		}).Return(nil, nil)

		launchTemplateVersion, generatedNodeRole, err := CreateNodeGroup(createNodeGroupOpts)
		Expect(err).ToNot(HaveOccurred())

		Expect(launchTemplateVersion).To(Equal("1"))
		Expect(generatedNodeRole).To(Equal("test"))
	})

	It("set Arm ami type", func() {
		createNodeGroupOpts.NodeGroup.Arm = aws.Bool(true)
		createNodeGroupOpts.NodeGroup.ImageID = nil

		ec2ServiceMock.EXPECT().CreateLaunchTemplateVersion(gomock.Any()).Return(&ec2.CreateLaunchTemplateVersionOutput{
			LaunchTemplateVersion: &ec2.LaunchTemplateVersion{
				LaunchTemplateName: aws.String("test"),
				LaunchTemplateId:   aws.String("test"),
				VersionNumber:      aws.Int64(1),
			},
		}, nil)

		cloudFormationServiceMock.EXPECT().CreateStack(gomock.Any()).Return(nil, nil)

		cloudFormationServiceMock.EXPECT().DescribeStacks(gomock.Any()).Return(
			&cloudformation.DescribeStacksOutput{
				Stacks: []*cloudformation.Stack{
					{
						StackStatus: aws.String(createCompleteStatus),
						Outputs: []*cloudformation.Output{
							{
								OutputKey:   aws.String("NodeInstanceRole"),
								OutputValue: aws.String("test"),
							},
						},
					},
				},
			}, nil)

		eksServiceMock.EXPECT().CreateNodegroup(&eks.CreateNodegroupInput{
			ClusterName:   aws.String(createNodeGroupOpts.Config.Spec.DisplayName),
			NodegroupName: createNodeGroupOpts.NodeGroup.NodegroupName,
			Labels:        createNodeGroupOpts.NodeGroup.Labels,
			ScalingConfig: &eks.NodegroupScalingConfig{
				DesiredSize: createNodeGroupOpts.NodeGroup.DesiredSize,
				MaxSize:     createNodeGroupOpts.NodeGroup.MaxSize,
				MinSize:     createNodeGroupOpts.NodeGroup.MinSize,
			},
			CapacityType: aws.String(eks.CapacityTypesSpot),
			LaunchTemplate: &eks.LaunchTemplateSpecification{
				Id:      aws.String("test"),
				Version: aws.String("1"),
			},
			InstanceTypes: createNodeGroupOpts.NodeGroup.SpotInstanceTypes,
			Subnets:       aws.StringSlice(createNodeGroupOpts.NodeGroup.Subnets),
			NodeRole:      aws.String("test"),
			AmiType:       aws.String(eks.AMITypesAl2Arm64),
		}).Return(nil, nil)

		launchTemplateVersion, generatedNodeRole, err := CreateNodeGroup(createNodeGroupOpts)
		Expect(err).ToNot(HaveOccurred())

		Expect(launchTemplateVersion).To(Equal("1"))
		Expect(generatedNodeRole).To(Equal("test"))
	})

	It("set ami type if image id not set", func() {
		createNodeGroupOpts.NodeGroup.ImageID = nil
		ec2ServiceMock.EXPECT().CreateLaunchTemplateVersion(gomock.Any()).Return(&ec2.CreateLaunchTemplateVersionOutput{
			LaunchTemplateVersion: &ec2.LaunchTemplateVersion{
				LaunchTemplateName: aws.String("test"),
				LaunchTemplateId:   aws.String("test"),
				VersionNumber:      aws.Int64(1),
			},
		}, nil)

		cloudFormationServiceMock.EXPECT().CreateStack(gomock.Any()).Return(nil, nil)

		cloudFormationServiceMock.EXPECT().DescribeStacks(gomock.Any()).Return(
			&cloudformation.DescribeStacksOutput{
				Stacks: []*cloudformation.Stack{
					{
						StackStatus: aws.String(createCompleteStatus),
						Outputs: []*cloudformation.Output{
							{
								OutputKey:   aws.String("NodeInstanceRole"),
								OutputValue: aws.String("test"),
							},
						},
					},
				},
			}, nil)

		eksServiceMock.EXPECT().CreateNodegroup(&eks.CreateNodegroupInput{
			ClusterName:   aws.String(createNodeGroupOpts.Config.Spec.DisplayName),
			NodegroupName: createNodeGroupOpts.NodeGroup.NodegroupName,
			Labels:        createNodeGroupOpts.NodeGroup.Labels,
			ScalingConfig: &eks.NodegroupScalingConfig{
				DesiredSize: createNodeGroupOpts.NodeGroup.DesiredSize,
				MaxSize:     createNodeGroupOpts.NodeGroup.MaxSize,
				MinSize:     createNodeGroupOpts.NodeGroup.MinSize,
			},
			CapacityType: aws.String(eks.CapacityTypesSpot),
			LaunchTemplate: &eks.LaunchTemplateSpecification{
				Id:      aws.String("test"),
				Version: aws.String("1"),
			},
			InstanceTypes: createNodeGroupOpts.NodeGroup.SpotInstanceTypes,
			Subnets:       aws.StringSlice(createNodeGroupOpts.NodeGroup.Subnets),
			NodeRole:      aws.String("test"),
			AmiType:       aws.String(eks.AMITypesAl2X8664),
		}).Return(nil, nil)

		launchTemplateVersion, generatedNodeRole, err := CreateNodeGroup(createNodeGroupOpts)
		Expect(err).ToNot(HaveOccurred())

		Expect(launchTemplateVersion).To(Equal("1"))
		Expect(generatedNodeRole).To(Equal("test"))
	})
	It("set resource tags", func() {
		createNodeGroupOpts.NodeGroup.ResourceTags = map[string]*string{
			"tag1": aws.String("val1"),
		}

		ec2ServiceMock.EXPECT().CreateLaunchTemplateVersion(gomock.Any()).Return(&ec2.CreateLaunchTemplateVersionOutput{
			LaunchTemplateVersion: &ec2.LaunchTemplateVersion{
				LaunchTemplateName: aws.String("test"),
				LaunchTemplateId:   aws.String("test"),
				VersionNumber:      aws.Int64(1),
			},
		}, nil)

		ec2ServiceMock.EXPECT().DescribeImages(gomock.Any()).Return(&ec2.DescribeImagesOutput{
			Images: []*ec2.Image{
				{
					RootDeviceName: aws.String("test"),
				},
			},
		}, nil)

		cloudFormationServiceMock.EXPECT().CreateStack(gomock.Any()).Return(nil, nil)

		cloudFormationServiceMock.EXPECT().DescribeStacks(gomock.Any()).Return(
			&cloudformation.DescribeStacksOutput{
				Stacks: []*cloudformation.Stack{
					{
						StackStatus: aws.String(createCompleteStatus),
						Outputs: []*cloudformation.Output{
							{
								OutputKey:   aws.String("NodeInstanceRole"),
								OutputValue: aws.String("test"),
							},
						},
					},
				},
			}, nil)

		eksServiceMock.EXPECT().CreateNodegroup(&eks.CreateNodegroupInput{
			ClusterName:   aws.String(createNodeGroupOpts.Config.Spec.DisplayName),
			NodegroupName: createNodeGroupOpts.NodeGroup.NodegroupName,
			Labels:        createNodeGroupOpts.NodeGroup.Labels,
			ScalingConfig: &eks.NodegroupScalingConfig{
				DesiredSize: createNodeGroupOpts.NodeGroup.DesiredSize,
				MaxSize:     createNodeGroupOpts.NodeGroup.MaxSize,
				MinSize:     createNodeGroupOpts.NodeGroup.MinSize,
			},
			CapacityType: aws.String(eks.CapacityTypesSpot),
			LaunchTemplate: &eks.LaunchTemplateSpecification{
				Id:      aws.String("test"),
				Version: aws.String("1"),
			},
			InstanceTypes: createNodeGroupOpts.NodeGroup.SpotInstanceTypes,
			Subnets:       aws.StringSlice(createNodeGroupOpts.NodeGroup.Subnets),
			NodeRole:      aws.String("test"),
			Tags: map[string]*string{
				"tag1": aws.String("val1"),
			},
		}).Return(nil, nil)

		launchTemplateVersion, generatedNodeRole, err := CreateNodeGroup(createNodeGroupOpts)
		Expect(err).ToNot(HaveOccurred())

		Expect(launchTemplateVersion).To(Equal("1"))
		Expect(generatedNodeRole).To(Equal("test"))
	})
})

var _ = Describe("installEBSCSIDriver", func() {
	var (
		mockController            *gomock.Controller
		eksServiceMock            *mock_services.MockEKSServiceInterface
		iamServiceMock            *mock_services.MockIAMServiceInterface
		cloudFormationServiceMock *mock_services.MockCloudFormationServiceInterface
		enableEBSCSIDriverInput   *EnableEBSCSIDriverInput
		oidcListProvidersOutput   *iam.ListOpenIDConnectProvidersOutput
		oidcCreateProviderOutput  *iam.CreateOpenIDConnectProviderOutput
		eksClusterOutput          *eks.DescribeClusterOutput
		eksCreateAddonOutput      *eks.CreateAddonOutput
		defaultAWSRegion          string
	)

	BeforeEach(func() {
		mockController = gomock.NewController(GinkgoT())
		eksServiceMock = mock_services.NewMockEKSServiceInterface(mockController)
		iamServiceMock = mock_services.NewMockIAMServiceInterface(mockController)
		cloudFormationServiceMock = mock_services.NewMockCloudFormationServiceInterface(mockController)
		enableEBSCSIDriverInput = &EnableEBSCSIDriverInput{
			EKSService: eksServiceMock,
			IAMService: iamServiceMock,
			CFService:  cloudFormationServiceMock,
			Config:     &eksv1.EKSClusterConfig{},
		}
		defaultAWSRegion = "us-east-1" // must use a default region to get OIDC thumbprint
		oidcListProvidersOutput = &iam.ListOpenIDConnectProvidersOutput{}
		oidcCreateProviderOutput = &iam.CreateOpenIDConnectProviderOutput{
			OpenIDConnectProviderArn: aws.String("arn:aws:iam::account:oidc-provider/oidc.eks.regions.amazonaws.com/id/AAABBBCCCDDDEEEFFF11122233344455"),
		}
		eksClusterOutput = &eks.DescribeClusterOutput{
			Cluster: &eks.Cluster{
				Identity: &eks.Identity{
					Oidc: &eks.OIDC{
						Issuer: aws.String(fmt.Sprintf("https://oidc.eks.%v.amazonaws.com/id/AAABBBCCCDDDEEEFFF11122233344455", defaultAWSRegion)),
					},
				},
			},
		}
	})

	AfterEach(func() {
		mockController.Finish()
	})

	It("should successfully create oidc provider", func() {
		oidcListProvidersOutput.OpenIDConnectProviderList = []*iam.OpenIDConnectProviderListEntry{
			{Arn: aws.String("arn:aws:iam::account:oidc-provider/oidc.eks.region.amazonaws.com/id/BBBAAACCCDDDEEEFFF11122233344455")},
		}
		iamServiceMock.EXPECT().ListOIDCProviders(gomock.Any()).Return(oidcListProvidersOutput, nil)
		eksServiceMock.EXPECT().DescribeCluster(gomock.Any()).Return(eksClusterOutput, nil)
		iamServiceMock.EXPECT().CreateOIDCProvider(gomock.Any()).Return(oidcCreateProviderOutput, nil)
		_, err := configureOIDCProvider(enableEBSCSIDriverInput.IAMService, enableEBSCSIDriverInput.EKSService, enableEBSCSIDriverInput.Config)
		Expect(err).To(Succeed())
	})

	It("should successfully use existing oidc provider", func() {
		oidcListProvidersOutput.OpenIDConnectProviderList = []*iam.OpenIDConnectProviderListEntry{
			{Arn: aws.String("arn:aws:iam::account:oidc-provider/oidc.eks.region.amazonaws.com/id/AAABBBCCCDDDEEEFFF11122233344455")},
		}
		eksServiceMock.EXPECT().DescribeCluster(gomock.Any()).Return(eksClusterOutput, nil)
		iamServiceMock.EXPECT().ListOIDCProviders(gomock.Any()).Return(oidcListProvidersOutput, nil)
		_, err := configureOIDCProvider(enableEBSCSIDriverInput.IAMService, enableEBSCSIDriverInput.EKSService, enableEBSCSIDriverInput.Config)
		Expect(err).To(Succeed())
	})

	It("should fail to list oidc providers", func() {
		iamServiceMock.EXPECT().ListOIDCProviders(gomock.Any()).Return(nil, fmt.Errorf("failed to list oidc providers"))
		_, err := configureOIDCProvider(enableEBSCSIDriverInput.IAMService, enableEBSCSIDriverInput.EKSService, enableEBSCSIDriverInput.Config)
		Expect(err).ToNot(Succeed())
	})

	It("should fail to create oidc provider", func() {
		oidcListProvidersOutput.OpenIDConnectProviderList = []*iam.OpenIDConnectProviderListEntry{
			{Arn: aws.String("arn:aws:iam::account:oidc-provider/oidc.eks.region.amazonaws.com/id/BBBAAACCCDDDEEEFFF11122233344455")},
		}
		iamServiceMock.EXPECT().ListOIDCProviders(gomock.Any()).Return(oidcListProvidersOutput, nil)
		eksServiceMock.EXPECT().DescribeCluster(gomock.Any()).Return(eksClusterOutput, nil)
		iamServiceMock.EXPECT().CreateOIDCProvider(gomock.Any()).Return(nil, fmt.Errorf("failed to create oidc provider"))
		_, err := configureOIDCProvider(enableEBSCSIDriverInput.IAMService, enableEBSCSIDriverInput.EKSService, enableEBSCSIDriverInput.Config)
		Expect(err).ToNot(Succeed())
	})

	It("should successfully create driver iam role", func() {
		cloudFormationServiceMock.EXPECT().CreateStack(gomock.Any()).Return(nil, nil)
		cloudFormationServiceMock.EXPECT().DescribeStacks(gomock.Any()).Return(
			&cloudformation.DescribeStacksOutput{
				Stacks: []*cloudformation.Stack{
					{
						StackStatus: aws.String(createCompleteStatus),
						Outputs: []*cloudformation.Output{
							{
								OutputKey:   aws.String("EBSCSIDriverRole"),
								OutputValue: aws.String("test"),
							},
						},
					},
				},
			}, nil)
		_, err := createEBSCSIDriverRole(enableEBSCSIDriverInput.CFService, enableEBSCSIDriverInput.Config, "")
		Expect(err).To(Succeed())
	})

	It("should fail to create driver iam role", func() {
		cloudFormationServiceMock.EXPECT().CreateStack(gomock.Any()).Return(nil, nil)
		cloudFormationServiceMock.EXPECT().DescribeStacks(gomock.Any()).Return(nil, fmt.Errorf("failed to describe stack"))
		_, err := createEBSCSIDriverRole(enableEBSCSIDriverInput.CFService, enableEBSCSIDriverInput.Config, "")
		Expect(err).ToNot(Succeed())
	})

	It("should successfully install addon", func() {
		eksCreateAddonOutput = &eks.CreateAddonOutput{
			Addon: &eks.Addon{
				AddonArn: aws.String("arn:aws::ebs-csi-driver"),
			},
		}
		eksServiceMock.EXPECT().CreateAddon(gomock.Any()).Return(eksCreateAddonOutput, nil)
		addonArn, err := installEBSAddon(enableEBSCSIDriverInput.EKSService, enableEBSCSIDriverInput.Config, "roleArn", "latest")
		Expect(err).To(Succeed())
		Expect(addonArn).To(Equal("arn:aws::ebs-csi-driver"))
	})

	It("should fail to install addon", func() {
		eksCreateAddonOutput = &eks.CreateAddonOutput{}
		eksServiceMock.EXPECT().CreateAddon(gomock.Any()).Return(nil, fmt.Errorf("failed to create addon"))
		_, err := installEBSAddon(enableEBSCSIDriverInput.EKSService, enableEBSCSIDriverInput.Config, "roleArn", "latest")
		Expect(err).ToNot(Succeed())
	})
})
