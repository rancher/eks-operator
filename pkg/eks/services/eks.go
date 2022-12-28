package services

import (
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/eks"
)

type EKSServiceInterface interface {
	CreateCluster(input *eks.CreateClusterInput) (*eks.CreateClusterOutput, error)
	DeleteCluster(input *eks.DeleteClusterInput) (*eks.DeleteClusterOutput, error)
	ListClusters(input *eks.ListClustersInput) (*eks.ListClustersOutput, error)
	DescribeCluster(input *eks.DescribeClusterInput) (*eks.DescribeClusterOutput, error)
	UpdateClusterConfig(input *eks.UpdateClusterConfigInput) (*eks.UpdateClusterConfigOutput, error)
	UpdateClusterVersion(input *eks.UpdateClusterVersionInput) (*eks.UpdateClusterVersionOutput, error)
	CreateNodegroup(input *eks.CreateNodegroupInput) (*eks.CreateNodegroupOutput, error)
	UpdateNodegroupConfig(input *eks.UpdateNodegroupConfigInput) (*eks.UpdateNodegroupConfigOutput, error)
	ListNodegroups(input *eks.ListNodegroupsInput) (*eks.ListNodegroupsOutput, error)
	DeleteNodegroup(input *eks.DeleteNodegroupInput) (*eks.DeleteNodegroupOutput, error)
	DescribeNodegroup(input *eks.DescribeNodegroupInput) (*eks.DescribeNodegroupOutput, error)
	UpdateNodegroupVersion(input *eks.UpdateNodegroupVersionInput) (*eks.UpdateNodegroupVersionOutput, error)
	TagResource(input *eks.TagResourceInput) (*eks.TagResourceOutput, error)
	UntagResource(input *eks.UntagResourceInput) (*eks.UntagResourceOutput, error)
}

type eksService struct {
	svc *eks.EKS
}

func NewEKSService(sess *session.Session) *eksService {
	return &eksService{
		svc: eks.New(sess),
	}
}

func (c *eksService) CreateCluster(input *eks.CreateClusterInput) (*eks.CreateClusterOutput, error) {
	return c.svc.CreateCluster(input)
}

func (c *eksService) DeleteCluster(input *eks.DeleteClusterInput) (*eks.DeleteClusterOutput, error) {
	return c.svc.DeleteCluster(input)
}

func (c *eksService) ListClusters(input *eks.ListClustersInput) (*eks.ListClustersOutput, error) {
	return c.svc.ListClusters(input)
}

func (c *eksService) DescribeCluster(input *eks.DescribeClusterInput) (*eks.DescribeClusterOutput, error) {
	return c.svc.DescribeCluster(input)
}

func (c *eksService) UpdateClusterConfig(input *eks.UpdateClusterConfigInput) (*eks.UpdateClusterConfigOutput, error) {
	return c.svc.UpdateClusterConfig(input)
}

func (c *eksService) CreateNodegroup(input *eks.CreateNodegroupInput) (*eks.CreateNodegroupOutput, error) {
	return c.svc.CreateNodegroup(input)
}

func (c *eksService) UpdateNodegroupConfig(input *eks.UpdateNodegroupConfigInput) (*eks.UpdateNodegroupConfigOutput, error) {
	return c.svc.UpdateNodegroupConfig(input)
}

func (c *eksService) DeleteNodegroup(input *eks.DeleteNodegroupInput) (*eks.DeleteNodegroupOutput, error) {
	return c.svc.DeleteNodegroup(input)
}

func (c *eksService) ListNodegroups(input *eks.ListNodegroupsInput) (*eks.ListNodegroupsOutput, error) {
	return c.svc.ListNodegroups(input)
}

func (c *eksService) DescribeNodegroup(input *eks.DescribeNodegroupInput) (*eks.DescribeNodegroupOutput, error) {
	return c.svc.DescribeNodegroup(input)
}

func (c *eksService) UpdateClusterVersion(input *eks.UpdateClusterVersionInput) (*eks.UpdateClusterVersionOutput, error) {
	return c.svc.UpdateClusterVersion(input)
}

func (c *eksService) TagResource(input *eks.TagResourceInput) (*eks.TagResourceOutput, error) {
	return c.svc.TagResource(input)
}

func (c *eksService) UntagResource(input *eks.UntagResourceInput) (*eks.UntagResourceOutput, error) {
	return c.svc.UntagResource(input)
}

func (c *eksService) UpdateNodegroupVersion(input *eks.UpdateNodegroupVersionInput) (*eks.UpdateNodegroupVersionOutput, error) {
	return c.svc.UpdateNodegroupVersion(input)
}
