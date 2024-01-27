package AWS_SNS

import (
	"github.com/pulumi/pulumi-aws/sdk/v4/go/aws/dynamodb"
	"github.com/pulumi/pulumi-aws/sdk/v4/go/aws/iam"
	"github.com/pulumi/pulumi-aws/sdk/v4/go/aws/lambda"
	"github.com/pulumi/pulumi-aws/sdk/v4/go/aws/sns"
	"github.com/pulumi/pulumi-gcp/sdk/v5/go/gcp/serviceaccount"
	"github.com/pulumi/pulumi-gcp/sdk/v5/go/gcp/storage"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func CreateSNSTopic(ctx *pulumi.Context) (*sns.Topic, error) {
	topic, err := sns.NewTopic(ctx, "Assignments-Submission-Notification-Topic", nil)
	if err != nil {
		return nil, err
	}
	return topic, nil
}

func CreateLambda(ctx *pulumi.Context, topic *sns.Topic, key *serviceaccount.Key, bucket *storage.Bucket, table *dynamodb.Table) error {
	// 创建IAM角色
	lambdaRole, err := iam.NewRole(ctx, "lambdaRole", &iam.RoleArgs{
		AssumeRolePolicy: pulumi.String(`{
                "Version": "2012-10-17",
                "Statement": [{
                    "Action": "sts:AssumeRole",
                    "Effect": "Allow",
                    "Principal": {
                        "Service": "lambda.amazonaws.com"
                    }
                }]
            }`),
	})
	if err != nil {
		return err
	}

	// 在Pulumi运行环境中
	_, err = iam.NewRolePolicyAttachment(ctx, "lambdaDynamoDBPolicyAttachment", &iam.RolePolicyAttachmentArgs{
		Role:      lambdaRole.Name,
		PolicyArn: pulumi.String("arn:aws:iam::aws:policy/AmazonDynamoDBFullAccess"),
	})
	if err != nil {
		return err
	}

	_, err = iam.NewRolePolicyAttachment(ctx, "lambdaLoggingPolicyAttachment", &iam.RolePolicyAttachmentArgs{
		Role:      lambdaRole.Name,
		PolicyArn: pulumi.String("arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"),
	})
	if err != nil {
		return err
	}

	// 读取本地 ZIP 文件路径
	zipPath, _ := ctx.GetConfig("serverless:zip")

	// 读取mailgun api 和 domain
	apikey, _ := ctx.GetConfig("mg:apikey")
	domain, _ := ctx.GetConfig("dns:domainName")
	// 创建 Lambda 函数
	lambdaFunc, err := lambda.NewFunction(ctx, "assignmentsSubmissionLambdaFunction", &lambda.FunctionArgs{
		Handler: pulumi.String("main"),
		Role:    lambdaRole.Arn,
		Runtime: pulumi.String("go1.x"),
		Code:    pulumi.NewFileArchive(zipPath),
		Environment: &lambda.FunctionEnvironmentArgs{
			Variables: pulumi.StringMap{
				"GOOGLE_CREDENTIALS": key.PrivateKey,
				"BUCKET_NAME":        bucket.Name,
				"MAILGUN_API":        pulumi.String(apikey),
				"EMAIL_DOMAIN":       pulumi.String(domain),
				"DYNAMO_TABLE":       table.Name,
			},
		},
	})
	if err != nil {
		return err
	}

	// 将 Lambda 函数订阅到 SNS 主题
	_, err = sns.NewTopicSubscription(ctx, "myTopicSubscription", &sns.TopicSubscriptionArgs{
		Topic:    topic.Arn,
		Protocol: pulumi.String("lambda"),
		Endpoint: lambdaFunc.Arn,
	})
	if err != nil {
		return err
	}

	// 读取 aws account id
	accountId, _ := ctx.GetConfig("awsAccount:Id")

	// 为 Lambda 函数添加权限，允许 SNS 主题触发该函数
	_, err = lambda.NewPermission(ctx, "myLambdaPermission", &lambda.PermissionArgs{
		Action:        pulumi.String("lambda:InvokeFunction"),
		Function:      lambdaFunc.Arn,
		Principal:     pulumi.String("sns.amazonaws.com"),
		SourceArn:     topic.Arn,
		SourceAccount: pulumi.String(accountId),
	})
	if err != nil {
		return err
	}

	return nil
}
