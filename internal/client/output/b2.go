package output

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"

	"github.com/Backblaze/blazer/b2"
	"github.com/SayaAndy/saya-today-thumbnail-generator/config"
	"github.com/SayaAndy/saya-today-thumbnail-generator/internal/client/input"
)

var _ OutputClient = (*B2OutputClient)(nil)

type B2OutputClient struct {
	prefix string
	bucket *b2.Bucket
	b2cl   *b2.Client
}

func NewB2OutputClient(cfg *config.OutputConfig) (OutputClient, error) {
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

func (c *B2OutputClient) GetWriter(path string, inputMetadata *input.MetadataStruct, outputContentType string) (io.WriteCloser, error) {
	obj := c.bucket.Object(c.prefix + path)
	if obj == nil {
		return nil, fmt.Errorf("failed to reference object in B2 bucket")
	}

	attrs := &b2.Attrs{Info: make(map[string]string)}
	attrs.Info["sha1-original"] = inputMetadata.Hash
	attrs.ContentType = outputContentType

	return obj.NewWriter(context.Background(), b2.WithAttrsOption(attrs)), nil
}

func (c *B2OutputClient) ReadMetadata(path string) (*MetadataStruct, error) {
	obj := c.bucket.Object(c.prefix + path)
	if obj == nil {
		return nil, fmt.Errorf("failed to reference object in B2 bucket")
	}

	attrs, err := obj.Attrs(context.Background())
	if err != nil {
		return nil, fmt.Errorf("get attributes for object: %w", err)
	}

	metadata := MetadataStruct{
		Name:         attrs.Name,
		StorageType:  "b2",
		Hash:         attrs.SHA1,
		HashOriginal: attrs.Info["sha1-original"],
		ContentType:  attrs.ContentType,
		FirstCreated: attrs.UploadTimestamp,
		LastModified: attrs.LastModified,
		Misc:         attrs.Info,
		Size:         attrs.Size,
	}

	switch attrs.Status {
	case b2.Uploaded:
		metadata.Misc["b2-status"] = "Uploaded"
	case b2.Folder:
		metadata.Misc["b2-status"] = "Folder"
	case b2.Hider:
		metadata.Misc["b2-status"] = "Hider"
	case b2.Started:
		metadata.Misc["b2-status"] = "Started"
	default:
		metadata.Misc["b2-status"] = "Unknown"
	}

	return &metadata, nil
}

func (c *B2OutputClient) IsMissing(path string) bool {
	obj := c.bucket.Object(c.prefix + path)
	if obj == nil {
		return true
	}

	attrs, err := obj.Attrs(context.Background())
	if err != nil {
		return true
	}

	attrsJson, _ := json.Marshal(attrs)
	slog.Debug("got object attrs", slog.String("path", path), slog.String("attrs", string(attrsJson)))

	if attrs.Size == 0 {
		return true
	}

	return attrs.Status == b2.Hider
}
