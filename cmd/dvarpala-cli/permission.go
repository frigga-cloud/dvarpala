package main

import (
	"fmt"
	"os"
	"text/tabwriter"

	"dvarpala/internal/services"

	"github.com/spf13/cobra"
)

func permissionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "permission",
		Aliases: []string{"perm"},
		Short:   "Grant and revoke group access to resources",
	}
	cmd.AddCommand(permissionGrantCmd(), permissionRevokeCmd(), permissionListCmd())
	return cmd
}

func permissionGrantCmd() *cobra.Command {
	var group, resource, permission string

	cmd := &cobra.Command{
		Use:     "grant",
		Short:   "Give a group access to a resource",
		Example: "  dvarpala-cli permission grant --group engineering --resource grafana --type read",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := openServices()
			if err != nil {
				return err
			}
			if err := svc.Permissions.Grant(cmd.Context(), group, resource, permission); err != nil {
				return err
			}
			fmt.Printf("Granted %s on %s to %s\n", permission, resource, group)
			return nil
		},
	}

	cmd.Flags().StringVar(&group, "group", "", "Group name (required)")
	cmd.Flags().StringVar(&resource, "resource", "", "Resource name (required)")
	cmd.Flags().StringVar(&permission, "type", "read", "read | write | admin | ssh | full")
	_ = cmd.MarkFlagRequired("group")
	_ = cmd.MarkFlagRequired("resource")

	return cmd
}

func permissionRevokeCmd() *cobra.Command {
	var group, resource, permission string

	cmd := &cobra.Command{
		Use:   "revoke",
		Short: "Remove a group's access to a resource",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := openServices()
			if err != nil {
				return err
			}
			if err := svc.Permissions.Revoke(cmd.Context(), group, resource, permission); err != nil {
				return err
			}
			fmt.Printf("Revoked %s on %s from %s\n", permission, resource, group)
			return nil
		},
	}

	cmd.Flags().StringVar(&group, "group", "", "Group name (required)")
	cmd.Flags().StringVar(&resource, "resource", "", "Resource name (required)")
	cmd.Flags().StringVar(&permission, "type", "read", "read | write | admin | ssh | full")
	_ = cmd.MarkFlagRequired("group")
	_ = cmd.MarkFlagRequired("resource")

	return cmd
}

func permissionListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list <group>",
		Short: "Show what a group may reach",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := openServices()
			if err != nil {
				return err
			}

			grants, err := svc.Permissions.ListForGroup(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if len(grants) == 0 {
				fmt.Printf("Group %s has no permissions.\n", args[0])
				return nil
			}

			printGrants(grants, false)
			return nil
		},
	}
}

// userAccessCmd answers the question the VPN will ask after authentication.
func userAccessCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "access <email>",
		Short: "Show every resource a user may reach, and via which group",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := openServices()
			if err != nil {
				return err
			}

			// Authorisation first: an inactive user reaches nothing.
			if _, err := svc.Users.IsAuthorised(cmd.Context(), args[0]); err != nil {
				fmt.Printf("VPN access DENIED: %v\n", err)
				return nil
			}

			grants, err := svc.Permissions.ResourcesForUser(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if len(grants) == 0 {
				fmt.Printf("%s is authorised but belongs to no group with permissions.\n", args[0])
				fmt.Println("They would reach nothing beyond the captive portal.")
				return nil
			}

			fmt.Printf("%s may reach:\n\n", args[0])
			printGrants(grants, true)
			return nil
		},
	}
}

func printGrants(grants []services.AccessGrant, showGroup bool) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	if showGroup {
		fmt.Fprintln(w, "RESOURCE\tTYPE\tADDRESS\tPERMISSION\tVIA GROUP")
	} else {
		fmt.Fprintln(w, "RESOURCE\tTYPE\tADDRESS\tPERMISSION")
	}

	for _, g := range grants {
		if showGroup {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
				g.Resource.Name, g.Resource.Type, services.Address(g.Resource),
				g.Permission, g.ViaGroup)
		} else {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
				g.Resource.Name, g.Resource.Type, services.Address(g.Resource), g.Permission)
		}
	}
	w.Flush()

	fmt.Printf("\n%d grant(s)\n", len(grants))
}
