package eks

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
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
		templateVersions := []*string{aws.String("1"), aws.String("2")}

		ec2ServiceMock.EXPECT().DeleteLaunchTemplateVersions(&ec2.DeleteLaunchTemplateVersionsInput{
			LaunchTemplateId: aws.String(templateID),
			Versions:         templateVersions,
		}).Return(nil, nil)

		DeleteLaunchTemplateVersions(ec2ServiceMock, templateID, templateVersions)
	})
})
