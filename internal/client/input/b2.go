package input

import (
	"context"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/Backblaze/blazer/b2"
	"github.com/SayaAndy/saya-today-thumbnail-generator/config"
)

var _ InputClient = (*B2InputClient)(nil)

type B2InputClient struct {
	prefix          string
	bucket          *b2.Bucket
	bucketName      string
	b2cl            *b2.Client
	knownExtensions []string
}

func NewB2InputClient(cfg *config.InputConfig) (InputClient, error) {
	if cfg.Storage.Type != "b2" {
		return nil, fmt.Errorf("invalid storage type for B2InputClient")
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

	return &B2InputClient{b2cl: b2cl, bucket: bucket, bucketName: b2cfg.BucketName, prefix: b2cfg.Prefix, knownExtensions: cfg.KnownExtensions}, nil
}

func (c *B2InputClient) Scan() ([]string, error) {
	filePaths := []string{}

	iter := c.bucket.List(context.Background(), b2.ListPrefix(c.prefix))

	for iter.Next() {
		obj := iter.Object()
		if obj == nil {
			return nil, fmt.Errorf("failed to reference object in B2 bucket")
		}

		attrs, err := obj.Attrs(context.Background())
		if err != nil {
			return nil, fmt.Errorf("get attributes for object: %w", err)
		}

		if attrs.Status != b2.Uploaded {
			continue
		}

		name := obj.Name()

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

	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("iterate over B2 objects: %w", err)
	}

	return filePaths, nil
}

func (c *B2InputClient) ReadMetadata(path string) (*MetadataStruct, error) {
	obj := c.bucket.Object(c.prefix + path)
	if obj == nil {
		return nil, fmt.Errorf("object not found in B2 bucket")
	}

	attrs, err := obj.Attrs(context.Background())
	if err != nil {
		return nil, fmt.Errorf("get attributes for object: %w", err)
	}

	metadata := MetadataStruct{
		Name:         attrs.Name,
		StorageType:  "b2",
		Hash:         attrs.SHA1,
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

func (c *B2InputClient) ID(path string) string {
	return fmt.Sprintf("b2://%s/%s%s", c.bucketName, c.prefix, path)
}

func (c *B2InputClient) GetReader(path string) (io.ReadCloser, error) {
	obj := c.bucket.Object(c.prefix + path)
	if obj == nil {
		return nil, fmt.Errorf("failed to reference object in B2 bucket")
	}

	return obj.NewReader(context.Background()), nil
}
