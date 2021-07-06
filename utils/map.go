package utils

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
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
	for key := range upstreamTags {
		_, ok := tags[key]
		if !ok {
			updateUntags = append(updateUntags, aws.String(key))
		}
	}

	if len(updateUntags) == 0 {
		return nil
	}
	return updateUntags
}

// MergeMaps will add all keys and values from map2 to map1.
func MergeMaps(map1, map2 map[string]string) map[string]string {
	if map1 == nil {
		map1 = make(map[string]string)
	}
	for key, value := range map2 {
		map1[key] = value
	}

	return map1
}

// SubtractMaps will remove all keys and values in map2 from map1.
func SubtractMaps(map1, map2 map[string]string) map[string]string {
	if map1 == nil {
		return nil
	}
	for key := range map2 {
		delete(map1, key)
	}

	return map1
}

func ValuesFromMap(m map[string]string) []string {
	s := make([]string, len(m))
	i := 0
	for _, value := range m {
		s[i] = value
		i++
	}

	return s
}

func GetInstanceTags(templateTags []*ec2.LaunchTemplateTagSpecification) map[string]*string {
	tags := make(map[string]*string)

	for _, tag := range templateTags {
		if aws.StringValue(tag.ResourceType) == ec2.ResourceTypeInstance {
			for _, t := range tag.Tags {
				tags[aws.StringValue(t.Key)] = t.Value
			}
		}
	}
	return tags
}

func CreateTagSpecs(instanceTags map[string]*string) []*ec2.LaunchTemplateTagSpecificationRequest {
	if len(instanceTags) == 0 {
		return nil
	}

	tags := make([]*ec2.Tag, 0)
	for key, value := range instanceTags {
		tags = append(tags, &ec2.Tag{Key: aws.String(key), Value: value})
	}
	return []*ec2.LaunchTemplateTagSpecificationRequest{
		{
			ResourceType: aws.String(ec2.ResourceTypeInstance),
			Tags:         tags,
		},
	}
}

func CompareStringMaps(map1, map2 map[string]string) bool {
	if len(map1) != len(map2) {
		return false
	}
	for key, value1 := range map1 {
		if value2, exists := map2[key]; !exists || value1 != value2 {
			return false
		}
	}
	return true
}
