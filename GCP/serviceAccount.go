package GCP

import (
	"github.com/pulumi/pulumi-gcp/sdk/v5/go/gcp/serviceaccount"
	"github.com/pulumi/pulumi-gcp/sdk/v5/go/gcp/storage"
	_ "github.com/pulumi/pulumi-gcp/sdk/v5/go/gcp/storage"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func CreateGCPServiceAccount(ctx *pulumi.Context) (*serviceaccount.Account, *serviceaccount.Key, *storage.Bucket, error) {
	// 创建service account
	account, err := serviceaccount.NewAccount(ctx, "myAccount", &serviceaccount.AccountArgs{
		AccountId:   pulumi.String("my-account-id"),
		DisplayName: pulumi.String("My Service Account"),
	})
	if err != nil {
		return nil, nil, nil, err
	}

	// Create a new GCP Storage Bucket.
	bucket, err := storage.NewBucket(ctx, "csye6225-bucket", nil)
	if err != nil {
		return nil, nil, nil, err
	}

	// Assign the service account the role of storage.objectAdmin which gives it full access to objects in the bucket
	_, err = storage.NewBucketIAMMember(ctx, "bucketObjectAdmin", &storage.BucketIAMMemberArgs{
		Bucket: bucket.Name,
		Role:   pulumi.String("roles/storage.objectAdmin"),
		Member: pulumi.Sprintf("serviceAccount:%s", account.Email),
	})
	if err != nil {
		return nil, nil, nil, err
	}

	// 创建一个新的 key
	key, err := serviceaccount.NewKey(ctx, "csye6225-key", &serviceaccount.KeyArgs{
		ServiceAccountId: account.Name,
	})
	if err != nil {
		return nil, nil, nil, err
	}

	return account, key, bucket, nil

}
