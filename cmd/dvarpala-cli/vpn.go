package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"dvarpala/internal/config"

	"github.com/spf13/cobra"
)

func vpnCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vpn",
		Short: "VPN certificate and profile commands",
	}
	cmd.AddCommand(vpnIssueCmd(), vpnListCmd(), vpnRevokeCmd(), vpnServerCertCmd())
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
		Example: "  dvarpala-cli vpn issue --user sam@acme.com\n" +
			"\n" +
			"  # fetched in one command, from your own machine:\n" +
			"  ssh -i key.pem ubuntu@server \\\n" +
			"    'dvarpala-cli vpn issue --user sam@acme.com --output -' > sam.ovpn",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := openServices()
			if err != nil {
				return err
			}

			// The address clients connect to, which is also the address an
			// operator reaches this machine on. Read rather than guessed, so
			// the commands printed below can be run as printed.
			host := "<this-server>"
			if cfg, err := config.Load(configPath); err == nil {
				if h := strings.TrimSpace(cfg.OpenVPN.Server.Host); h != "" {
					host = h
				}
			}

			validity := time.Duration(days) * 24 * time.Hour
			config, err := svc.VPNConfigs.Issue(cmd.Context(), email, name, validity)
			if err != nil {
				return err
			}

			// Straight out, when asked for. This is what makes fetching a
			// profile one command instead of four: an operator can run this
			// over ssh from their own machine and redirect the result into a
			// file, rather than writing it here, changing its owner, copying
			// it down and remembering to delete both copies.
			if output == "-" {
				// Nothing else on stdout: the caller is redirecting it into a
				// file. Silence is what success looks like here, which is
				// worth knowing after a run of commands that fail quietly.
				fmt.Fprint(cmd.OutOrStdout(), config.ConfigData)
				fmt.Fprintf(cmd.ErrOrStderr(),
					"Issued for %s, expires %s. Written to standard output - "+
						"if you redirected it into a file, that file now holds "+
						"the profile and nothing was printed here.\n",
					email, config.ExpiresAt.Format("2006-01-02"))
				return nil
			}

			// Somewhere this can actually be written.
			//
			// This command runs as the service user, because that is who may
			// read the database password - and that user cannot write to an
			// administrator's home directory. A bare "--output sam.ovpn"
			// therefore failed with "permission denied" for everybody, on the
			// very command the documentation demonstrates.
			if output == "" {
				output = filepath.Join(profileDir, filepath.Base(email)+".ovpn")
			}

			if dir := filepath.Dir(output); dir != "" && dir != "." {
				if err := os.MkdirAll(dir, 0o750); err != nil {
					return fmt.Errorf("preparing %s: %w", dir, err)
				}
			}

			// The profile contains the user's private key.
			if err := os.WriteFile(output, []byte(config.ConfigData), 0o600); err != nil {
				return fmt.Errorf("writing %s: %w\n\n"+
					"This command runs as the %q user, which cannot write everywhere\n"+
					"an administrator can. Leave --output off and it is written to\n"+
					"%s, or give a path that user can write.",
					output, err, serviceUser, profileDir)
			}

			fmt.Printf("Issued VPN profile for %s\n", email)
			fmt.Printf("  common name: %s\n", email)
			fmt.Printf("  expires:     %s\n", config.ExpiresAt.Format("2006-01-02"))
			fmt.Printf("  written to:  %s (mode 0600 - contains a private key)\n", output)
			// The whole command, ready to run, from the machine the operator
			// is sitting at. "Copy it to the person" is advice, not an
			// instruction: it leaves somebody to work out that the file is
			// owned by another user, that it has to be staged somewhere
			// readable first, and that scp runs from their own machine rather
			// than from this one.
			fmt.Println()
			fmt.Println("It holds a private key, and is owned by the service user.")
			fmt.Println()
			fmt.Println("Fetch it in one command, ON YOUR OWN COMPUTER:")
			fmt.Println()
			fmt.Printf("  ssh -i <SSH-KEY> ubuntu@%s \\\n", host)
			fmt.Printf("    'dvarpala-cli vpn issue --user %s --output -' > %s.ovpn\n",
				email, filepath.Base(email))
			fmt.Println()
			fmt.Println()
			fmt.Println("<SSH-KEY> is whatever got you on to this machine a moment ago -")
			fmt.Println("the .pem file, not the .ovpn above. Those are different files")
			fmt.Println("with different jobs: the .pem is yours and opens this server,")
			fmt.Println("the .ovpn is theirs and opens the VPN.")
			fmt.Println()
			fmt.Println("Find its path with, ON YOUR OWN COMPUTER:")
			fmt.Println("  ls ~/*/scripts/installation/dvarpala-deployment/*.pem")
			fmt.Println()
			fmt.Println("If you have an entry in ~/.ssh/config for this server it is")
			fmt.Println("simply:")
			fmt.Println()
			fmt.Printf("  ssh <host> 'dvarpala-cli vpn issue --user %s --output -' > %s.ovpn\n",
				email, filepath.Base(email))
			fmt.Println()
			fmt.Println("That command prints nothing when it works - the profile goes")
			fmt.Println("into the file rather than to the screen. Nothing opens by")
			fmt.Println("itself either. To hand it to the VPN client:")
			fmt.Println()
			fmt.Printf("  open %s.ovpn          # macOS\n", filepath.Base(email))
			fmt.Printf("  xdg-open %s.ovpn      # Linux\n", filepath.Base(email))
			fmt.Println()
			fmt.Println("Then press connect in the client. Give the file to the person it")
			fmt.Println("belongs to directly - not by email - and remove the copy left")
			fmt.Println("here:")
			fmt.Printf("  sudo rm %q\n", output)
			return nil
		},
	}

	cmd.Flags().StringVar(&email, "user", "", "User email (required)")
	cmd.Flags().StringVar(&name, "name", "", "Label for this profile, e.g. laptop")
	cmd.Flags().StringVar(&output, "output", "",
		"Where to write the .ovpn file, or \"-\" for standard output "+
			"(default: "+profileDir+")")
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

