package services

import (
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/iam"
)

type IAMServiceInterface interface {
	GetRole(input *iam.GetRoleInput) (*iam.GetRoleOutput, error)
	ListOIDCProviders(input *iam.ListOpenIDConnectProvidersInput) (*iam.ListOpenIDConnectProvidersOutput, error)
	CreateOIDCProvider(input *iam.CreateOpenIDConnectProviderInput) (*iam.CreateOpenIDConnectProviderOutput, error)
}

type iamService struct {
	svc *iam.IAM
}

func NewIAMService(sess *session.Session) IAMServiceInterface {
	return &iamService{
		svc: iam.New(sess),
	}
}

func (c *iamService) GetRole(input *iam.GetRoleInput) (*iam.GetRoleOutput, error) {
	return c.svc.GetRole(input)
}

func (c *iamService) ListOIDCProviders(input *iam.ListOpenIDConnectProvidersInput) (*iam.ListOpenIDConnectProvidersOutput, error) {
	return c.svc.ListOpenIDConnectProviders(input)
}

func (c *iamService) CreateOIDCProvider(input *iam.CreateOpenIDConnectProviderInput) (*iam.CreateOpenIDConnectProviderOutput, error) {
	return c.svc.CreateOpenIDConnectProvider(input)
}
