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
	b2cl            *b2.Client
	knownExtensions []string
}

func NewB2InputClient(cfg *config.InputConfig) (*B2InputClient, error) {
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

	return &B2InputClient{b2cl: b2cl, bucket: bucket, prefix: b2cfg.Prefix, knownExtensions: cfg.KnownExtensions}, nil
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

		filePaths = append(filePaths, name)
	}

	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("iterate over B2 objects: %w", err)
	}

	return filePaths, nil
}

func (c *B2InputClient) ReadMetadata(path string) (map[string]string, error) {
	obj := c.bucket.Object(path)
	if obj == nil {
		return nil, fmt.Errorf("object not found in B2 bucket")
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

func (c *B2InputClient) GetReader(path string) (io.Reader, error) {
	obj := c.bucket.Object(path)
	if obj == nil {
		return nil, fmt.Errorf("failed to reference object in B2 bucket")
	}

	reader := obj.NewReader(context.Background())

	return reader, nil
}
