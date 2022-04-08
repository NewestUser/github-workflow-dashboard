package backend

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/newestuser/github-workflow-dashboard/formatter"
	"github.com/newestuser/github-workflow-dashboard/github"

	log "github.com/sirupsen/logrus"
)

type Options struct {
	Port                int
	Filters             []*github.WorkflowFilter
	PollInterval        time.Duration
	LatestOnly          bool
	ParseWorkflowParams bool
}

func NewServer(client *github.WorkflowClient, opts *Options) *Server {
	return &Server{
		client:     client,
		opts:       opts,
		stateMutex: sync.Mutex{},
		state:      newStateRepo(),
	}
}

type Server struct {
	client *github.WorkflowClient
	opts   *Options

	stateMutex sync.Mutex
	state      *stateRepository
}

type stateRepository struct {
	data map[RepoId]*repoState
}

func newStateRepo() *stateRepository {
	return &stateRepository{
		data: make(map[RepoId]*repoState),
	}
}

func (r *stateRepository) setMulti(newState []*repoState) {
	for _, s := range newState {
		r.set(s)
	}
}

func (r *stateRepository) set(newState *repoState) {
	if r.data == nil {
		r.data = make(map[RepoId]*repoState)
	}
	r.data[newState.repo] = newState

}

func (r *stateRepository) filter(owner, repo, workflow string) []*repoState {
	result := make([]*repoState, 0)
	// find all
	if owner == "" && repo == "" && workflow == "" {
		for _, val := range r.data {
			result = append(result, val)
		}
		return result
	}

	// find by owner
	if owner != "" && repo == "" && workflow == "" {
		for _, val := range r.data {
			if val.repo.owner == owner {
				result = append(result, val)
			}
		}
		return result
	}

	// find by owner and repo
	if owner != "" && repo != "" && workflow == "" {
		for _, val := range r.data {
			if val.repo.owner == owner && val.repo.name == repo {
				result = append(result, val)
			}
		}
		return result
	}

	// find by owner, repo and workflow
	if owner != "" && repo != "" && workflow != "" {
		for _, val := range r.data {
			if val.repo.owner == owner && val.repo.name == repo {

				tmpState := &repoState{
					repo: val.repo,
					uts:  val.uts,
					runs: make([]*github.WorkflowRun, 0),
				}
				for _, run := range val.runs {
					if run.WorkflowName == workflow {
						tmpState.runs = append(tmpState.runs, run)
					}
				}

				result = append(result, tmpState)
			}
		}
		return result
	}

	// no other filters are supported
	panic(fmt.Errorf("filtering workflow runs by (owner=%s, repo=%s, workflow=%s) is not supported", owner, repo, workflow))
}

type RepoId struct {
	owner string
	name  string
}

func (r RepoId) String() string {
	return fmt.Sprintf("%s/%s", r.owner, r.name)
}

type repoState struct {
	repo RepoId
	runs []*github.WorkflowRun
	uts  time.Time
}

var dashboardTemplate = template.Must(template.New("dashboard").Parse(dashboardHTMLTemplate))
var repositoryTemplate = template.Must(template.New("repository").Parse(repositoryHTMLTemplate))

func (s *Server) Start() error {
	log.Info("starting web server on port ", s.opts.Port)
	go s.pollGithubWorkflows()

	r := mux.NewRouter()
	r.HandleFunc("/", dashboard(s))

	r.HandleFunc("/api/{owner}", ownerJson(s))
	r.HandleFunc("/api/{owner}/{repo}", repoJson(s))
	r.HandleFunc("/api/{owner}/{repo}/{workflow}", workflowJson(s))

	r.HandleFunc("/{owner}", ownerDashboard(s))
	r.HandleFunc("/{owner}/{repo}", repoDashboard(s))
	r.HandleFunc("/{owner}/{repo}/{workflow}", workflowDashboard(s))

	return http.ListenAndServe(fmt.Sprintf(":%d", s.opts.Port), r)
}

func filterAndRenderRepoSections(w http.ResponseWriter, server *Server, owner, repo, workflow string) {
	state, _ := server.getState()
	repoState := state.filter(owner, repo, workflow)

	sort.Slice(repoState, func(i, j int) bool {
		return repoState[i].repo.String() < repoState[j].repo.String()
	})

	repoHTML, err := renderMultipleRepoHTMLSections(repoState)
	if err != nil {
		log.Error(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	renderDashboard(w, &dashboardHTMLViewModel{
		Repositories: repoHTML,
	})
}

func ownerDashboard(server *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)
		owner := params["owner"]
		filterAndRenderRepoSections(w, server, owner, "", "")
	}
}

func repoDashboard(server *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)
		owner := params["owner"]
		repo := params["repo"]

		filterAndRenderRepoSections(w, server, owner, repo, "")
	}
}

func workflowDashboard(server *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)
		owner := params["owner"]
		repo := params["repo"]
		workflow := params["workflow"]
		filterAndRenderRepoSections(w, server, owner, repo, workflow)
	}
}

// Serve a dashboard with all available workflow runs
func dashboard(server *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		filterAndRenderRepoSections(w, server, "", "", "")
	}
}

func ownerJson(server *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)
		owner := params["owner"]
		serveWorkflowJson(w, server, owner, "", "")
	}
}

func repoJson(server *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)
		owner := params["owner"]
		repo := params["repo"]
		serveWorkflowJson(w, server, owner, repo, "")
	}
}

func workflowJson(server *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)
		owner := params["owner"]
		repo := params["repo"]
		workflow := params["workflow"]
		serveWorkflowJson(w, server, owner, repo, workflow)
	}
}

