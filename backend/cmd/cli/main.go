// Package main is the Cobra CLI entrypoint of novel2av-backend.
//
// It shares the same domain + service layer as the HTTP API.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/novel2av/backend/internal/config"
	"github.com/novel2av/backend/internal/domain"
	"github.com/novel2av/backend/internal/infra/db"
	"github.com/novel2av/backend/internal/infra/observability"
	"github.com/novel2av/backend/internal/infra/queue"
	"github.com/novel2av/backend/internal/infra/storage"
	"github.com/novel2av/backend/internal/service"
)

func main() {
	root := &cobra.Command{Use: "novel2av", Short: "novel2av backend CLI"}
	root.AddCommand(projectCmd(), pipelineCmd(), chapterCmd(), characterCmd(), mediaCmd(), migrateCmd(), devCmd())

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// withServices builds the service graph from env config.
func withServices(ctx context.Context) (*service.Services, func(), error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, nil, err
	}
	observability.Setup(cfg.LogLevel)
	pool, err := db.NewPool(ctx, cfg.DBURL)
	if err != nil {
		return nil, nil, fmt.Errorf("db: %w", err)
	}
	sto, err := storage.NewMinIO(ctx, cfg.S3)
	if err != nil {
		pool.Close()
		return nil, nil, fmt.Errorf("minio: %w", err)
	}
	q, err := queue.NewAsynqClient(cfg.RedisAIURL, cfg.RedisURL)
	if err != nil {
		pool.Close()
		return nil, nil, fmt.Errorf("queue: %w", err)
	}
	return service.New(pool, sto, q), func() {
		pool.Close()
	}, nil
}

// --- project ----------------------------------------------------------------

func projectCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "project", Short: "manage projects"}
	cmd.AddCommand(projectListCmd(), projectShowCmd(), projectDeleteCmd())
	return cmd
}

func projectListCmd() *cobra.Command {
	var limit, offset int
	c := &cobra.Command{
		Use: "list", Short: "list projects",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
			defer cancel()
			svcs, cleanup, err := withServices(ctx)
			if err != nil {
				return err
			}
			defer cleanup()
			items, err := svcs.Project.List(ctx, "00000000-0000-0000-0000-000000000001", limit, offset)
			if err != nil {
				return err
			}
			return json.NewEncoder(os.Stdout).Encode(items)
		},
	}
	c.Flags().IntVar(&limit, "limit", 20, "page size")
	c.Flags().IntVar(&offset, "offset", 0, "page offset")
	return c
}

func projectShowCmd() *cobra.Command {
	return &cobra.Command{
		Use: "show <id>", Short: "show project", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
			defer cancel()
			svcs, cleanup, err := withServices(ctx)
			if err != nil {
				return err
			}
			defer cleanup()
			p, err := svcs.Project.Get(ctx, args[0])
			if err != nil {
				return err
			}
			return json.NewEncoder(os.Stdout).Encode(p)
		},
	}
}

func projectDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use: "delete <id>", Short: "delete project", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			svcs, cleanup, err := withServices(ctx)
			if err != nil {
				return err
			}
			defer cleanup()
			if err := svcs.Project.Delete(ctx, args[0]); err != nil {
				return err
			}
			fmt.Fprintln(os.Stdout, "deleted", args[0])
			return nil
		},
	}
}

// --- placeholders so the binary still builds ---------------------------------

func pipelineCmd() *cobra.Command  { return &cobra.Command{Use: "pipeline", Short: "trigger / inspect pipelines"} }
func chapterCmd() *cobra.Command   { return &cobra.Command{Use: "chapter", Short: "chapter operations"} }
func characterCmd() *cobra.Command { return &cobra.Command{Use: "character", Short: "character operations"} }
func mediaCmd() *cobra.Command     { return &cobra.Command{Use: "media", Short: "media utilities"} }
func migrateCmd() *cobra.Command {
	return &cobra.Command{Use: "migrate", Short: "run SQL migrations"}
}
func devCmd() *cobra.Command { return &cobra.Command{Use: "dev", Short: "developer helpers"} }

var _ = domain.ProjectCreated // keep import used while other commands land
