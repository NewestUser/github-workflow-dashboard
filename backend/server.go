package backend

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/newestuser/github-workflow-dashboard/formatter"
	"github.com/newestuser/github-workflow-dashboard/github"

	log "github.com/sirupsen/logrus"
)

type Options struct {
	Port                int
	Filter              *github.WorkflowFilter
	PollInterval        time.Duration
	LatestOnly          bool
	ParseWorkflowParams bool
}

func NewServer(client *github.WorkflowClient, opts *Options) *Server {
	return &Server{
		client:     client,
		opts:       opts,
		stateMutex: sync.Mutex{},
	}
}

type Server struct {
	client *github.WorkflowClient
	opts   *Options

	stateMutex sync.Mutex
	state      *workflowState
}

type workflowState struct {
	runs []*github.WorkflowRun
	uts  time.Time
}

var dashboardTemplate = template.Must(template.New("dashboard").Parse(dashboardHTMLTemplate))

func (s *Server) Start() error {
	log.Info("starting web server on port ", s.opts.Port)
	go s.pollGithubWorkflows()

	r := mux.NewRouter()
	r.HandleFunc("/", dashboard(s))
	r.HandleFunc("/{owner}/{repo}/{workflow}", workflowDashboard(s))

	return http.ListenAndServe(fmt.Sprintf(":%d", s.opts.Port), r)
}

// Serve a dashboard with all available workflow runs
func dashboard(server *Server) http.HandlerFunc {
	metaInfoFunc := func(*http.Request) (string, time.Time) {
		title := server.opts.Filter.Repo
		updateTs, _ := server.getState()
		return title, updateTs.uts
	}

	workflwoFetcherFunc := func(*http.Request) ([]*github.WorkflowRun, error) {
		state, err := server.getState()
		if err != nil {
			return nil, err
		}

		return state.runs, nil
	}

	return serveWorkflowRuns(metaInfoFunc, workflwoFetcherFunc)
}

// Serve a dashboard with runs for specific workflow
func workflowDashboard(server *Server) http.HandlerFunc {
	metaInfoFunc := func(r *http.Request) (string, time.Time) {
		params := mux.Vars(r)
		repo := params["repo"]
		worfklow := params["workflow"]

		updateTs, _ := server.getState()
		return fmt.Sprintf("%s/%s", repo, worfklow), updateTs.uts
	}

	workflwoFetcherFunc := func(r *http.Request) ([]*github.WorkflowRun, error) {
		state, err := server.getState()
		if err != nil {
			return nil, err
		}

		params := mux.Vars(r)
		owner := params["owner"]
		repo := params["repo"]
		worfklow := params["workflow"]

		return filterRuns(state.runs, owner, repo, worfklow), nil
	}

	return serveWorkflowRuns(metaInfoFunc, workflwoFetcherFunc)
}

func filterRuns(runs []*github.WorkflowRun, owner string, repo string, name string) []*github.WorkflowRun {
	result := make([]*github.WorkflowRun, 0)

	for _, run := range runs {
		if run.WorkflowOwner == owner && run.WorkflowRepo == repo && run.WorkflowName == name {
			result = append(result, run)
		}
	}

	return result
}

func serveWorkflowRuns(pageMetaInfoFecher func(*http.Request) (string, time.Time), workflowFetcher func(*http.Request) ([]*github.WorkflowRun, error)) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		runs, err := workflowFetcher(r)
		if err != nil {
			log.Error(err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		htmlBody, err := formatter.ToHTMLWithCustomLink(runs, func(run *github.WorkflowRun) string {
			return fmt.Sprintf("/%s/%s/%s", run.WorkflowOwner, run.WorkflowRepo, run.WorkflowName)
		})
		if err != nil {
			log.Error(err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		title, uts := pageMetaInfoFecher(r)
		htmlPage := dashboardHTML{
			PageTitle:      fmt.Sprintf("%s workflows", title),
			LastUpdateTime: fmt.Sprintf("%s ago", time.Since(uts).Round(time.Second)),
			Body:           template.HTML(htmlBody),
		}
		err = dashboardTemplate.Execute(w, htmlPage)

		if err != nil {
			log.Error(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}

func (s *Server) pollGithubWorkflows() {
	for tick := range time.Tick(s.opts.PollInterval) {
		state, err := s.fetchState(tick)
		if err != nil {
			continue
		}
		s.setState(*state)
	}
}

func (s *Server) fetchState(timestamp time.Time) (*workflowState, error) {
	syncTs := timestamp.UTC().Round(time.Second)
	execTs := time.Now()

	var runs []*github.WorkflowRun
	var err error
	ctx := context.Background()

	if s.opts.LatestOnly {
		runs, err = s.client.FetchLatestWorkflowRuns(ctx, s.opts.Filter)
	} else {
		runs, err = s.client.FetchWorkflowRuns(ctx, s.opts.Filter)
	}

	if err != nil {
		err = fmt.Errorf("synctime: [%s] failed fetching workflow stats, workflows: %v err: %v", syncTs, filterNames(runs), err)
		log.Error(err.Error())
		return nil, err
	}

	if s.opts.ParseWorkflowParams {
		for _, run := range runs {
			params, err := s.client.FetchWorkflowRunParams(ctx, s.opts.Filter, run.JobRunID)
			if err != nil {
				log.Warn("synctime: [", syncTs, "] failed fetching workflow params for workflow: ", run.WorkflowName, " runId: ", run.JobRunID, ", it will be omitted, err: ", err)
				continue
			}
			run.WorkflowParams = params
		}
	}

	log.Info("synctime: [", syncTs, "] successfully retrieved workflow runs in ", time.Since(execTs).Round(time.Second), ", worfklows: ", filterNames(runs))

	return &workflowState{
		runs: runs,
		uts:  timestamp,
	}, nil
}

func filterNames(runs []*github.WorkflowRun) []string {
	namesMap := map[string]bool{}
	for _, run := range runs {
		namesMap[run.WorkflowName] = true
	}

	names := make([]string, 0)
	for k, _ := range namesMap {
		names = append(names, k)
	}

	return names
}

func (s *Server) setState(state workflowState) {
	s.stateMutex.Lock()
	s.state = &state
	s.stateMutex.Unlock()
}

func (s *Server) getState() (*workflowState, error) {
	s.stateMutex.Lock()
	defer s.stateMutex.Unlock()

	if s.state == nil {
		state, err := s.fetchState(time.Now())
		if err != nil {
			return nil, err
		} else {
			s.state = state
		}
	}

	return s.state, nil
}

type dashboardHTML struct {
	PageTitle      string
	LastUpdateTime string
	Body           template.HTML
}

const dashboardHTMLTemplate = `
<html>
	<head>
		<meta charset="utf-8">
		<meta name="viewport" content="width=device-width, initial-scale=1, minimal-ui">
		<title>Github Workflow Scraper</title>
		<meta name="color-scheme" content="light dark">
		<link rel="stylesheet" href="https://sindresorhus.com/github-markdown-css/github-markdown.css">
		<style>
			body {
				box-sizing: border-box;
				min-width: 200px;
				margin: 0 auto;
				padding: 45px;
			}	

			@media (prefers-color-scheme: dark) {
				body {
					background-color: #0d1117;
				}
			}
		</style>
		<link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/github-fork-ribbon-css/0.2.3/gh-fork-ribbon.min.css">
	</head>

	<body>
		<article class="markdown-body">
			<h1>{{.PageTitle}}</h1>
			<h4>Last update: {{.LastUpdateTime}}</h4>	
			{{.Body}}
		</article>
	</body>
	
</html>
`
