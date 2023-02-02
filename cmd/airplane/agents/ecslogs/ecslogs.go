package ecslogs

import (
	"context"
	"log"
	"regexp"
	"time"

	"github.com/airplanedev/cli/pkg/agents"
	"github.com/airplanedev/cli/pkg/cli"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type config struct {
	root *cli.Config

	ecsClusterName      string
	logTaskDefs         bool
	maxLogsAge          time.Duration
	region              string
	taskFilterRegexpStr string
}

// New returns a new ecslogs command.
func New(c *cli.Config) *cobra.Command {
	var cfg = config{
		root: c,
	}

	cmd := &cobra.Command{
		Use:   "ecslogs",
		Short: "Gets logs for self-hosted agents running in ECS",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cmd.Root().Context(), cfg)
		},
	}
	cmd.Flags().StringVar(
		&cfg.ecsClusterName,
		"cluster-name",
		"",
		"Name of ECS cluster containing agent",
	)
	cmd.Flags().BoolVar(
		&cfg.logTaskDefs,
		"log-task-defs",
		false,
		"Log out task definitions for Airplane agent service tasks",
	)
	cmd.Flags().DurationVar(
		&cfg.maxLogsAge,
		"max-logs-age",
		6*time.Hour,
		"Number of hours back to start log fetching",
	)
	cmd.Flags().StringVar(
		&cfg.region,
		"region",
		"",
		"AWS region containing agent; defaults to current region if not set",
	)
	cmd.Flags().StringVar(
		&cfg.taskFilterRegexpStr,
		"task-filter",
		"[:]airplane",
		"Regex to use for filtering out Airplane-related tasks (applied to group name)",
	)

	if err := cmd.MarkFlagRequired("cluster-name"); err != nil {
		log.Fatal(err)
	}

	return cmd
}

// Run runs the ecslogs command.
func run(ctx context.Context, cfg config) error {
	var awsConfig aws.Config
	var err error

	if cfg.region != "" {
		awsConfig, err = awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(cfg.region))
	} else {
		awsConfig, err = awsconfig.LoadDefaultConfig(ctx)
	}
	if err != nil {
		return errors.Wrap(err, "loading default AWS config")
	}

	ecsClient := ecs.NewFromConfig(awsConfig)
	cwClient := cloudwatchlogs.NewFromConfig(awsConfig)

	taskFilterRegexp, err := regexp.Compile(cfg.taskFilterRegexpStr)
	if err != nil {
		return errors.Wrap(err, "compiling task filter regexp")
	}

	taskInfos, err := agents.GetTaskInfos(
		ctx,
		ecsClient,
		cwClient,
		agents.GetTaskInfosOptions{
			ClusterName:      cfg.ecsClusterName,
			MaxLogsAge:       cfg.maxLogsAge,
			TaskFilterRegexp: taskFilterRegexp,
		},
	)
	if err != nil {
		return err
	}

	agents.PrintTaskInfos(ctx, taskInfos, cfg.logTaskDefs)
	return nil
}
