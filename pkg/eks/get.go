package eks

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/eks"
	eksv1 "github.com/rancher/eks-operator/pkg/apis/eks.cattle.io/v1"
	"github.com/rancher/eks-operator/pkg/eks/services"
)

type GetClusterStatusOpts struct {
	EKSService services.EKSServiceInterface
	Config     *eksv1.EKSClusterConfig
}

func GetClusterState(opts GetClusterStatusOpts) (*eks.DescribeClusterOutput, error) {
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

func GetLaunchTemplateVersions(opts GetLaunchTemplateVersionsOpts) (*ec2.DescribeLaunchTemplateVersionsOutput, error) {
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
