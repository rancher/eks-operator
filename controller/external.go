package controller

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/eks"
	eksv1 "github.com/rancher/eks-operator/pkg/apis/eks.cattle.io/v1"
	wranglerv1 "github.com/rancher/wrangler/v2/pkg/generated/controllers/core/v1"
)

// StartAWSSessions starts AWS sessions.
func StartAWSSessions(secretsCache wranglerv1.SecretCache, spec eksv1.EKSClusterConfigSpec) (*session.Session, *eks.EKS, error) {
	sess, err := newAWSSession(secretsCache, spec)
	if err != nil {
		return nil, nil, fmt.Errorf("error getting new aws session: %v", err)
	}
	return sess, eks.New(sess), nil
}

// NodeGroupIssueIsUpdatable checks to see the node group can be updated with the given issue code.
func NodeGroupIssueIsUpdatable(code string) bool {
	return code == eks.NodegroupIssueCodeAsgInstanceLaunchFailures ||
		code == eks.NodegroupIssueCodeInstanceLimitExceeded ||
		code == eks.NodegroupIssueCodeInsufficientFreeAddresses ||
		code == eks.NodegroupIssueCodeClusterUnreachable
}
