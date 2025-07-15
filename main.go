package main

import (
	"flag"
	"log/slog"
	"sync"

	"github.com/SayaAndy/saya-today-thumbnail-generator/config"
	"github.com/SayaAndy/saya-today-thumbnail-generator/internal/client/input"
	"github.com/SayaAndy/saya-today-thumbnail-generator/internal/client/output"
	"github.com/SayaAndy/saya-today-thumbnail-generator/internal/processor"
)

var (
	configPath = flag.String("c", "config.json", "Path to the configuration file")
)

func main() {
	flag.Parse()

	slog.Info("starting thumbnail generator...")
	// slog.SetLogLoggerLevel(slog.LevelDebug)

	cfg := &config.Config{}
	if err := config.LoadConfig(*configPath, cfg); err != nil {
		panic(err)
	}

	inputClient, err := input.NewB2InputClient(&cfg.Input)
	if err != nil {
		panic(err)
	}

	outputClient, err := output.NewB2OutputClient(&cfg.Output)
	if err != nil {
		panic(err)
	}

	converter, err := processor.NewWebpProcessor(&cfg.Processor)
	if err != nil {
		panic(err)
	}

	generalLogger := slog.With(
		slog.String("input_storage", cfg.Input.Storage.Type),
		slog.String("processor_type", cfg.Processor.Type),
		slog.String("output_storage", cfg.Output.Storage.Type),
	)
	generalLogger.Info("initialized clients and processor")

	files, err := inputClient.Scan()
	if err != nil {
		panic(err)
	}
	generalLogger.Info("scanned files", slog.Int("file_count", len(files)))

	semaphore := make(chan struct{}, cfg.MaxConcurrentJobs)
	var wg sync.WaitGroup
	wg.Add(len(files))

	for i, file := range files {
		go func(index int, inputName string) {
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			defer wg.Done()

			outputName := converter.DeductOutputPath(inputName)
			fileLogger := generalLogger.With(slog.String("input_path", inputName), slog.String("output_path", outputName), slog.Int("file_index", index))

			inputMetadata, err := inputClient.ReadMetadata(inputName)
			if err != nil {
				fileLogger.Error("fail to read metadata of (supposedly existing) input file", slog.String("error", err.Error()))
				return
			}

			if !cfg.ForceRewrite && !outputClient.IsMissing(outputName) {
				outputMetadata, err := outputClient.ReadMetadata(outputName)
				if err != nil {
					fileLogger.Error("fail to read metadata of (supposedly existing) output file", slog.String("error", err.Error()))
					return
				}
				if inputMetadata.Hash == outputMetadata.HashOriginal {
					fileLogger.Info("skip already processed file (based on equal hash)", slog.String("input_hash", inputMetadata.Hash))
					return
				}
			}

			fileLogger.Info("start to process file")
			reader, err := inputClient.GetReader(inputName)
			if err != nil {
				fileLogger.Error("fail to get reader for input file", slog.String("error", err.Error()))
				return
			}

			writer, err := outputClient.GetWriter(outputName, inputMetadata)
			if err != nil {
				fileLogger.Error("fail to get writer for output file", slog.String("error", err.Error()))
				return
			}

			if err := converter.Process(inputMetadata.ContentType, reader, writer); err != nil {
				fileLogger.Error("fail to convert file", slog.String("error", err.Error()))
				return
			}

			fileLogger.Info("successfully processed file", slog.String("input_hash", inputMetadata.Hash))
		}(i, file)
	}

	wg.Wait()
	generalLogger.Info("all files processed successfully, exiting")
}
