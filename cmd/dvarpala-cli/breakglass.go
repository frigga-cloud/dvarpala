package main

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"dvarpala/internal/auth"
	"dvarpala/internal/config"
	"dvarpala/internal/database"
	"dvarpala/internal/redis"
	"dvarpala/internal/services"

	"github.com/spf13/cobra"
	gormlogger "gorm.io/gorm/logger"
)

// breakGlassCmd issues a way back in when the ordinary one is broken.
//
// Sign-in codes are the only route on to this network. That is a deliberate
// choice, and it has one consequence worth planning for: if mail stops
// working, nobody can sign in - including whoever would repair the mail
// server. This is the answer to that, and it exists so the answer is never
// "rebuild the server".
func breakGlassCmd() *cobra.Command {
	var (
		reason   string
		minutes  int
		clientIP string
		portal   string
	)

	cmd := &cobra.Command{
		Use:   "break-glass <email>",
		Short: "Issue emergency access when sign-in codes cannot be delivered",
		Long: "Creates a short-lived session for an existing, active account and\n" +
			"prints a single-use link that signs that person in.\n\n" +
			"This skips one thing: proving the person controls their mailbox.\n" +
			"Everything else still applies - the account must exist, it must be\n" +
			"active, and its groups decide what opens. A deactivated account\n" +
			"stays deactivated.\n\n" +
			"Every use is recorded, including refusals.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			email := strings.ToLower(strings.TrimSpace(args[0]))

			if clientIP != "" && net.ParseIP(clientIP) == nil {
				return fmt.Errorf("--client-ip %q is not an IP address", clientIP)
			}

			cfg, err := config.Load(configPath)
			if err != nil {
				return fmt.Errorf("loading config from %s: %w", configPath, err)
			}

			db, err := database.NewConnection(cfg.Database)
			if err != nil {
				return fmt.Errorf("connecting to database: %w", err)
			}
			db.Logger = gormlogger.Discard

			rdb, err := redis.NewClient(cfg.Redis)
			if err != nil {
				return fmt.Errorf("connecting to redis: %w\n\n"+
					"Sessions live in Redis, so emergency access needs it running "+
					"even though everything else here does not.", err)
			}
			defer rdb.Close()

			svc := services.New(db.DB, cfg)
			sessions := auth.NewSessionService(rdb,
				time.Duration(cfg.Auth.SessionDuration)*time.Second)

			bg := auth.NewBreakGlass(rdb, sessions, svc.Users, svc.Audit)

			ctx := context.Background()
			grant, err := bg.Issue(ctx, email, clientIP, reason,
				time.Duration(minutes)*time.Minute)
			if err != nil {
				return err
			}

			printGrant(grant, portal, clientIP)
			return nil
		},
	}

	cmd.Flags().StringVar(&reason, "reason", "",
		"why this was needed (required, and recorded)")
	cmd.Flags().IntVar(&minutes, "minutes", 15,
		"how long the session lasts")
	cmd.Flags().StringVar(&clientIP, "client-ip", "",
		"tunnel address to bind, so the VPN opens too and not only the console")
	cmd.Flags().StringVar(&portal, "portal", "http://172.30.100.1:8080",
		"portal address, used to build the link")
	_ = cmd.MarkFlagRequired("reason")

	return cmd
}

func printGrant(g *auth.Grant, portal, clientIP string) {
	link := fmt.Sprintf("%s/break-glass?code=%s", strings.TrimRight(portal, "/"), g.Code)

	fmt.Printf("Emergency access for %s\n\n", g.Email)
	fmt.Printf("  Groups   %s\n", strings.Join(g.Groups, ", "))
	fmt.Printf("  Expires  %s (in %s)\n",
		g.Expires.Format("15:04:05"), time.Until(g.Expires).Round(time.Minute))
	if clientIP != "" {
		fmt.Printf("  Tunnel   %s - network access, not only the console\n", clientIP)
	} else {
		fmt.Printf("  Tunnel   not bound - this opens the console, not the network\n")
	}

	fmt.Printf("\nOpen this once, in a browser on the VPN:\n\n  %s\n\n", link)

	fmt.Println("The link works once and then stops. If it does not open, the")
	fmt.Println("portal address is probably wrong for where you are - pass the")
	fmt.Println("right one with --portal.")
}
