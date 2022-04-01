package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/newestuser/github-workflow-dashboard/backend"
	"github.com/newestuser/github-workflow-dashboard/formatter"
	"github.com/newestuser/github-workflow-dashboard/github"
	"golang.org/x/oauth2"
)

const Version = "v0.10"
const ClientName = "github-workflow-dashboard"

const csvSeparator = ","
const cmdListArgSeparator = ";"

type options struct {
	token              string
	owners             stringArray
	repos              stringArray
	latestOnly         bool
	limit              int
	parseParams        bool
	formatMod          string
	serverMod          bool
	serverPort         int
	serverPollInterval int
	workflows          [][]string
}

func (opts *options) isValid() (bool, string) {
	if len(opts.owners) == 0 {
		return false, "provide at least one owner"
	}

	for _, owner := range opts.owners {
		if owner == "" {
			return false, "owner can't be empty"
		}
	}

	if len(opts.repos) == 0 {
		return false, "provide at least one repo"
	}

	for _, repo := range opts.repos {
		if repo == "" {
			return false, "repo can't be empty"
		}
	}

	if len(opts.workflows) == 0 {
		return false, "provide at least one workflow"
	}

	if len(opts.owners) != len(opts.repos) || len(opts.owners) != len(opts.workflows) {
		return false, fmt.Sprintf("number of owners, repos and set of workflows do not match, owners=%d, repos=%d, workflow sets=%d",
			len(opts.owners), len(opts.repos), len(opts.owners))
	}

	if opts.limit < 0 {
		return false, fmt.Sprintf("limit must be >= 0, limit=%d", opts.limit)
	}

	if opts.limit > 1 && opts.latestOnly {
		return false, fmt.Sprintf("can't have both limit > 1 and fetch latest-only, limit=%d", opts.limit)
	}

	if opts.formatMod != "ascii" && opts.formatMod != "json" {
		return false, fmt.Sprintf(`format "%s" not supported`, opts.formatMod)
	}

	return true, ""
}

func (opts *options) GetLimit() int {
	if opts.latestOnly {
		return 1
	}

	return opts.limit
}

func main() {
	fs := flag.NewFlagSet(ClientName, flag.ExitOnError)

	version := fs.Bool("version", false, "Print version and exit")

	opts := &options{
		workflows: [][]string{},
		owners:    stringArray{},
		repos:     stringArray{},
	}

	fs.StringVar(&opts.token, "token", getStrEnv("WORKFLOW_TOKEN"), "Github API token, see: https://docs.github.com/en/articles/creating-an-access-token-for-command-line-use")
	fs.Var(&opts.owners, "owner", "Github repository owner")
	fs.Var(&opts.repos, "repo", "Github repository")
	fs.BoolVar(&opts.latestOnly, "latest-only", getBoolEnvOr("WORKFLOW_LATEST_ONLY", false), "Fetch only the latest run of the github workflow")
	fs.IntVar(&opts.limit, "limit", getIntEnvOr("WORKFLOW_LIMIT", 0), "Max number of runs to be fetched for each workflow (0 means fetch all)")
	fs.BoolVar(&opts.parseParams, "parse-params", getBoolEnvOr("WORKFLOW_PARSE_PARAMS", false), "Parse workflow run params from log files")
	fs.StringVar(&opts.formatMod, "format", getStrEnvOr("WORKFLOW_FORMAT", "ascii"), "The format in which to print the workflow stats (ascii, json)")
	fs.BoolVar(&opts.serverMod, "server-mod", getBoolEnvOr("WORKFLOW_SERVER_MOD", false), "Start a web server that periodically pulls github workflow stats")
	fs.IntVar(&opts.serverPort, "server-port", getIntEnvOr("WORKFLOW_SERVER_PORT", 8080), "The port on which to start the web server if running in server-mod")
	fs.IntVar(&opts.serverPollInterval, "server-poll-interval", getIntEnvOr("WORKFLOW_SERVER_POLL_INTERVAL", 5), "Interval in minutes used to poll github workflows")

	fs.Usage = func() {
		fmt.Printf("Usage: %s [global flags] '<workflow>'\n", ClientName)
		fmt.Printf("\nglobal flags:\n")
		fs.PrintDefaults()
		fmt.Print(example)
	}

	cmdArgs := os.Args[1:]
	if err := fs.Parse(cmdArgs); err != nil {
		log.Fatal(err)
	}

	if !isFlagPassed(fs, "owner") {
		opts.owners = getStrArrayEnv("WORKFLOW_OWNER")
	}
	if !isFlagPassed(fs, "repo") {
		opts.repos = getStrArrayEnv("WORKFLOW_REPO")
	}

	cliArgs := fs.Args()
	if len(cliArgs) == 0 {
		opts.workflows = loadEnvVarWorkflows()
	} else {
		opts.workflows = parseWorkflowCliArgs(cliArgs)
	}

	if *version {
		fmt.Printf("Version: %s\n", Version)
		return
	}

	if isValid, msg := opts.isValid(); !isValid {
		fmt.Printf("error: %s\n\n", msg)
		fs.Usage()
		os.Exit(1)
	}

	var err error = nil
	if opts.serverMod {
		err = executeAsServer(opts)
	} else {
		err = executeAsCmd(opts)
	}

	if err != nil {
		log.Fatalln(err)
	}
}

func executeAsServer(opts *options) error {
	filters := newWorkflowFilters(opts)

	srvOpts := &backend.Options{
		Port:                opts.serverPort,
		Filters:             filters,
		PollInterval:        time.Duration(opts.serverPollInterval) * time.Minute,
		LatestOnly:          opts.latestOnly,
		ParseWorkflowParams: opts.parseParams,
	}

	client := newGithubClient(context.Background(), opts)
	server := backend.NewServer(client, srvOpts)

	return server.Start()
}

