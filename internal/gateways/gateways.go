package gateways

import (
	"database/sql"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/cccliai/app/internal/config"
	"github.com/cccliai/app/internal/security"
	"github.com/google/uuid"
)

type Gateway struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	Description        string `json:"description,omitempty"`
	Provider           string `json:"provider"`
	Endpoint           string `json:"endpoint,omitempty"`
	DefaultModel       string `json:"defaultModel,omitempty"`
	Models             string `json:"models,omitempty"`
	RequirePairing     bool   `json:"requirePairing"`
	AllowPublicBind    bool   `json:"allowPublicBind"`
	WorkspaceOnly      bool   `json:"workspaceOnly"`
	DaemonEnabled      bool   `json:"daemonEnabled"`
	DaemonPID          *int64 `json:"daemonPid,omitempty"`
	DaemonPort         *int64 `json:"daemonPort,omitempty"`
	AutonomousEnabled  bool   `json:"autonomousEnabled"`
	AutonomousInterval string `json:"autonomousInterval,omitempty"`
	AutonomousMaxIter  int64  `json:"autonomousMaxIterations"`
	ReflectionEnabled  bool   `json:"reflectionEnabled"`
	Status             string `json:"status"`
	LastError          string `json:"lastError,omitempty"`
	StartedAt          *int64 `json:"startedAt,omitempty"`
	CreatedAt          int64  `json:"createdAt"`
	UpdatedAt          int64  `json:"updatedAt"`
	MaskedAPIKey       string `json:"maskedApiKey,omitempty"`
}

func HandleOnboard(database *sql.DB, args []string) {
	flags := flag.NewFlagSet("onboard", flag.ContinueOnError)
	flags.SetOutput(os.Stdout)

	provider := flags.String("provider", "deepseek", "AI provider name")
	model := flags.String("model", "deepseek-chat", "AI model name")
	apiKey := flags.String("api-key", "", "provider API key")
	name := flags.String("name", "default", "gateway name")

	if err := flags.Parse(args); err != nil {
		fmt.Printf("Failed to parse onboard options: %v\n", err)
		return
	}

	normalizedProvider := strings.ToLower(strings.TrimSpace(*provider))
	if normalizedProvider == "" {
		normalizedProvider = "deepseek"
	}

	normalizedModel := strings.TrimSpace(*model)
	if normalizedModel == "" {
		normalizedModel = defaultModel(normalizedProvider)
	}

	key := strings.TrimSpace(*apiKey)
	if key == "" {
		key = defaultAPIKey(normalizedProvider)
	}

	encryptedKey := ""
	if key != "" {
		var err error
		encryptedKey, err = security.Encrypt(key)
		if err != nil {
			fmt.Printf("Failed to encrypt API key: %v\n", err)
			return
		}
	}

	now := time.Now().UnixMilli()
	id := "gw_" + uuid.New().String()[:8]

	_, err := database.Exec(`
		INSERT INTO gateways (
			id, name, provider, endpoint, api_key_encrypted, default_model, models,
			require_pairing, allow_public_bind, workspace_only, status, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, 1, 0, 1, 'stopped', ?, ?)
		ON CONFLICT(name) DO UPDATE SET
			provider = excluded.provider,
			endpoint = excluded.endpoint,
			api_key_encrypted = excluded.api_key_encrypted,
			default_model = excluded.default_model,
			models = excluded.models,
			updated_at = excluded.updated_at
	`,
		id,
		strings.TrimSpace(*name),
		normalizedProvider,
		defaultEndpoint(normalizedProvider),
		encryptedKey,
		normalizedModel,
		normalizedModel,
		now,
		now,
	)
	if err != nil {
		fmt.Printf("Failed to save gateway: %v\n", err)
		return
	}

	fmt.Printf("Gateway '%s' configured with provider '%s' and model '%s'.\n", strings.TrimSpace(*name), normalizedProvider, normalizedModel)
	if key == "" {
		fmt.Printf("No API key was stored. Set %s before starting the gateway.\n", defaultEnvKey(normalizedProvider))
	}
}

