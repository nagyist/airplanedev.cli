package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/airplanedev/cli/pkg/logger"
	"github.com/olekukonko/tablewriter"
)

// PrintTaskInfos prints out a summary of the argument TaskInfo structs to stdout.
func PrintTaskInfos(ctx context.Context, taskInfos []TaskInfo, includeTaskDefs bool) {
	fmt.Println("Airplane-related tasks:")

	tw := tablewriter.NewWriter(os.Stdout)
	tw.SetBorder(false)
	tw.SetHeader([]string{"id", "group", "created at", "task revision", "status"})
	tw.SetAutoWrapText(false)

	for _, ti := range taskInfos {
		tw.Append([]string{
			ti.ID,
			ti.Group,
			ti.CreatedAt.Format(time.RFC3339),
			ti.GetTaskRevision(),
			ti.CurrentStatus,
		})
	}

	tw.Render()

	fmt.Println("")

	for _, ti := range taskInfos {
		if ti.IsForAgentService() {
			if includeTaskDefs {
				taskDefinitionJSON, err := json.MarshalIndent(ti.TaskDefinition, "", "  ")
				if err != nil {
					logger.Error("Error marshalling task definition: %+v", err)
				} else {
					fmt.Printf("Task definition for task %s:\n", ti.ID)
					fmt.Println(string(taskDefinitionJSON))
					fmt.Println("")
				}
			}

			fmt.Printf("Logs for task %s:\n", ti.ID)

			for _, log := range ti.Logs {
				if len(log.ParsedLog) > 0 {
					// Make time stamp prettier by truncating the nanoseconds
					var prettyTimeStampStr string
					timeStampStr, ok := log.ParsedLog["time"].(string)

					if ok {
						timeStamp, err := time.Parse(time.RFC3339Nano, timeStampStr)
						if err != nil {
							prettyTimeStampStr = timeStampStr
						} else {
							prettyTimeStampStr = timeStamp.Format(time.RFC3339)
						}
					} else {
						prettyTimeStampStr = timeStampStr
					}

					fmt.Printf(
						"%s [%s] %s\n",
						prettyTimeStampStr,
						log.ParsedLog["severity"],
						log.ParsedLog["message"],
					)
				} else {
					fmt.Println(log.RawLog)
				}
			}

			fmt.Println("")
		}
	}
}
