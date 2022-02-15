package github

//////////////////////////////////////////////////////////////////////////////////
// Inspired by https://gist.github.com/bauergeorg/856de20e2a99f930b384291d36b189a5
//////////////////////////////////////////////////////////////////////////////////

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"time"

	g "github.com/google/go-github/v42/github"
)

type WorkflowRun struct {
	WorkflowOwner    string    `json:"workflowOwner"`
	WorkflowRepo     string    `json:"workflowRepo"`
	WorkflowName     string    `json:"workflowName"`
	WorkflowID       int       `json:"workflowId"`
	JobRunID         int       `json:"jobRunId"`
	JobHTMLURL       string    `json:"jobHtmlUrl"`
	JobLogsURL       string    `json:"jobLogsUrl"`
	JobRunNumber     int       `json:"jobRunNumber"`
	JobConclusion    string    `json:"jobConclusion"`
	JobStatus        string    `json:"jobStatus"`
	JobEvent         string    `json:"jobEvent"`
	JobRunTime       time.Time `json:"jobRunTime"`
	JobBranch        string    `json:"jobBranch"`
	JobCommitSha     string    `json:"jobCommitSha"`
	JobCommitAuthor  string    `json:"jobCommitAuthor"`
	JobCommitMessage string    `json:"jobCommitMessage"`
	JobCommitTime    time.Time `json:"jobCommitTitle"`
}

type WorkflowRunInput map[string]string

// Use this as a filter to narrow down which workflow runs to be queried
type WorkflowFilter struct {
	Owner         string
	Repo          string
	WorkflowNames []string
}

type WorkflowClient struct {
	client g.Client
}

func NewWorkflowClient(httpClient *http.Client) *WorkflowClient {
	return &WorkflowClient{
		client: *g.NewClient(httpClient),
	}
}

func (c *WorkflowClient) FetchWorkflowRuns(ctx context.Context, filter *WorkflowFilter) ([]*WorkflowRun, error) {
	runs, err := queryAndAdaptWorkflowRuns(&c.client, ctx, filter)
	if err != nil {
		return nil, err
	}

	return sortWorkflowRuns(runs), nil
}

func (c *WorkflowClient) FetchLatestWorkflowRuns(ctx context.Context, filter *WorkflowFilter) ([]*WorkflowRun, error) {
	runs, err := c.FetchWorkflowRuns(ctx, filter)
	if err != nil {
		return nil, err
	}

	latestRuns := map[string]*WorkflowRun{}

	for _, run := range runs {
		if existing, ok := latestRuns[run.WorkflowName]; ok {
			if existing.JobRunTime.Before(run.JobRunTime) {
				latestRuns[run.WorkflowName] = run
			}
		} else {
			latestRuns[run.WorkflowName] = run
		}
	}

	result := make([]*WorkflowRun, 0)

	for _, run := range latestRuns {
		result = append(result, run)
	}

	return sortWorkflowRuns(result), nil
}

func (c *WorkflowClient) FetchWorkflowRunInput(ctx context.Context, filter *WorkflowFilter, runId int) (*WorkflowRunInput, error) {
	// TODO fetch logs and pars them for `env` variables

	// url, resp, err := c.client.Actions.GetWorkflowRunLogs(ctx, filter.Owner, filter.Repo, int64(runId), true)
	// if err != nil {
	// 	return nil, err
	// }

	// _ = url
	// _ = resp

	// bytes, err := ioutil.ReadAll(resp.Body)

	// if err != nil {
	// 	return nil, err
	// }

	// fmt.Println(url)
	// fmt.Println(string(bytes))
	return nil, nil
}

