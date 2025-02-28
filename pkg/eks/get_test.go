package eks

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/eks/types"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	eksv1 "github.com/rancher/eks-operator/pkg/apis/eks.cattle.io/v1"
	"github.com/rancher/eks-operator/pkg/eks/services/mock_services"
)

var _ = Describe("GetClusterState", func() {
	var (
		mockController          *gomock.Controller
		eksServiceMock          *mock_services.MockEKSServiceInterface
		getClusterStatusOptions *GetClusterStatusOpts
	)

	BeforeEach(func() {
		mockController = gomock.NewController(GinkgoT())
		eksServiceMock = mock_services.NewMockEKSServiceInterface(mockController)
		getClusterStatusOptions = &GetClusterStatusOpts{
			EKSService: eksServiceMock,
			Config: &eksv1.EKSClusterConfig{
				Spec: eksv1.EKSClusterConfigSpec{
					DisplayName: "test-cluster",
				},
			},
		}
	})

	AfterEach(func() {
		mockController.Finish()
	})

	It("should successfully get cluster state", func() {
		eksServiceMock.EXPECT().DescribeCluster(ctx,
			&eks.DescribeClusterInput{
				Name: aws.String(getClusterStatusOptions.Config.Spec.DisplayName),
			},
		).Return(&eks.DescribeClusterOutput{}, nil)
		clusterState, err := GetClusterState(ctx, getClusterStatusOptions)
		Expect(err).ToNot(HaveOccurred())
		Expect(clusterState).ToNot(BeNil())
	})

	It("should fail to get cluster state", func() {
		eksServiceMock.EXPECT().DescribeCluster(ctx, gomock.Any()).Return(nil, errors.New("error getting cluster state"))
		_, err := GetClusterState(ctx, getClusterStatusOptions)
		Expect(err).To(HaveOccurred())
	})
})

var _ = Describe("GetLaunchTemplateVersions", func() {
	var (
		mockController           *gomock.Controller
		ec2ServiceMock           *mock_services.MockEC2ServiceInterface
		getLaunchTemplateOptions *GetLaunchTemplateVersionsOpts
	)

	BeforeEach(func() {
		mockController = gomock.NewController(GinkgoT())
		ec2ServiceMock = mock_services.NewMockEC2ServiceInterface(mockController)
		getLaunchTemplateOptions = &GetLaunchTemplateVersionsOpts{
			EC2Service:       ec2ServiceMock,
			LaunchTemplateID: aws.String("test-launch-template-id"),
			Versions:         aws.StringSlice([]string{"1", "2"}),
		}
	})

	AfterEach(func() {
		mockController.Finish()
	})

	It("should successfully get launch template versions", func() {
		ec2ServiceMock.EXPECT().DescribeLaunchTemplateVersions(ctx,
			&ec2.DescribeLaunchTemplateVersionsInput{
				LaunchTemplateId: getLaunchTemplateOptions.LaunchTemplateID,
				Versions:         aws.ToStringSlice(getLaunchTemplateOptions.Versions),
			},
		).Return(&ec2.DescribeLaunchTemplateVersionsOutput{}, nil)
		ltVersion, err := GetLaunchTemplateVersions(ctx, getLaunchTemplateOptions)
		Expect(err).ToNot(HaveOccurred())
		Expect(ltVersion).ToNot(BeNil())
	})

	It("should fail to get launch template versions", func() {
		ec2ServiceMock.EXPECT().DescribeLaunchTemplateVersions(ctx, gomock.Any()).Return(nil, errors.New("error getting launch template versions"))
		_, err := GetLaunchTemplateVersions(ctx, getLaunchTemplateOptions)
		Expect(err).To(HaveOccurred())
	})

	It("should fail to get launch template versions when template id is missing", func() {
		getLaunchTemplateOptions.LaunchTemplateID = nil
		_, err := GetLaunchTemplateVersions(ctx, getLaunchTemplateOptions)
		Expect(err).To(HaveOccurred())
	})

	It("should fail to get launch template versions when versions are missing", func() {
		getLaunchTemplateOptions.Versions = nil
		_, err := GetLaunchTemplateVersions(ctx, getLaunchTemplateOptions)
		Expect(err).To(HaveOccurred())
	})

	var _ = Describe("GetEBSCSIAddon", func() {
		var (
			mockController            *gomock.Controller
			eksServiceMock            *mock_services.MockEKSServiceInterface
			iamServiceMock            *mock_services.MockIAMServiceInterface
			cloudFormationServiceMock *mock_services.MockCloudFormationServiceInterface
			eksDescribeAddonOutput    *eks.DescribeAddonOutput
			enableEBSCSIDriverInput   *EnableEBSCSIDriverInput
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
		})

		AfterEach(func() {
			mockController.Finish()
		})
		It("should detect that addon is already installed", func() {
			eksDescribeAddonOutput = &eks.DescribeAddonOutput{
				Addon: &ekstypes.Addon{
					AddonArn: aws.String("arn:aws::ebs-csi-driver"),
				},
			}
			eksServiceMock.EXPECT().DescribeAddon(ctx, gomock.Any()).Return(eksDescribeAddonOutput, nil)
			addonArn, err := CheckEBSAddon(ctx, enableEBSCSIDriverInput.Config.Spec.DisplayName, enableEBSCSIDriverInput.EKSService)
			Expect(err).To(Succeed())
			Expect(addonArn).To(Equal("arn:aws::ebs-csi-driver"))
		})

		It("should detect that addon is not installed", func() {
			eksDescribeAddonOutput = &eks.DescribeAddonOutput{}
			eksServiceMock.EXPECT().DescribeAddon(ctx, gomock.Any()).Return(eksDescribeAddonOutput, nil)
			addonArn, err := CheckEBSAddon(ctx, enableEBSCSIDriverInput.Config.Spec.DisplayName, enableEBSCSIDriverInput.EKSService)
			Expect(err).To(Succeed())
			Expect(addonArn).To(Equal(""))
		})

		It("should fail to check if addon is not installed", func() {
			eksDescribeAddonOutput = &eks.DescribeAddonOutput{}
			eksServiceMock.EXPECT().DescribeAddon(ctx, gomock.Any()).Return(nil, fmt.Errorf("failed to describe addon"))
			_, err := CheckEBSAddon(ctx, enableEBSCSIDriverInput.Config.Spec.DisplayName, enableEBSCSIDriverInput.EKSService)
			Expect(err).ToNot(Succeed())
		})
	})
})

