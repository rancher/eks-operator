package controller

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/rancher/eks-operator/pkg/eks/services/mock_services"
)

var _ = Describe("BuildUpstreamClusterState", func() {
	var (
		mockController *gomock.Controller
		eksServiceMock *mock_services.MockEKSServiceInterface
		ec2ServiceMock *mock_services.MockEC2ServiceInterface
		testCtx        context.Context
	)

	BeforeEach(func() {
		mockController = gomock.NewController(GinkgoT())
		eksServiceMock = mock_services.NewMockEKSServiceInterface(mockController)
		ec2ServiceMock = mock_services.NewMockEC2ServiceInterface(mockController)
		testCtx = context.Background()
	})

	AfterEach(func() {
		mockController.Finish()
	})

	Context("when resource name differs from AWS cluster name", func() {
		It("should use AWS cluster name for AWS API calls and resource name for error messages", func() {
			// Setup: resource name != AWS cluster name (common in import scenarios)
			resourceName := "rancher-cluster-123"
			awsClusterName := "actual-aws-cluster"
			managedTemplateID := ""

			// Mock cluster state with the actual AWS cluster name
			clusterState := &eks.DescribeClusterOutput{
				Cluster: &ekstypes.Cluster{
					Name:    aws.String(awsClusterName),
					Version: aws.String("1.28"),
					Arn:     aws.String("arn:aws:eks:us-west-2:123456789012:cluster/actual-aws-cluster"),
					ResourcesVpcConfig: &ekstypes.VpcConfigResponse{
						EndpointPublicAccess:  true,
						EndpointPrivateAccess: true,
						PublicAccessCidrs:     []string{"0.0.0.0/0"},
						SubnetIds:             []string{"subnet-123"},
						SecurityGroupIds:      []string{"sg-123"},
					},
					Logging: &ekstypes.Logging{
						ClusterLogging: []ekstypes.LogSetup{
							{
								Enabled: aws.Bool(true),
								Types:   []ekstypes.LogType{ekstypes.LogTypeApi},
							},
						},
					},
					Tags:             map[string]string{"env": "test"},
					RoleArn:          aws.String("arn:aws:iam::123456789012:role/eks-service-role"),
					EncryptionConfig: []ekstypes.EncryptionConfig{},
				},
			}

			nodeGroupStates := []*eks.DescribeNodegroupOutput{}

			// CRITICAL: Verify CheckEBSAddon is called with AWS cluster name, not resource name
			eksServiceMock.EXPECT().DescribeAddon(
				testCtx,
				gomock.Any(),
			).DoAndReturn(func(ctx context.Context, input *eks.DescribeAddonInput) (*eks.DescribeAddonOutput, error) {
				// Verify the cluster name passed is the AWS cluster name
				Expect(aws.ToString(input.ClusterName)).To(Equal(awsClusterName),
					"CheckEBSAddon should receive AWS cluster name, not resource name")
				Expect(aws.ToString(input.AddonName)).To(Equal("aws-ebs-csi-driver"))

				// Return that addon is not installed
				return &eks.DescribeAddonOutput{}, nil
			}).Times(1)

			// Execute
			upstreamSpec, arn, err := BuildUpstreamClusterState(
				testCtx,
				resourceName,
				managedTemplateID,
				clusterState,
				nodeGroupStates,
				ec2ServiceMock,
				eksServiceMock,
				false,
			)

			// Verify results
			Expect(err).ToNot(HaveOccurred())
			Expect(upstreamSpec).ToNot(BeNil())
			Expect(arn).To(Equal("arn:aws:eks:us-west-2:123456789012:cluster/actual-aws-cluster"))

			// Verify DisplayName is set to AWS cluster name
			Expect(upstreamSpec.DisplayName).To(Equal(awsClusterName),
				"DisplayName should be set to AWS cluster name")
		})

		It("should include resource name in error messages for operator context", func() {
			resourceName := "rancher-cluster-456"
			awsClusterName := "production-cluster"
			managedTemplateID := "lt-managed-123"

			clusterState := &eks.DescribeClusterOutput{
				Cluster: &ekstypes.Cluster{
					Name:    aws.String(awsClusterName),
					Version: aws.String("1.28"),
					Arn:     aws.String("arn:aws:eks:us-west-2:123456789012:cluster/production-cluster"),
					ResourcesVpcConfig: &ekstypes.VpcConfigResponse{
						EndpointPublicAccess:  true,
						EndpointPrivateAccess: true,
						SubnetIds:             []string{"subnet-123"},
						SecurityGroupIds:      []string{"sg-123"},
					},
					RoleArn: aws.String("arn:aws:iam::123456789012:role/eks-service-role"),
				},
			}

			// Node group with managed launch template that doesn't exist
			nodeGroupStates := []*eks.DescribeNodegroupOutput{
				{
					Nodegroup: &ekstypes.Nodegroup{
						NodegroupName: aws.String("test-ng"),
						Status:        ekstypes.NodegroupStatusActive,
						Version:       aws.String("1.28"),
						ScalingConfig: &ekstypes.NodegroupScalingConfig{
							MinSize:     aws.Int32(1),
							MaxSize:     aws.Int32(3),
							DesiredSize: aws.Int32(2),
						},
						LaunchTemplate: &ekstypes.LaunchTemplateSpecification{
							Id:      aws.String(managedTemplateID),
							Version: aws.String("1"),
						},
						Subnets:  []string{"subnet-123"},
						NodeRole: aws.String("arn:aws:iam::123456789012:role/node-role"),
					},
				},
			}

			// Mock CheckEBSAddon with AWS cluster name
			eksServiceMock.EXPECT().DescribeAddon(
				testCtx,
				gomock.Any(),
			).DoAndReturn(func(ctx context.Context, input *eks.DescribeAddonInput) (*eks.DescribeAddonOutput, error) {
				Expect(aws.ToString(input.ClusterName)).To(Equal(awsClusterName))
				return &eks.DescribeAddonOutput{}, nil
			}).Times(1)

			// Mock launch template lookup that will fail
			ec2ServiceMock.EXPECT().DescribeLaunchTemplateVersions(
				testCtx,
				gomock.Any(),
			).Return(&ec2.DescribeLaunchTemplateVersionsOutput{
				LaunchTemplateVersions: []ec2types.LaunchTemplateVersion{},
			}, nil).Times(1)

			// Execute
			_, _, err := BuildUpstreamClusterState(
				testCtx,
				resourceName,
				managedTemplateID,
				clusterState,
				nodeGroupStates,
				ec2ServiceMock,
				eksServiceMock,
				false,
			)

			// Verify error message contains resource name for operator context
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(resourceName),
				"Error message should contain resource name for operator debugging context")
			Expect(err.Error()).To(ContainSubstring("test-ng"))
		})
	})

	Context("when resource name matches AWS cluster name", func() {
		It("should work correctly with matching names", func() {
			clusterName := "my-cluster"
			managedTemplateID := ""

			clusterState := &eks.DescribeClusterOutput{
				Cluster: &ekstypes.Cluster{
					Name:    aws.String(clusterName),
					Version: aws.String("1.28"),
					Arn:     aws.String("arn:aws:eks:us-west-2:123456789012:cluster/my-cluster"),
					ResourcesVpcConfig: &ekstypes.VpcConfigResponse{
						EndpointPublicAccess:  true,
						EndpointPrivateAccess: false,
						SubnetIds:             []string{"subnet-123"},
						SecurityGroupIds:      []string{"sg-123"},
					},
					RoleArn: aws.String("arn:aws:iam::123456789012:role/eks-service-role"),
				},
			}

			nodeGroupStates := []*eks.DescribeNodegroupOutput{}

			// Mock CheckEBSAddon
			eksServiceMock.EXPECT().DescribeAddon(
				testCtx,
				gomock.Any(),
			).DoAndReturn(func(ctx context.Context, input *eks.DescribeAddonInput) (*eks.DescribeAddonOutput, error) {
				Expect(aws.ToString(input.ClusterName)).To(Equal(clusterName))
				return &eks.DescribeAddonOutput{
					Addon: &ekstypes.Addon{
						AddonArn: aws.String("arn:aws:eks:us-west-2:123456789012:addon/my-cluster/aws-ebs-csi-driver/abc"),
					},
				}, nil
			}).Times(1)

			// Execute
			upstreamSpec, _, err := BuildUpstreamClusterState(
				testCtx,
				clusterName,
				managedTemplateID,
				clusterState,
				nodeGroupStates,
				ec2ServiceMock,
				eksServiceMock,
				false,
			)

			// Verify
			Expect(err).ToNot(HaveOccurred())
			Expect(upstreamSpec).ToNot(BeNil())
			Expect(upstreamSpec.DisplayName).To(Equal(clusterName))
			Expect(aws.ToBool(upstreamSpec.EBSCSIDriver)).To(BeTrue())
		})
	})
})
