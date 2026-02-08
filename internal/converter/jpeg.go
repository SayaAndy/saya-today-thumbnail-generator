package converter

import (
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/SayaAndy/saya-today-thumbnail-generator/config"
	"github.com/SayaAndy/saya-today-thumbnail-generator/internal/client/input"
	"github.com/SayaAndy/saya-today-thumbnail-generator/internal/client/output"
	"github.com/kolesa-team/go-webp/webp"
	"golang.org/x/image/draw"
)

var _ Converter = (*JpegConverter)(nil)

type JpegConverter struct {
	maxWidth      int
	maxHeight     int
	extensionName string
	quality       int
	outputClient  output.OutputClient
}

func NewJpegConverter(cfg *config.ConverterConfig) (Converter, error) {
	if cfg.Type != "jpeg" {
		return nil, fmt.Errorf("invalid storage type for JpegConverter")
	}
	jpegCfg := cfg.Config.(*config.JpegConfig)

	outputClient, err := output.NewOutputClientMap[cfg.Output.Storage.Type](&cfg.Output)
	if err != nil {
		return nil, fmt.Errorf("fail to initialize output client: %w", err)
	}

	extensionName := ".jpg"
	if jpegCfg.ExtensionName != "" {
		extensionName = "." + jpegCfg.ExtensionName
	}

	return &JpegConverter{jpegCfg.Size.MaxWidth, jpegCfg.Size.MaxHeight, extensionName, jpegCfg.Quality, outputClient}, nil
}

func (p *JpegConverter) Process(inputMetadata *input.MetadataStruct, reader io.Reader, outputName string) error {
	var src image.Image

	writer, err := p.outputClient.GetWriter(outputName, inputMetadata, "image/jpeg")
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
	case "image/webp":
		src, err = webp.Decode(reader, nil)
		if err != nil {
			return fmt.Errorf("decode webp: %w", err)
		}
	default:
		return fmt.Errorf("unsupported content type: %s", inputMetadata.ContentType)
	}

	xCoef := 1.0
	if p.maxWidth > 0 {
		xCoef = float64(p.maxWidth) / float64(src.Bounds().Max.X)
	}
	yCoef := 1.0
	if p.maxHeight > 0 {
		yCoef = float64(p.maxHeight) / float64(src.Bounds().Max.Y)
	}
	slog.Debug("calculated coefficients", slog.Float64("x_coef", xCoef), slog.Float64("y_coef", yCoef))

	minCoef := xCoef
	if yCoef < minCoef {
		minCoef = yCoef
	}

	if minCoef >= 1.0 {
		return jpeg.Encode(writer, src, &jpeg.Options{Quality: p.quality})
	}

	dst := image.NewRGBA(image.Rect(0, 0, int(float64(src.Bounds().Max.X)*minCoef+0.5), int(float64(src.Bounds().Max.Y)*minCoef+0.5)))
	draw.CatmullRom.Scale(dst, dst.Rect, src, src.Bounds(), draw.Over, nil)

	return jpeg.Encode(writer, dst, &jpeg.Options{Quality: p.quality})
}

func (p *JpegConverter) DeductOutputPath(inputPath string) string {
	withoutExt, _ := strings.CutSuffix(inputPath, filepath.Ext(inputPath))
	return withoutExt + p.extensionName
}

func (p *JpegConverter) ReadMetadata(path string) (*output.MetadataStruct, error) {
	return p.outputClient.ReadMetadata(path)
}

func (p *JpegConverter) IsMissing(path string) bool {
	return p.outputClient.IsMissing(path)
}
