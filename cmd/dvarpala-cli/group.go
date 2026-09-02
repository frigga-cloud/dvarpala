package main

import (
	"fmt"
	"os"
	"text/tabwriter"

	"dvarpala/internal/services"

	"github.com/spf13/cobra"
)

func groupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "group",
		Short: "Group management commands",
	}

	cmd.AddCommand(
		groupCreateCmd(),
		groupListCmd(),
		groupShowCmd(),
		groupAssignCmd(),
		groupRemoveCmd(),
	)
	return cmd
}

func groupCreateCmd() *cobra.Command {
	var name, description, parent string

	cmd := &cobra.Command{
		Use:     "create",
		Short:   "Create a group",
		Example: "  dvarpala-cli group create --name engineering --description \"Engineering team\"",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := openServices()
			if err != nil {
				return err
			}

			group, err := svc.Groups.CreateGroup(cmd.Context(), services.CreateGroupRequest{
				Name:        name,
				Description: description,
				Parent:      parent,
			})
			if err != nil {
				return err
			}

			fmt.Printf("Created group %s (id %d)\n", group.Name, group.ID)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Group name (required)")
	cmd.Flags().StringVar(&description, "description", "", "What the group is for")
	cmd.Flags().StringVar(&parent, "parent", "", "Parent group name, for hierarchy")
	_ = cmd.MarkFlagRequired("name")

	return cmd
}

func groupListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all groups",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := openServices()
			if err != nil {
				return err
			}

			groups, err := svc.Groups.ListGroups(cmd.Context())
			if err != nil {
				return err
			}
			if len(groups) == 0 {
				fmt.Println("No groups.")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
			fmt.Fprintln(w, "ID\tNAME\tMEMBERS\tDESCRIPTION")
			for _, g := range groups {
				fmt.Fprintf(w, "%d\t%s\t%d\t%s\n",
					g.ID, g.Name, len(g.Users), dash(g.Description))
			}
			w.Flush()

			fmt.Printf("\n%d group(s)\n", len(groups))
			return nil
		},
	}
}

func groupShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <name>",
		Short: "Show a group and its members",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := openServices()
			if err != nil {
				return err
			}

			group, err := svc.Groups.GetGroupByName(cmd.Context(), args[0])
			if err != nil {
				return err
			}

			fmt.Printf("Group:       %s\n", group.Name)
			fmt.Printf("Description: %s\n", dash(group.Description))
			fmt.Printf("Members:     %d\n", len(group.Users))
			for _, u := range group.Users {
				fmt.Printf("  - %s (%s)\n", u.Email, u.Status)
			}
			return nil
		},
	}
}

func groupAssignCmd() *cobra.Command {
	var email, group string

	cmd := &cobra.Command{
		Use:     "assign",
		Short:   "Add a user to a group",
		Example: "  dvarpala-cli group assign --user sam@acme.com --group engineering",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := openServices()
			if err != nil {
				return err
			}

			if err := svc.Groups.AddUserToGroup(cmd.Context(), email, group); err != nil {
				return err
			}

			fmt.Printf("Added %s to %s\n", email, group)
			return nil
		},
	}

	cmd.Flags().StringVar(&email, "user", "", "User email (required)")
	cmd.Flags().StringVar(&group, "group", "", "Group name (required)")
	_ = cmd.MarkFlagRequired("user")
	_ = cmd.MarkFlagRequired("group")

	return cmd
}

func groupRemoveCmd() *cobra.Command {
	var email, group string

	cmd := &cobra.Command{
		Use:   "remove",
		Short: "Remove a user from a group",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := openServices()
			if err != nil {
				return err
			}

			if err := svc.Groups.RemoveUserFromGroup(cmd.Context(), email, group); err != nil {
				return err
			}

			fmt.Printf("Removed %s from %s\n", email, group)
			return nil
		},
	}

	cmd.Flags().StringVar(&email, "user", "", "User email (required)")
	cmd.Flags().StringVar(&group, "group", "", "Group name (required)")
	_ = cmd.MarkFlagRequired("user")
	_ = cmd.MarkFlagRequired("group")

	return cmd
}
