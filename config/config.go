package config

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/go-playground/validator/v10"
)

type Config struct {
	Input             InputConfig     `json:"Input" validate:"required"`
	Processor         ProcessorConfig `json:"Processor" validate:"required"`
	Output            OutputConfig    `json:"Output" validate:"required"`
	MaxConcurrentJobs int             `json:"MaxConcurrentJobs" validate:"required,min=1"`
	ForceRewrite      bool            `json:"ForceRewrite" validate:"required"`
}

type InputConfig struct {
	Storage         StorageConfig `json:"Storage" validate:"required"`
	KnownExtensions []string      `json:"KnownExtensions" validate:"required,min=0,dive,min=1"`
}

type StorageConfig struct {
	Type   string `json:"Type" validate:"required,oneof=b2 local"`
	Config any    `json:"Config" validate:"required"`
}

func (sc *StorageConfig) UnmarshalJSON(data []byte) error {
	var tmp struct {
		Type   string          `json:"Type"`
		Config json.RawMessage `json:"Config"`
	}

	if err := json.Unmarshal(data, &tmp); err != nil {
		return err
	}

	sc.Type = tmp.Type

	switch tmp.Type {
	case "b2":
		var b2Config B2Config
		if err := json.Unmarshal(tmp.Config, &b2Config); err != nil {
			return fmt.Errorf("unmarshal B2Config: %w", err)
		}
		sc.Config = &b2Config
	case "local":
		var localConfig LocalConfig
		if err := json.Unmarshal(tmp.Config, &localConfig); err != nil {
			return fmt.Errorf("unmarshal LocalConfig: %w", err)
		}
		sc.Config = &localConfig
	default:
		return fmt.Errorf("unsupported storage type: %s", tmp.Type)
	}

	return nil
}

type B2Config struct {
	BucketName     string `json:"BucketName" validate:"required,min=1"`
	Region         string `json:"Region" validate:"required,min=1"`
	Prefix         string `json:"Prefix"`
	KeyID          string `json:"KeyID"`
	ApplicationKey string `json:"ApplicationKey"`
}

type LocalConfig struct {
	Path string `json:"Path" validate:"required,min=1"`
}

type ProcessorConfig struct {
	Type   string `json:"Type" validate:"required,oneof=webp"`
	Config any    `json:"Config" validate:"required"`
}

func (pc *ProcessorConfig) UnmarshalJSON(data []byte) error {
	var tmp struct {
		Type   string          `json:"Type"`
		Config json.RawMessage `json:"Config"`
	}

	if err := json.Unmarshal(data, &tmp); err != nil {
		return err
	}

	pc.Type = tmp.Type

	switch tmp.Type {
	case "webp":
		var webpConfig WebpConfig
		if err := json.Unmarshal(tmp.Config, &webpConfig); err != nil {
			return fmt.Errorf("unmarshal WebpConfig: %w", err)
		}
		pc.Config = &webpConfig
	default:
		return fmt.Errorf("unsupported storage type: %s", tmp.Type)
	}

	return nil
}

type WebpConfig struct {
	Quality int        `json:"Quality" validate:"required,min=1,max=100"`
	Size    SizeConfig `json:"Size" validate:"required"`
}

type SizeConfig struct {
	MaxWidth  int `json:"MaxWidth" validate:"required,min=0"`
	MaxHeight int `json:"MaxHeight" validate:"required,min=0"`
}

type OutputConfig struct {
	Storage StorageConfig `json:"Storage" validate:"required"`
}

func LoadConfig(path string, config *Config) error {
	fileBytes, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	expandedFileBytes := []byte(os.ExpandEnv(string(fileBytes)))

	if err = json.Unmarshal(expandedFileBytes, config); err != nil {
		return err
	}

	return nil
}

func InitConfig(path string) (*Config, error) {
	config := &Config{}
	if err := LoadConfig(path, config); err != nil {
		return nil, err
	}

	validate := validator.New(validator.WithRequiredStructEnabled())
	if err := validate.Struct(config); err != nil {
		return nil, err
	}

	return config, nil
}
