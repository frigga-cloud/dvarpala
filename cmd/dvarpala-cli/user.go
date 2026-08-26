package main

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"dvarpala/internal/services"

	"github.com/spf13/cobra"
)

func userCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "user",
		Short: "User management commands",
	}

	cmd.AddCommand(userCreateCmd(), userListCmd(), userShowCmd(),
		userAccessCmd(), userDeactivateCmd(), userActivateCmd())
	return cmd
}

func userCreateCmd() *cobra.Command {
	var email, name, department string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a user",
		Example: "  dvarpala-cli user create --email sam@acme.com " +
			"--name \"Sam Patel\" --department Engineering",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := openServices()
			if err != nil {
				return err
			}

			user, err := svc.Users.CreateUser(cmd.Context(), services.CreateUserRequest{
				Email:      email,
				FullName:   name,
				Department: department,
			})
			if err != nil {
				return err
			}

			fmt.Printf("Created user %s (id %d, status %s)\n",
				user.Email, user.ID, user.Status)
			return nil
		},
	}

	cmd.Flags().StringVar(&email, "email", "", "Email address (required)")
	cmd.Flags().StringVar(&name, "name", "", "Full name")
	cmd.Flags().StringVar(&department, "department", "", "Department")
	_ = cmd.MarkFlagRequired("email")

	return cmd
}

func userListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all users",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := openServices()
			if err != nil {
				return err
			}

			list, err := svc.Users.ListUsers(cmd.Context())
			if err != nil {
				return err
			}

			if len(list) == 0 {
				fmt.Println("No users.")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
			fmt.Fprintln(w, "ID\tEMAIL\tNAME\tDEPARTMENT\tSTATUS\tGROUPS")
			for _, u := range list {
				names := make([]string, 0, len(u.Groups))
				for _, g := range u.Groups {
					names = append(names, g.Name)
				}
				groups := strings.Join(names, ",")
				if groups == "" {
					groups = "-"
				}

				fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%s\n",
					u.ID, u.Email, dash(u.FullName), dash(u.Department), u.Status, groups)
			}
			w.Flush()

			fmt.Printf("\n%d user(s)\n", len(list))
			return nil
		},
	}
}

func userShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <email>",
		Short: "Show a single user, and whether they may be granted VPN access",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := openServices()
			if err != nil {
				return err
			}

			user, err := svc.Users.GetUserByEmail(cmd.Context(), args[0])
			if err != nil {
				return err
			}

			fmt.Printf("Email:      %s\n", user.Email)
			fmt.Printf("Name:       %s\n", dash(user.FullName))
			fmt.Printf("Department: %s\n", dash(user.Department))
			fmt.Printf("Status:     %s\n", user.Status)
			fmt.Printf("Created:    %s\n", user.CreatedAt.Format("2006-01-02 15:04"))

			// The same check the VPN authentication path will make.
			if _, err := svc.Users.IsAuthorised(cmd.Context(), user.Email); err != nil {
				fmt.Printf("VPN access: DENIED (%v)\n", err)
			} else {
				fmt.Printf("VPN access: ALLOWED\n")
			}
			return nil
		},
	}
}

func userDeactivateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "deactivate <email>",
		Short: "Deactivate a user, preventing future VPN authentication",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := openServices()
			if err != nil {
				return err
			}

			if err := svc.Users.DeactivateUser(cmd.Context(), args[0]); err != nil {
				return err
			}

			fmt.Printf("Deactivated %s\n", args[0])
			fmt.Println("They cannot sign in again. Any tunnel they were holding")
			fmt.Println("has been closed, but a browser session already open lasts")
			fmt.Println("until it expires - end it with: dvarpala-cli session end <tunnel-ip>")
			fmt.Println()
			fmt.Println("To reverse this:  dvarpala-cli user activate " + args[0])
			return nil
		},
	}
}

// userActivateCmd reopens an account.
//
// Deactivation had no counterpart until this existed. An administrator who
// deactivated themselves by mistake was locked out of their own system, and
// break-glass is deliberately no help - it refuses an inactive account,
// because it exists to get past broken mail rather than past a decision.
// The only way back was editing the database by hand.
func userActivateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "activate <email>",
		Short: "Return a deactivated user to service",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := openServices()
			if err != nil {
				return err
			}

			if err := svc.Users.ActivateUser(cmd.Context(), args[0]); err != nil {
				return err
			}

			fmt.Printf("Activated %s\n", args[0])
			fmt.Println("Their groups, permissions and VPN profile were never removed,")
			fmt.Println("so the account is back as it was.")
			return nil
		},
	}
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
