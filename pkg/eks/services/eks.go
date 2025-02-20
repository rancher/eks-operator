package services

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/sirupsen/logrus"
)

type EKSServiceInterface interface {
	CreateCluster(ctx context.Context, input *eks.CreateClusterInput) (*eks.CreateClusterOutput, error)
	DeleteCluster(ctx context.Context, input *eks.DeleteClusterInput) (*eks.DeleteClusterOutput, error)
	ListClusters(ctx context.Context, input *eks.ListClustersInput) (*eks.ListClustersOutput, error)
	DescribeCluster(ctx context.Context, input *eks.DescribeClusterInput) (*eks.DescribeClusterOutput, error)
	UpdateClusterConfig(ctx context.Context, input *eks.UpdateClusterConfigInput) (*eks.UpdateClusterConfigOutput, error)
	UpdateClusterVersion(ctx context.Context, input *eks.UpdateClusterVersionInput) (*eks.UpdateClusterVersionOutput, error)
	CreateNodegroup(ctx context.Context, input *eks.CreateNodegroupInput) (*eks.CreateNodegroupOutput, error)
	UpdateNodegroupConfig(ctx context.Context, input *eks.UpdateNodegroupConfigInput) (*eks.UpdateNodegroupConfigOutput, error)
	ListNodegroups(ctx context.Context, input *eks.ListNodegroupsInput) (*eks.ListNodegroupsOutput, error)
	DeleteNodegroup(ctx context.Context, input *eks.DeleteNodegroupInput) (*eks.DeleteNodegroupOutput, error)
	DescribeNodegroup(ctx context.Context, input *eks.DescribeNodegroupInput) (*eks.DescribeNodegroupOutput, error)
	UpdateNodegroupVersion(ctx context.Context, input *eks.UpdateNodegroupVersionInput) (*eks.UpdateNodegroupVersionOutput, error)
	TagResource(ctx context.Context, input *eks.TagResourceInput) (*eks.TagResourceOutput, error)
	UntagResource(ctx context.Context, input *eks.UntagResourceInput) (*eks.UntagResourceOutput, error)
	CreateAddon(ctx context.Context, input *eks.CreateAddonInput) (*eks.CreateAddonOutput, error)
	DescribeAddon(ctx context.Context, input *eks.DescribeAddonInput) (*eks.DescribeAddonOutput, error)
	DescribeUpdates(ctx context.Context, input *eks.ListUpdatesInput, completedUpdates map[string]bool) ([]*eks.DescribeUpdateOutput, error)
}

type eksService struct {
	svc *eks.Client
}

func NewEKSService(cfg aws.Config) EKSServiceInterface {
	return &eksService{
		svc: eks.NewFromConfig(cfg),
	}
}

func (c *eksService) CreateCluster(ctx context.Context, input *eks.CreateClusterInput) (*eks.CreateClusterOutput, error) {
	return c.svc.CreateCluster(ctx, input)
}

func (c *eksService) DeleteCluster(ctx context.Context, input *eks.DeleteClusterInput) (*eks.DeleteClusterOutput, error) {
	return c.svc.DeleteCluster(ctx, input)
}

func (c *eksService) ListClusters(ctx context.Context, input *eks.ListClustersInput) (*eks.ListClustersOutput, error) {
	return c.svc.ListClusters(ctx, input)
}

func (c *eksService) DescribeCluster(ctx context.Context, input *eks.DescribeClusterInput) (*eks.DescribeClusterOutput, error) {
	return c.svc.DescribeCluster(ctx, input)
}

func (c *eksService) UpdateClusterConfig(ctx context.Context, input *eks.UpdateClusterConfigInput) (*eks.UpdateClusterConfigOutput, error) {
	return c.svc.UpdateClusterConfig(ctx, input)
}

func (c *eksService) CreateNodegroup(ctx context.Context, input *eks.CreateNodegroupInput) (*eks.CreateNodegroupOutput, error) {
	return c.svc.CreateNodegroup(ctx, input)
}

func (c *eksService) UpdateNodegroupConfig(ctx context.Context, input *eks.UpdateNodegroupConfigInput) (*eks.UpdateNodegroupConfigOutput, error) {
	return c.svc.UpdateNodegroupConfig(ctx, input)
}

func (c *eksService) DeleteNodegroup(ctx context.Context, input *eks.DeleteNodegroupInput) (*eks.DeleteNodegroupOutput, error) {
	return c.svc.DeleteNodegroup(ctx, input)
}

func (c *eksService) ListNodegroups(ctx context.Context, input *eks.ListNodegroupsInput) (*eks.ListNodegroupsOutput, error) {
	return c.svc.ListNodegroups(ctx, input)
}

func (c *eksService) DescribeNodegroup(ctx context.Context, input *eks.DescribeNodegroupInput) (*eks.DescribeNodegroupOutput, error) {
	return c.svc.DescribeNodegroup(ctx, input)
}

func (c *eksService) UpdateClusterVersion(ctx context.Context, input *eks.UpdateClusterVersionInput) (*eks.UpdateClusterVersionOutput, error) {
	return c.svc.UpdateClusterVersion(ctx, input)
}

func (c *eksService) TagResource(ctx context.Context, input *eks.TagResourceInput) (*eks.TagResourceOutput, error) {
	return c.svc.TagResource(ctx, input)
}

func (c *eksService) UntagResource(ctx context.Context, input *eks.UntagResourceInput) (*eks.UntagResourceOutput, error) {
	return c.svc.UntagResource(ctx, input)
}

func (c *eksService) UpdateNodegroupVersion(ctx context.Context, input *eks.UpdateNodegroupVersionInput) (*eks.UpdateNodegroupVersionOutput, error) {
	return c.svc.UpdateNodegroupVersion(ctx, input)
}

func (c *eksService) CreateAddon(ctx context.Context, input *eks.CreateAddonInput) (*eks.CreateAddonOutput, error) {
	return c.svc.CreateAddon(ctx, input)
}

func (c *eksService) DescribeAddon(ctx context.Context, input *eks.DescribeAddonInput) (*eks.DescribeAddonOutput, error) {
	return c.svc.DescribeAddon(ctx, input)
}

func (c *eksService) DescribeUpdates(ctx context.Context, input *eks.ListUpdatesInput, completedUpdates map[string]bool) ([]*eks.DescribeUpdateOutput, error) {
	var updateIDs []string

	for {
		resp, err := c.svc.ListUpdates(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("failed to list updates: %v", err)
		}

		// Ignore fetching already completed updates
		for _, updateID := range resp.UpdateIds {
			if completedUpdates != nil && !completedUpdates[updateID] {
				logrus.Tracef("[%s] update already completed", updateID)
				updateIDs = append(updateIDs, updateID)
			}
		}
		if resp.NextToken == nil {
			break
		}
		input.NextToken = resp.NextToken
	}

	if len(updateIDs) == 0 {
		return []*eks.DescribeUpdateOutput{}, nil
	}

	updates := []*eks.DescribeUpdateOutput{}

	for _, updateID := range updateIDs {
		resp, err := c.svc.DescribeUpdate(ctx, &eks.DescribeUpdateInput{
			Name:     aws.String(*input.Name),
			UpdateId: aws.String(updateID),
		})
		if err != nil {
			logrus.Errorf("Failed to describe eks update %s: %v", updateID, err)
			return nil, err
		}
		updates = append(updates, resp)
	}

	return updates, nil
}