var _ = Describe("GetClusterUpdates", func() {
	var (
		mockController          *gomock.Controller
		eksServiceMock          *mock_services.MockEKSServiceInterface
		getClusterStatusOptions *GetClusterStatusOpts
		ctx                     context.Context
	)

	BeforeEach(func() {
		mockController = gomock.NewController(GinkgoT())
		eksServiceMock = mock_services.NewMockEKSServiceInterface(mockController)
		ctx = context.Background()

		getClusterStatusOptions = &GetClusterStatusOpts{
			EKSService: eksServiceMock,
			Config: &eksv1.EKSClusterConfig{
				Spec: eksv1.EKSClusterConfigSpec{
					DisplayName: "test-cluster",
				},
				Status: eksv1.EKSClusterConfigStatus{
					CompletedUpdateIDs: []string{"update-123"},
				},
			},
		}
	})

	AfterEach(func() {
		mockController.Finish()
	})

	It("should successfully get cluster updates", func() {
		mockUpdates := []*eks.DescribeUpdateOutput{
			{
				Update: &types.Update{
					Id:     aws.String("update-1"),
					Status: types.UpdateStatusSuccessful,
				},
			},
			{
				Update: &types.Update{
					Id:     aws.String("update-2"),
					Status: types.UpdateStatusFailed,
				},
			},
			{
				Update: &types.Update{
					Id:     aws.String("update-3"),
					Status: types.UpdateStatusCancelled,
				},
			},
			{
				Update: &types.Update{
					Id:     aws.String("update-4"),
					Status: types.UpdateStatusInProgress,
				},
			},
		}

		eksServiceMock.EXPECT().DescribeUpdates(ctx, &eks.ListUpdatesInput{
			Name: aws.String(getClusterStatusOptions.Config.Spec.DisplayName),
		}, gomock.Any()).Return(mockUpdates, nil)

		inProgressUpdates, completedUpdates, err := GetClusterUpdates(ctx, getClusterStatusOptions)

		Expect(err).ToNot(HaveOccurred())
		Expect(inProgressUpdates).ToNot(BeNil())
		// Only successful updates should be returned
		Expect(completedUpdates).To(ContainElements("update-1", "update-2", "update-3"))
		// In-progress updates should not be marked completed
		Expect(completedUpdates).ToNot(ContainElement("update-4"))
		Expect(inProgressUpdates).To(Equal([]*types.Update{{
			Id:     aws.String("update-4"),
			Status: types.UpdateStatusInProgress,
		}}))
	})

	It("should return an error when DescribeUpdates fails", func() {
		eksServiceMock.EXPECT().DescribeUpdates(ctx, gomock.Any(), gomock.Any()).Return(nil, errors.New("error getting updates"))
		_, _, err := GetClusterUpdates(ctx, getClusterStatusOptions)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("error getting updates"))
	})

	It("should return an error when cluster name is empty", func() {
		getClusterStatusOptions.Config.Spec.DisplayName = ""
		_, _, err := GetClusterUpdates(ctx, getClusterStatusOptions)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("cluster name is empty"))
	})
})