func queryAndAdaptWorkflowRuns(client *g.Client, ctx context.Context, filter *WorkflowFilter) ([]*WorkflowRun, error) {
	workflowRuns, err := queryWorkflowRuns(client, ctx, filter)
	if err != nil {
		return nil, err
	}

	result := make([]*WorkflowRun, 0)

	for _, workflowRun := range workflowRuns {
		commitSha := ""
		commitAuthor := ""
		commitMessage := ""
		commitTime := time.Time{}
		if workflowRun.HeadCommit != nil {
			commitSha = workflowRun.HeadCommit.GetSHA()
			if commitSha == "" {
				// not sure why but sometimes GetSHA doesn't return anything and that is why I default to this field
				commitSha = workflowRun.HeadCommit.GetID()
			}

			commitMessage = workflowRun.HeadCommit.GetMessage()
			if workflowRun.HeadCommit.GetAuthor() != nil {
				commitAuthor = workflowRun.HeadCommit.GetAuthor().GetName()
			}
			if !workflowRun.HeadCommit.GetTimestamp().Time.IsZero() {
				commitTime = workflowRun.HeadCommit.GetTimestamp().Time
			}
		}

		runResult := &WorkflowRun{
			WorkflowOwner:    filter.Owner,
			WorkflowRepo:     filter.Repo,
			WorkflowName:     workflowRun.GetName(),
			WorkflowID:       int(workflowRun.GetWorkflowID()),
			JobRunID:         int(workflowRun.GetID()),
			JobHTMLURL:       workflowRun.GetHTMLURL(),
			JobLogsURL:       workflowRun.GetLogsURL(),
			JobRunNumber:     workflowRun.GetRunNumber(),
			JobConclusion:    workflowRun.GetConclusion(),
			JobStatus:        workflowRun.GetStatus(),
			JobEvent:         workflowRun.GetEvent(),
			JobRunTime:       workflowRun.CreatedAt.Time,
			JobBranch:        workflowRun.GetHeadBranch(),
			JobCommitSha:     commitSha,
			JobCommitAuthor:  commitAuthor,
			JobCommitMessage: commitMessage,
			JobCommitTime:    commitTime,
		}
		result = append(result, runResult)
	}

	return result, nil
}

func queryWorkflowRuns(client *g.Client, ctx context.Context, filter *WorkflowFilter) ([]*g.WorkflowRun, error) {
	if len(filter.WorkflowNames) == 0 {

		workflowRuns, _, err := client.Actions.ListRepositoryWorkflowRuns(ctx, filter.Owner, filter.Repo, &g.ListWorkflowRunsOptions{})
		if err != nil {
			return nil, err
		}
		return workflowRuns.WorkflowRuns, nil
	}

	existingWorkflows, err := listAllWorkflows(client, ctx, filter)
	if err != nil {
		return nil, err
	}

	workflowIds := map[string]int{}
	for _, workflowName := range filter.WorkflowNames {
		id := resolveWorkflowId(workflowName, existingWorkflows)
		if id == -1 {
			return nil, fmt.Errorf("can't resolve ID of workflow with name '%s'", workflowName)
		}

		workflowIds[workflowName] = id
	}

	filteredRuns := make([]*g.WorkflowRun, 0)
	for name, id := range workflowIds {
		workflowRuns, _, err := client.Actions.ListWorkflowRunsByID(ctx, filter.Owner, filter.Repo, int64(id), nil)
		if err != nil {
			return nil, fmt.Errorf("couldn't retrieve workflow runs for workflow '%s', err: %s", name, err)
		}

		filteredRuns = append(filteredRuns, workflowRuns.WorkflowRuns...)
	}

	return filteredRuns, nil
}

func listAllWorkflows(client *g.Client, ctx context.Context, filter *WorkflowFilter) ([]*g.Workflow, error) {
	allResults := make([]*g.Workflow, 0)

	var pageSize int = 100
	var workflowChunk *g.Workflows = nil
	workflowChunk, _, err := client.Actions.ListWorkflows(ctx, filter.Owner, filter.Repo, &g.ListOptions{PerPage: pageSize})
	if err != nil {
		return nil, err
	}

	allResults = append(allResults, workflowChunk.Workflows...)

	for workflowChunk.GetTotalCount() > len(allResults) {
		page := int(len(allResults)/pageSize) + 1

		workflowChunk, _, err = client.Actions.ListWorkflows(ctx, filter.Owner, filter.Repo, &g.ListOptions{Page: page, PerPage: pageSize})
		if err != nil {
			return nil, err
		}
		allResults = append(allResults, workflowChunk.Workflows...)
	}

	return allResults, nil
}

func resolveWorkflowId(name string, workflows []*g.Workflow) int {
	for _, workflow := range workflows {
		if workflow.GetName() == name {
			return int(workflow.GetID())
		}
	}

	return -1
}

func sortWorkflowRuns(runs []*WorkflowRun) []*WorkflowRun {
	sort.Slice(runs, func(i, j int) bool {
		if runs[i].WorkflowName == runs[j].WorkflowName {
			return runs[i].JobRunTime.After(runs[j].JobRunTime)
		}

		return runs[i].WorkflowName > runs[j].WorkflowName
	})

	return runs
}
