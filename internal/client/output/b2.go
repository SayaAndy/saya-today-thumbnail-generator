package output

import (
	"context"
	"fmt"
	"io"

	"github.com/Backblaze/blazer/b2"
	"github.com/SayaAndy/saya-today-thumbnail-generator/config"
)

var _ OutputClient = (*B2OutputClient)(nil)

type B2OutputClient struct {
	prefix string
	bucket *b2.Bucket
	b2cl   *b2.Client
}

func NewB2OutputClient(cfg *config.OutputConfig) (*B2OutputClient, error) {
	if cfg.Storage.Type != "b2" {
		return nil, fmt.Errorf("invalid storage type for B2OutputClient")
	}
	b2cfg := cfg.Storage.Config.(*config.B2Config)

	b2cl, err := b2.NewClient(context.Background(), b2cfg.KeyID, b2cfg.ApplicationKey)
	if err != nil {
		return nil, err
	}

	bucket, err := b2cl.Bucket(context.Background(), b2cfg.BucketName)
	if err != nil {
		return nil, err
	}

	return &B2OutputClient{b2cl: b2cl, bucket: bucket, prefix: b2cfg.Prefix}, nil
}

func (c *B2OutputClient) GetWriter(path string) (io.Writer, error) {
	obj := c.bucket.Object(c.prefix + path)
	if obj == nil {
		return nil, fmt.Errorf("failed to reference object in B2 bucket")
	}

	return obj.NewWriter(context.Background()), nil
}

func (c *B2OutputClient) ReadMetadata(path string) (map[string]string, error) {
	obj := c.bucket.Object(c.prefix + path)
	if obj == nil {
		return nil, fmt.Errorf("failed to reference object in B2 bucket")
	}

	attrs, err := obj.Attrs(context.Background())
	if err != nil {
		return nil, fmt.Errorf("get attributes for object: %w", err)
	}

	metadata := attrs.Info
	metadata["Name"] = attrs.Name
	metadata["Size"] = fmt.Sprintf("%d", attrs.Size)
	metadata["ContentType"] = attrs.ContentType
	metadata["LastModified"] = attrs.LastModified.Format("2006-01-02T15:04:05Z")
	metadata["SHA1"] = attrs.SHA1
	metadata["UploadTimestamp"] = attrs.UploadTimestamp.Format("2006-01-02T15:04:05Z")

	switch attrs.Status {
	case b2.Uploaded:
		metadata["Status"] = "Uploaded"
	case b2.Folder:
		metadata["Status"] = "Folder"
	case b2.Hider:
		metadata["Status"] = "Hider"
	case b2.Started:
		metadata["Status"] = "Started"
	default:
		metadata["Status"] = "Unknown"
	}

	return metadata, nil
}

func (c *B2OutputClient) IsMissing(path string) (bool, error) {
	obj := c.bucket.Object(c.prefix + path)
	if obj == nil {
		return false, fmt.Errorf("failed to reference object in B2 bucket")
	}

	attrs, err := obj.Attrs(context.Background())
	if err != nil {
		return true, fmt.Errorf("get attributes for object: %w", err)
	}

	return attrs.Status == b2.Hider, nil
}
