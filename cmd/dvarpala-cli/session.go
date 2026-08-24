package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"dvarpala/internal/auth"
	"dvarpala/internal/config"
	"dvarpala/internal/redis"

	"github.com/spf13/cobra"
)

// sessionCmd answers "who is on the network right now".
//
// Sessions live in Redis rather than the database, so until now the only way
// to see one was to know the key layout and read it by hand. That is the
// question an operator asks most often and it had the least accessible
// answer.
func sessionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "session",
		Short: "Inspect and end live sessions",
	}
	cmd.AddCommand(sessionListCmd(), sessionEndCmd())
	return cmd
}

// openSessions connects to Redis alone. No database: a session is entirely a
// Redis object, and needing Postgres to answer "who is connected" would make
// this useless in exactly the outage where it is most wanted.
func openSessions() (*auth.SessionService, *redis.Client, error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, nil, fmt.Errorf("loading config from %s: %w", configPath, err)
	}

	rdb, err := redis.NewClient(cfg.Redis)
	if err != nil {
		return nil, nil, fmt.Errorf("connecting to redis: %w", err)
	}

	return auth.NewSessionService(rdb,
		time.Duration(cfg.Auth.SessionDuration)*time.Second), rdb, nil
}

func sessionListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "Show who is signed in right now",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			sessions, rdb, err := openSessions()
			if err != nil {
				return err
			}
			defer rdb.Close()

			active, err := sessions.Active(context.Background())
			if err != nil {
				return err
			}

			if len(active) == 0 {
				fmt.Println("Nobody is signed in.")
				return nil
			}

			fmt.Printf("%-26s  %-13s  %-15s  %-8s  %-7s  %s\n",
				"WHO", "SIGNED IN VIA", "TUNNEL IP", "EXPIRES", "NETWORK", "GROUPS")

			for _, a := range active {
				tunnel := a.ClientIP
				if tunnel == "" {
					tunnel = "-"
				}

				// The distinction that matters: a session the VPN can find
				// opens the network; one it cannot signs in a browser only.
				network := "open"
				if !a.OnTunnel {
					network = "closed"
				}

				fmt.Printf("%-26s  %-13s  %-15s  %-8s  %-7s  %s\n",
					truncate(a.Email, 26),
					truncate(a.Provider, 13),
					truncate(tunnel, 15),
					compactDuration(a.Remaining),
					network,
					strings.Join(a.Groups, ","))
			}

			fmt.Printf("\n%d session(s). \"closed\" means signed in but with no tunnel bound,\n"+
				"so the portal opens and the network does not.\n", len(active))
			return nil
		},
	}
}

// sessionEndCmd is the kill switch. Deactivating an account stops future
// sign-ins but leaves an open session running; this is what actually removes
// somebody who is already on the network.
func sessionEndCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "end <tunnel-ip>",
		Short: "End the session at a tunnel address, closing network access",
		Long: "Revokes the session bound to a VPN client address.\n\n" +
			"Deactivating a user prevents them signing in again but does not\n" +
			"touch a session already open. This does.\n\n" +
			"The tunnel itself stays up until the client reconnects or is\n" +
			"disconnected; what stops is the access behind it.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientIP := args[0]

			sessions, rdb, err := openSessions()
			if err != nil {
				return err
			}
			defer rdb.Close()

			ctx := context.Background()

			// Read it first, so the confirmation can name who was removed
			// rather than only which address went quiet.
			sess, err := sessions.GetByClientIP(ctx, clientIP)
			if err != nil {
				return fmt.Errorf("no session at %s", clientIP)
			}

			if err := sessions.RevokeByClientIP(ctx, clientIP); err != nil {
				return err
			}

			fmt.Printf("Ended %s's session at %s.\n\n", sess.Email, clientIP)
			fmt.Println("Their tunnel is still connected. To close it as well, run")
			fmt.Printf("  dvarpala-firewall.sh revoke %s\n", clientIP)
			fmt.Println("on the VPN server, which also drops connections already open.")
			return nil
		},
	}
}

// compactDuration renders a remaining lifetime for a narrow column.
func compactDuration(d time.Duration) string {
	switch {
	case d <= 0:
		return "expiring"
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	default:
		return fmt.Sprintf("%dh%02dm", int(d.Hours()), int(d.Minutes())%60)
	}
}
