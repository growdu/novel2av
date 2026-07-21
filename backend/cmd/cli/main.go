// Package main is the Cobra CLI entrypoint of novel2av-backend.
//
// It shares the same domain + service layer as the HTTP API, so a CLI
// command and the corresponding REST endpoint never diverge.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	root := &cobra.Command{
		Use:   "novel2av",
		Short: "novel2av backend CLI",
		Long:  "Admin / batch / debug CLI for the novel2av platform backend (Go).",
	}
	root.AddCommand(projectCmd(), pipelineCmd(), chapterCmd(), characterCmd(), mediaCmd(), migrateCmd(), devCmd())

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// ---- placeholder command trees ----------------------------------------------
// Concrete subcommands will be implemented alongside their service counterparts.

func projectCmd() *cobra.Command     { return &cobra.Command{Use: "project", Short: "manage projects"} }
func pipelineCmd() *cobra.Command   { return &cobra.Command{Use: "pipeline", Short: "trigger / inspect pipelines"} }
func chapterCmd() *cobra.Command    { return &cobra.Command{Use: "chapter", Short: "chapter operations"} }
func characterCmd() *cobra.Command  { return &cobra.Command{Use: "character", Short: "character operations"} }
func mediaCmd() *cobra.Command      { return &cobra.Command{Use: "media", Short: "media utilities"} }
func migrateCmd() *cobra.Command    { return &cobra.Command{Use: "migrate", Short: "run SQL migrations"} }
func devCmd() *cobra.Command        { return &cobra.Command{Use: "dev", Short: "developer helpers (seed, token-cost, ...)"} }
