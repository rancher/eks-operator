package controller

import (
	"strings"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/eks"
)

func isClusterConflict(err error) bool {
	if awsErr, ok := err.(awserr.Error); ok {
		return awsErr.Code() == eks.ErrCodeResourceInUseException
	}

	return false
}

func doesNotExist(err error) bool {
	// There is no better way of doing this because AWS API does not distinguish between a attempt to delete a stack
	// (or key pair) that does not exist, and, for example, a malformed delete request, so we have to parse the error
	// message
	if err != nil {
		return strings.Contains(err.Error(), "does not exist")
	}

	return false
}

func launchTemplateVersionDoesNotExist(errorCode string) bool {
	return errorCode == ec2.LaunchTemplateErrorCodeLaunchTemplateVersionDoesNotExist ||
		errorCode == ec2.LaunchTemplateErrorCodeLaunchTemplateIdDoesNotExist
}

func notFound(err error) bool {
	if awsErr, ok := err.(awserr.Error); ok {
		return awsErr.Code() == eks.ErrCodeResourceNotFoundException ||
			strings.Contains(awsErr.Code(), "VersionNotFound")
	}

	return false
}

// NodeGroupIssueIsUpdatable checks to see the node group can be updated with the given issue code.
func NodeGroupIssueIsUpdatable(code string) bool {
	return code == eks.NodegroupIssueCodeAsgInstanceLaunchFailures ||
		code == eks.NodegroupIssueCodeInstanceLimitExceeded ||
		code == eks.NodegroupIssueCodeInsufficientFreeAddresses ||
		code == eks.NodegroupIssueCodeClusterUnreachable
}
