package output

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/SayaAndy/saya-today-thumbnail-generator/config"
	"github.com/SayaAndy/saya-today-thumbnail-generator/internal/client/input"
)

var _ OutputClient = (*S3OutputClient)(nil)

type S3OutputClient struct {
	prefix     string
	bucketName string
	s3cl       *s3.Client
}

func NewS3OutputClient(cfg *config.OutputConfig) (OutputClient, error) {
	if cfg.Storage.Type != "s3" {
		return nil, fmt.Errorf("invalid storage type for S3OutputClient")
	}
	s3cfg := cfg.Storage.Config.(*config.S3Config)

	s3cl, err := newS3Client(s3cfg)
	if err != nil {
		return nil, fmt.Errorf("create S3 client: %w", err)
	}

	return &S3OutputClient{
		s3cl:       s3cl,
		bucketName: s3cfg.BucketName,
		prefix:     s3cfg.Prefix,
	}, nil
}

func (c *S3OutputClient) GetWriter(path string, inputMetadata *input.MetadataStruct, outputContentType string) (io.WriteCloser, error) {
	key := c.prefix + path

	return &s3WriteCloser{
		key:               key,
		bucketName:        c.bucketName,
		s3cl:              c.s3cl,
		contentType:       outputContentType,
		hashOriginal:      inputMetadata.Hash,
		buf:               &bytes.Buffer{},
	}, nil
}

func (c *S3OutputClient) ReadMetadata(path string) (*MetadataStruct, error) {
	key := c.prefix + path

	head, err := c.s3cl.HeadObject(context.Background(), &s3.HeadObjectInput{
		Bucket: aws.String(c.bucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("head S3 object %s: %w", key, err)
	}

	metadata := MetadataStruct{
		Name:         key,
		StorageType:  "s3",
		Hash:         strings.Trim(aws.ToString(head.ETag), "\""),
		HashOriginal: head.Metadata["sha1-original"],
		ContentType:  aws.ToString(head.ContentType),
		Misc:         head.Metadata,
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

func (c *S3OutputClient) IsMissing(path string) bool {
	key := c.prefix + path

	head, err := c.s3cl.HeadObject(context.Background(), &s3.HeadObjectInput{
		Bucket: aws.String(c.bucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		return true
	}

	headJson, _ := json.Marshal(head)
	slog.Debug("got object attrs", slog.String("path", path), slog.String("attrs", string(headJson)))

	if head.ContentLength == nil || *head.ContentLength == 0 {
		return true
	}

	return false
}

// s3WriteCloser buffers writes and uploads to S3 on Close.
type s3WriteCloser struct {
	key          string
	bucketName   string
	s3cl         *s3.Client
	contentType  string
	hashOriginal string
	buf          *bytes.Buffer
}

func (w *s3WriteCloser) Write(p []byte) (int, error) {
	return w.buf.Write(p)
}

func (w *s3WriteCloser) Close() error {
	_, err := w.s3cl.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:      aws.String(w.bucketName),
		Key:         aws.String(w.key),
		Body:        bytes.NewReader(w.buf.Bytes()),
		ContentType: aws.String(w.contentType),
		Metadata: map[string]string{
			"sha1-original": w.hashOriginal,
		},
	})
	if err != nil {
		return fmt.Errorf("put S3 object %s: %w", w.key, err)
	}
	return nil
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
