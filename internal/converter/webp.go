package converter

import (
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"log/slog"
	"strings"

	"golang.org/x/image/draw"

	"github.com/SayaAndy/saya-today-thumbnail-generator/config"
	"github.com/SayaAndy/saya-today-thumbnail-generator/internal/client/input"
	"github.com/SayaAndy/saya-today-thumbnail-generator/internal/client/output"
	"github.com/kolesa-team/go-webp/encoder"
	"github.com/kolesa-team/go-webp/webp"
)

var _ Converter = (*WebpConverter)(nil)

type WebpConverter struct {
	maxWidth     int
	maxHeight    int
	quality      int
	outputClient output.OutputClient
}

func NewWebpConverter(cfg *config.ConverterConfig) (Converter, error) {
	if cfg.Type != "webp" {
		return nil, fmt.Errorf("invalid storage type for WebpConverter")
	}
	webpCfg := cfg.Config.(*config.WebpConfig)

	outputClient, err := output.NewOutputClientMap[cfg.Output.Storage.Type](&cfg.Output)
	if err != nil {
		return nil, fmt.Errorf("fail to initialize output client: %w", err)
	}

	return &WebpConverter{webpCfg.Size.MaxWidth, webpCfg.Size.MaxHeight, webpCfg.Quality, outputClient}, nil
}

func (p *WebpConverter) Process(inputMetadata *input.MetadataStruct, reader io.Reader, outputName string) error {
	var src image.Image

	writer, err := p.outputClient.GetWriter(outputName, inputMetadata)
	if err != nil {
		return fmt.Errorf("fail to initialize writer for output: %w", err)
	}
	defer writer.Close()

	switch inputMetadata.ContentType {
	case "image/jpeg":
		src, err = jpeg.Decode(reader)
		if err != nil {
			return fmt.Errorf("decode jpeg: %w", err)
		}
	case "image/png":
		src, err = png.Decode(reader)
		if err != nil {
			return fmt.Errorf("decode png: %w", err)
		}
	default:
		return fmt.Errorf("unsupported content type: %s", inputMetadata.ContentType)
	}

	opts, err := encoder.NewLossyEncoderOptions(encoder.PresetDefault, float32(p.quality))
	if err != nil {
		return fmt.Errorf("create webp encoder options: %w", err)
	}

	xCoef := float64(p.maxWidth) / float64(src.Bounds().Max.X)
	if p.maxWidth == 0 {
		xCoef = 1
	}
	yCoef := float64(p.maxHeight) / float64(src.Bounds().Max.Y)
	if p.maxHeight == 0 {
		yCoef = 1
	}
	slog.Debug("calculated coefficients", slog.Float64("x_coef", xCoef), slog.Float64("y_coef", yCoef))

	if xCoef > 1 && yCoef > 1 {
		return webp.Encode(writer, src, opts)
	}

	minCoef := xCoef
	if yCoef < minCoef {
		minCoef = yCoef
	}

	dst := image.NewRGBA(image.Rect(0, 0, int(float64(src.Bounds().Max.X)*minCoef+0.5), int(float64(src.Bounds().Max.Y)*minCoef+0.5)))
	draw.CatmullRom.Scale(dst, dst.Rect, src, src.Bounds(), draw.Over, nil)

	return webp.Encode(writer, dst, opts)
}

func (p *WebpConverter) DeductOutputPath(inputPath string) string {
	pathParts := strings.Split(inputPath, ".")
	if len(pathParts) < 2 {
		return inputPath + ".webp"
	}
	pathParts[len(pathParts)-1] = "webp"
	return strings.Join(pathParts, ".")
}

func (p *WebpConverter) ReadMetadata(path string) (*output.MetadataStruct, error) {
	return p.outputClient.ReadMetadata(path)
}

func (p *WebpConverter) IsMissing(path string) bool {
	return p.outputClient.IsMissing(path)
}