func HandleStatus(database *sql.DB) {
	list, err := ListGateways(database)
	if err != nil {
		fmt.Printf("Failed to load gateways: %v\n", err)
		return
	}

	fmt.Println("cccliai status")
	if len(list) == 0 {
		fmt.Println("No gateways configured. Run 'cccliai onboard' first.")
		return
	}

	for _, gateway := range list {
		apiKeyStatus := "not stored"
		if gateway.MaskedAPIKey != "" {
			apiKeyStatus = gateway.MaskedAPIKey
		}

		fmt.Printf("- %s [%s]\n", gateway.Name, gateway.Status)
		fmt.Printf("  Provider: %s\n", gateway.Provider)
		fmt.Printf("  Model: %s\n", gateway.DefaultModel)
		fmt.Printf("  API key: %s\n", apiKeyStatus)
	}
}

func ListGateways(database *sql.DB) ([]Gateway, error) {
	rows, err := database.Query(`
		SELECT
			id, name, description, provider, endpoint, api_key_encrypted, default_model, models,
			require_pairing, allow_public_bind, workspace_only, daemon_enabled, daemon_pid, daemon_port,
			autonomous_enabled, autonomous_interval, autonomous_max_iterations, reflection_enabled,
			status, last_error, started_at, created_at, updated_at
		FROM gateways
		ORDER BY created_at ASC, name ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Gateway
	for rows.Next() {
		var gateway Gateway
		var description, endpoint, apiKeyEncrypted, defaultModel, models sql.NullString
		var autonomousInterval, status, lastError sql.NullString
		var daemonPID, daemonPort, startedAt sql.NullInt64
		var requirePairing, allowPublicBind, workspaceOnly int
		var daemonEnabled, autonomousEnabled, reflectionEnabled int
		var autonomousMaxIter sql.NullInt64
		var createdAt, updatedAt sql.NullInt64

		err := rows.Scan(
			&gateway.ID,
			&gateway.Name,
			&description,
			&gateway.Provider,
			&endpoint,
			&apiKeyEncrypted,
			&defaultModel,
			&models,
			&requirePairing,
			&allowPublicBind,
			&workspaceOnly,
			&daemonEnabled,
			&daemonPID,
			&daemonPort,
			&autonomousEnabled,
			&autonomousInterval,
			&autonomousMaxIter,
			&reflectionEnabled,
			&status,
			&lastError,
			&startedAt,
			&createdAt,
			&updatedAt,
		)
		if err != nil {
			return nil, err
		}

		gateway.Description = description.String
		gateway.Endpoint = endpoint.String
		gateway.DefaultModel = defaultModel.String
		gateway.Models = models.String
		gateway.RequirePairing = requirePairing != 0
		gateway.AllowPublicBind = allowPublicBind != 0
		gateway.WorkspaceOnly = workspaceOnly != 0
		gateway.DaemonEnabled = daemonEnabled != 0
		gateway.AutonomousEnabled = autonomousEnabled != 0
		gateway.AutonomousInterval = autonomousInterval.String
		gateway.AutonomousMaxIter = autonomousMaxIter.Int64
		gateway.ReflectionEnabled = reflectionEnabled != 0
		gateway.Status = status.String
		gateway.LastError = lastError.String
		gateway.CreatedAt = createdAt.Int64
		gateway.UpdatedAt = updatedAt.Int64

		if daemonPID.Valid {
			gateway.DaemonPID = &daemonPID.Int64
		}
		if daemonPort.Valid {
			gateway.DaemonPort = &daemonPort.Int64
		}
		if startedAt.Valid {
			gateway.StartedAt = &startedAt.Int64
		}
		if apiKeyEncrypted.Valid && apiKeyEncrypted.String != "" {
			gateway.MaskedAPIKey = "stored"
		}

		list = append(list, gateway)
	}

	return list, rows.Err()
}

func defaultEndpoint(provider string) string {
	switch provider {
	case "zai":
		return "https://open.bigmodel.cn/api/paas/v4"
	case "deepseek":
		return "https://api.deepseek.com"
	default:
		return ""
	}
}

func defaultModel(provider string) string {
	switch provider {
	case "zai":
		return "zai-chat"
	default:
		return "deepseek-chat"
	}
}

func defaultAPIKey(provider string) string {
	return config.ProviderAPIKey(provider)
}

func defaultEnvKey(provider string) string {
	return config.ProviderAPIKeyHint(provider)
}
