package controller

import (
	"errors"
	"strings"

	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
)

func isResourceInUse(err error) bool {
	var riu *ekstypes.ResourceInUseException
	return errors.As(err, &riu)
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

func notFound(err error) bool {
	var rnf *ekstypes.ResourceNotFoundException
	if errors.As(err, &rnf) {
		return true
	}

	if err != nil {
		return strings.Contains(err.Error(), "VersionNotFound")
	}
	return false
}
