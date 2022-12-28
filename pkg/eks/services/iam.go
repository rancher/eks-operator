package services

import (
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/iam"
)

type IAMServiceInterface interface {
	GetRole(input *iam.GetRoleInput) (*iam.GetRoleOutput, error)
}

type iamService struct {
	svc *iam.IAM
}

func NewIAMService(sess *session.Session) *iamService {
	return &iamService{
		svc: iam.New(sess),
	}
}

func (c *iamService) GetRole(input *iam.GetRoleInput) (*iam.GetRoleOutput, error) {
	return c.svc.GetRole(input)
}
