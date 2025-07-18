package config

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/go-playground/validator/v10"
)

type Config struct {
	Input             InputConfig     `json:"Input" validate:"required"`
	Converter         ConverterConfig `json:"Converter" validate:"required"`
	Output            OutputConfig    `json:"Output" validate:"required"`
	MaxConcurrentJobs int             `json:"MaxConcurrentJobs" validate:"required,min=1"`
	ForceRewrite      bool            `json:"ForceRewrite" validate:"required"`
	LogLevel          slog.Level      `json:"LogLevel" validate:"required"`
}

type InputConfig struct {
	Storage         InputStorageConfig `json:"Storage" validate:"required"`
	KnownExtensions []string           `json:"KnownExtensions" validate:"required,min=0,dive,min=1"`
}

type InputStorageConfig struct {
	Type   string `json:"Type" validate:"required,oneof=b2 local-unix"`
	Config any    `json:"Config" validate:"required"`
}

func (sc *InputStorageConfig) UnmarshalJSON(data []byte) error {
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
	case "local-unix":
		var localUnixConfig InputLocalUnixConfig
		if err := json.Unmarshal(tmp.Config, &localUnixConfig); err != nil {
			return fmt.Errorf("unmarshal LocalUnixConfig: %w", err)
		}
		sc.Config = &localUnixConfig
	default:
		return fmt.Errorf("unsupported storage type: %s", tmp.Type)
	}

	return nil
}

type OutputStorageConfig struct {
	Type   string `json:"Type" validate:"required,oneof=b2 local-unix"`
	Config any    `json:"Config" validate:"required"`
}

func (sc *OutputStorageConfig) UnmarshalJSON(data []byte) error {
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
	case "local-unix":
		var localUnixConfig OutputLocalUnixConfig
		if err := json.Unmarshal(tmp.Config, &localUnixConfig); err != nil {
			return fmt.Errorf("unmarshal LocalUnixConfig: %w", err)
		}
		sc.Config = &localUnixConfig
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

type InputLocalUnixConfig struct {
	MaxDepth int    `json:"MaxDepth" validate:"required,min=0"`
	Path     string `json:"Path" validate:"required,min=1"`
}

type OutputConfig struct {
	Storage OutputStorageConfig `json:"Storage" validate:"required"`
}

type OutputLocalUnixConfig struct {
	Path                     string `json:"Path" validate:"required,min=1"`
	DirPermissionMode        string `json:"DirPermissionMode" validate:"required,min=3"`
	FilePermissionMode       string `json:"FilePermissionMode" validate:"required,min=3"`
	AttributesImplementation string `json:"AttributesImplementation" validate:"required,oneof=xattr none"`
}

type ConverterConfig struct {
	Type   string `json:"Type" validate:"required,oneof=webp"`
	Config any    `json:"Config" validate:"required"`
}

func (pc *ConverterConfig) UnmarshalJSON(data []byte) error {
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
