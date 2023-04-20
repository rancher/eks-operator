package eks

import (
	"errors"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	eksv1 "github.com/rancher/eks-operator/pkg/apis/eks.cattle.io/v1"
	"github.com/rancher/eks-operator/pkg/eks/services/mock_services"
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
		Expect(CreateCluster(*clustercCreateOptions)).To(Succeed())
	})

	It("should fail to create a cluster", func() {
		eksServiceMock.EXPECT().CreateCluster(gomock.Any()).Return(nil, errors.New("error creating cluster"))
		Expect(CreateCluster(*clustercCreateOptions)).ToNot(Succeed())
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
		mockController             *gomock.Controller
		cloudFormationsServiceMock *mock_services.MockCloudFormationServiceInterface
		stackCreationOptions       *CreateStackOptions
	)

	BeforeEach(func() {
		mockController = gomock.NewController(GinkgoT())
		cloudFormationsServiceMock = mock_services.NewMockCloudFormationServiceInterface(mockController)
		stackCreationOptions = &CreateStackOptions{
			CloudFormationService: cloudFormationsServiceMock,
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
		cloudFormationsServiceMock.EXPECT().CreateStack(&cloudformation.CreateStackInput{
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

		cloudFormationsServiceMock.EXPECT().DescribeStacks(
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

		describeStacksOutput, err := CreateStack(*stackCreationOptions)
		Expect(err).ToNot(HaveOccurred())

		Expect(describeStacksOutput).ToNot(BeNil())
	})

	It("should fail to create a stack if CreateStack returns error", func() {
		cloudFormationsServiceMock.EXPECT().CreateStack(gomock.Any()).Return(nil, errors.New("error"))

		_, err := CreateStack(*stackCreationOptions)
		Expect(err).To(HaveOccurred())
	})

	It("should fail to create a stack if stack already exists", func() {
		cloudFormationsServiceMock.EXPECT().CreateStack(gomock.Any()).Return(nil, awserr.New(cloudformation.ErrCodeAlreadyExistsException, "", nil))
		cloudFormationsServiceMock.EXPECT().DescribeStacks(gomock.Any()).Return(
			&cloudformation.DescribeStacksOutput{
				Stacks: []*cloudformation.Stack{
					{
						StackStatus: aws.String(createCompleteStatus),
					},
				},
			}, nil)

		_, err := CreateStack(*stackCreationOptions)
		Expect(err).ToNot(HaveOccurred())
	})

	It("should fail to create a stack if DescribeStack return errors", func() {
		cloudFormationsServiceMock.EXPECT().CreateStack(gomock.Any()).Return(nil, nil)
		cloudFormationsServiceMock.EXPECT().DescribeStacks(gomock.Any()).Return(nil, errors.New("error"))

		_, err := CreateStack(*stackCreationOptions)
		Expect(err).To(HaveOccurred())
	})

	It("should fail to create a stack if stack status is CREATE_FAILED", func() {
		cloudFormationsServiceMock.EXPECT().CreateStack(gomock.Any()).Return(nil, nil)
		cloudFormationsServiceMock.EXPECT().DescribeStacks(gomock.Any()).Return(
			&cloudformation.DescribeStacksOutput{
				Stacks: []*cloudformation.Stack{
					{
						StackStatus: aws.String(createFailedStatus),
					},
				},
			}, nil)
		cloudFormationsServiceMock.EXPECT().DescribeStackEvents(
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

		_, err := CreateStack(*stackCreationOptions)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring(createFailedStatus))
	})

	It("should fail to create a stack if stack status is ROLLBACK_IN_PROGRESS", func() {
		cloudFormationsServiceMock.EXPECT().CreateStack(gomock.Any()).Return(nil, nil)
		cloudFormationsServiceMock.EXPECT().DescribeStacks(gomock.Any()).Return(
			&cloudformation.DescribeStacksOutput{
				Stacks: []*cloudformation.Stack{
					{
						StackStatus: aws.String(rollbackInProgressStatus),
					},
				},
			}, nil)
		cloudFormationsServiceMock.EXPECT().DescribeStackEvents(
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

		_, err := CreateStack(*stackCreationOptions)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring(rollbackInProgressStatus))
	})
})
