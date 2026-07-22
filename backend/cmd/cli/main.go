package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
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
	return service.New(pool, sto, q), func() { pool.Close() }, nil
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

// --- chapter ----------------------------------------------------------------

func chapterCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "chapter", Short: "chapter operations"}
	cmd.AddCommand(chapterSplitCmd(), chapterIngestCmd(), chapterListCmd(), chapterRenameCmd())
	return cmd
}

func chapterSplitCmd() *cobra.Command {
	return &cobra.Command{
		Use: "split <project_id>", Short: "enqueue ai:split_chapters", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
			defer cancel()
			svcs, cleanup, err := withServices(ctx)
			if err != nil {
				return err
			}
			defer cleanup()
			jobID, err := svcs.Chapter.TriggerSplit(ctx, args[0])
			if err != nil {
				return err
			}
			fmt.Fprintln(os.Stdout, "queued", jobID)
			return nil
		},
	}
}

func chapterIngestCmd() *cobra.Command {
	return &cobra.Command{
		Use: "ingest <project_id>", Short: "pull split result from MinIO and upsert chapters", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 60*time.Second)
			defer cancel()
			svcs, cleanup, err := withServices(ctx)
			if err != nil {
				return err
			}
			defer cleanup()
			n, err := svcs.Chapter.IngestSplitResult(ctx, args[0])
			if err != nil {
				return err
			}
			fmt.Fprintln(os.Stdout, "ingested", n)
			return nil
		},
	}
}

func chapterListCmd() *cobra.Command {
	return &cobra.Command{
		Use: "list <project_id>", Short: "list chapters", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
			defer cancel()
			svcs, cleanup, err := withServices(ctx)
			if err != nil {
				return err
			}
			defer cleanup()
			items, err := svcs.Chapter.List(ctx, args[0])
			if err != nil {
				return err
			}
			return json.NewEncoder(os.Stdout).Encode(items)
		},
	}
}

func chapterRenameCmd() *cobra.Command {
	return &cobra.Command{
		Use: "rename <chapter_id> <new_title>", Short: "rename a chapter", Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
			defer cancel()
			svcs, cleanup, err := withServices(ctx)
			if err != nil {
				return err
			}
			defer cleanup()
			title := args[1]
			c, err := svcs.Chapter.Patch(ctx, args[0], &title, nil)
			if err != nil {
				return err
			}
			return json.NewEncoder(os.Stdout).Encode(c)
		},
	}
}

// --- placeholders / migrate -------------------------------------------------

func pipelineCmd() *cobra.Command  { return &cobra.Command{Use: "pipeline", Short: "trigger / inspect pipelines"} }
func characterCmd() *cobra.Command { return &cobra.Command{Use: "character", Short: "character operations"} }
func mediaCmd() *cobra.Command     { return &cobra.Command{Use: "media", Short: "media utilities"} }
func devCmd() *cobra.Command      { return &cobra.Command{Use: "dev", Short: "developer helpers"} }

func migrateCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "migrate", Short: "run SQL migrations"}
	cmd.AddCommand(migrateUpCmd())
	return cmd
}

func migrateUpCmd() *cobra.Command {
	var dir string
	c := &cobra.Command{
		Use: "up", Short: "apply pending SQL migrations in lexical order",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if dir == "" {
				dir = "./migrations"
			}
			if _, err := os.Stat(dir); err != nil {
				return fmt.Errorf("migrations dir not found: %w", err)
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 60*time.Second)
			defer cancel()
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			pool, err := db.NewPool(ctx, cfg.DBURL)
			if err != nil {
				return err
			}
			defer pool.Close()
			if _, err := pool.Exec(ctx, `
				CREATE TABLE IF NOT EXISTS schema_migrations (
					id text PRIMARY KEY,
					applied_at timestamptz NOT NULL DEFAULT now()
				)`); err != nil {
				return err
			}
			var files []string
			err = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return err
				}
				if filepath.Ext(p) != ".sql" {
					return nil
				}
				files = append(files, p)
				return nil
			})
			if err != nil {
				return err
			}
			sort.Strings(files)
			for _, f := range files {
				id := filepath.Base(f)
				var exists bool
				if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE id=$1)`, id).Scan(&exists); err != nil {
					return err
				}
				if exists {
					fmt.Fprintln(os.Stdout, "skip", id)
					continue
				}
				body, err := os.ReadFile(f)
				if err != nil {
					return err
				}
				if _, err := pool.Exec(ctx, string(body)); err != nil {
					return fmt.Errorf("%s: %w", id, err)
				}
				if _, err := pool.Exec(ctx, `INSERT INTO schema_migrations(id) VALUES ($1)`, id); err != nil {
					return err
				}
				fmt.Fprintln(os.Stdout, "applied", id)
			}
			return nil
		},
	}
	c.Flags().StringVar(&dir, "dir", "./migrations", "migrations directory")
	return c
}

var _ = domain.ProjectCreated
