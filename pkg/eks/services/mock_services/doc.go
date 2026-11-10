package mock_services

// Run go generate to regenerate this mock.
//
//go:generate go tool -modfile ../../../../gotools/mockgen/go.mod mockgen -destination cloudformation_mock.go -package mock_services -source ../cloudformation.go CloudFormationServiceInterface
//go:generate go tool -modfile ../../../../gotools/mockgen/go.mod mockgen -destination eks_mock.go -package mock_services -source ../eks.go EKSServiceInterface
//go:generate go tool -modfile ../../../../gotools/mockgen/go.mod mockgen -destination iam_mock.go -package mock_services -source ../iam.go IAMServiceInterface
//go:generate go tool -modfile ../../../../gotools/mockgen/go.mod mockgen -destination ec2_mock.go -package mock_services -source ../ec2.go EC2ServiceInterface
