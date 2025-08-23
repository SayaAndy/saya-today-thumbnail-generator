package input

import (
	"fmt"
	"io"
	"log/slog"
	"mime"
	"os"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/SayaAndy/saya-today-thumbnail-generator/config"
)

var _ InputClient = (*LocalUnixInputClient)(nil)

type LocalUnixInputClient struct {
	path            string
	maxDepth        int
	knownExtensions []string
}

func NewLocalUnixInputClient(cfg *config.InputConfig) (InputClient, error) {
	if cfg.Storage.Type != "local-unix" {
		return nil, fmt.Errorf("invalid storage type for LocalUnixInputClient")
	}
	localCfg := cfg.Storage.Config.(*config.InputLocalUnixConfig)

	return &LocalUnixInputClient{
		path:            localCfg.Path,
		maxDepth:        localCfg.MaxDepth,
		knownExtensions: cfg.KnownExtensions,
	}, nil
}

func (c *LocalUnixInputClient) Scan() ([]string, error) {
	return c.recursiveScan(c.path, c.maxDepth)
}

func (c *LocalUnixInputClient) recursiveScan(dir string, depth int) ([]string, error) {
	filePaths := make([]string, 0)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("fail to read directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() && depth > 0 {
			subFilePaths, err := c.recursiveScan(dir+entry.Name()+"/", depth-1)
			if err != nil {
				return nil, fmt.Errorf("fail to scan subdirectory '%s': %w", entry.Name(), err)
			}
			filePaths = append(filePaths, subFilePaths...)
		} else if !entry.IsDir() {
			nameParts := strings.Split(entry.Name(), ".")
			if len(nameParts) < 2 {
				continue
			}
			if slices.Contains(c.knownExtensions, strings.ToLower(nameParts[len(nameParts)-1])) {
				filePaths = append(filePaths, strings.TrimPrefix(dir+entry.Name(), c.path))
			}
			fmt.Println(entry.Name())
		}
	}

	return filePaths, nil
}

func (c *LocalUnixInputClient) ReadMetadata(path string) (*MetadataStruct, error) {
	nodePathParts := strings.Split(path, "/")
	nodeName := nodePathParts[len(nodePathParts)-1]
	nodeNameParts := strings.Split(nodeName, ".")
	nodeExt := ""
	if len(nodeNameParts) >= 2 {
		nodeExt = nodeNameParts[len(nodeNameParts)-1]
	}
	slog.Debug("got a file extension", slog.String("extension", nodeExt), slog.String("path", path), slog.String("filename", nodeName))

	fileInfo, err := os.Stat(c.path + path)
	if err != nil {
		return nil, fmt.Errorf("fail to read file info: %w", err)
	}

	stat_t := fileInfo.Sys().(*syscall.Stat_t)
	creationTime := time.Unix(stat_t.Ctimespec.Sec, stat_t.Ctimespec.Nsec)

	return &MetadataStruct{
		Name:         fileInfo.Name(),
		StorageType:  "local-unix",
		Hash:         strconv.FormatInt(fileInfo.ModTime().Unix(), 16),
		ContentType:  mime.TypeByExtension("." + nodeExt),
		FirstCreated: creationTime,
		LastModified: fileInfo.ModTime(),
		Size:         fileInfo.Size(),
		Misc:         map[string]string{},
	}, nil
}

func (c *LocalUnixInputClient) GetReader(path string) (io.ReadCloser, error) {
	return os.Open(c.path + path)
}
