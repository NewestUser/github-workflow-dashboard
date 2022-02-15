package formatter

import (
	"fmt"
	"strings"
	"text/template"
	"time"

	"github.com/NewestUser/github-workflow-dashboard/github"
)

var workflowRunHtmlTmpl = template.Must(template.New("workflowTable").Parse(workflowRunHtml))

func ToHTML(runs []*github.WorkflowRun) (string, error) {
	return ToHTMLWithCustomLink(runs, func(r *github.WorkflowRun) string { return "" })
}

func ToHTMLWithCustomLink(runs []*github.WorkflowRun, titleUrlFunc func(*github.WorkflowRun) string) (string, error) {
	tableRows := &strings.Builder{}

	err := workflowRunHtmlTmpl.Execute(tableRows, &multipleWorkflowRunsDataModel{Workflows: adaptMultipleWorkflowModels(runs, titleUrlFunc)})

	if err != nil {
		return "", err
	}

	return tableRows.String(), nil
}

type multipleWorkflowRunsDataModel struct {
	Workflows []*workflowRunModel
}

func adaptMultipleWorkflowModels(runs []*github.WorkflowRun, titleUrlFunc func(*github.WorkflowRun) string) []*workflowRunModel {
	result := make([]*workflowRunModel, len(runs))
	for i, run := range runs {
		result[i] = adaptWorkflowModel(run, titleUrlFunc)
	}
	return result
}

func adaptWorkflowModel(run *github.WorkflowRun, titleUrlFunc func(*github.WorkflowRun) string) *workflowRunModel {
	return &workflowRunModel{
		WorkflowName:     run.WorkflowName,
		WorkflowURL:	  titleUrlFunc(run),
		WorkflowID:       run.WorkflowID,
		JobRunID:         run.JobRunID,
		JobHTMLURL:       run.JobHTMLURL,
		JobLogsURL:       run.JobLogsURL,
		JobRunNumber:     run.JobRunNumber,
		JobConclusion:    run.JobConclusion,
		JobStatus:        run.JobStatus,
		JobEvent:         run.JobEvent,
		JobRunTime:       timeSince(run.JobRunTime),
		JobBranch:        run.JobBranch,
		JobCommitSha:     truncateStr(run.JobCommitSha, 10),
		JobCommitAuthor:  run.JobCommitAuthor,
		JobCommitMessage: run.JobCommitMessage,
		JobCommitTime:    timeSince(run.JobCommitTime),
	}
}

func timeSince(t time.Time) string {
	return fmt.Sprintf("%s ago", time.Since(t).Round(time.Minute))
}

func truncateStr(value interface{}, size int) string {
	v := fmt.Sprintf("%v", value)
	if len(v) < size {
		return v
	}

	return v[0:size]
}

type workflowRunModel struct {
	WorkflowName     string
	WorkflowURL		 string
	WorkflowID       int
	JobRunID         int
	JobHTMLURL       string
	JobLogsURL       string
	JobRunNumber     int
	JobConclusion    string
	JobStatus        string
	JobEvent         string
	JobRunTime       string
	JobBranch        string
	JobCommitSha     string
	JobCommitAuthor  string
	JobCommitMessage string
	JobCommitTime    string
}

const workflowRunHtml = `
<table>
	<thead>
		<tr>
			<th>Workflow</th>
			<th>#</th>
			<th>Status</th>
			<th>Branch</th>
			<th>Author</th>
			<th>Commit Msg</th>
			<th>Commit</th>
			<th>Commit Time</th>
			<th>Run Time</th>
		</tr>
	</thead>
	<tbody>
		{{range .Workflows}}
			<tr>
				{{if eq .WorkflowURL ""}}
					<td>{{.WorkflowName}}</td>
				{{else}}
					<td><a href="{{.WorkflowURL}}">{{.WorkflowName}}</a></td>
				{{end}}
				<td><a href="{{.JobHTMLURL}}">#{{.JobRunNumber}}</a></td>
				<td><b>{{.JobStatus}}</b></td>
				<td><b>{{.JobBranch}}</td>
				<td>{{.JobCommitAuthor}}</td>
				<td>{{.JobCommitMessage}}</td>
				<td>{{.JobCommitSha}}</td>
				<td>{{.JobCommitTime}}</td>
				<td>{{.JobRunTime}}</td>
			</tr>
		{{end}}
	</tbody>
</table>
`
