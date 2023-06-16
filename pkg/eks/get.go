package eks

import (
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/eks"
	eksv1 "github.com/rancher/eks-operator/pkg/apis/eks.cattle.io/v1"
	"github.com/rancher/eks-operator/pkg/eks/services"
)

type GetClusterStatusOpts struct {
	EKSService services.EKSServiceInterface
	Config     *eksv1.EKSClusterConfig
}

func GetClusterState(opts *GetClusterStatusOpts) (*eks.DescribeClusterOutput, error) {
	return opts.EKSService.DescribeCluster(
		&eks.DescribeClusterInput{
			Name: aws.String(opts.Config.Spec.DisplayName),
		})
}

type GetLaunchTemplateVersionsOpts struct {
	EC2Service       services.EC2ServiceInterface
	LaunchTemplateID *string
	Versions         []*string
}

func GetLaunchTemplateVersions(opts *GetLaunchTemplateVersionsOpts) (*ec2.DescribeLaunchTemplateVersionsOutput, error) {
	if opts.LaunchTemplateID == nil {
		return nil, fmt.Errorf("launch template ID is nil")
	}

	if opts.Versions == nil {
		return nil, fmt.Errorf("launch template versions are nil")
	}

	return opts.EC2Service.DescribeLaunchTemplateVersions(
		&ec2.DescribeLaunchTemplateVersionsInput{
			LaunchTemplateId: opts.LaunchTemplateID,
			Versions:         opts.Versions,
		})
}

// CheckEBSAddon checks if the EBS CSI driver add-on is installed. If it is, it will return
// the ARN of the add-on. If it is not, it will return an empty string. Otherwise, it will return an error
func CheckEBSAddon(eksService services.EKSServiceInterface, config *eksv1.EKSClusterConfig) (string, error) {
	input := eks.DescribeAddonInput{
		AddonName:   aws.String(ebsCSIAddonName),
		ClusterName: aws.String(config.Spec.DisplayName),
	}

	output, err := eksService.DescribeAddon(&input)
	if err != nil {
		var genericAWSErr awserr.Error
		if errors.As(err, &genericAWSErr) && genericAWSErr.Code() == eks.ErrCodeResourceNotFoundException {
			return "", nil
		}
		return "", err
	}
	if output.Addon == nil {
		return "", nil
	}

	return *output.Addon.AddonArn, nil
}
