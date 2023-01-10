package mock_services

// Run go generate to regenerate this mock.
//
//go:generate ../../../../bin/mockgen -destination cloudformation_mock.go -package mock_services -source ../cloudformation.go CloudFormationServiceInterface
//go:generate ../../../../bin/mockgen -destination eks_mock.go -package mock_services -source ../eks.go EKSServiceInterface
//go:generate ../../../../bin/mockgen -destination iam_mock.go -package mock_services -source ../iam.go IAMServiceInterface
//go:generate ../../../../bin/mockgen -destination ec2_mock.go -package mock_services -source ../ec2.go EC2ServiceInterface