// vpnServerCertCmd issues the certificate the OpenVPN server presents.
//
// It is signed by the same authority as the client certificates, so a client
// profile carries one CA certificate and trusts exactly this server.
func vpnServerCertCmd() *cobra.Command {
	var host, outCert, outKey string
	var days int

	cmd := &cobra.Command{
		Use:   "server-cert",
		Short: "Issue the VPN server's own certificate",
		Example: "  dvarpala-cli vpn server-cert --host vpn.example.com \\\n" +
			"      --out-cert /opt/dvarpala/certs/server.crt \\\n" +
			"      --out-key /opt/dvarpala/certs/server.key",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := openServices()
			if err != nil {
				return err
			}

			cred, err := svc.VPNConfigs.IssueServerCert(
				"dvarpala-server", host, time.Duration(days)*24*time.Hour)
			if err != nil {
				return err
			}

			if err := os.WriteFile(outCert, []byte(cred.Certificate), 0o644); err != nil {
				return fmt.Errorf("writing %s: %w", outCert, err)
			}
			if err := os.WriteFile(outKey, []byte(cred.PrivateKey), 0o600); err != nil {
				return fmt.Errorf("writing %s: %w", outKey, err)
			}

			fmt.Printf("Issued server certificate for %s\n", host)
			fmt.Printf("  expires: %s\n", cred.NotAfter.Format("2006-01-02"))
			return nil
		},
	}

	cmd.Flags().StringVar(&host, "host", "", "Hostname or IP clients connect to (required)")
	cmd.Flags().StringVar(&outCert, "out-cert", "", "Where to write the certificate (required)")
	cmd.Flags().StringVar(&outKey, "out-key", "", "Where to write the private key (required)")
	cmd.Flags().IntVar(&days, "days", 1825, "How long the certificate is valid")
	_ = cmd.MarkFlagRequired("host")
	_ = cmd.MarkFlagRequired("out-cert")
	_ = cmd.MarkFlagRequired("out-key")

	return cmd
}

// Where profiles are written, and who this command runs as.
//
// The service user owns the certificates and may read the database password,
// so the CLI becomes that user - and it therefore cannot write into an
// administrator's home directory. A directory it owns removes the problem
// rather than explaining it.
const (
	profileDir  = "/opt/dvarpala/profiles"
	serviceUser = "dvarpala"
)
