/*
Package cmd implements the command-line interface for the Caramba framework.
It provides various commands for managing agents, running examples, and testing functionality.
*/
package cmd

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

/*
Embed a mini filesystem into the binary to hold the default config file.
This will be written to the home directory of the user running the service,
which allows a developer to easily override the config file.
*/
//go:embed cfg/*
var embedded embed.FS

/*
rootCmd represents the base command when called without any subcommands
*/
var (
	projectName  = "a2a-go"
	cfgFile      string
	openaiAPIKey string

	rootCmd = &cobra.Command{
		Use:   "a2a-go",
		Short: "A reference implementation of the Agent-to-Agent (A2A) protocol",
		Long:  longRoot,
	}
)

/*
Execute is the main entry point for the Caramba CLI. It initializes the root command
and executes it.
*/
func Execute() error {
	return rootCmd.Execute()
}

/*
init is a function that initializes the root command and sets up the persistent flags.
*/
func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(
		&cfgFile,
		"config",
		"config.yml",
		"config file (default is $HOME/."+projectName+"/config.yml)",
	)

	rootCmd.PersistentFlags().StringVar(
		&openaiAPIKey,
		"openai-api-key",
		os.Getenv("OPENAI_API_KEY"),
		"API key for the OpenAI provider",
	)
}

/*
initConfig is a function that initializes the configuration for the Caramba CLI.
It writes the default config file to the user's home directory if it doesn't exist,
and then reads the config file from the user's home directory.
*/
func initConfig() {
	var err error

	if err = writeConfig(); err != nil {
		log.Fatal(err)
	}

	viper.SetConfigName("config")
	viper.SetConfigType("yml")
	// Add user config directory (~/.a2a-go)
	home, _ := os.UserHomeDir()
	viper.AddConfigPath(home + "/." + projectName)

	if err = viper.ReadInConfig(); err != nil {
		log.Fatal(err)
		return
	}
	// If OpenAI API key provided via flag, set environment variable for provider
	if openaiAPIKey != "" {
		_ = os.Setenv("OPENAI_API_KEY", openaiAPIKey)
	}
}

/*
writeConfig is a function that writes the default config file to the user's home directory.
*/
func writeConfig() (err error) {
	var (
		home, _ = os.UserHomeDir()
		fh      fs.File
		buf     bytes.Buffer
	)

	// Create the config directory once before processing files
	configDir := home + "/.a2a-go"
	if !CheckFileExists(configDir) {
		if err = os.MkdirAll(configDir, os.ModePerm); err != nil {
			return fmt.Errorf("failed to create config directory: %w", err)
		}
	}

	for _, file := range []string{cfgFile} {
		fullPath := configDir + "/" + file

		if CheckFileExists(fullPath) {
			continue
		}

		if fh, err = embedded.Open("cfg/" + file); err != nil {
			return fmt.Errorf("failed to open embedded config file: %w", err)
		}

		if _, err = io.Copy(&buf, fh); err != nil {
			fh.Close()
			return fmt.Errorf("failed to read embedded config file: %w", err)
		}

		if err = os.WriteFile(fullPath, buf.Bytes(), 0644); err != nil {
			fh.Close()
			return fmt.Errorf("failed to write config file: %w", err)
		}

		log.Println("wrote config file to", fullPath)
		buf.Reset()
		fh.Close()
	}

	return nil
}

func CheckFileExists(filePath string) bool {
	_, error := os.Stat(filePath)
	return !errors.Is(error, os.ErrNotExist)
}

/*
longRoot contains the detailed help text for the root command.
*/
var longRoot = `
a2a-go is a reference Go implementation of the Agent-to-Agent (A2A) protocol by Google.
It provides a simple and flexible way to build multi-agent systems in Go.
`
