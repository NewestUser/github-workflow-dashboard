## CLI capable of retrieving github action workflows stats

![dashboard-image](./assets/github-workflow-dashboard.png)

### Example usage

- Dashboard mod
```shell
github-workflow-dashboard -server-mod -owner Azure -repo k8s-deploy  "Create release PR" "Tag and create release draft"
```

- CLI mod
```shell
github-workflow-dashboard -owner Azure -repo k8s-deploy  "Create release PR" "Tag and create release draft"
```

### Binaries
Prebuild binaries can be found [here](./bin/).  
In order to rebuild the project run `make go-build`

### Manual

```shell
Usage: github-workflow-dashboard [global flags] '<workflow>'

global flags:
  -format string
        The format in which to print the workflow stats (ascii, json) (default "ascii")
  -latest-only
        Fetch only the latest run of the github workflow
  -limit int
        Max number of runs to be fetched for each workflow (0 means fetch all)
  -owner string
        Github repository owner
  -parse-params
        Parse workflow run params from log files
  -repo string
        Github repository
  -server-mod
        Start a web server that periodically pulls github workflow stats
  -server-poll-interval int
        Interval in minutes used to poll github workflows (default 5)
  -server-port int
        The port on which to start the web server if running in server-mod (default 8080)
  -token string
        Github API token, see: https://docs.github.com/en/articles/creating-an-access-token-for-command-line-use
  -version
        Print version and exit

example:
        github-workflow-dashboard -owner Azure -repo k8s-deploy  "Create release PR" "Tag and create release draft"
```

### Environment variables
Command line args take precedence over env variables. If a cmd arg is not passed and an env variable is present then the env variable will be used.

```
WORKFLOW_TOKEN
WORKFLOW_OWNER
WORKFLOW_REPO
WORKFLOW_LATEST_ONLY
WORKFLOW_LIMIT
WORKFLOW_PARSE_PARAMS
WORKFLOW_FORMAT
WORKFLOW_SERVER_MOD
WORKFLOW_SERVER_PORT 
WORKFLOW_SERVER_POLL_INTERVAL
WORKFLOW_CSV
```

### Tracking multiple repositories

#### Using CLI args
```shell
github-workflow-dashboard -owner ownerA -owner ownerB -repo repoA -repo repoB "ownerA_repoA_workflow1,ownerA_repoA_worklow2" "ownerB_repoB_workflow1"

# Example
github-workflow-dashboard -owner Azure -owner actions -repo k8s-deploy -repo checkout  "Create release PR,Tag and create release draft" "Build and Test"
```

#### Using environment variables
```shell
export WORKFLOW_OWNER="ownerA;ownerB"
export WORKFLOW_REPO="repoA;repoB"
export WORKFLOW_CSV="ownerA_repoA_workflow1,ownerA_repoA_worklow2;ownerB_repoB_workflow1"

github-workflow-dashboard

# Example
export WORKFLOW_OWNER="Azure;actions"
export WORKFLOW_REPO="k8s-deploy;checkout"
export WORKFLOW_CSV="Create release PR,Tag and create release draft;Build and Test"

github-workflow-dashboard
```

### Running with docker

- Using Make
```shell
make docker-build
make docker-run
```

- Using Docker
```shell
docker build -t github-workflow-dashboard .

docker run -it -rm -e WORKFLOW_SERVER_MOD=true -e WORKFLOW_OWNER="Azure" -e WORKFLOW_REPO="k8s-deploy" -p 8080:8080 github-workflow-dashboard
```

