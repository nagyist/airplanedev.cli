package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/cli/pkg/utils/pointers"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/pkg/errors"
)

// TaskInfo bundles together information about a task from the ECS
// and CW APIs.
type TaskInfo struct {
	ID               string
	Group            string
	CreatedAt        time.Time
	DesiredStatus    string
	CurrentStatus    string
	TaskDefintionARN string

	// These fields are only filled in for a subset of tasks
	TaskDefinition *types.TaskDefinition
	Logs           []ContainerLog
}

// ContainerLog stores a single cloudwatch log line.
type ContainerLog struct {
	LogGroup  string
	LogStream string
	RawLog    string
	ParsedLog map[string]interface{}
	Timestamp int64
}

// IsForAgentService returns whether the given TaskInfo is for the Airplane agent
// service (as opposed to a task runbroker).
func (t *TaskInfo) IsForAgentService() bool {
	return strings.HasPrefix(t.Group, "service:")
}

// GetTaskRevision gets the task revision name from a TaskInfo by looking at the
// stream log prefix on the runbroker container (if present).
func (t *TaskInfo) GetTaskRevision() string {
	for _, container := range t.TaskDefinition.ContainerDefinitions {
		if pointers.ToString(container.Name) == "runbroker" {
			// The stream prefix will be something like runbroker-stdapi-2774733778221563910
			// or runbroker-trv20220701z88kvec9xjc-17830583929927356542
			logStreamPrefix := container.LogConfiguration.Options["awslogs-stream-prefix"]
			streamPrefixComponents := strings.Split(logStreamPrefix, "-")

			if len(streamPrefixComponents) >= 2 {
				return streamPrefixComponents[1]
			}
		}
	}

	return ""
}

// PopulateLogs populates the Logs fields in a TaskInfo. The logs are expensive to
// fetch, so this should only be done for tasks where we require this info.
func (t *TaskInfo) PopulateLogs(
	ctx context.Context,
	ecsClient *ecs.Client,
	cwClient *cloudwatchlogs.Client,
	maxLogAge time.Duration,
	maxLogLines int,
) error {
	now := time.Now().UTC()
	var err error

	for _, container := range t.TaskDefinition.ContainerDefinitions {
		logGroup := container.LogConfiguration.Options["awslogs-group"]
		logStreamPrefix := container.LogConfiguration.Options["awslogs-stream-prefix"]

		if logGroup == "" || logStreamPrefix == "" {
			logger.Warning(
				"Log group and/or stream missing for container %s in task %s",
				container.Name,
				t.ID,
			)
			continue
		}

		logStream := fmt.Sprintf(
			"%s/%s/%s",
			logStreamPrefix,
			pointers.ToString(container.Name),
			t.ID,
		)
		t.Logs, err = getLogs(
			ctx,
			cwClient,
			logGroup,
			logStream,
			now.Add(-maxLogAge),
			maxLogLines,
		)
		if err != nil {
			return err
		}
	}

	return nil
}

// GetTaskInfosOptions stores options for the GetTaskInfos function call.
type GetTaskInfosOptions struct {
	ClusterName      string
	TaskFilterRegexp *regexp.Regexp
	MaxLogAge        time.Duration
	MaxLogLines      int
}

