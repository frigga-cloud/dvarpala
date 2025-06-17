package main

import (
	"log"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	var rootCmd = &cobra.Command{
		Use:   "dvarpala-cli",
		Short: "Dvarpala VPN management CLI",
		Long:  "Command line interface for managing Dvarpala VPN users, groups, and resources",
	}

	// Add subcommands
	rootCmd.AddCommand(userCmd())
	rootCmd.AddCommand(groupCmd())
	rootCmd.AddCommand(vpnCmd())
	rootCmd.AddCommand(adminCmd())

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
}

func userCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "user",
		Short: "User management commands",
	}
}

func groupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "group",
		Short: "Group management commands",
	}
}

func vpnCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "vpn",
		Short: "VPN management commands",
	}
}

func adminCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "admin",
		Short: "Administrative commands",
	}
}
