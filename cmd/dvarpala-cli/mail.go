package main

import (
	"fmt"
	"os"
	"time"

	"dvarpala/internal/auth"
	"dvarpala/internal/config"

	"github.com/spf13/cobra"
)

func mailCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mail",
		Short: "Check that Dvarpala can send email",
	}
	cmd.AddCommand(mailTestCmd())
	return cmd
}

// mailTestCmd sends one real message and reports exactly what happened.
//
// Sign-in codes are the only way on to the network, so mail working is not a
// detail of this system - it is the system. This exists so that can be proven
// in one command, before anybody is depending on it, and so that when it
// breaks the mail server's own words are visible rather than a generic
// failure on somebody's login page.
func mailTestCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "test <email>",
		Short: "Send a test sign-in code to an address",
		Long: "Sends a real message through the configured mail server and prints\n" +
			"what the server said. Nothing is written to the database, and no\n" +
			"login attempt is created - this only exercises delivery.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			to := args[0]

			cfg, err := config.Load(configPath)
			if err != nil {
				return fmt.Errorf("loading config from %s: %w", configPath, err)
			}

			s := cfg.Auth.SMTP
			if s.Host == "" {
				return fmt.Errorf("no mail server is configured.\n\n" +
					"Set auth.smtp.host in the config file, or AUTH_SMTP_HOST in the\n" +
					"environment. Without it, code sign-in only works in debug mode,\n" +
					"where codes are written to the server log instead of being sent.")
			}

			port := s.Port
			if port == 0 {
				port = 587
			}

			fmt.Printf("Sending to     %s\n", to)
			fmt.Printf("Through        %s:%d\n", s.Host, port)
			fmt.Printf("As             %s\n", s.From)
			if s.Username != "" {
				fmt.Printf("Signing in as  %s\n", s.Username)
			} else {
				fmt.Printf("Signing in as  (no credentials configured)\n")
			}
			fmt.Println()

			mailer := &auth.SMTPMailer{
				Host:     s.Host,
				Port:     port,
				Username: s.Username,
				Password: s.Password,
				From:     s.From,
				FromName: s.FromName,
			}

			// A recognisable stand-in. Not a real code: nothing here creates a
			// login attempt, so it could not be used to sign in even if it
			// reached the wrong person.
			started := time.Now()
			err = mailer.SendCode(to, "000000")
			took := time.Since(started).Round(time.Millisecond)

			if err != nil {
				fmt.Fprintf(os.Stderr, "FAILED after %s\n\n%v\n\n", took, err)
				fmt.Fprintln(os.Stderr, hintFor(err))
				return fmt.Errorf("mail was not sent")
			}

			fmt.Printf("Accepted by %s in %s.\n\n", s.Host, took)
			fmt.Println("The server has taken responsibility for the message. Check the")
			fmt.Println("inbox, and the spam folder - a message accepted here can still")
			fmt.Println("be filtered at the far end.")
			return nil
		},
	}
}

// hintFor turns a mail server's refusal into the thing to go and do about it.
func hintFor(err error) string {
	switch msg := err.Error(); {
	case contains(msg, "535", "Username and Password not accepted", "authentication failed"):
		return "The mail server rejected the credentials.\n" +
			"For Google Workspace this usually means an ordinary account password\n" +
			"was used. Programs need an app password instead - generate one at\n" +
			"myaccount.google.com/apppasswords and put it in AUTH_SMTP_PASSWORD."

	case contains(msg, "no such host"):
		return "The mail server's name could not be resolved. Check auth.smtp.host\n" +
			"for a typo, and that this machine has working DNS."

	case contains(msg, "connection refused"):
		return "Nothing is listening on that port. Check auth.smtp.port - 587 for a\n" +
			"provider, 25 for a relay on this machine."

	case contains(msg, "i/o timeout", "deadline exceeded"):
		return "The mail server never answered. On AWS this is usually the outbound\n" +
			"block on port 25 - use port 587 with a provider, which is not blocked."

	case contains(msg, "clear text"):
		return "The server would not encrypt the connection, so the password was not\n" +
			"sent. Use port 587 or 465, which providers offer for exactly this."

	case contains(msg, "rejected the sender"):
		return "The account you signed in with is not allowed to send as that\n" +
			"address. Make auth.smtp.from match auth.smtp.username, or have an\n" +
			"administrator permit the alias."

	default:
		return "The message above is the mail server's own words. It is the most\n" +
			"accurate description of what went wrong."
	}
}

func contains(haystack string, needles ...string) bool {
	for _, n := range needles {
		if len(n) > 0 && indexFold(haystack, n) >= 0 {
			return true
		}
	}
	return false
}

// indexFold is a case-insensitive strings.Index; mail servers are not
// consistent about the case of their messages.
func indexFold(s, substr string) int {
	ls, lsub := lower(s), lower(substr)
	for i := 0; i+len(lsub) <= len(ls); i++ {
		if ls[i:i+len(lsub)] == lsub {
			return i
		}
	}
	return -1
}

func lower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}
