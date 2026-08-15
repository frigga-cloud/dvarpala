package main

import (
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

func vpnCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vpn",
		Short: "VPN certificate and profile commands",
	}
	cmd.AddCommand(vpnIssueCmd(), vpnListCmd(), vpnRevokeCmd())
	return cmd
}

func vpnIssueCmd() *cobra.Command {
	var email, name, output string
	var days int

	cmd := &cobra.Command{
		Use:   "issue",
		Short: "Issue a VPN profile for a user",
		Long: "Issues a certificate whose common name is the user's email, so the VPN\n" +
			"can identify who has connected. Any previously issued profile for that\n" +
			"user is revoked, so each person holds exactly one credential.",
		Example: "  dvarpala-cli vpn issue --user sam@acme.com --output sam.ovpn",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := openServices()
			if err != nil {
				return err
			}

			validity := time.Duration(days) * 24 * time.Hour
			config, err := svc.VPNConfigs.Issue(cmd.Context(), email, name, validity)
			if err != nil {
				return err
			}

			if output == "" {
				output = filepath.Base(email) + ".ovpn"
			}
			// The profile contains the user's private key.
			if err := os.WriteFile(output, []byte(config.ConfigData), 0o600); err != nil {
				return fmt.Errorf("writing %s: %w", output, err)
			}

			fmt.Printf("Issued VPN profile for %s\n", email)
			fmt.Printf("  common name: %s\n", email)
			fmt.Printf("  expires:     %s\n", config.ExpiresAt.Format("2006-01-02"))
			fmt.Printf("  written to:  %s (mode 0600 - contains a private key)\n", output)
			return nil
		},
	}

	cmd.Flags().StringVar(&email, "user", "", "User email (required)")
	cmd.Flags().StringVar(&name, "name", "", "Label for this profile, e.g. laptop")
	cmd.Flags().StringVar(&output, "output", "", "Where to write the .ovpn file")
	cmd.Flags().IntVar(&days, "days", 365, "How long the certificate is valid")
	_ = cmd.MarkFlagRequired("user")

	return cmd
}

func vpnListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List issued VPN profiles",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := openServices()
			if err != nil {
				return err
			}

			configs, err := svc.VPNConfigs.ListAll(cmd.Context())
			if err != nil {
				return err
			}
			if len(configs) == 0 {
				fmt.Println("No VPN profiles issued.")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
			fmt.Fprintln(w, "ID\tUSER\tPROFILE\tSTATUS\tEXPIRES")
			for _, c := range configs {
				expires := "-"
				if c.ExpiresAt != nil {
					expires = c.ExpiresAt.Format("2006-01-02")
				}
				fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\n",
					c.ID, dash(c.User.Email), dash(c.ConfigName), c.Status, expires)
			}
			w.Flush()

			fmt.Printf("\n%d profile(s)\n", len(configs))
			return nil
		},
	}
}

func vpnRevokeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "revoke <email>",
		Short: "Revoke a user's VPN profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := openServices()
			if err != nil {
				return err
			}
			if err := svc.VPNConfigs.Revoke(cmd.Context(), args[0]); err != nil {
				return err
			}

			fmt.Printf("Revoked VPN profile for %s\n", args[0])
			fmt.Println("Note: no certificate revocation list is generated yet, so the")
			fmt.Println("certificate can still complete a TLS handshake. Access is still")
			fmt.Println("refused, because authentication checks the database.")
			return nil
		},
	}
}
