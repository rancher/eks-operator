package eks

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	"github.com/rancher/eks-operator/pkg/eks/services/mock_services"
)

var _ = Describe("deleteLaunchTemplateVersions", func() {
	var (
		mockController *gomock.Controller
		ec2ServiceMock *mock_services.MockEC2ServiceInterface
	)

	BeforeEach(func() {
		mockController = gomock.NewController(GinkgoT())
		ec2ServiceMock = mock_services.NewMockEC2ServiceInterface(mockController)
	})

	AfterEach(func() {
		mockController.Finish()
	})

	It("should try to delete launch template versions", func() {
		templateID := "templateID"
		templateVersions := []string{"1", "2"}

		ec2ServiceMock.EXPECT().DeleteLaunchTemplateVersions(ctx, &ec2.DeleteLaunchTemplateVersionsInput{
			LaunchTemplateId: aws.String(templateID),
			Versions:         templateVersions,
		}).Return(nil, nil)

		DeleteLaunchTemplateVersions(ctx, ec2ServiceMock, templateID, aws.StringSlice(templateVersions))
	})
})
