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
	ChartYAMLPath         string
	ConfigurationYAMLPath string
	ValuesYAMLPath        string
}

// checkError checks the specified error.
// If the error is not nil, it is being logged with the specified level.
// If the specified level is zapcore.FatalLevel the program aborts on error.
func checkError(err error, logger *zap.Logger, level zapcore.Level, message string, fields ...zap.Field) (isError bool) {
	if err == nil {
		return false
	}

	fields2 := append([]zap.Field{zap.Error(err)}, fields...)
	log(logger, level, message, fields2...)

	if level == zapcore.FatalLevel {
		os.Exit(-1)
	}

	return true
}

// generateChart takes the current chart and the basic configuration values and generates the chart values from it.
func generateChart(chart interface{}, configuration interface{}) (generatedChart interface{}) {
	generatedChart = chart
	generatedChart.(map[interface{}]interface{})["appVersion"] = configuration.(map[interface{}]interface{})["factorio"].(map[interface{}]interface{})["kubernetes"].(map[interface{}]interface{})["imageTag"].(string)

	return generatedChart
}

// generateValues takes the basic configuration values and generates expanded values from it.
// It includes expanding raw files into configMap arguments.
func generateValues(configuration interface{}) (values interface{}, err error) {
	values = configuration
	configurationDirectory := values.(map[interface{}]interface{})["factorio"].(map[interface{}]interface{})["paths"].(map[interface{}]interface{})["configuration"].(string)

	for fileIndex, rawFile := range values.(map[interface{}]interface{})["factorio"].(map[interface{}]interface{})["rawFiles"].([]interface{}) {
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
		values.(map[interface{}]interface{})["factorio"].(map[interface{}]interface{})["rawFiles"].([]interface{})[fileIndex].(map[interface{}]interface{})["name"] = name
		values.(map[interface{}]interface{})["factorio"].(map[interface{}]interface{})["rawFiles"].([]interface{})[fileIndex].(map[interface{}]interface{})["path"] = filepath.Join(configurationDirectory, name)
		values.(map[interface{}]interface{})["factorio"].(map[interface{}]interface{})["rawFiles"].([]interface{})[fileIndex].(map[interface{}]interface{})["value"] = string(rawFileBytes)
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

	chart, err := readYAMLFile(arguments.ChartYAMLPath)
	checkError(err, logger, zapcore.FatalLevel, "reading chart YAML file")

	configuration, err := readYAMLFile(arguments.ConfigurationYAMLPath)
	checkError(err, logger, zapcore.FatalLevel, "reading configuration YAML file")

	chart = generateChart(chart, configuration)
	err = writeYAMLFile(chart, arguments.ChartYAMLPath, os.ModePerm)
	checkError(err, logger, zapcore.FatalLevel, "writing chart YAML file")

	values, err := generateValues(configuration)
	checkError(err, logger, zapcore.FatalLevel, "loading configuration file")

	err = writeYAMLFile(values, arguments.ValuesYAMLPath, os.ModePerm)
	checkError(err, logger, zapcore.FatalLevel, "writing values YAML file")
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

	flags.StringVar(&arguments.ChartYAMLPath, "chart-yaml-path", "charts/factorio/Chart.yaml", "Factorio Chart.yaml chart path.")
	flags.StringVar(&arguments.ConfigurationYAMLPath, "configuration-yaml-path", "config/configuration.yaml", "YAML file describing basic Kubernetes configurations for Factorio.")
	flags.StringVar(&arguments.ValuesYAMLPath, "values-yaml-path", "charts/factorio/values.yaml", "Factorio values.yaml chart path.")

	err = flags.Parse(rawArguments)
	if err != nil {
		return nil, errors.Wrap(err, "parsing CLI flags returned error")
	}

	return arguments, nil
}

// readYAMLFile reads and decodes the specified YAML file.
func readYAMLFile(yamlPath string) (content interface{}, err error) {
	if yamlPath == "" {
		return nil, nil
	}

	yamlFile, err := os.OpenFile(yamlPath, os.O_RDONLY, 0777)
	if err != nil {
		return nil, errors.Wrapf(err, "opening YAML file, YAML path: '%+v'", yamlPath)
	}
	defer func() { _ = yamlFile.Close() }()

	contentBytes, err := ioutil.ReadAll(yamlFile)
	if err != nil {
		return nil, errors.Wrapf(err, "reading YAML file, YAML path: '%+v'", yamlPath)
	}

	err = yamlFile.Close()
	if err != nil {
		return nil, errors.Wrapf(err, "closing YAML file, YAML path: '%+v'", yamlPath)
	}

	err = yaml.Unmarshal(contentBytes, &content)
	if err != nil {
		return nil, errors.Wrapf(err, "decoding yaml content, content: '%+v'", string(contentBytes))
	}

	return content, nil
}

// writeYAMLFile encodes and writes the given content to the specified path as a YAML file.
func writeYAMLFile(content interface{}, yamlPath string, permissions os.FileMode) (err error) {
	contentBytes, err := yaml.Marshal(content)
	if err != nil {
		return errors.Wrapf(err, "encoding YAML content, yaml_path: '%+v', content: '%+v'", yamlPath, content)
	}

	err = ioutil.WriteFile(yamlPath, contentBytes, permissions)
	if err != nil {
		return errors.Wrapf(err, "writing to file, yaml_path: '%+v', content: '%+v'", yamlPath, string(contentBytes))
	}

	return nil
}
