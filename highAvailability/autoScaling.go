package highAvailability

import (
	"encoding/base64"
	"fmt"
	"github.com/pulumi/pulumi-aws/sdk/v4/go/aws/iam"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/alb"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/autoscaling"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/cloudwatch"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/ec2"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func CreateApp_LoadBalanSecurityGroup(ctx *pulumi.Context, vpcId pulumi.IDOutput) (*ec2.SecurityGroup, *ec2.SecurityGroup, error) {
	sg, err := ec2.NewSecurityGroup(ctx, "webapp_security_group",
		&ec2.SecurityGroupArgs{
			VpcId:       vpcId,
			Description: pulumi.String("webapp security group"),
		},
	)
	if err != nil {
		return nil, nil, err
	}
	// 数据库egress 3306
	_, err = ec2.NewSecurityGroupRule(ctx, fmt.Sprint("database_egress"),
		&ec2.SecurityGroupRuleArgs{
			Type:            pulumi.String("egress"),
			SecurityGroupId: sg.ID(),
			Protocol:        pulumi.String("tcp"),
			FromPort:        pulumi.Int(3306),
			ToPort:          pulumi.Int(3306),
			Description:     pulumi.String("database_egress"),
			CidrBlocks:      pulumi.StringArray{pulumi.String("0.0.0.0/0")},
		})

	// CloudWatch egress 443
	_, err = ec2.NewSecurityGroupRule(ctx, fmt.Sprint("EC2_to_CloudWatch_egress"),
		&ec2.SecurityGroupRuleArgs{
			Type:            pulumi.String("egress"),
			SecurityGroupId: sg.ID(),
			Protocol:        pulumi.String("tcp"),
			FromPort:        pulumi.Int(443),
			ToPort:          pulumi.Int(443),
			CidrBlocks:      pulumi.StringArray{pulumi.String("0.0.0.0/0")},
			Description:     pulumi.String("Allows HTTPS traffic to AWS CloudWatch"),
		})

	// LoadBalancer egress 8080
	_, err = ec2.NewSecurityGroupRule(ctx, fmt.Sprint("EC2_to_LoadBalancer_egress"),
		&ec2.SecurityGroupRuleArgs{
			Type:            pulumi.String("egress"),
			SecurityGroupId: sg.ID(),
			Protocol:        pulumi.String("tcp"),
			FromPort:        pulumi.Int(8080),
			ToPort:          pulumi.Int(8080),
			CidrBlocks:      pulumi.StringArray{pulumi.String("0.0.0.0/0")},
			Description:     pulumi.String("Allows traffic to AWS LoadBalancer"),
		})

	// app ingress 8080
	loadBalanceSG, err := CreateLoadBalancerSG(ctx, vpcId)
	_, err = ec2.NewSecurityGroupRule(ctx, "appPortIngress", &ec2.SecurityGroupRuleArgs{
		Type:                  pulumi.String("ingress"),
		FromPort:              pulumi.Int(8080),
		ToPort:                pulumi.Int(8080),
		Protocol:              pulumi.String("tcp"),
		SecurityGroupId:       sg.ID(),
		SourceSecurityGroupId: loadBalanceSG.ID(),
	})
	if err != nil {
		return nil, nil, err
	}

	// ssh ingress 22
	_, err = ec2.NewSecurityGroupRule(ctx, "sshIngress", &ec2.SecurityGroupRuleArgs{
		Type:            pulumi.String("ingress"),
		FromPort:        pulumi.Int(22),
		ToPort:          pulumi.Int(22),
		Protocol:        pulumi.String("tcp"),
		SecurityGroupId: sg.ID(),
		CidrBlocks:      pulumi.StringArray{pulumi.String("0.0.0.0/0")},
		Ipv6CidrBlocks:  pulumi.StringArray{pulumi.String("::/0")},
	})
	if err != nil {
		return nil, nil, err
	}
	return sg, loadBalanceSG, nil
}
func CreateAutoScalingTemplate(ctx *pulumi.Context, role *iam.Role, appSecurityGroup *ec2.SecurityGroup, userdata pulumi.StringOutput) (*ec2.LaunchTemplate, error) {
	// 创建启动模板
	keyPairName, _ := ctx.GetConfig("ami:keyPair")
	customAmiID, _ := ctx.GetConfig("ami:ID")

	// create an instance profile
	instanceProfile, err := iam.NewInstanceProfile(ctx, "instanceProfile", &iam.InstanceProfileArgs{
		Role: role.Name,
	})

	// create launch template
	//launchConfiguration, err := ec2.NewLaunchConfiguration(ctx, "serverLaunchConfiguration", &ec2.LaunchConfigurationArgs{
	//	ImageId:                  pulumi.String(customAmiID),
	//	InstanceType:             pulumi.String("t2.micro"),
	//	KeyName:                  pulumi.String(keyPairName),
	//	IamInstanceProfile:       instanceProfile,
	//	SecurityGroups:           pulumi.StringArray{appSecurityGroup.ID()},
	//	AssociatePublicIpAddress: pulumi.Bool(true),
	//	UserData:                 userdata,
	//})
	userDataBase64 := userdata.ApplyT(func(ud string) string {
		return base64.StdEncoding.EncodeToString([]byte(ud))
	}).(pulumi.StringOutput)

	launchTemplate, err := ec2.NewLaunchTemplate(ctx, "serverLaunchTemplate", &ec2.LaunchTemplateArgs{
		Name:         pulumi.String("EC2-launch-template"),
		ImageId:      pulumi.String(customAmiID),
		InstanceType: pulumi.String("t2.micro"),
		KeyName:      pulumi.String(keyPairName),
		IamInstanceProfile: &ec2.LaunchTemplateIamInstanceProfileArgs{
			Name: instanceProfile.Name,
		},
		NetworkInterfaces: ec2.LaunchTemplateNetworkInterfaceArray{
			&ec2.LaunchTemplateNetworkInterfaceArgs{
				AssociatePublicIpAddress: pulumi.String("true"),
				SecurityGroups:           pulumi.StringArray{appSecurityGroup.ID()},
			},
		},
		UserData: userDataBase64,
	})
	if err != nil {
		return nil, err
	}

	return launchTemplate, nil
}

func CreateAutoScalingGroup(ctx *pulumi.Context, targetGroup *alb.TargetGroup, template *ec2.LaunchTemplate, subnetIds []pulumi.IDOutput) (*autoscaling.Group, error) {
	env, _ := ctx.GetConfig("aws:profile")
	var subnetIDs pulumi.StringArray
	for _, id := range subnetIds {
		subnetIDs = append(subnetIDs, id)
	}
	asg, err := autoscaling.NewGroup(ctx, "appAutoScalingGroup", &autoscaling.GroupArgs{
		Name:            pulumi.String("csye6225-asg"),
		DefaultCooldown: pulumi.Int(60),
		LaunchTemplate: &autoscaling.GroupLaunchTemplateArgs{
			Id:      template.ID(),
			Version: pulumi.String("$Latest"),
		},
		MinSize:            pulumi.Int(1),
		MaxSize:            pulumi.Int(3),
		DesiredCapacity:    pulumi.Int(1),
		VpcZoneIdentifiers: subnetIDs,
		TargetGroupArns: pulumi.StringArray{
			targetGroup.Arn,
		},
		Tags: autoscaling.GroupTagArray{
			&autoscaling.GroupTagArgs{
				Key:               pulumi.String("Name"),
				Value:             pulumi.String("csye6225-webserver-ec2-instance"),
				PropagateAtLaunch: pulumi.Bool(true),
			},
			&autoscaling.GroupTagArgs{
				Key:               pulumi.String("Environment"),
				Value:             pulumi.String(env),
				PropagateAtLaunch: pulumi.Bool(true),
			},
		},
	})
	if err != nil {
		return nil, err
	}

	// 创建扩展策略
	scaleUpPolicy, err := autoscaling.NewPolicy(ctx, "scaleUpPolicy", &autoscaling.PolicyArgs{
		AdjustmentType:        pulumi.String("ChangeInCapacity"),
		ScalingAdjustment:     pulumi.Int(1),
		Cooldown:              pulumi.Int(60),
		AutoscalingGroupName:  asg.Name,
		MetricAggregationType: pulumi.String("Average"),
		PolicyType:            pulumi.String("SimpleScaling"),
	})
	if err != nil {
		return nil, err
	}

	// Scale Up Alarm
	var scaleUpArn pulumi.Array
	scaleUpArn = append(scaleUpArn, scaleUpPolicy.Arn)
	_, err = cloudwatch.NewMetricAlarm(ctx, "scaleUpAlarm", &cloudwatch.MetricAlarmArgs{
		AlarmDescription:   pulumi.String("Scale up if CPU > 5%"),
		ComparisonOperator: pulumi.String("GreaterThanThreshold"),
		EvaluationPeriods:  pulumi.Int(1),
		MetricName:         pulumi.String("CPUUtilization"),
		Namespace:          pulumi.String("AWS/EC2"),
		Period:             pulumi.Int(60),
		Statistic:          pulumi.String("Average"),
		Threshold:          pulumi.Float64(5),
		AlarmActions:       scaleUpArn,
		Dimensions: pulumi.StringMap{
			"AutoScalingGroupName": asg.Name,
		},
	})

	// 创建缩减策略
	scaleDownPolicy, err := autoscaling.NewPolicy(ctx, "scaleDownPolicy", &autoscaling.PolicyArgs{
		AdjustmentType:        pulumi.String("ChangeInCapacity"),
		ScalingAdjustment:     pulumi.Int(-1), // Decrement by 1
		Cooldown:              pulumi.Int(60),
		AutoscalingGroupName:  asg.Name,
		MetricAggregationType: pulumi.String("Average"),
		PolicyType:            pulumi.String("SimpleScaling"),
	})
	if err != nil {
		return nil, err
	}

	// Scale Down Alarm
	var scaleDownArn pulumi.Array
	scaleDownArn = append(scaleDownArn, scaleDownPolicy.Arn)
	_, err = cloudwatch.NewMetricAlarm(ctx, "scaleDownAlarm", &cloudwatch.MetricAlarmArgs{
		AlarmDescription:   pulumi.String("Scale down if CPU < 3%"),
		ComparisonOperator: pulumi.String("LessThanThreshold"),
		EvaluationPeriods:  pulumi.Int(1),
		MetricName:         pulumi.String("CPUUtilization"),
		Namespace:          pulumi.String("AWS/EC2"),
		Period:             pulumi.Int(60),
		Statistic:          pulumi.String("Average"),
		Threshold:          pulumi.Float64(3),
		AlarmActions:       scaleDownArn,
		Dimensions: pulumi.StringMap{
			"AutoScalingGroupName": asg.Name,
		},
	})
	if err != nil {
		return nil, err
	}

	return asg, nil
}