func executeAsCmd(opts *options) error {
	ctx := context.Background()
	client := newGithubClient(ctx, opts)
	filters := newWorkflowFilters(opts)

	var workflowRuns []*github.WorkflowRun
	var err error

	if opts.latestOnly {
		workflowRuns, err = fetchMultiple(ctx, filters, client.FetchLatestWorkflowRuns)
	} else {
		workflowRuns, err = fetchMultiple(ctx, filters, client.FetchWorkflowRuns)
	}

	if err != nil {
		return err
	}

	if opts.parseParams {
		for _, filter := range filters {
			if err := client.EnrichWorkflowRunsWithParams(ctx, filter, workflowRuns); err != nil {
				return err
			}
		}
	}

	result, err := formatCmdOutput(workflowRuns, opts)
	if err != nil {
		return err
	}
	fmt.Println(result)
	return nil
}

func fetchMultiple(ctx context.Context, filters []*github.WorkflowFilter, fetcher workflowFetcherFunc) ([]*github.WorkflowRun, error) {
	allRuns := make([]*github.WorkflowRun, 0)
	for _, filter := range filters {
		runs, err := fetcher(ctx, filter)
		if err != nil {
			return nil, err
		}

		allRuns = append(allRuns, runs...)
	}
	return allRuns, nil
}

func formatCmdOutput(runs []*github.WorkflowRun, opts *options) (string, error) {
	if opts.formatMod == "json" {
		return formatter.ToJson(runs)
	}
	return formatter.ToAscii(runs)
}

func newGithubClient(ctx context.Context, opts *options) *github.WorkflowClient {
	var client *http.Client = nil
	if opts.token != "" {
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: opts.token},
		)
		client = oauth2.NewClient(ctx, ts)
	} else {
		client = nil
	}

	return github.NewWorkflowClient(client)
}

func newWorkflowFilters(opts *options) []*github.WorkflowFilter {
	filters := make([]*github.WorkflowFilter, 0)

	// repos, owners and workflows should have the same length
	for i := range opts.repos {
		owner := opts.owners[i]
		repo := opts.repos[i]
		workflows := opts.workflows[i]

		filter := &github.WorkflowFilter{
			Owner:         owner,
			Repo:          repo,
			WorkflowNames: workflows,
			Limit:         opts.GetLimit(),
		}

		filters = append(filters, filter)
	}

	return filters
}

// Parse an environment variable strings that has the format: "foo,bar,zar;far,mar,gar"
func loadEnvVarWorkflows() [][]string {
	allWorkflowCsvs := os.Getenv("WORKFLOW_CSV")
	if len(allWorkflowCsvs) == 0 {
		return [][]string{}
	}

	workflowCsvSet := strings.Split(allWorkflowCsvs, cmdListArgSeparator)

	return parseWorkflowCsvs(workflowCsvSet)
}

func parseWorkflowCliArgs(args []string) [][]string {
	if isForMultipleRepositories(args) {
		return parseWorkflowCsvs(args)
	}

	// the args here are the workflow names of a single repostiory
	return [][]string{args}
}

// If the array of arguments passed to the CLI contains "," then this would mean the user intents
// to pass a set of workflows for different repositoreis.
func isForMultipleRepositories(args []string) bool {
	for _, arg := range args {
		if strings.Contains(arg, csvSeparator) {
			return true
		}
	}
	return false
}

// Parse an array of csv strings where each csv represents a set of github workflows for a given repository.
// Example:
// 		Input: 	["foo,bar,zar" "far,mar,gar"]
//		Output:	[ ["foo" "bar" "zar"] ["far" "mar" "gar"] ]
func parseWorkflowCsvs(csvs []string) [][]string {
	result := make([][]string, 0)

	for _, csv := range csvs {
		csvValues := make([]string, 0)
		for _, value := range strings.Split(csv, csvSeparator) {
			csvValues = append(csvValues, value)
		}
		if len(csvValues) > 0 {
			result = append(result, csvValues)
		}
	}

	return result
}

func getStrArrayEnv(name string) stringArray {
	allValues := os.Getenv(name)
	if len(allValues) == 0 {
		return stringArray{}
	}

	arr := stringArray{}
	for _, value := range strings.Split(allValues, cmdListArgSeparator) {
		_ = arr.Set(value)
	}
	return arr
}

func getStrEnv(name string) string {
	return os.Getenv(name)
}

func getStrEnvOr(name string, other string) string {
	value := os.Getenv(name)
	if value == "" {
		return other
	}

	return value
}

func getIntEnvOr(name string, other int) int {
	value := os.Getenv(name)

	if value == "" {
		return other
	}

	intValue, err := strconv.Atoi(value)
	if err != nil {
		log.Fatalf("environment variable %s=%s can't be parsed to int", name, value)
	}

	return intValue
}

func getBoolEnvOr(name string, other bool) bool {
	value := os.Getenv(name)

	if value == "" {
		return other
	}

	boolValue, err := strconv.ParseBool(value)
	if err != nil {
		log.Fatalf("environment variable %s=%s can't be parsed to int", name, value)
	}

	return boolValue
}

func isFlagPassed(fs *flag.FlagSet, name string) bool {
	found := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}

type workflowFetcherFunc func(context.Context, *github.WorkflowFilter) ([]*github.WorkflowRun, error)

type stringArray []string

func (f *stringArray) String() string {
	return fmt.Sprintf("%v", *f)
}

func (f *stringArray) Set(value string) error {
	*f = append(*f, value)
	return nil
}

var example = fmt.Sprintf(`
example:
	%s -owner Azure -repo k8s-deploy  "Create release PR" "Tag and create release draft"
`, ClientName)
