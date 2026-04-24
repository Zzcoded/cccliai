package main

import (
	"fmt"
	"os"

	"github.com/cccliai/app/internal/db"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

var feishuAppID string
var feishuAppSecret string
var feishuVerificationToken string

var channelCmd = &cobra.Command{
	Use:   "channel",
	Short: "Manage messaging channels",
}

var channelBindFeishuCmd = &cobra.Command{
	Use:   "bind-feishu",
	Short: "Bind a Feishu app to the gateway",
	Run: func(cmd *cobra.Command, args []string) {
		database, err := db.Initialize(cfg)
		if err != nil {
			fmt.Printf("Failed to initialize db: %v\n", err)
			os.Exit(1)
		}
		defer database.Close()

		if feishuAppID == "" || feishuAppSecret == "" {
			fmt.Println("Error: --app-id and --app-secret are required to bind feishu")
			os.Exit(1)
		}

		channelID := "ch_" + uuid.New().String()[:8]
		configJSON := fmt.Sprintf(`{"appId":"%s","appSecret":"%s","verificationToken":"%s"}`, feishuAppID, feishuAppSecret, feishuVerificationToken)

		// Keep only one feishu channel to avoid ambiguous routing.
		if _, err := database.Exec(`DELETE FROM channels WHERE type = 'feishu'`); err != nil {
			fmt.Printf("Failed to cleanup old feishu channels: %v\n", err)
			return
		}

		_, err = database.Exec(`
			INSERT OR REPLACE INTO channels (id, type, name, config, enabled, status)
			VALUES (?, 'feishu', 'feishu_bot', ?, 1, 'stopped')`,
			channelID, configJSON,
		)
		if err != nil {
			fmt.Printf("Failed to bind feishu channel: %v\n", err)
			return
		}

		fmt.Printf("Feishu channel bound successfully.\n")
		fmt.Printf("Set Feishu event callback URL to: http://<your-host>:42617/webhook/feishu\n")
	},
}

func init() {
	channelBindFeishuCmd.Flags().StringVar(&feishuAppID, "app-id", "", "Feishu App ID")
	channelBindFeishuCmd.Flags().StringVar(&feishuAppSecret, "app-secret", "", "Feishu App Secret")
	channelBindFeishuCmd.Flags().StringVar(&feishuVerificationToken, "verification-token", "", "Feishu event Verification Token")

	channelCmd.AddCommand(channelBindFeishuCmd)
	rootCmd.AddCommand(channelCmd)
}
