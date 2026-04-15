package input

import (
	"context"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/SayaAndy/saya-today-thumbnail-generator/config"
)

var _ InputClient = (*S3InputClient)(nil)

type S3InputClient struct {
	prefix          string
	bucketName      string
	s3cl            *s3.Client
	knownExtensions []string
}

func NewS3InputClient(cfg *config.InputConfig) (InputClient, error) {
	if cfg.Storage.Type != "s3" {
		return nil, fmt.Errorf("invalid storage type for S3InputClient")
	}
	s3cfg := cfg.Storage.Config.(*config.S3Config)

	s3cl, err := newS3Client(s3cfg)
	if err != nil {
		return nil, fmt.Errorf("create S3 client: %w", err)
	}

	return &S3InputClient{
		s3cl:            s3cl,
		bucketName:      s3cfg.BucketName,
		prefix:          s3cfg.Prefix,
		knownExtensions: cfg.KnownExtensions,
	}, nil
}

func (c *S3InputClient) Scan() ([]string, error) {
	var filePaths []string

	paginator := s3.NewListObjectsV2Paginator(c.s3cl, &s3.ListObjectsV2Input{
		Bucket: aws.String(c.bucketName),
		Prefix: aws.String(c.prefix),
	})

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(context.Background())
		if err != nil {
			return nil, fmt.Errorf("list S3 objects: %w", err)
		}

		for _, obj := range output.Contents {
			name := aws.ToString(obj.Key)

			if len(c.knownExtensions) != 0 {
				nameParts := strings.Split(name, ".")
				if len(nameParts) < 2 {
					continue
				}
				ext := strings.ToLower(nameParts[len(nameParts)-1])
				if !slices.Contains(c.knownExtensions, ext) {
					continue
				}
			}

			filePaths = append(filePaths, strings.TrimPrefix(name, c.prefix))
		}
	}

	return filePaths, nil
}

func (c *S3InputClient) ReadMetadata(path string) (*MetadataStruct, error) {
	key := c.prefix + path

	head, err := c.s3cl.HeadObject(context.Background(), &s3.HeadObjectInput{
		Bucket: aws.String(c.bucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("head S3 object %s: %w", key, err)
	}

	metadata := MetadataStruct{
		Name:        key,
		StorageType: "s3",
		Hash:        strings.Trim(aws.ToString(head.ETag), "\""),
		ContentType: aws.ToString(head.ContentType),
		Misc:        head.Metadata,
	}

	if head.ContentLength != nil {
		metadata.Size = *head.ContentLength
	}
	if head.LastModified != nil {
		metadata.LastModified = *head.LastModified
		metadata.FirstCreated = *head.LastModified
	}

	return &metadata, nil
}

func (c *S3InputClient) ID(path string) string {
	return fmt.Sprintf("s3://%s/%s%s", c.bucketName, c.prefix, path)
}

func (c *S3InputClient) GetReader(path string) (io.ReadCloser, error) {
	key := c.prefix + path

	output, err := c.s3cl.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket: aws.String(c.bucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("get S3 object %s: %w", key, err)
	}

	return output.Body, nil
}

func newS3Client(cfg *config.S3Config) (*s3.Client, error) {
	opts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(cfg.Region),
	}

	if cfg.AccessKeyID != "" && cfg.SecretAccessKey != "" {
		opts = append(opts, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(), opts...)
	if err != nil {
		return nil, fmt.Errorf("load AWS config: %w", err)
	}

	var s3Opts []func(*s3.Options)
	if cfg.Endpoint != "" {
		s3Opts = append(s3Opts, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		})
	}
	s3Opts = append(s3Opts, func(o *s3.Options) {
		o.UsePathStyle = cfg.UsePathStyle
	})

	return s3.NewFromConfig(awsCfg, s3Opts...), nil
}
