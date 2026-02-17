package templates

// These are the CloudFormation templates used for EKS clusters, when making edits here ensure the whitespace is correct.

const (
	VpcTemplate = `---
AWSTemplateFormatVersion: '2010-09-09'
Description: 'Amazon EKS Sample VPC'

Parameters:

  VpcBlock:
    Type: String
    Default: 192.168.0.0/16
    Description: The CIDR range for the VPC. This should be a valid private (RFC 1918) CIDR range.

  Subnet01Block:
    Type: String
    Default: 192.168.64.0/18
    Description: CidrBlock for subnet 01 within the VPC

  Subnet02Block:
    Type: String
    Default: 192.168.128.0/18
    Description: CidrBlock for subnet 02 within the VPC

  Subnet03Block:
    Type: String
    Default: 192.168.192.0/18
    Description: CidrBlock for subnet 03 within the VPC. This is used only if the region has more than 2 AZs.

Metadata:
  AWS::CloudFormation::Interface:
    ParameterGroups:
      -
        Label:
          default: "Worker Network Configuration"
        Parameters:
          - VpcBlock
          - Subnet01Block
          - Subnet02Block
          - Subnet03Block

Conditions:
  Has2Azs:
    Fn::Or:
      - Fn::Equals:
        - {Ref: 'AWS::Region'}
        - ap-south-1
      - Fn::Equals:
        - {Ref: 'AWS::Region'}
        - ap-northeast-2
      - Fn::Equals:
        - {Ref: 'AWS::Region'}
        - ca-central-1
      - Fn::Equals:
        - {Ref: 'AWS::Region'}
        - cn-north-1

  HasMoreThan2Azs:
    Fn::Not:
      - Condition: Has2Azs

Resources:
  VPC:
    Type: AWS::EC2::VPC
    Properties:
      CidrBlock:  !Ref VpcBlock
      EnableDnsSupport: true
      EnableDnsHostnames: true
      Tags:
      - Key: Name
        Value: !Sub '${AWS::StackName}-VPC'

  InternetGateway:
    Type: "AWS::EC2::InternetGateway"

  VPCGatewayAttachment:
    Type: "AWS::EC2::VPCGatewayAttachment"
    Properties:
      InternetGatewayId: !Ref InternetGateway
      VpcId: !Ref VPC

  RouteTable:
    Type: AWS::EC2::RouteTable
    Properties:
      VpcId: !Ref VPC
      Tags:
      - Key: Name
        Value: Public Subnets
      - Key: Network
        Value: Public

  Route:
    DependsOn: VPCGatewayAttachment
    Type: AWS::EC2::Route
    Properties:
      RouteTableId: !Ref RouteTable
      DestinationCidrBlock: 0.0.0.0/0
      GatewayId: !Ref InternetGateway

  Subnet01:
    Type: AWS::EC2::Subnet
    Metadata:
      Comment: Subnet 01
    Properties:
      MapPublicIpOnLaunch: true
      AvailabilityZone:
        Fn::Select:
        - '0'
        - Fn::GetAZs:
            Ref: AWS::Region
      CidrBlock:
        Ref: Subnet01Block
      VpcId:
        Ref: VPC
      Tags:
      - Key: Name
        Value: !Sub "${AWS::StackName}-Subnet01"
      - Key: kubernetes.io/role/elb
        Value: 1

  Subnet02:
    Type: AWS::EC2::Subnet
    Metadata:
      Comment: Subnet 02
    Properties:
      MapPublicIpOnLaunch: true
      AvailabilityZone:
        Fn::Select:
        - '1'
        - Fn::GetAZs:
            Ref: AWS::Region
      CidrBlock:
        Ref: Subnet02Block
      VpcId:
        Ref: VPC
      Tags:
      - Key: Name
        Value: !Sub "${AWS::StackName}-Subnet02"
      - Key: kubernetes.io/role/elb
        Value: 1

  Subnet03:
    Condition: HasMoreThan2Azs
    Type: AWS::EC2::Subnet
    Metadata:
      Comment: Subnet 03
    Properties:
      MapPublicIpOnLaunch: true
      AvailabilityZone:
        Fn::Select:
        - '2'
        - Fn::GetAZs:
            Ref: AWS::Region
      CidrBlock:
        Ref: Subnet03Block
      VpcId:
        Ref: VPC
      Tags:
      - Key: Name
        Value: !Sub "${AWS::StackName}-Subnet03"
      - Key: kubernetes.io/role/elb
        Value: 1

  Subnet01RouteTableAssociation:
    Type: AWS::EC2::SubnetRouteTableAssociation
    Properties:
      SubnetId: !Ref Subnet01
      RouteTableId: !Ref RouteTable

  Subnet02RouteTableAssociation:
    Type: AWS::EC2::SubnetRouteTableAssociation
    Properties:
      SubnetId: !Ref Subnet02
      RouteTableId: !Ref RouteTable

  Subnet03RouteTableAssociation:
    Condition: HasMoreThan2Azs
    Type: AWS::EC2::SubnetRouteTableAssociation
    Properties:
      SubnetId: !Ref Subnet03
      RouteTableId: !Ref RouteTable

Outputs:

  SubnetIds:
    Description: All subnets in the VPC
    Value:
      Fn::If:
      - HasMoreThan2Azs
      - !Join [ ",", [ !Ref Subnet01, !Ref Subnet02, !Ref Subnet03 ] ]
      - !Join [ ",", [ !Ref Subnet01, !Ref Subnet02 ] ]

  VpcId:
    Description: The VPC Id
    Value: !Ref VPC
`
	VpcIPv6Template = `---
AWSTemplateFormatVersion: '2010-09-09'
Description: 'VPC with IPv4, IPv6, public subnets'

Parameters:

  VpcBlock:
    Type: String
    Default: 192.168.0.0/16
    Description: The CIDR range for the VPC.

  PublicSubnet01Block:
    Type: String
    Default: 192.168.0.0/18
    Description: CidrBlock for public subnet 01 within the VPC

  PublicSubnet02Block:
    Type: String
    Default: 192.168.64.0/18
    Description: CidrBlock for public subnet 02 within the VPC

  PublicSubnet03Block:
    Type: String
    Default: 192.168.128.0/18
    Description: CidrBlock for public subnet 03 within the VPC. This is used only if the region has more than 2 AZs.

Metadata:
  AWS::CloudFormation::Interface:
    ParameterGroups:
      -
        Label:
          default: "Worker Network Configuration"
        Parameters:
          - VpcBlock
          - PublicSubnet01Block
          - PublicSubnet02Block
          - PublicSubnet03Block

Conditions:
  Has2Azs:
    Fn::Or:
      - Fn::Equals:
        - {Ref: 'AWS::Region'}
        - ap-south-1
      - Fn::Equals:
        - {Ref: 'AWS::Region'}
        - ap-northeast-2
      - Fn::Equals:
        - {Ref: 'AWS::Region'}
        - ca-central-1
      - Fn::Equals:
        - {Ref: 'AWS::Region'}
        - cn-north-1

  HasMoreThan2Azs:
    Fn::Not:
      - Condition: Has2Azs

Resources:
  #
  # Public VPC
  #
  VPC:
    Type: AWS::EC2::VPC
    Properties:
      CidrBlock: !Ref VpcBlock
      EnableDnsHostnames: true
      EnableDnsSupport: true
      Tags:
        - Key: Name
          Value:
            Fn::Sub: "${AWS::StackName}/VPC"
  IPv6CidrBlock:
    Type: AWS::EC2::VPCCidrBlock
    Properties:
      AmazonProvidedIpv6CidrBlock: true
      VpcId:
        Ref: VPC

  #
  # Internet gateways (ipv4, and egress for ipv6)
  #
  InternetGateway:
    Type: AWS::EC2::InternetGateway
    Properties:
      Tags:
        - Key: Name
          Value:
            Fn::Sub: "${AWS::StackName}/InternetGateway"
  VPCGatewayAttachment:
    Type: AWS::EC2::VPCGatewayAttachment
    Properties:
      InternetGatewayId:
        Ref: InternetGateway
      VpcId:
        Ref: VPC
  #
  # Routing - public subnets
  #
  PublicRouteTable:
    Type: AWS::EC2::RouteTable
    Properties:
      VpcId:
        Ref: VPC
      Tags:
        - Key: Name
          Value:
            Fn::Sub: "${AWS::StackName}/PublicRouteTable"
  PublicSubnetDefaultRoute:
    Type: AWS::EC2::Route
    DependsOn:
      - VPCGatewayAttachment
    Properties:
      DestinationCidrBlock: 0.0.0.0/0
      GatewayId:
        Ref: InternetGateway
      RouteTableId:
        Ref: PublicRouteTable
  PublicSubnetDefaultIpv6Route:
    Type: AWS::EC2::Route
    Properties:
      DestinationIpv6CidrBlock: ::/0
      GatewayId:
        Ref: InternetGateway
      RouteTableId:
        Ref: PublicRouteTable
  #
  # Public subnets
  # Note: AssignIpv6AddressOnCreation: true is required but a CFN limitation right now, need to add this manually
  SubnetPublic01:
    Type: AWS::EC2::Subnet
    Metadata:
      Comment: Subnet 01
    DependsOn: IPv6CidrBlock
    Properties:
      AvailabilityZone:
        Fn::Select:
        - '0'
        - Fn::GetAZs:
            Ref: AWS::Region
      CidrBlock:
        Ref: PublicSubnet01Block
      Ipv6CidrBlock: !Select [ 0, !Cidr [ !Select [ 0, !GetAtt VPC.Ipv6CidrBlocks], 8, 64 ]]
      MapPublicIpOnLaunch: true
      AssignIpv6AddressOnCreation: true
      Tags:
        - Key: kubernetes.io/role/elb
          Value: '1'
        - Key: Name
          Value:
            Fn::Sub: "${AWS::StackName}/SubnetPublic01"
      VpcId:
        Ref: VPC
  SubnetPublic02:
    Type: AWS::EC2::Subnet
    DependsOn: IPv6CidrBlock
    Properties:
      AvailabilityZone:
        Fn::Select:
        - '1'
        - Fn::GetAZs:
            Ref: AWS::Region
      CidrBlock:
        Ref: PublicSubnet02Block
      Ipv6CidrBlock: !Select [ 1, !Cidr [ !Select [ 0, !GetAtt VPC.Ipv6CidrBlocks], 8, 64 ]]
      MapPublicIpOnLaunch: true
      AssignIpv6AddressOnCreation: true
      Tags:
        - Key: kubernetes.io/role/elb
          Value: '1'
        - Key: Name
          Value:
            Fn::Sub: "${AWS::StackName}/SubnetPublic02"
      VpcId:
        Ref: VPC

  SubnetPublic03:
    Condition: HasMoreThan2Azs
    Type: AWS::EC2::Subnet
    DependsOn: IPv6CidrBlock
    Properties:
      AvailabilityZone:
        Fn::Select:
        - '2'
        - Fn::GetAZs:
            Ref: AWS::Region
      CidrBlock:
        Ref: PublicSubnet03Block
      Ipv6CidrBlock: !Select [ 2, !Cidr [ !Select [ 0, !GetAtt VPC.Ipv6CidrBlocks], 8, 64 ]]
      MapPublicIpOnLaunch: true
      AssignIpv6AddressOnCreation: true
      Tags:
        - Key: kubernetes.io/role/elb
          Value: '1'
        - Key: Name
          Value:
            Fn::Sub: "${AWS::StackName}/SubnetPublic03"
      VpcId:
        Ref: VPC

  #
  # Public route table associations
  #
  RouteTableAssociationPublic01:
    Type: AWS::EC2::SubnetRouteTableAssociation
    Properties:
      RouteTableId:
        Ref: PublicRouteTable
      SubnetId:
        Ref: SubnetPublic01
  RouteTableAssociationPublic02:
    Type: AWS::EC2::SubnetRouteTableAssociation
    Properties:
      RouteTableId:
        Ref: PublicRouteTable
      SubnetId:
        Ref: SubnetPublic02

  RouteTableAssociationPublic03:
    Condition: HasMoreThan2Azs
    Type: AWS::EC2::SubnetRouteTableAssociation
    Properties:
      RouteTableId:
        Ref: PublicRouteTable
      SubnetId:
        Ref: SubnetPublic03

Outputs:
  SubnetIds:
    Description: All public subnets in the VPC
    Value:
      Fn::If:
      - HasMoreThan2Azs
      - !Join [ ",", [ !Ref SubnetPublic01, !Ref SubnetPublic02, !Ref SubnetPublic03 ] ]
      - !Join [ ",", [ !Ref SubnetPublic01, !Ref SubnetPublic02 ] ]

  VpcId:
    Description: The VPC Id
    Value: !Ref VPC
`
	NodeInstanceRoleTemplate = `---
AWSTemplateFormatVersion: 2010-09-09
Description: Amazon EKS - Node Group


Resources:

  NodeInstanceRole:
    Type: AWS::IAM::Role
    Properties:
      AssumeRolePolicyDocument:
        Version: 2012-10-17
        Statement:
          - Effect: Allow
            Principal:
              Service: {{.EC2Service}}
            Action: sts:AssumeRole
      Path: "/"
      ManagedPolicyArns:
        - {{.AWSArnPrefix}}:iam::aws:policy/AmazonEKSWorkerNodePolicy
        - {{.AWSArnPrefix}}:iam::aws:policy/AmazonEKS_CNI_Policy
        - {{.AWSArnPrefix}}:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly

Outputs:

  NodeInstanceRole:
    Description: The node instance role
    Value: !GetAtt NodeInstanceRole.Arn
`
	NodeInstanceRoleIPv6Template = `---
AWSTemplateFormatVersion: 2010-09-09
Description: Amazon EKS - Node Group (IPv6)


Resources:

  NodeInstanceRole:
    Type: AWS::IAM::Role
    Properties:
      AssumeRolePolicyDocument:
        Version: 2012-10-17
        Statement:
          - Effect: Allow
            Principal:
              Service: {{.EC2Service}}
            Action: sts:AssumeRole
      Path: "/"
      ManagedPolicyArns:
        - {{.AWSArnPrefix}}:iam::aws:policy/AmazonEKSWorkerNodePolicy
        - {{.AWSArnPrefix}}:iam::aws:policy/AmazonEKS_CNI_Policy
        - {{.AWSArnPrefix}}:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly
      Policies:
      - PolicyName: RancherManaged_AllowIPv6ForCNI
        PolicyDocument:
          Version: '2012-10-17'
          Statement:
            - Effect: Allow
              Action:
                - ec2:AssignIpv6Addresses
                - ec2:UnassignIpv6Addresses
              Resource: "*"

Outputs:

  NodeInstanceRole:
    Description: The node instance role
    Value: !GetAtt NodeInstanceRole.Arn
`
	ServiceRoleTemplate = `---
AWSTemplateFormatVersion: '2010-09-09'
Description: 'Amazon EKS Service Role'


Resources:

  AWSServiceRoleForAmazonEKS:
    Type: AWS::IAM::Role
    Properties:
      AssumeRolePolicyDocument:
        Version: '2012-10-17'
        Statement:
        - Effect: Allow
          Principal:
            Service:
            - eks.amazonaws.com
          Action:
          - sts:AssumeRole
      ManagedPolicyArns:
        - {{.AWSArnPrefix}}:iam::aws:policy/AmazonEKSServicePolicy
        - {{.AWSArnPrefix}}:iam::aws:policy/AmazonEKSClusterPolicy

Outputs:

  RoleArn:
    Description: The role that EKS will use to create AWS resources for Kubernetes clusters
    Value: !GetAtt AWSServiceRoleForAmazonEKS.Arn
    Export:
      Name: !Sub "${AWS::StackName}-RoleArn"

`
	EBSCSIDriverTemplate = `---
AWSTemplateFormatVersion: '2010-09-09'
Description: 'Amazon EKS EBS CSI Driver Role'


Parameters:

  AmazonEBSCSIDriverPolicyArn:
    Type: String
    Default: {{.AWSArnPrefix}}:iam::aws:policy/service-role/AmazonEBSCSIDriverPolicy
    Description: The ARN of the managed policy

Resources:

  AWSEBSCSIDriverRoleForAmazonEKS:
    Type: AWS::IAM::Role
    Properties:
      AssumeRolePolicyDocument:
        Version: '2012-10-17'
        Statement:
        - Effect: Allow
          Principal:
            Federated:
            - !Sub "{{.AWSArnPrefix}}:iam::${AWS::AccountId}:oidc-provider/oidc.eks.{{.Region}}.{{.AWSDomain}}/id/{{.ProviderID}}"
          Action: sts:AssumeRoleWithWebIdentity
          Condition:
            StringEquals: {
              "oidc.eks.{{.Region}}.{{.AWSDomain}}/id/{{.ProviderID}}:sub": "system:serviceaccount:kube-system:ebs-csi-controller-sa",
              "oidc.eks.{{.Region}}.{{.AWSDomain}}/id/{{.ProviderID}}:aud": "sts.{{.AWSDomain}}"
            }
      Path: "/"
      ManagedPolicyArns:
      - !Ref AmazonEBSCSIDriverPolicyArn

Outputs:

  EBSCSIDriverRole:
    Description: The role that EKS will for enabling the EBS CSI driver
    Value: !GetAtt AWSEBSCSIDriverRoleForAmazonEKS.Arn
    Export:
      Name: !Sub "${AWS::StackName}-RoleArn"

`
)
