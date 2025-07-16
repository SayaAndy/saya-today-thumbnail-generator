package output

import (
	"fmt"
	"io"
	"mime"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/SayaAndy/saya-today-thumbnail-generator/config"
	"github.com/SayaAndy/saya-today-thumbnail-generator/internal/client/input"
	"golang.org/x/sys/unix"
)

var _ OutputClient = (*LocalUnixOutputClient)(nil)

type LocalUnixOutputClient struct {
	path     string
	fileMode uint32
	dirMode  uint32
	attrMode string
}

func NewLocalUnixOutputClient(cfg *config.OutputConfig) (OutputClient, error) {
	if cfg.Storage.Type != "local-unix" {
		return nil, fmt.Errorf("invalid storage type for LocalUnixOutputClient")
	}
	localCfg := cfg.Storage.Config.(*config.OutputLocalUnixConfig)

	fpm, err := strconv.ParseInt(localCfg.FilePermissionMode, 8, 32)
	if err != nil {
		return nil, fmt.Errorf("fail to parse file permission mode as an octal number: %w", err)
	}

	dpm, err := strconv.ParseInt(localCfg.DirPermissionMode, 8, 32)
	if err != nil {
		return nil, fmt.Errorf("fail to parse directory permission mode as an octal number: %w", err)
	}

	return &LocalUnixOutputClient{localCfg.Path, uint32(fpm), uint32(dpm), localCfg.AttributesImplementation}, nil
}

func (c *LocalUnixOutputClient) GetWriter(path string, inputMetadata *input.MetadataStruct) (io.WriteCloser, error) {
	pathSegments := strings.Split(path, "/")
	dirpath := strings.Join(pathSegments[0:len(pathSegments)-1], "/")
	if err := os.MkdirAll(c.path+dirpath, os.FileMode(c.dirMode)); err != nil {
		return nil, fmt.Errorf("fail to mkdir parent directories for a path: %w", err)
	}

	var err error
	if _, err = os.Stat(c.path + path); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("fail to stat a file, although file exists: %w", err)
	}
	if os.IsNotExist(err) {
		f, err := os.OpenFile(c.path+path, os.O_WRONLY|os.O_CREATE, os.FileMode(c.fileMode))
		if err != nil {
			return nil, fmt.Errorf("fail to create a file: %w", err)
		}
		f.Close()
	}

	switch c.attrMode {
	case "xattr":
		if err := unix.Setxattr(c.path+path, "user.originalfile.mddate", []byte(strconv.FormatInt(inputMetadata.LastModified.Unix(), 16)), 0); err != nil {
			return nil, fmt.Errorf("fail to write user.originalfile.mddate xattribute: %w", err)
		}
	case "none":
	default:
		return nil, fmt.Errorf("unknown attributes implementation: %s", c.attrMode)
	}

	return os.OpenFile(c.path+path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.FileMode(c.fileMode))
}

func (c *LocalUnixOutputClient) ReadMetadata(path string) (*MetadataStruct, error) {
	nodePathParts := strings.Split(path, "/")
	nodeName := nodePathParts[len(nodePathParts)-1]
	nodeNameParts := strings.Split(nodeName, ".")
	nodeExt := ""
	if len(nodeNameParts) >= 2 {
		nodeExt = nodeNameParts[len(nodeNameParts)-1]
	}

	fileInfo, err := os.Stat(c.path + path)
	if err != nil {
		return nil, fmt.Errorf("fail to read file info: %w", err)
	}

	stat_t := fileInfo.Sys().(*syscall.Stat_t)
	creationTime := time.Unix(stat_t.Ctim.Sec, stat_t.Ctim.Nsec)

	misc := map[string]string{
		"Size": strconv.FormatInt(fileInfo.Size(), 10),
	}

	mddateOriginal := make([]byte, 0)
	switch c.attrMode {
	case "xattr":
		sz, err := unix.Getxattr(c.path+path, "user.originalfile.mddate", nil)
		if err != nil {
			return nil, fmt.Errorf("fail to get size of user.originalfile.mddate attribute: %w", err)
		}
		mddateOriginal = make([]byte, sz)
		if _, err = unix.Getxattr(c.path+path, "user.originalfile.mddate", mddateOriginal); err != nil {
			return nil, fmt.Errorf("fail to get user.originalfile.mddate attribute: %w", err)
		}
	case "none":
	default:
		return nil, fmt.Errorf("unknown attributes implementation: %s", c.attrMode)
	}

	return &MetadataStruct{
		Name:         fileInfo.Name(),
		StorageType:  "local-unix",
		Hash:         strconv.FormatInt(fileInfo.ModTime().Unix(), 16),
		HashOriginal: string(mddateOriginal),
		ContentType:  mime.TypeByExtension("." + nodeExt),
		FirstCreated: creationTime,
		LastModified: fileInfo.ModTime(),
		Misc:         misc,
	}, nil
}

func (c *LocalUnixOutputClient) IsMissing(path string) bool {
	_, err := os.Stat(c.path + path)
	return err != nil
}
