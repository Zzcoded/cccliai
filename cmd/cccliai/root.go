package main

import (
	"fmt"
	"os"

	"github.com/cccliai/app/internal/config"
	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfg *config.Config

var rootCmd = &cobra.Command{
	Use:     "cccliai",
	Short:   "cccliai - Multi-Gateway AI Orchestration Platform",
	Version: "0.2.0",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Load config once for all subcommands
		cfg = config.LoadConfig()
	},
}

func init() {
	// Load local .env files when running from project/workspace.
	_ = godotenv.Load(".env", ".env.local")

	// Setup Viper for environment variables
	viper.AutomaticEnv()
	viper.SetEnvPrefix("CCCLIAI")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
