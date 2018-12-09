package main

import (
	"flag"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-yaml/yaml"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type cliArguments struct {
	ConfigINI          string
	MapGenSettingsJSON string
	MapSettingsJSON    string
	ModListJSON        string
	ServerSettingsJSON string
	ValuesYAML         string
}

func addFileToValues(values interface{}, filePath string) (newValues interface{}, err error) {
	bytes, err := readFile(filePath)
	if err != nil {
		return nil, err
	}

	key := strings.TrimSuffix(strings.Replace(filepath.Base(filePath), "-", "_", -1), ".json")
	values.(map[interface{}]interface{})["factorio"].(map[interface{}]interface{})[key] = string(bytes)

	return values, nil
}

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

func main() {
	logger := newLogger()
	defer func() { _ = logger.Sync() }()

	rawArguments := os.Args[1:]
	arguments, err := parseArguments(rawArguments)
	checkError(err, logger, zapcore.FatalLevel, "parsing CLI arguments failed")

	valuesFile, err := os.OpenFile(arguments.ValuesYAML, os.O_RDWR|os.O_CREATE, 0777)
	checkError(err, logger, zapcore.FatalLevel, "opening output file", zap.String("output_yaml", arguments.ValuesYAML))
	defer func() { _ = valuesFile.Close() }()

	yamlBytes, err := ioutil.ReadAll(valuesFile)
	checkError(err, logger, zapcore.FatalLevel, "reading output file", zap.String("output_yaml", arguments.ValuesYAML))

	var values interface{}
	err = yaml.Unmarshal(yamlBytes, &values)
	checkError(err, logger, zapcore.FatalLevel, "unmarshalling output yaml values", zap.String("raw_yaml", string(yamlBytes)))

	values, err = addFileToValues(values, arguments.MapGenSettingsJSON)
	checkError(err, logger, zapcore.FatalLevel, "adding file to values", zap.String("path", arguments.MapGenSettingsJSON))

	values, err = addFileToValues(values, arguments.MapSettingsJSON)
	checkError(err, logger, zapcore.FatalLevel, "adding file to values", zap.String("path", arguments.MapSettingsJSON))

	values, err = addFileToValues(values, arguments.ModListJSON)
	checkError(err, logger, zapcore.FatalLevel, "adding file to values", zap.String("path", arguments.ModListJSON))

	values, err = addFileToValues(values, arguments.ServerSettingsJSON)
	checkError(err, logger, zapcore.FatalLevel, "adding file to values", zap.String("path", arguments.ServerSettingsJSON))

	valueBytes, err := yaml.Marshal(values)
	checkError(err, logger, zapcore.FatalLevel, "marshalling output yaml values", zap.Any("raw_values", values))

	err = ioutil.WriteFile(arguments.ValuesYAML, valueBytes, 0777)
	checkError(err, logger, zapcore.FatalLevel, "writing values YAML", zap.String("path", arguments.ValuesYAML))
}

func newLogger() (logger *zap.Logger) {
	logger, err := zap.NewProduction()
	if err != nil {
		panic(errors.Wrap(err, "creating logger failed"))
	}

	return logger
}

func parseArguments(rawArguments []string) (arguments cliArguments, err error) {
	flags := flag.NewFlagSet("cli_arguments", flag.ContinueOnError)
	flags.StringVar(&arguments.ConfigINI, "config-ini", "", "Config INI file path.")
	flags.StringVar(&arguments.MapGenSettingsJSON, "map-gen-settings-json", "", "Map gen settings JSON file path.")
	flags.StringVar(&arguments.MapSettingsJSON, "map-settings-json", "", "Map settings JSON file path.")
	flags.StringVar(&arguments.ModListJSON, "mod-list-json", "", "Mod list JSON file path.")
	flags.StringVar(&arguments.ServerSettingsJSON, "server-settings-json", "", "Server settings JSON file path.")
	flags.StringVar(&arguments.ValuesYAML, "values-yaml", "", "Helm values YAML file path.")

	err = flags.Parse(rawArguments)
	if err != nil {
		return arguments, errors.Wrap(err, "parsing CLI flags returned error")
	}

	return arguments, nil
}

func readFile(filePath string) (content []byte, err error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}

	return ioutil.ReadAll(file)
}
