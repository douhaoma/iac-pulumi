package highAvailability

import (
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/alb"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/ec2"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func CreateLoadBalancerSG(ctx *pulumi.Context, vpcId pulumi.IDOutput) (*ec2.SecurityGroup, error) {
	// 创建安全组
	lbSecGroup, err := ec2.NewSecurityGroup(ctx, "loadBalancerSG", &ec2.SecurityGroupArgs{
		Description: pulumi.String("Load balancer security group"),
		VpcId:       vpcId,
	})
	if err != nil {
		return nil, err
	}

	// 添加入口规则 - HTTP
	_, err = ec2.NewSecurityGroupRule(ctx, "httpIngress", &ec2.SecurityGroupRuleArgs{
		Type:            pulumi.String("ingress"),
		FromPort:        pulumi.Int(80),
		ToPort:          pulumi.Int(80),
		Protocol:        pulumi.String("tcp"),
		CidrBlocks:      pulumi.StringArray{pulumi.String("0.0.0.0/0")},
		SecurityGroupId: lbSecGroup.ID(),
	})
	if err != nil {
		return nil, err
	}

	// 添加入口规则 - HTTPS
	_, err = ec2.NewSecurityGroupRule(ctx, "httpsIngress", &ec2.SecurityGroupRuleArgs{
		Type:            pulumi.String("ingress"),
		FromPort:        pulumi.Int(443),
		ToPort:          pulumi.Int(443),
		Protocol:        pulumi.String("tcp"),
		CidrBlocks:      pulumi.StringArray{pulumi.String("0.0.0.0/0")},
		SecurityGroupId: lbSecGroup.ID(),
	})
	if err != nil {
		return nil, err
	}

	// egress from lb to ec2
	_, err = ec2.NewSecurityGroupRule(ctx, "toEC2Egress", &ec2.SecurityGroupRuleArgs{
		Type:            pulumi.String("egress"),
		FromPort:        pulumi.Int(8080),
		ToPort:          pulumi.Int(8080),
		Protocol:        pulumi.String("tcp"),
		CidrBlocks:      pulumi.StringArray{pulumi.String("0.0.0.0/0")},
		SecurityGroupId: lbSecGroup.ID(),
	})
	if err != nil {
		return nil, err
	}
	return lbSecGroup, nil
}

func CreateLoadBalancer(ctx *pulumi.Context, targetGroup *alb.TargetGroup, lbSG *ec2.SecurityGroup, subnetIds []pulumi.IDOutput) (*alb.LoadBalancer, error) {
	// 创建应用程序负载均衡器
	var subnetIDs pulumi.StringArray
	for _, id := range subnetIds {
		subnetIDs = append(subnetIDs, id)
	}

	lb, err := alb.NewLoadBalancer(ctx, "appLoadBalancer", &alb.LoadBalancerArgs{
		Internal:         pulumi.Bool(false),
		SecurityGroups:   pulumi.StringArray{lbSG.ID()}, // 负载均衡器的安全组
		Subnets:          subnetIDs,
		LoadBalancerType: pulumi.String("application"),
	})
	if err != nil {
		return nil, err
	}

	// 创建 80 负载均衡器监听器
	_, err = alb.NewListener(ctx, "appListener", &alb.ListenerArgs{
		LoadBalancerArn: lb.Arn,
		Port:            pulumi.Int(80),
		DefaultActions: alb.ListenerDefaultActionArray{
			&alb.ListenerDefaultActionArgs{
				Type:           pulumi.String("forward"),
				TargetGroupArn: targetGroup.Arn, // 使用自动伸缩组的目标组
			},
		},
	})
	if err != nil {
		return nil, err
	}

	// 创建 443 负载均衡器监听器
	c := config.New(ctx, "")
	certificateArn := c.RequireSecret("certificateArn")
	_, err = alb.NewListener(ctx, "httpsListener", &alb.ListenerArgs{
		LoadBalancerArn: lb.Arn,
		Port:            pulumi.Int(443),
		Protocol:        pulumi.String("HTTPS"),
		SslPolicy:       pulumi.String("ELBSecurityPolicy-2016-08"), // 根据需要选择合适的 SSL 政策
		CertificateArn:  certificateArn,                             // ssl 证书 ARN
		DefaultActions: alb.ListenerDefaultActionArray{
			&alb.ListenerDefaultActionArgs{
				Type:           pulumi.String("forward"),
				TargetGroupArn: targetGroup.Arn,
			},
		},
	})

	if err != nil {
		return nil, err
	}
	return lb, nil
}
