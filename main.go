package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"flag"
	"hash/crc32"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/SayaAndy/saya-today-thumbnail-generator/config"
	"github.com/SayaAndy/saya-today-thumbnail-generator/internal/client/input"
	"github.com/SayaAndy/saya-today-thumbnail-generator/internal/converter"
)

var (
	configPath    = flag.String("c", "config.json", "Path to the configuration file")
	sigTermChan   = make(chan os.Signal, 1)
	cacheMapMutex = &sync.RWMutex{}
	cacheMap      = make(map[string]map[uint32]struct{})
)

func main() {
	signal.Notify(sigTermChan, os.Interrupt, syscall.SIGTERM)

	flag.Parse()

	cfg := &config.Config{}
	if err := config.LoadConfig(*configPath, cfg); err != nil {
		slog.Error("fail to load configuration", slog.String("error", err.Error()))
		os.Exit(1)
	}

	slog.SetLogLoggerLevel(cfg.LogLevel)
	slog.Info("starting thumbnail generator...")

	select {
	case <-sigTermChan:
		slog.Info("exiting due to termination signal")
		os.Exit(130)
	default:
	}

	inputClient, err := input.NewInputClientMap[cfg.Input.Storage.Type](&cfg.Input)
	if err != nil {
		slog.Error("fail to initialize input client", slog.String("error", err.Error()))
		os.Exit(1)
	}

	converters := make([]converter.Converter, 0, len(cfg.Converters))
	converterTypes := make([]string, 0, len(cfg.Converters))
	converterHashes := make([]uint32, 0, len(cfg.Converters))
	for _, converterCfg := range cfg.Converters {
		converterBytes, _ := json.Marshal(converterCfg)
		conv, err := converter.NewConverterMap[converterCfg.Type](&converterCfg)
		if err != nil {
			slog.Error("fail to initialize converter", slog.String("error", err.Error()))
			os.Exit(1)
		}
		converters = append(converters, conv)
		converterTypes = append(converterTypes, converterCfg.Type)
		converterHashes = append(converterHashes, crc32.ChecksumIEEE(converterBytes))
	}

	generalLogger := slog.With(slog.String("input_storage", cfg.Input.Storage.Type))
	generalLogger.Info("initialized input client and converters", slog.String("converter_types", strings.Join(converterTypes, " ")))

	select {
	case <-sigTermChan:
		generalLogger.Info("exiting due to termination signal")
		os.Exit(130)
	default:
	}

	files, err := inputClient.Scan()
	if err != nil {
		generalLogger.Error("fail to scan input files", slog.String("error", err.Error()))
		os.Exit(1)
	}
	fileCount := len(files)
	generalLogger.Info("scanned files", slog.Int("file_count", fileCount))

	select {
	case <-sigTermChan:
		generalLogger.Info("exiting due to termination signal")
		os.Exit(130)
	default:
	}

	if cfg.Input.CacheProcessed {
		cacheFile, err := os.OpenFile(cfg.Input.CacheProcessedCsvPath, os.O_CREATE|os.O_RDONLY, 0644)
		if err != nil {
			generalLogger.Error("fail to initialize cache file", slog.String("cache_path", cfg.Input.CacheProcessedCsvPath), slog.String("error", err.Error()))
			os.Exit(1)
		}
		defer cacheFile.Close()

		csvReader := csv.NewReader(cacheFile)
		for {
			rec, err := csvReader.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				generalLogger.Error("fail to initialize cache while reading file", slog.String("cache_path", cfg.Input.CacheProcessedCsvPath), slog.String("error", err.Error()))
				os.Exit(1)
			}
			if len(rec) != 2 {
				generalLogger.Error("fail to initialize cache while reading file", slog.Int("record_length", len(rec)), slog.String("error", "incorrect format: expected '{image-name},{semicolon-separated-processor-hashes}'"))
				os.Exit(1)
			}
			hashes := strings.Split(rec[1], ";")
			cacheMap[rec[0]] = make(map[uint32]struct{})
			for _, hash := range hashes {
				hashUint, err := strconv.ParseUint(hash, 10, 32)
				if err != nil {
					generalLogger.Error("fail to initialize cache while reading file", slog.Int("record_length", len(rec)), slog.String("error", err.Error()))
				}
				cacheMap[rec[0]][uint32(hashUint)] = struct{}{}
			}
		}
	}

	processSemaphore := make(chan struct{}, cfg.MaxProcessThreads)
	queueSemaphore := make(chan struct{}, cfg.MaxPreProcessThreads)
	var wg sync.WaitGroup
	wg.Add(fileCount)

	for i, file := range files {
		queueSemaphore <- struct{}{}
		go func(index int, inputName string) {
			fileLogger := generalLogger.With(slog.String("input_path", inputName), slog.Int("file_index", index))

			threadSigTermChannel := make(chan os.Signal, 1)
			signal.Notify(threadSigTermChannel, os.Interrupt, syscall.SIGTERM)

			defer func() { <-queueSemaphore; wg.Done() }()

			select {
			case <-threadSigTermChannel:
				fileLogger.Info("exiting due to termination signal")
				return
			default:
			}

			var inputMetadata *input.MetadataStruct

			id := inputClient.ID(file)
			if _, ok := cacheMap[id]; !ok {
				cacheMapMutex.Lock()
				cacheMap[id] = make(map[uint32]struct{})
				cacheMapMutex.Unlock()
			}

			convertersToLaunch := []int{}
			for j, conv := range converters {
				if cfg.Input.CacheProcessed {
					cacheMapMutex.RLock()
					if _, ok := cacheMap[id][converterHashes[j]]; ok {
						cacheMapMutex.RUnlock()
						fileLogger.Info("skip already processed file (based on cache file containing it and processor)",
							slog.String("file_id", id),
							slog.Uint64("conv_hash", uint64(converterHashes[j])),
							slog.Int("conv_index", j))
						continue
					}
					cacheMapMutex.RUnlock()
				}
				outputName := conv.DeductOutputPath(inputName)
				originalInputHash := ""
				convLogger := fileLogger.With(slog.String("output_path", outputName), slog.Int("conv_index", j))

				if inputMetadata == nil {
					inputMetadata, err = inputClient.ReadMetadata(inputName)
					if err != nil {
						fileLogger.Warn("fail to read metadata of (supposedly existing) input file", slog.String("error", err.Error()))
						return
					}
				}

				if !cfg.ForceRewrite && !conv.IsMissing(outputName) {
					outputMetadata, err := conv.ReadMetadata(outputName)
					if err != nil {
						convLogger.Warn("fail to read metadata of (supposedly existing) output file", slog.String("error", err.Error()))
						continue
					}
					originalInputHash = outputMetadata.HashOriginal
					if inputMetadata.Hash == originalInputHash {
						convLogger.Info("skip already processed file (based on equal hash)", slog.String("input_hash", inputMetadata.Hash))
						cacheMapMutex.Lock()
						cacheMap[id][converterHashes[j]] = struct{}{}
						cacheMapMutex.Unlock()
						continue
					}
				}
				convertersToLaunch = append(convertersToLaunch, j)
			}

			if len(convertersToLaunch) == 0 {
				return
			}

			processSemaphore <- struct{}{}
			defer func() { <-processSemaphore }()

			fileLogger.Info("start to process file", slog.String("input_hash", inputMetadata.Hash))
			reader, err := inputClient.GetReader(inputName)
			if err != nil {
				fileLogger.Warn("fail to get reader for input file", slog.String("error", err.Error()))
				return
			}

			fileContent := make([]byte, inputMetadata.Size)
			if _, err = reader.Read(fileContent); err != nil {
				fileLogger.Warn("fail to read content of input file", slog.String("error", err.Error()))
				return
			}
			reader.Close()

			for _, convIndex := range convertersToLaunch {
				conv := converters[convIndex]
				outputName := conv.DeductOutputPath(inputName)
				convLogger := fileLogger.With(slog.String("output_path", outputName), slog.Int("conv_index", convIndex))

				if err := conv.Process(inputMetadata, bytes.NewReader(fileContent), outputName); err != nil {
					convLogger.Warn("fail to convert file", slog.String("error", err.Error()))
					return
				}

				convLogger.Info("successfully processed file", slog.String("input_hash", inputMetadata.Hash))
				cacheMapMutex.Lock()
				cacheMap[id][converterHashes[convIndex]] = struct{}{}
				cacheMapMutex.Unlock()
			}
		}(i, file)
	}

	wg.Wait()

	select {
	case <-sigTermChan:
		generalLogger.Info("exiting due to termination signal")
		os.Exit(130)
	default:
	}

	generalLogger.Info("all files processed successfully")

	if cfg.Input.CacheProcessed {
		generalLogger.Info("writing cache file")
		cacheFile, err := os.OpenFile(cfg.Input.CacheProcessedCsvPath, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			generalLogger.Error("error writing cache into file")
		} else {
			for id, hashes := range cacheMap {
				var hashStrings = make([]string, 0, len(hashes))
				for hash := range hashes {
					hashStrings = append(hashStrings, strconv.FormatUint(uint64(hash), 10))
				}
				cacheFile.Write([]byte(id))
				cacheFile.Write([]byte{','})
				cacheFile.WriteString(strings.Join(hashStrings, ";"))
				cacheFile.Write([]byte{'\n'})
			}
		}
	}

	os.Exit(0)
}
