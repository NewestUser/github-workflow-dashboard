package formatter

import (
	"fmt"
	"strings"

	"github.com/newestuser/github-workflow-dashboard/github"
	"github.com/olekukonko/tablewriter"
)

func ToAscii(runs []*github.WorkflowRun) (string, error) {

	output := &strings.Builder{}
	table := tablewriter.NewWriter(output)

	table.SetHeader([]string{"workflow", "#", "status", "branch", "author", "commit message", "commit", "commit timestamp", "run timestamp"})
	table.SetBorders(tablewriter.Border{Left: true, Top: false, Right: true, Bottom: false})
	table.SetCenterSeparator("|")

	for _, worfklowRun := range runs {
		row := mapAsciiRow(worfklowRun)
		table.Append(row)
	}
	table.Render()

	return output.String(), nil
}

func mapAsciiRow(run *github.WorkflowRun) []string {

	var commitSha = run.JobCommitSha
	if len(run.JobCommitSha) > 10 {
		commitSha = run.JobCommitSha[0:10]
	}

	return []string{
		run.WorkflowName,
		fmt.Sprintf("%d", run.JobRunNumber),
		run.JobStatus,
		run.JobBranch,
		run.JobCommitAuthor,
		run.JobCommitMessage,
		commitSha,
		run.JobCommitTime.UTC().String(),
		run.JobRunTime.UTC().String()}
}
