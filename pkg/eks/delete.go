package eks

import (
	"context"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/rancher/eks-operator/pkg/eks/services"
	"github.com/sirupsen/logrus"
)

func DeleteLaunchTemplateVersions(ctx context.Context, ec2Service services.EC2ServiceInterface, templateID string, templateVersions []*string) {
	launchTemplateDeleteVersionInput := &ec2.DeleteLaunchTemplateVersionsInput{
		LaunchTemplateId: aws.String(templateID),
		Versions:         aws.ToStringSlice(templateVersions),
	}

	var err error
	var deleteVersionsOutput *ec2.DeleteLaunchTemplateVersionsOutput
	for i := 0; i < 5; i++ {
		deleteVersionsOutput, err = ec2Service.DeleteLaunchTemplateVersions(ctx, launchTemplateDeleteVersionInput)

		if deleteVersionsOutput != nil {
			templateVersions = templateVersions[:0]
			for _, version := range deleteVersionsOutput.UnsuccessfullyDeletedLaunchTemplateVersions {
				if !launchTemplateVersionDoesNotExist(string(version.ResponseError.Code)) {
					templateVersions = append(templateVersions, aws.String(strconv.Itoa(int(*version.VersionNumber))))
				}
			}
		}

		if err == nil || len(templateVersions) == 0 {
			return
		}

		launchTemplateDeleteVersionInput.Versions = aws.ToStringSlice(templateVersions)
		time.Sleep(10 * time.Second)
	}

	logrus.Warnf("could not delete versions [%v] of launch template [%s]: %v, will not retry",
		aws.ToStringSlice(templateVersions),
		*launchTemplateDeleteVersionInput.LaunchTemplateId,
		err,
	)
}

func launchTemplateVersionDoesNotExist(errorCode string) bool {
	return errorCode == string(ec2types.LaunchTemplateErrorCodeLaunchTemplateVersionDoesNotExist) ||
		errorCode == string(ec2types.LaunchTemplateErrorCodeLaunchTemplateIdDoesNotExist)
}
