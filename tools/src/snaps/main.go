package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/datawire/ambassador-agent/pkg/api/agent"
	snapshotTypes "github.com/emissary-ingress/emissary/v3/pkg/snapshot/v1"

	"github.com/intel/tfortools"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
)

const defaultImage = "docker.io/datawiredev/kat-server:3.0.1-0.20220817135951-2cb28ef4f415"

var rootCmd = &cobra.Command{
	Use:   "snaps",
	Short: "An Ambassador Cloud snapshots dev tool",
}

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "install the agentcom pod and service",
	Args:  cobra.RangeArgs(0, 1),
	RunE: func(cmd *cobra.Command, args []string) error {
		image := defaultImage
		if len(args) == 1 {
			image = args[0]
		}

		app, err := newApp(image)
		if err != nil {
			return err
		}

		return app.ensureAgentCom(cmd.Context())
	},
}

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "uninstall the agentcom pod and service",
	RunE: func(cmd *cobra.Command, args []string) error {
		app, err := newApp("")
		if err != nil {
			return err
		}

		return app.uninstall(cmd.Context())
	},
}

var patchCmd = &cobra.Command{
	Use:   "patch namespace name",
	Short: "point a deployment towards the mock agentcom",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		app, err := newApp("")
		if err != nil {
			return err
		}

		if app.ensureAgentCom(cmd.Context()); err != nil {
			return fmt.Errorf("error ensuring installation: %w", err)
		}

		return app.patchDeployment(cmd.Context(), args[0], args[1])
	},
}

var unpatchCmd = &cobra.Command{
	Use:   "unpatch namespace name",
	Short: "undo pointing a deployment towards the mock agentcom",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		app, err := newApp("")
		if err != nil {
			return err
		}

		return app.unpatchDeployment(cmd.Context(), args[0], args[1])
	},
}

var snapshotCmd = func() *cobra.Command {
	var (
		cmd = cobra.Command{
			Use:   "snapshot",
			Short: "get snapshot from the mock agentcom",
		}

		flags        = cmd.Flags()
		timeout      time.Duration
		fullSnapshot bool
		format       string
	)

	flags.DurationVar(&timeout, "timeout", 10*time.Second, "how long to wait for snapshot")
	flags.BoolVar(&fullSnapshot, "full", false, "show the entire snapshot")
	flags.StringVarP(&format, "format", "f", "", "format output")

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		app, err := newApp("")
		if err != nil {
			return err
		}

		out := cmd.OutOrStdout()

		ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
		defer cancel()

		snapshotBytes, err := app.snapshotBytes(ctx)
		if err != nil {
			return err
		}

		if fullSnapshot {
			if format != "" {
				var snapshot agent.Snapshot
				if err := json.Unmarshal(snapshotBytes, &snapshot); err != nil {
					return fmt.Errorf("unable to unmarshal full snapshot: %w", err)
				}
				tfortools.OutputToTemplate(out, "", format, &snapshot, tfortools.NewConfig(tfortools.OptAllFns))
				return nil
			}
			out.Write(snapshotBytes)
			return nil
		}

		var ss agent.Snapshot
		if err := json.Unmarshal(snapshotBytes, &ss); err != nil {
			return fmt.Errorf("unable to unmarshal full snapshot: %w", err)
		}

		snapshotBytes = ss.RawSnapshot

		if format != "" {
			var snapshot snapshotTypes.Snapshot
			err = json.Unmarshal(snapshotBytes, &snapshot)
			if err != nil {
				return err
			}
			tfortools.OutputToTemplate(out, "", format, &snapshot, tfortools.NewConfig(tfortools.OptAllFns))
			return nil
		}

		cmd.OutOrStdout().Write(snapshotBytes)

		return nil
	}

	return &cmd
}

func main() {
	rootCmd.AddCommand(installCmd)
	rootCmd.AddCommand(uninstallCmd)
	rootCmd.AddCommand(patchCmd)
	rootCmd.AddCommand(unpatchCmd)
	rootCmd.AddCommand(snapshotCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
