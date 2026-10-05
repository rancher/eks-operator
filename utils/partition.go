package utils

import (
	"github.com/aws/aws-sdk-go/aws/endpoints"
)

// AWSPartitionForRegion returns the AWS partition the given region belongs to,
// defaulting to the standard AWS partition for empty or unknown regions.
func AWSPartitionForRegion(region string) endpoints.Partition {
	if p, ok := endpoints.PartitionForRegion(endpoints.DefaultPartitions(), region); ok {
		return p
	}
	return endpoints.AwsPartition()
}

// AWSDNSSuffix returns the DNS suffix of the partition the given region belongs to.
func AWSDNSSuffix(region string) string {
	return AWSPartitionForRegion(region).DNSSuffix()
}

// AWSARNPrefix returns the ARN prefix of the partition the given region belongs to.
func AWSARNPrefix(region string) string {
	return "arn:" + AWSPartitionForRegion(region).ID()
}

// SupportsDualStackEndpoints returns true for every AWS partition except AWS
// China, which is the only partition where a service used by the operator, EC2,
// has no working dual-stack endpoint: ec2.<region>.amazonaws.com.cn exists but
// ec2.<region>.api.amazonwebservices.com.cn does not.
func SupportsDualStackEndpoints(region string) bool {
	return AWSPartitionForRegion(region).ID() != endpoints.AwsCnPartitionID
}