// Serve github workflow data as a json response
func serveWorkflowJson(w http.ResponseWriter, server *Server, owner, repo, workflow string) {
	state, err := server.getState()
	if err != nil {
		log.Error(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	result := make([]*github.WorkflowRun, 0)
	multiRepState := state.filter(owner, repo, workflow)
	for _, value := range multiRepState {
		result = append(result, value.runs...)
	}

	w.Header().Add("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(result); err != nil {
		log.Error(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func renderMultipleRepoHTMLSections(state []*repoState) ([]template.HTML, error) {
	sections := make([]template.HTML, 0)
	for _, repoState := range state {
		repoHtml, err := renderRepoHTMLSection(repoState)
		if err != nil {
			return nil, err
		}
		sections = append(sections, repoHtml)
	}
	return sections, nil
}

func renderRepoHTMLSection(repoState *repoState) (template.HTML, error) {
	htmlBody, err := formatter.ToHTMLWithCustomLink(repoState.runs, func(run *github.WorkflowRun) string {
		return fmt.Sprintf("/%s/%s/%s", run.WorkflowOwner, run.WorkflowRepo, run.WorkflowName)
	})

	if err != nil {
		return "", err
	}

	repoHtmlModel := repsotioryHTMLViewModel{
		Owner:          repoState.repo.owner,
		Repository:     repoState.repo.name,
		LastUpdateTime: fmt.Sprintf("%s ago", time.Since(repoState.uts).Round(time.Second)),
		Body:           template.HTML(htmlBody),
	}

	repoHtml := &strings.Builder{}
	if err := repositoryTemplate.Execute(repoHtml, repoHtmlModel); err != nil {
		return "", err
	}

	return template.HTML(repoHtml.String()), nil
}

func renderDashboard(w http.ResponseWriter, viewModel *dashboardHTMLViewModel) {
	if err := dashboardTemplate.Execute(w, viewModel); err != nil {
		log.Error(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) pollGithubWorkflows() {
	// trigger poll imiediately after which it should be periodic
	results := s.fetchAllStatesIgnoringErrors(time.Now())
	s.updateState(results)

	for tick := range time.Tick(s.opts.PollInterval) {
		results := s.fetchAllStatesIgnoringErrors(tick)
		s.updateState(results)
	}
}

func (s *Server) fetchAllStatesIgnoringErrors(uts time.Time) []*repoState {
	fetchExecTs := time.Now()
	allResults := make([]*repoState, 0)
	allRepos := strings.Builder{}

	for _, f := range s.opts.Filters {
		allRepos.WriteString(f.GetRepoId().String())
		allRepos.WriteString(" ")
	}

	log.Info("Start fetching state for all repos: ", allRepos.String())
	for _, filter := range s.opts.Filters {
		repoExecTs := time.Now()
		ownerAndRepo := fmt.Sprintf("%s/%s", filter.Owner, filter.Repo)

		log.Info("Fetching state for repo: ", ownerAndRepo)
		repoResult, err := s.fetchState(filter, uts)
		if err != nil {
			log.Warn("Failed fetching state for repo: ", ownerAndRepo, " after ", time.Since(repoExecTs).Round(time.Second))
			continue
		}

		log.Info("Successfully fetched state for repo: ", ownerAndRepo, " in ", time.Since(repoExecTs).Round(time.Second), " runs: ", filterNames(repoResult.runs))
		allResults = append(allResults, repoResult)
	}
	log.Info("Successfully fetched state of all repos in ", time.Since(fetchExecTs).Round(time.Second))
	return allResults
}

func (s *Server) fetchState(filter *github.WorkflowFilter, timestamp time.Time) (*repoState, error) {
	var runs []*github.WorkflowRun
	var err error
	ctx := context.Background()

	if s.opts.LatestOnly {
		runs, err = s.client.FetchLatestWorkflowRuns(ctx, filter)
	} else {
		runs, err = s.client.FetchWorkflowRuns(ctx, filter)
	}

	if err != nil {
		return nil, err
	}

	if s.opts.ParseWorkflowParams {
		for _, run := range runs {
			params, err := s.client.FetchWorkflowRunParams(ctx, filter, run.JobRunID)
			if err != nil {
				log.Warn("failed fetching workflow params for ", fmt.Sprintf("%s/%s", filter.Owner, filter.Repo), " workflow: ", run.WorkflowName, " runId: ", run.JobRunID, ", it will be omitted, err: ", err)
				continue
			}
			run.WorkflowParams = params
		}
	}

	return &repoState{
		repo: RepoId{owner: filter.Owner, name: filter.Repo},
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

func (s *Server) getState() (*stateRepository, error) {
	return s.state, nil
}

func (s *Server) updateState(newState []*repoState) {
	s.state.setMulti(newState)
}

func (s *Server) lockState() {
	s.stateMutex.Lock()
}

func (s *Server) unlockState() {
	s.stateMutex.Unlock()
}

type dashboardHTMLViewModel struct {
	Repositories []template.HTML
}

type repsotioryHTMLViewModel struct {
	Owner          string
	Repository     string
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
			<h2><a href="/">Home</a></h2>
			<br/>
			{{ range $repository := .Repositories }}
				{{ $repository }}
				<br/>
			{{ end }}
		</article>
	</body>
	
</html>
`

const repositoryHTMLTemplate = `
<section>
	<h2><a href="/{{.Owner}}">{{.Owner}}</a>/<a href="/{{.Owner}}/{{.Repository}}">{{.Repository}}</a></h2>
	<h4>Last update: {{.LastUpdateTime}}</h4>	
	{{.Body}}
<section>
`
