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
	eksv1 "github.com/rancher/eks-operator/pkg/apis/eks.cattle.io/v1"
	"github.com/rancher/eks-operator/pkg/eks/services"
	"github.com/sirupsen/logrus"
)

type GetClusterStatusOpts struct {
	EKSService services.EKSServiceInterface
	Config     *eksv1.EKSClusterConfig
}

func GetClusterState(ctx context.Context, opts *GetClusterStatusOpts) (*eks.DescribeClusterOutput, error) {
	return opts.EKSService.DescribeCluster(ctx,
		&eks.DescribeClusterInput{
			Name: aws.String(opts.Config.Spec.DisplayName),
		})
}

func GetClusterUpdates(ctx context.Context, opts *GetClusterStatusOpts) ([]*types.Update, []string, error) {
	log := logrus.WithField("function", "GetClusterUpdates")

	if opts.Config.Spec.DisplayName == "" {
		log.Error("cluster name is empty")
		return nil, nil, errors.New("cluster name is empty")
	}

	completedUpdates := make(map[string]bool)
	for _, updateID := range opts.Config.Status.CompletedUpdateIDs {
		completedUpdates[updateID] = true
	}
	log.Debugf("completed updates: %v", completedUpdates)

	log.Trace("fetching updates from EKS service")
	updatesOut, err := opts.EKSService.DescribeUpdates(ctx, &eks.ListUpdatesInput{
		Name: aws.String(opts.Config.Spec.DisplayName),
	}, completedUpdates)
	if err != nil {
		log.Errorf("failed to fetch updates: %v", err)
		return nil, nil, err
	}

	log.Debugf("retrieved %d updates", len(updatesOut))

	var inProgressUpdates []*types.Update
	var newCompletedUpdateIDs []string

	for _, out := range updatesOut {
		if out.Update.Status != ekstypes.UpdateStatusInProgress {
			newCompletedUpdateIDs = append(newCompletedUpdateIDs, aws.ToString(out.Update.Id))
		} else {
			inProgressUpdates = append(inProgressUpdates, out.Update)
		}
	}

	log.Debugf("new completed updates: %v", newCompletedUpdateIDs)
	return inProgressUpdates, newCompletedUpdateIDs, nil
}

type GetLaunchTemplateVersionsOpts struct {
	EC2Service       services.EC2ServiceInterface
	LaunchTemplateID *string
	Versions         []*string
}

func GetLaunchTemplateVersions(ctx context.Context, opts *GetLaunchTemplateVersionsOpts) (*ec2.DescribeLaunchTemplateVersionsOutput, error) {
	if opts.LaunchTemplateID == nil {
		return nil, fmt.Errorf("launch template ID is nil")
	}

	if opts.Versions == nil {
		return nil, fmt.Errorf("launch template versions are nil")
	}

	return opts.EC2Service.DescribeLaunchTemplateVersions(ctx,
		&ec2.DescribeLaunchTemplateVersionsInput{
			LaunchTemplateId: opts.LaunchTemplateID,
			Versions:         aws.ToStringSlice(opts.Versions),
		})
}

// CheckEBSAddon checks if the EBS CSI driver add-on is installed. If it is, it will return
// the ARN of the add-on. If it is not, it will return an empty string. Otherwise, it will return an error
func CheckEBSAddon(ctx context.Context, clusterName string, eksService services.EKSServiceInterface) (string, error) {
	input := eks.DescribeAddonInput{
		AddonName:   aws.String(ebsCSIAddonName),
		ClusterName: aws.String(clusterName),
	}

	output, err := eksService.DescribeAddon(ctx, &input)
	if err != nil {
		var rnf *ekstypes.ResourceNotFoundException
		if errors.As(err, &rnf) {
			return "", nil
		}
		return "", err
	}
	if output.Addon == nil {
		return "", nil
	}

	return *output.Addon.AddonArn, nil
}
