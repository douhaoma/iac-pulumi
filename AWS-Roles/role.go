package AWS_Roles

import (
	"github.com/pulumi/pulumi-aws/sdk/v4/go/aws/iam"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func CreateEC2Role(ctx *pulumi.Context) (*iam.Role, error) {
	// 创建 IAM 角色
	role, err := iam.NewRole(ctx, "CloudWatchAndSNS", &iam.RoleArgs{
		AssumeRolePolicy: pulumi.String(`{
                "Version": "2012-10-17",
                "Statement": [{
                    "Action": "sts:AssumeRole",
                    "Effect": "Allow",
                    "Principal": {
                        "Service": "ec2.amazonaws.com"
                    }
                }]
            }`),
	})
	if err != nil {
		return nil, err
	}
	return role, nil
}

func CreateCWPolicyAttachment(ctx *pulumi.Context, role *iam.Role) error {
	_, err := iam.NewRolePolicyAttachment(ctx, "CloudWatchPolicyAttachment", &iam.RolePolicyAttachmentArgs{
		Role:      role.Name,
		PolicyArn: pulumi.String("arn:aws:iam::aws:policy/CloudWatchAgentServerPolicy"),
	})
	if err != nil {
		return err
	}
	return nil
}
func CreateSNSPolicyAttachment(ctx *pulumi.Context, role *iam.Role) error {
	// 创建一个策略，仅允许访问 SNS
	snsPolicy, err := iam.NewPolicy(ctx, "SNSPolicyAttachment", &iam.PolicyArgs{
		Policy: pulumi.String(`{
                "Version": "2012-10-17",
                "Statement": [{
                    "Effect": "Allow",
                    "Action": "sns:Publish",
                    "Resource": "*"
                }]
            }`),
	})
	if err != nil {
		return err
	}

	// 将策略附加到新角色
	_, err = iam.NewRolePolicyAttachment(ctx, "snsRolePolicyAttachment", &iam.RolePolicyAttachmentArgs{
		Role:      role.Name,
		PolicyArn: snsPolicy.Arn,
	})
	if err != nil {
		return err
	}

	return nil
}
