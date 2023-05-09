package eks

import (
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/rancher/eks-operator/pkg/eks/services"
	"github.com/sirupsen/logrus"
)

func DeleteLaunchTemplateVersions(ec2Service services.EC2ServiceInterface, templateID string, templateVersions []*string) {
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

func launchTemplateVersionDoesNotExist(errorCode string) bool {
	return errorCode == ec2.LaunchTemplateErrorCodeLaunchTemplateVersionDoesNotExist ||
		errorCode == ec2.LaunchTemplateErrorCodeLaunchTemplateIdDoesNotExist
}
