package github

import (
	"reflect"
	"testing"
)

func TestParseParamsFromLogs1(t *testing.T) {

	log := newLogFile(`
2022-02-24T11:10:21.2859625Z ##[group]GITHUB_TOKEN Permissions
2022-02-24T11:10:21.2862230Z Actions: write
2022-02-24T11:10:21.2863760Z Checks: write
2022-02-24T11:10:21.2865364Z Contents: write
2022-02-24T11:10:21.2866870Z Deployments: write
2022-02-24T11:10:21.2868417Z Discussions: write
2022-02-24T11:10:21.2869930Z Issues: write
2022-02-24T11:10:21.2871433Z Metadata: read
2022-02-24T11:10:21.2873516Z Packages: write
2022-02-24T11:10:21.2875918Z Pages: write
2022-02-24T11:10:21.2878497Z PullRequests: write
2022-02-24T11:10:21.2881450Z RepositoryProjects: write
2022-02-24T11:10:21.2883660Z SecurityEvents: write
2022-02-24T11:10:21.2968905Z Statuses: write
2022-02-24T11:10:21.2970062Z ##[endgroup]
2022-02-24T11:10:21.2977393Z Secret source: Actions
2022-02-24T11:10:21.2978962Z Prepare workflow directory
2022-02-24T11:10:21.5053939Z Prepare all required actions
2022-02-24T11:10:21.5328010Z Getting action download info
2022-02-24T11:10:21.8416964Z Download action repository 'actions/checkout@v2' (SHA:ec3a7ce113134d7a93b817d10a8272cb61118579)
2022-02-24T11:10:22.3962301Z Download action repository 'actions/setup-java@v1' (SHA:d202f5dbf7256730fb690ec59f6381650114feb2)
2022-02-24T11:10:22.8379540Z Download action repository 'stCarolas/setup-maven@v4' (SHA:1d56b37995622db66cce1214d81014b09807fb5a)
2022-02-24T11:10:24.8619017Z ##[group]Run actions/checkout@v2
2022-02-24T11:10:24.8619909Z with:
2022-02-24T11:10:24.8620689Z   repository: foo/bar
2022-02-24T11:10:24.8621874Z   token: ***
2022-02-24T11:10:24.8622630Z   ssh-strict: true
2022-02-24T11:10:24.8623406Z   persist-credentials: true
2022-02-24T11:10:24.8624480Z   clean: true
2022-02-24T11:10:24.8625455Z   fetch-depth: 1
2022-02-24T11:10:24.8626174Z   lfs: false
2022-02-24T11:10:24.8626890Z   submodules: false
2022-02-24T11:10:24.8627684Z env:
2022-02-24T11:10:24.8628366Z   param-1: foo
2022-02-24T11:10:24.8629094Z   param-2: bar
2022-02-24T11:10:24.8629861Z ##[endgroup]
2022-02-24T11:10:25.0404068Z Syncing repository: foo/bar
2022-02-24T11:10:25.0407211Z ##[group]Getting Git version info
2022-02-24T11:10:25.0408661Z Working directory is '/home/runner/work/foo/foo'
2022-02-24T11:10:25.0410386Z [command]/usr/bin/git version`)

	got, err := parseJobRunLog(log)
	if err != nil {
		t.Errorf("got error: %s", err)
	}

	want := JobRunParams{
		"param-1": "foo",
		"param-2": "bar",
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, wanted %q", got, want)
	}
}

func TestParseParamsFromLogs2(t *testing.T) {
	log := newLogFile(`
2022-02-24T11:28:55.2803768Z ##[group]Run actions/checkout@v2
2022-02-24T11:28:55.2804587Z with:
2022-02-24T11:28:55.2805181Z   repository: RepoOwner/Repo
2022-02-24T11:28:55.2806220Z   token: ***
2022-02-24T11:28:55.2806887Z   ssh-strict: true
2022-02-24T11:28:55.2807477Z   persist-credentials: true
2022-02-24T11:28:55.2808060Z   clean: true
2022-02-24T11:28:55.2808584Z   fetch-depth: 1
2022-02-24T11:28:55.2809124Z   lfs: false
2022-02-24T11:28:55.2809647Z   submodules: false
2022-02-24T11:28:55.2810181Z env:
2022-02-24T11:28:55.2810792Z   xxx: 203
2022-02-24T11:28:55.2811677Z   yyy: aaa
2022-02-24T11:28:55.2812310Z ##[endgroup]
2022-02-24T11:28:55.5660582Z Syncing repository: RepoOwner/Repo
2022-02-24T11:28:55.5660582Z Syncing repository: RepoOwner/Repo
2022-02-24T11:28:55.5663836Z ##[group]Getting Git version info
2022-02-24T11:28:55.5664965Z Working directory is '/home/runner/foo/bar'
2022-02-24T11:28:55.5666118Z [command]/usr/bin/git version
2022-02-24T11:28:55.5684649Z git version 2.31.1
2022-02-24T11:28:55.5720314Z ##[endgroup]`)

	got, err := parseJobRunLog(log)
	if err != nil {
		t.Errorf("got error: %s", err)
	}

	want := JobRunParams{
		"xxx": "203",
		"yyy": "aaa",
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, wanted %q", got, want)
	}
}

