package main

import (
	"fmt"
	"os"

	"dvarpala/internal/config"
	"dvarpala/internal/database"
	"dvarpala/internal/services"

	"github.com/spf13/cobra"
	gormlogger "gorm.io/gorm/logger"
)

// configPath is set by the persistent --config flag.
var configPath string

func main() {
	rootCmd := &cobra.Command{
		Use:   "dvarpala-cli",
		Short: "Dvarpala VPN management CLI",
		Long: "Command line interface for managing Dvarpala VPN users, groups, and resources.\n\n" +
			"Runs on the VPN server, where it can reach the database directly.",
		// Errors are printed once, by main, rather than also by Cobra.
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	rootCmd.PersistentFlags().StringVar(&configPath, "config",
		"configs/environment.yaml", "Config file path")

	rootCmd.AddCommand(userCmd())
	rootCmd.AddCommand(groupCmd())
	rootCmd.AddCommand(resourceCmd())
	rootCmd.AddCommand(permissionCmd())
	rootCmd.AddCommand(vpnCmd())
	rootCmd.AddCommand(adminCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// openServices connects to the database and builds the service layer.
//
// This is the same services.New() the HTTP server uses, so the CLI and the
// API share one implementation of every operation.
func openServices() (*services.Services, error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("loading config from %s: %w", configPath, err)
	}

	db, err := database.NewConnection(cfg.Database)
	if err != nil {
		return nil, fmt.Errorf("connecting to database: %w", err)
	}

	// A CLI should print results, not SQL.
	db.Logger = gormlogger.Discard

	return services.New(db.DB), nil
}

// The command groups below are registered so they appear in help. Their
// subcommands are added as each phase implements them.

func vpnCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "vpn",
		Short: "VPN management commands (not yet implemented)",
	}
}

func adminCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "admin",
		Short: "Administrative commands (not yet implemented)",
	}
}
