package formatter

import (
	"fmt"
	"sort"
	"strings"

	"github.com/newestuser/github-workflow-dashboard/github"
	"github.com/olekukonko/tablewriter"
)

func ToAscii(runs []*github.WorkflowRun) (string, error) {
	output := &strings.Builder{}
	table := tablewriter.NewWriter(output)

	header := []string{"workflow", "#", "status", "branch", "commiter", "commit msg", "commit", "commit time", "run time"}

	if containsParams(runs) {
		header = append(header, "params")
	}

	table.SetHeader(header)
	table.SetBorders(tablewriter.Border{Left: true, Top: true, Right: true, Bottom: false})
	table.SetCenterSeparator("|")

	for _, worfklowRun := range runs {
		row := mapAsciiRow(worfklowRun, containsParams(runs))
		table.Append(row)
	}
	table.Render()

	return output.String(), nil
}

func mapAsciiRow(run *github.WorkflowRun, includeParams bool) []string {

	var commitSha = run.JobCommitSha
	if len(run.JobCommitSha) > 10 {
		commitSha = run.JobCommitSha[0:10]
	}

	asciRow := []string{
		run.WorkflowName,
		fmt.Sprintf("%d", run.JobRunNumber),
		run.JobStatus,
		run.JobBranch,
		run.JobCommitAuthor,
		run.JobCommitMessage,
		commitSha,
		run.JobCommitTime.UTC().String(),
		run.JobRunTime.UTC().String()}

	if includeParams {
		asciRow = append(asciRow, mapAsciiParams(run.WorkflowParams.Params))
	}

	return asciRow
}

func containsParams(runs []*github.WorkflowRun) bool {
	for _, run := range runs {
		if run.WorkflowParams != nil {
			return true
		}
	}

	return false
}

func mapAsciiParams(params []github.JobRunParams) string {
	paramSet := map[string]string{}

	for _, p := range params {
		for k, v := range p {
			paramSet[k] = v
		}
	}

	sortedParams := make([]string, 0)
	for k, v := range paramSet {
		sortedParams = append(sortedParams, fmt.Sprintf("%s: %s", k, v))
	}

	sort.Slice(sortedParams, func(i, j int) bool {
		return sortedParams[i] > sortedParams[j]
	})

	str := strings.Builder{}
	for _, p := range sortedParams {
		str.WriteString(p)
		str.WriteString("\n")
	}
	
	return str.String()
}
