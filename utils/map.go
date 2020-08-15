package utils

import (
	"github.com/aws/aws-sdk-go/aws"
)

func GetKeyValuesToUpdate(tags map[string]string, upstreamTags map[string]string) map[string]*string {
	if len(tags) == 0 {
		return nil
	}

	if len(upstreamTags) == 0 {
		return aws.StringMap(tags)
	}

	updateTags := make(map[string]*string)
	for key, val := range tags {
		if upstreamTags[key] != val {
			updateTags[key] = aws.String(val)
		}
	}

	if len(updateTags) == 0 {
		return nil
	}
	return updateTags
}

func GetKeysToDelete(tags map[string]string, upstreamTags map[string]string) []*string {
	if len(upstreamTags) == 0 {
		return nil
	}

	var updateUntags []*string
	for key, val := range upstreamTags {
		if len(tags) == 0 || tags[key] != val {
			updateUntags = append(updateUntags, aws.String(key))
		}
	}

	if len(updateUntags) == 0 {
		return nil
	}
	return updateUntags
}