func TestParseParamsFromLogs3(t *testing.T) {
	log := newLogFile(`
2022-04-01T09:20:29.3070469Z apiVersion: apps/v1
2022-04-01T09:20:29.3071090Z kind: Deployment
2022-04-01T09:20:29.3071651Z metadata:
2022-04-01T09:20:29.3072342Z   name: foo-bar-zar
2022-04-01T09:20:29.3072965Z spec:
2022-04-01T09:20:29.3073494Z   replicas: 1
2022-04-01T09:20:29.3074004Z   selector:
2022-04-01T09:20:29.3074585Z     matchLabels:
2022-04-01T09:20:29.3075264Z       app: xxx
2022-04-01T09:20:29.3075846Z   template:
2022-04-01T09:20:29.3076407Z     metadata:
2022-04-01T09:20:29.3076951Z       labels:
2022-04-01T09:20:29.3077606Z         app: xxx
2022-04-01T09:20:29.3078184Z     spec:
2022-04-01T09:20:29.3078736Z       containers:
2022-04-01T09:20:29.3079444Z       - env:
2022-04-01T09:20:29.3080119Z         - name: env_var_1
2022-04-01T09:20:29.3080809Z           value: env_var_value_1
2022-04-01T09:20:29.3081590Z         - name: env_var_2
2022-04-01T09:20:29.3082208Z           value: env_var_value_2
2022-04-01T09:20:29.3089521Z         image: docker-repo/foo/bar
2022-04-01T09:20:29.3090310Z         imagePullPolicy: Always
2022-04-01T09:20:29.3091006Z         name: xxx-frontend
2022-04-01T09:20:29.3091535Z         ports:
2022-04-01T09:20:29.3092254Z         - containerPort: 8888
2022-04-01T09:20:29.3092804Z         resources:
2022-04-01T09:20:29.3093384Z           limits:
2022-04-01T09:20:29.3093892Z             cpu: 400m
2022-04-01T09:20:29.3094400Z             memory: 512Mi
2022-04-01T09:20:29.3094898Z           requests:
2022-04-01T09:20:29.3095405Z             cpu: 200m
2022-04-01T09:20:29.3095908Z             memory: 256Mi
2022-04-01T09:20:29.3139811Z 
2022-04-01T09:20:29.3249842Z ##[group]Run Azure/k8s-deploy@v3.0
2022-04-01T09:20:29.3250244Z with:
2022-04-01T09:20:29.3250769Z   manifests: /home/runner/actions-runner/_work/_temp/baked-template-1648804829312.yaml
2022-04-01T09:20:29.3251331Z   namespace: foo-bar-namespace
2022-04-01T09:20:29.3251717Z   action: deploy
2022-04-01T09:20:29.3252271Z   images: docker-repo/foo/bar:xyz
2022-04-01T09:20:29.3252787Z   strategy: none
2022-04-01T09:20:29.3253168Z   route-method: service
2022-04-01T09:20:29.3253585Z   version-switch-buffer: 0
2022-04-01T09:20:29.3254013Z   traffic-split-method: pod
2022-04-01T09:20:29.3254475Z   baseline-and-canary-replicas: 0
2022-04-01T09:20:29.3254907Z   percentage: 0
2022-04-01T09:20:29.3255263Z   force: false
2022-04-01T09:20:29.3255756Z   token: ***
2022-04-01T09:20:29.3256094Z env:
2022-04-01T09:20:29.3256428Z   key1: value1
2022-04-01T09:20:29.3256804Z   key2: value2
2022-04-01T09:20:29.3257352Z   foo: foovar
2022-04-01T09:20:29.3257966Z   bar_zar: 1234
2022-04-01T09:20:29.3258824Z ##[endgroup]
2022-04-01T09:20:29.4357517Z Deploying manifests
2022-04-01T09:20:29.4403195Z ##[warning]Deployment strategy is not recognized.
2022-04-01T09:20:29.4417258Z [command]/usr/bin/kubectl apply -f /tmp/baked-template-1648804829312.yaml --
2022-04-01T09:20:43.6534863Z shell: /bin/bash -e {0}
2022-04-01T09:20:43.6535491Z env:
2022-04-01T09:20:43.6536049Z   key1: value1
2022-04-01T09:20:43.6536622Z   key2: value2
2022-04-01T09:20:43.6537416Z   foo: foovar
2022-04-01T09:20:43.6538649Z   bar_zar: 1234
2022-04-01T09:20:43.6539454Z ##[endgroup]
2022-04-01T09:20:43.8171324Z deployment.apps/xxxx restarted
2022-04-01T09:20:43.8251120Z Post job cleanup.`)

	got, err := parseJobRunLog(log)
	if err != nil {
		t.Errorf("got error: %s", err)
	}

	want := JobRunParams{
		"key1": "value1",
		"key2": "value2",
		"foo": "foovar",
		"bar_zar": "1234",
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, wanted %q", got, want)
	}
}

func newLogFile(body string) *inMemoryFile {
	return &inMemoryFile{name: "dummy-log-file", body: []byte(body)}
}
