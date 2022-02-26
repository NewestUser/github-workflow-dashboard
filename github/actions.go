package github

//////////////////////////////////////////////////////////////////////////////////
// Inspired by https://gist.github.com/bauergeorg/856de20e2a99f930b384291d36b189a5
//////////////////////////////////////////////////////////////////////////////////

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"sort"
	"time"

	g "github.com/google/go-github/v42/github"
	log "github.com/sirupsen/logrus"
)

const maxPageSize = 100

type WorkflowRun struct {
	WorkflowOwner    string             `json:"workflowOwner"`
	WorkflowRepo     string             `json:"workflowRepo"`
	WorkflowName     string             `json:"workflowName"`
	WorkflowID       int                `json:"workflowId"`
	JobRunID         int                `json:"jobRunId"`
	JobHTMLURL       string             `json:"jobHtmlUrl"`
	JobLogsURL       string             `json:"jobLogsUrl"`
	JobRunNumber     int                `json:"jobRunNumber"`
	JobConclusion    string             `json:"jobConclusion"`
	JobStatus        string             `json:"jobStatus"`
	JobEvent         string             `json:"jobEvent"`
	JobRunTime       time.Time          `json:"jobRunTime"`
	JobBranch        string             `json:"jobBranch"`
	JobCommitSha     string             `json:"jobCommitSha"`
	JobCommitAuthor  string             `json:"jobCommitAuthor"`
	JobCommitMessage string             `json:"jobCommitMessage"`
	JobCommitTime    time.Time          `json:"jobCommitTitle"`
	WorkflowParams   *WorkflowRunParams `json:"worfklowParams"`
}

type WorkflowRunParams struct {
	WorkflowOwner string         `json:"workflowOwner"`
	WorkflowRepo  string         `json:"workflowRepo"`
	RunId         int            `json:"jobRunId"`
	Params        []JobRunParams `json:"jobParams"`
}

type JobRunParams map[string]string

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

func (c *WorkflowClient) EnrichWorkflowRunsWithParams(ctx context.Context, filter *WorkflowFilter, runs []*WorkflowRun) error {
	for _, run := range runs {
		params, err := c.FetchWorkflowRunParams(ctx, filter, run.JobRunID)
		if err != nil {
			log.Warn(fmt.Sprintf("Failed fetching workflow params for workflow: %v runId: %d, it will be ommitedd, err: %v", run.WorkflowName, run.JobRunID, err))
		}
		run.WorkflowParams = params
	}

	return nil
}

func (c *WorkflowClient) FetchWorkflowRunParams(ctx context.Context, filter *WorkflowFilter, runId int) (*WorkflowRunParams, error) {
	// c.client.Actions.GetWorkflowJobLogs(ctx, filter.Owner, filter.Repo, int64(runId), true)
	url, _, err := c.client.Actions.GetWorkflowRunLogs(ctx, filter.Owner, filter.Repo, int64(runId), true)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("GET", url.String(), nil)
	if err != nil {
		return nil, err
	}

	respBody := new(bytes.Buffer)

	resp, err := c.client.Do(ctx, req, respBody)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s %s - response: %s %s", req.Method, req.URL, resp.Status, respBody.String())
	}

	workflowLogs, err := readZip(*respBody)
	if err != nil {
		return nil, err
	}

	params, err := parseWorkflowParams(workflowLogs)
	if err != nil {
		return nil, err
	}

	return &WorkflowRunParams{WorkflowOwner: filter.Owner, WorkflowRepo: filter.Repo, RunId: runId, Params: params}, nil
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
	existingWorkflows, err := listAllWorkflows(client, ctx, filter)
	if err != nil {
		return nil, err
	}

	currentFilter := *filter
	if len(filter.WorkflowNames) == 0 {
		allWorkflows := make([]string, len(existingWorkflows))
		for i, workflow := range existingWorkflows {
			allWorkflows[i] = workflow.GetName()
		}
		currentFilter.WorkflowNames = allWorkflows
	}

	workflowIds := map[string]int{}
	for _, workflowName := range currentFilter.WorkflowNames {
		id := resolveWorkflowId(workflowName, existingWorkflows)
		if id == -1 {
			return nil, fmt.Errorf("can't resolve ID of workflow with name '%s'", workflowName)
		}

		workflowIds[workflowName] = id
	}

	filteredRuns := make([]*g.WorkflowRun, 0)
	for name, id := range workflowIds {
		pageOptions := &g.ListWorkflowRunsOptions{ListOptions: *newPageOption(0, maxPageSize)}
		workflowRuns, _, err := client.Actions.ListWorkflowRunsByID(ctx, filter.Owner, filter.Repo, int64(id), pageOptions)
		if err != nil {
			return nil, fmt.Errorf("couldn't retrieve workflow runs for workflow '%s', err: %s", name, err)
		}

		filteredRuns = append(filteredRuns, workflowRuns.WorkflowRuns...)
	}

	return filteredRuns, nil
}

func listAllWorkflows(client *g.Client, ctx context.Context, filter *WorkflowFilter) ([]*g.Workflow, error) {
	allResults := make([]*g.Workflow, 0)

	var pageSize = maxPageSize
	var workflowChunk *g.Workflows = nil
	workflowChunk, _, err := client.Actions.ListWorkflows(ctx, filter.Owner, filter.Repo, newPageOption(0, pageSize))
	if err != nil {
		return nil, err
	}

	allResults = append(allResults, workflowChunk.Workflows...)

	for workflowChunk.GetTotalCount() > len(allResults) {
		page := int(len(allResults)/pageSize) + 1

		workflowChunk, _, err = client.Actions.ListWorkflows(ctx, filter.Owner, filter.Repo, newPageOption(page, pageSize))
		if err != nil {
			return nil, err
		}
		allResults = append(allResults, workflowChunk.Workflows...)
	}

	return allResults, nil
}

func newPageOption(page int, perPage int) *g.ListOptions {
	return &g.ListOptions{Page: page, PerPage: perPage}
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
