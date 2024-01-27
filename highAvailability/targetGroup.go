package highAvailability

import (
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/alb"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func CreateTargetGroup(ctx *pulumi.Context, vpcId pulumi.IDOutput) (*alb.TargetGroup, error) {
	// 创建目标组
	targetGroup, err := alb.NewTargetGroup(ctx, "appTargetGroup", &alb.TargetGroupArgs{
		Port:     pulumi.Int(8080), // app跑的端口
		Protocol: pulumi.String("HTTP"),
		VpcId:    vpcId, // 指定 VPC ID
		HealthCheck: &alb.TargetGroupHealthCheckArgs{
			Protocol: pulumi.String("HTTP"),
			Path:     pulumi.String("/healthz"), // 健康检查路径为 /healthz
			Port:     pulumi.String("8080"),     // 端口为8080
		},
		TargetType: pulumi.String("instance"),
	})
	if err != nil {
		return nil, err
	}
	return targetGroup, nil
}
