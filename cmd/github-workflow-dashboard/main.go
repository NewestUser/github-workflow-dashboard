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

const Version = "0.1"
const ClientName = "github-workflow-dashboard"

type options struct {
	token              string
	owner              string
	repo               string
	formatMod          string
	serverMod          bool
	serverPort         int
	serverPollInterval int
	workflows          []string
}

func (opts *options) isValid() bool {
	if opts.owner == "" {
		return false
	}

	if opts.repo == "" {
		return false
	}

	if opts.formatMod != "ascii" && opts.formatMod != "json" {
		return false
	}

	return true
}

func main() {
	fs := flag.NewFlagSet(ClientName, flag.ExitOnError)

	version := fs.Bool("version", false, "Print version and exit")

	opts := &options{
		workflows: []string{},
	}

	fs.StringVar(&opts.token, "token", getStrEnv("WORKFLOW_TOKEN"), "Github API token, see: https://docs.github.coim/en/articles/creating-an-access-token-for-command-line-use")
	fs.StringVar(&opts.owner, "owner", getStrEnv("WORKFLOW_OWNER"), "Github repository owner")
	fs.StringVar(&opts.repo, "repo", getStrEnv("WORKFLOW_REPO"), "Github repository")
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

	cliArgs := fs.Args()
	if len(cliArgs) == 0 {
		cliArgs = strings.Split(os.Getenv("WORKFLOW_CSV"), ",")
	}
	opts.workflows = cliArgs

	if *version {
		fmt.Printf("Version: %s\n", Version)
		return
	}

	if !opts.isValid() {
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
	filter := newWorkflowFilter(opts)

	srvOpts := &backend.Options{
		Port:         opts.serverPort,
		Filter:       filter,
		PollInterval: time.Duration(opts.serverPollInterval) * time.Minute,
	}

	client := newGithubClient(context.Background(), opts)
	server := backend.NewServer(client, srvOpts)

	return server.Start()
}

func executeAsCmd(opts *options) error {
	ctx := context.Background()
	client := newGithubClient(ctx, opts)
	filter := newWorkflowFilter(opts)

	workflowRuns, err := client.FetchLatestWorkflowRuns(ctx, filter)
	if err != nil {
		return err
	}

	result, err := formatCmdOutput(workflowRuns, opts)
	if err != nil {
		return err
	}
	fmt.Println(result)
	return nil
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

func newWorkflowFilter(opts *options) *github.WorkflowFilter {
	return &github.WorkflowFilter{
		Owner:         opts.owner,
		Repo:          opts.repo,
		WorkflowNames: opts.workflows,
	}
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

func getIntEnv(name string) int {
	value := os.Getenv(name)

	intValue, err := strconv.Atoi(value)
	if err != nil {
		log.Fatalf("environment variable %s=%s can't be parsed to int", name, value)
	}

	return intValue
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

func getBoolEnv(name string) bool {
	value := os.Getenv(name)

	boolValue, err := strconv.ParseBool(value)
	if err != nil {
		log.Fatalf("environment variable %s=%s can't be parsed to int", name, value)
	}

	return boolValue
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

var example = fmt.Sprintf(`
example:
	%s -owner Azure -repo k8s-deploy  "Create release PR" "Tag and create release draft"
`, ClientName)
