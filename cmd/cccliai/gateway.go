package main

import (
	"fmt"
	"os"
	"time"

	"github.com/cccliai/app/internal/db"
	"github.com/cccliai/app/internal/server"
	"github.com/spf13/cobra"
)

var port string

var gatewayCmd = &cobra.Command{
	Use:   "gateway",
	Short: "Start the gateway server",
	Run: func(cmd *cobra.Command, args []string) {
		database, err := db.Initialize(cfg)
		if err != nil {
			fmt.Printf("Failed to initialize db: %v\n", err)
			os.Exit(1)
		}
		defer database.Close()

		var id, name, status string
		err = database.QueryRow("SELECT id, name, status FROM gateways LIMIT 1").Scan(&id, &name, &status)
		if err != nil {
			fmt.Println("⚠️  No gateway found. Run 'cccliai onboard' first.")
			os.Exit(1)
		}

		fmt.Printf("🚀 Starting cccliai Gateway\nGateway Name: %s\nStatus: %s\n", name, status)

		database.Exec("UPDATE gateways SET status = 'running', updated_at = ? WHERE id = ?",
			time.Now().UnixMilli(), id)

		importServer := server.NewServer(database, cfg)

		if err := importServer.Start(port); err != nil {
			fmt.Printf("Server failed: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	gatewayCmd.Flags().StringVarP(&port, "port", "p", "42617", "Gateway port")
	rootCmd.AddCommand(gatewayCmd)
}