// GetTaskInfos gets information about all Airplane-related tasks in the
// argument cluster, including recent CloudWatch logs for Airplane agent
// service tasks.
func GetTaskInfos(
	ctx context.Context,
	ecsClient *ecs.Client,
	cwClient *cloudwatchlogs.Client,
	opts GetTaskInfosOptions,
) ([]TaskInfo, error) {
	taskARNs := []string{}

	logger.Log("Getting all tasks in ECS cluster")
	tasksPaginator := ecs.NewListTasksPaginator(
		ecsClient,
		&ecs.ListTasksInput{
			Cluster: pointers.String(opts.ClusterName),
		},
	)

	for tasksPaginator.HasMorePages() {
		output, err := tasksPaginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		taskARNs = append(taskARNs, output.TaskArns...)
	}

	taskARNChunks := chunkSlice(taskARNs, 100)
	taskInfos := []TaskInfo{}

	logger.Log("Describing all tasks in cluster")
	for _, taskARNChunk := range taskARNChunks {
		describeTasksResp, err := ecsClient.DescribeTasks(
			ctx,
			&ecs.DescribeTasksInput{
				Tasks:   taskARNChunk,
				Cluster: pointers.String(opts.ClusterName),
			},
		)
		if err != nil {
			return nil, err
		}
		for _, task := range describeTasksResp.Tasks {
			taskARNComponents := strings.Split(pointers.ToString(task.TaskArn), "/")

			var taskID string

			if len(taskARNComponents) > 0 {
				taskID = taskARNComponents[len(taskARNComponents)-1]
			}

			taskGroup := pointers.ToString(task.Group)

			// Only collect data for airplane-related tasks
			if opts.TaskFilterRegexp.MatchString(taskGroup) {
				taskDefResp, err := ecsClient.DescribeTaskDefinition(
					ctx,
					&ecs.DescribeTaskDefinitionInput{
						TaskDefinition: task.TaskDefinitionArn,
					},
				)
				if err != nil {
					return nil, errors.Wrap(err, "describing task definition")
				}

				taskInfos = append(
					taskInfos,
					TaskInfo{
						ID:               taskID,
						Group:            taskGroup,
						CreatedAt:        pointers.ToTime(task.CreatedAt),
						DesiredStatus:    pointers.ToString(task.DesiredStatus),
						CurrentStatus:    pointers.ToString(task.LastStatus),
						TaskDefintionARN: pointers.ToString(task.TaskDefinitionArn),
						TaskDefinition:   taskDefResp.TaskDefinition,
					},
				)
			}
		}
	}

	logger.Log("Found %d Airplane-related tasks in cluster", len(taskInfos))

	for i := 0; i < len(taskInfos); i++ {
		taskInfos[i].GetTaskRevision()

		if taskInfos[i].IsForAgentService() {
			logger.Log("Populating logs for Airplane service task %s", taskInfos[i].ID)
			if err := taskInfos[i].PopulateLogs(
				ctx,
				ecsClient,
				cwClient,
				opts.MaxLogAge,
				opts.MaxLogLines,
			); err != nil {
				return nil, err
			}
		}
	}

	// Sort by group and then createdAt
	sort.Slice(taskInfos, func(a, b int) bool {
		ti1 := taskInfos[a]
		ti2 := taskInfos[b]

		return ti1.Group < ti2.Group ||
			(ti1.Group == ti2.Group && ti1.CreatedAt.Before(ti2.CreatedAt))
	})

	return taskInfos, nil
}

// chunkSlice divides a slice into chunks of a maximum size.
// Adapted from example in
// https://freshman.tech/snippets/go/split-slice-into-chunks/.
func chunkSlice(slice []string, maxChunkSize int) [][]string {
	var chunks [][]string
	for i := 0; i < len(slice); i += maxChunkSize {
		end := i + maxChunkSize

		if end > len(slice) {
			end = len(slice)
		}

		chunks = append(chunks, slice[i:end])
	}

	return chunks
}

// getLogs gets all Cloudwatch logs for a single group and stream starting
// from a particular start time.
func getLogs(
	ctx context.Context,
	cwClient *cloudwatchlogs.Client,
	logGroup string,
	logStream string,
	startTime time.Time,
	maxLines int,
) ([]ContainerLog, error) {
	logger.Log("Getting logs for group %s, stream %s", logGroup, logStream)

	nextToken := ""
	containerLogs := []ContainerLog{}

	for {
		logEventsResp, err := cwClient.GetLogEvents(
			ctx,
			&cloudwatchlogs.GetLogEventsInput{
				StartFromHead: pointers.Bool(false),
				LogStreamName: pointers.String(logStream),
				LogGroupName:  pointers.String(logGroup),
				NextToken:     pointers.String(nextToken),
				StartTime:     pointers.Int64(startTime.UnixMilli()),
			},
		)
		if err != nil {
			return nil, errors.Wrap(err, "getting cloudwatch log events")
		}

		for _, event := range logEventsResp.Events {
			cl := ContainerLog{
				LogGroup:  logGroup,
				LogStream: logStream,
				RawLog:    pointers.ToString(event.Message),
				ParsedLog: map[string]interface{}{},
				Timestamp: pointers.ToInt64(event.Timestamp),
			}

			err := json.Unmarshal([]byte(cl.RawLog), &cl.ParsedLog)
			if err != nil {
				// Just swallow the error
				logger.Log("Message is not valid JSON: %s", cl.RawLog)
			}

			containerLogs = append(containerLogs, cl)
		}

		if len(containerLogs) >= maxLines {
			break
		}

		respNextToken := pointers.ToString(logEventsResp.NextBackwardToken)

		if respNextToken == nextToken {
			break
		}
		nextToken = respNextToken
	}

	sort.Slice(containerLogs, func(a, b int) bool {
		return containerLogs[a].Timestamp < containerLogs[b].Timestamp
	})

	if len(containerLogs) > maxLines {
		containerLogs = containerLogs[len(containerLogs)-1-maxLines : len(containerLogs)-1]
	}

	return containerLogs, nil
}
