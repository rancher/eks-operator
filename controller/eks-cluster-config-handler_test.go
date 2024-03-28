package controller

import (
	"errors"

	"github.com/aws/aws-sdk-go-v2/aws"

	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"

	awssdkeks "github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	eksv1 "github.com/rancher/eks-operator/pkg/apis/eks.cattle.io/v1"
	"github.com/rancher/eks-operator/pkg/eks"

	"github.com/rancher/eks-operator/pkg/eks/services/mock_services"
	"github.com/rancher/eks-operator/pkg/test"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("importCluster", func() {
	var (
		eksConfig               *eksv1.EKSClusterConfig
		getClusterStatusOptions *eks.GetClusterStatusOpts
		mockController          *gomock.Controller
		eksServiceMock          *mock_services.MockEKSServiceInterface
	)

	BeforeEach(func() {
		mockController = gomock.NewController(GinkgoT())
		eksServiceMock = mock_services.NewMockEKSServiceInterface(mockController)
		eksConfig = &eksv1.EKSClusterConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "default",
			},
			Spec: eksv1.EKSClusterConfigSpec{
				DisplayName:         "test",
				Region:              "test",
				PrivateAccess:       aws.Bool(true),
				PublicAccess:        aws.Bool(true),
				PublicAccessSources: []string{"test"},
				Tags:                map[string]string{"test": "test"},
				LoggingTypes:        []string{"test"},
				KubernetesVersion:   aws.String("test"),
				SecretsEncryption:   aws.Bool(true),
				KmsKey:              aws.String("test"),
			},
		}
		getClusterStatusOptions = &eks.GetClusterStatusOpts{
			EKSService: eksServiceMock,
			Config: &eksv1.EKSClusterConfig{
				Spec: eksv1.EKSClusterConfigSpec{
					DisplayName: "test",
				},
			},
		}

		Expect(cl.Create(ctx, eksConfig)).To(Succeed())
	})

	AfterEach(func() {
		Expect(test.CleanupAndWait(ctx, cl, eksConfig)).To(Succeed())
	})

	It("should get cluster state", func() {
		eksServiceMock.EXPECT().DescribeCluster(ctx,
			&awssdkeks.DescribeClusterInput{
				Name: aws.String(getClusterStatusOptions.Config.Spec.DisplayName),
			},
		).Return(&awssdkeks.DescribeClusterOutput{}, nil).AnyTimes()
		clusterState, err := eks.GetClusterState(ctx, getClusterStatusOptions)
		Expect(err).ToNot(HaveOccurred())
		Expect(clusterState).ToNot(BeNil())
	})
})

var _ = Describe("delete stack", func() {
	var (
		mockController            *gomock.Controller
		mockCloudformationService *mock_services.MockCloudFormationServiceInterface
		newStyleName              string
		oldStyleName              string
	)

	BeforeEach(func() {
		mockController = gomock.NewController(GinkgoT())
		newStyleName = "test"
		mockCloudformationService = mock_services.NewMockCloudFormationServiceInterface(mockController)
	})

	AfterEach(func() {
		mockController.Finish()
	})

	It("should successfully delete a stack", func() {
		mockCloudformationService.EXPECT().DescribeStacks(ctx,
			&cloudformation.DescribeStacksInput{
				StackName: &newStyleName,
			},
		).Return(
			&cloudformation.DescribeStacksOutput{
				Stacks: []cftypes.Stack{
					{
						StackName: &newStyleName,
					},
				},
			}, nil)

		mockCloudformationService.EXPECT().DeleteStack(ctx, &cloudformation.DeleteStackInput{
			StackName: &newStyleName,
		}).Return(nil, nil)

		newerr := deleteStack(ctx, mockCloudformationService, newStyleName, "")
		Expect(newerr).ToNot(HaveOccurred())
	})
	It("should successfully delete a stack with old style name", func() {
		mockCloudformationService.EXPECT().DescribeStacks(ctx,
			&cloudformation.DescribeStacksInput{
				StackName: &oldStyleName,
			},
		).Return(
			&cloudformation.DescribeStacksOutput{
				Stacks: []cftypes.Stack{
					{
						StackName: &oldStyleName,
					},
				},
			}, nil)

		mockCloudformationService.EXPECT().DeleteStack(ctx, &cloudformation.DeleteStackInput{
			StackName: &oldStyleName,
		}).Return(nil, nil)

		newerr := deleteStack(ctx, mockCloudformationService, "", oldStyleName)
		Expect(newerr).ToNot(HaveOccurred())
	})

	It("should fail to delete a stack if DescribeStacks returns no stacks", func() {
		mockCloudformationService.EXPECT().DescribeStacks(ctx,
			&cloudformation.DescribeStacksInput{
				StackName: &newStyleName,
			},
		).Return(&cloudformation.DescribeStacksOutput{}, nil)

		mockCloudformationService.EXPECT().DeleteStack(ctx, &cloudformation.DeleteStackInput{
			StackName: &newStyleName,
		}).Return(nil, errors.New("error"))
		newerr := deleteStack(ctx, mockCloudformationService, newStyleName, "")
		Expect(newerr).To(HaveOccurred())
	})
})
