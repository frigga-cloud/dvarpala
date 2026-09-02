package main

import (
	"fmt"
	"os"
	"text/tabwriter"

	"dvarpala/internal/services"

	"github.com/spf13/cobra"
)

func resourceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resource",
		Short: "Protected resource commands",
	}
	cmd.AddCommand(resourceCreateCmd(), resourceListCmd())
	return cmd
}

func resourceCreateCmd() *cobra.Command {
	var name, kind, url, ip, description string
	var port int

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Register a protected resource",
		Example: "  dvarpala-cli resource create --name grafana --type dashboard --ip 10.0.5.20 --port 3000\n" +
			"  dvarpala-cli resource create --name prod-db --type database --ip 10.0.5.44 --port 5432",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := openServices()
			if err != nil {
				return err
			}

			r, err := svc.Resources.CreateResource(cmd.Context(), services.CreateResourceRequest{
				Name: name, Type: kind, URL: url, IPAddress: ip,
				Port: port, Description: description,
			})
			if err != nil {
				return err
			}

			fmt.Printf("Created resource %s (%s at %s)\n", r.Name, r.Type, services.Address(*r))
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Resource name (required)")
	cmd.Flags().StringVar(&kind, "type", "", "dashboard | vm | database | service (required)")
	cmd.Flags().StringVar(&url, "url", "", "URL, for dashboards and services")
	cmd.Flags().StringVar(&ip, "ip", "", "IP address, or a range like 10.20.0.0/16; required for vm and database")
	cmd.Flags().IntVar(&port, "port", 0, "Port")
	cmd.Flags().StringVar(&description, "description", "", "What this resource is")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("type")

	return cmd
}

func resourceListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List protected resources",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := openServices()
			if err != nil {
				return err
			}

			list, err := svc.Resources.ListResources(cmd.Context())
			if err != nil {
				return err
			}
			if len(list) == 0 {
				fmt.Println("No resources.")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
			fmt.Fprintln(w, "ID\tNAME\tTYPE\tADDRESS\tDESCRIPTION")
			for _, r := range list {
				fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\n",
					r.ID, r.Name, r.Type, services.Address(r), dash(r.Description))
			}
			w.Flush()

			fmt.Printf("\n%d resource(s)\n", len(list))
			return nil
		},
	}
}
