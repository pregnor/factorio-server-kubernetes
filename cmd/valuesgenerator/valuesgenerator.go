package main

import (
	"flag"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	"github.com/go-yaml/yaml"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// cliArguments aggregates the libargen CLI arguments.
type cliArguments struct {
	ConfigurationPath string
	ValuesYAMLPath    string
}

// checkError checks the specified error.
// If the error is not nil, it is being logged with the specified level.
// If the specified level is zapcore.FatalLevel the program aborts on error.
func checkError(err error, logger *zap.Logger, level zapcore.Level, message string, fields ...zap.Field) {
	if err == nil {
		return
	}

	fields2 := append([]zap.Field{zap.Error(err)}, fields...)
	log(logger, level, message, fields2...)

	if level == zapcore.FatalLevel {
		os.Exit(-1)
	}
}

// loadConfigurationFile loads a configuration file with flags and raw configuration file values.
func loadConfigurationFile(configurationPath string) (values map[interface{}]interface{}, err error) {
	if configurationPath == "" {
		return values, nil
	}

	configurationFile, err := os.OpenFile(configurationPath, os.O_RDONLY, 0777)
	if err != nil {
		return values, errors.Wrapf(err, "opening configuration file, configuration file JSON path: '%+v'", configurationPath)
	}

	configurationBytes, err := ioutil.ReadAll(configurationFile)
	if err != nil {
		return values, errors.Wrapf(err, "reading configuration file, configuration file JSON path: '%+v'", configurationPath)
	}

	err = yaml.Unmarshal(configurationBytes, &values)
	if err != nil {
		return values, errors.Wrapf(err, "decoding configuration file, raw configurations: '%+v'", string(configurationBytes))
	}

	configurationDirectory := values["factorio"].(map[interface{}]interface{})["paths"].(map[interface{}]interface{})["configuration"].(string)

	for fileIndex, rawFile := range values["factorio"].(map[interface{}]interface{})["rawFiles"].([]interface{}) {
		source := rawFile.(map[interface{}]interface{})["source"].(string)
		rawFile, err := os.OpenFile(source, os.O_RDONLY, 0777)
		if err != nil {
			return values, errors.Wrapf(err, "opening raw file, raw file: '%+v'", rawFile)
		}

		rawFileBytes, err := ioutil.ReadAll(rawFile)
		if err != nil {
			return values, errors.Wrapf(err, "reading raw file, raw file: '%+v'", rawFile)
		}

		name := path.Base(source)
		values["factorio"].(map[interface{}]interface{})["rawFiles"].([]interface{})[fileIndex].(map[interface{}]interface{})["name"] = name
		values["factorio"].(map[interface{}]interface{})["rawFiles"].([]interface{})[fileIndex].(map[interface{}]interface{})["path"] = filepath.Join(configurationDirectory, name)
		values["factorio"].(map[interface{}]interface{})["rawFiles"].([]interface{})[fileIndex].(map[interface{}]interface{})["value"] = string(rawFileBytes)
	}

	return values, nil
}

// log logs the specified message with the given fields on the level provided.
func log(logger *zap.Logger, level zapcore.Level, message string, fields ...zap.Field) {
	switch level {
	case zapcore.DebugLevel:
		logger.Debug(message, fields...)
	case zapcore.DPanicLevel:
		logger.DPanic(message, fields...)
	case zapcore.ErrorLevel:
		logger.Error(message, fields...)
	case zapcore.FatalLevel:
		logger.Fatal(message, fields...)
	case zapcore.InfoLevel:
		logger.Info(message, fields...)
	case zapcore.PanicLevel:
		logger.Panic(message, fields...)
	case zapcore.WarnLevel:
		logger.Warn(message, fields...)
	}
}

// main is the entry point of the Golang application.
func main() {
	logger := newLogger()
	defer func() { _ = logger.Sync() }()

	rawArguments := os.Args[1:]
	arguments, err := parseArguments(rawArguments)
	checkError(err, logger, zapcore.FatalLevel, "parsing CLI arguments")

	values, err := loadConfigurationFile(arguments.ConfigurationPath)
	checkError(err, logger, zapcore.FatalLevel, "loading configuration file")

	valuesBytes, err := yaml.Marshal(values)
	checkError(err, logger, zapcore.FatalLevel, "marshalling values", zap.Any("values", values))

	err = ioutil.WriteFile(arguments.ValuesYAMLPath, valuesBytes, 0777)
	checkError(err, logger, zapcore.FatalLevel, "writing values file", zap.String("values_yaml_path", arguments.ValuesYAMLPath), zap.Any("values", values))
}

// newLogger creates a new Zap logger.
func newLogger() (logger *zap.Logger) {
	logger, err := zap.NewProduction()
	if err != nil {
		panic(errors.Wrap(err, "creating logger failed"))
	}

	return logger
}

// parseArguments returns the parsed command line arguments.
func parseArguments(rawArguments []string) (arguments *cliArguments, err error) {
	arguments = &cliArguments{}
	flags := flag.NewFlagSet("cli_arguments", flag.ContinueOnError)

	flags.StringVar(&arguments.ConfigurationPath, "configuration-path", "config/configuration.yaml", "JSON file containing flags and raw configuration file content. Raw file paths are automatically expanded into their values.")
	flags.StringVar(&arguments.ValuesYAMLPath, "values-yaml-path", "charts/factorio/values.yaml", "Factorio chart values.yaml path.")

	err = flags.Parse(rawArguments)
	if err != nil {
		return nil, errors.Wrap(err, "parsing CLI flags returned error")
	}

	return arguments, nil
}
