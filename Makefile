docker-build:
	docker build -t github-workflow-dashboard .

docker-run:
	docker run -it --rm -p 8080:8080 -e WORKFLOW_SERVER_MOD=true -e WORKFLOW_OWNER="Azure" -e WORKFLOW_REPO="k8s-deploy" -e WORKFLOW_CSV="Create release PR,Tag and create release draft" github-workflow-dashboard

go-build:
	env GOOS=linux GOARCH=386 go build -o ./bin/github-workflow-dashboard-linux-386 ./cmd/github-workflow-dashboard 
	env GOOS=windows GOARCH=386 go build -o ./bin/github-workflow-dashboard-windows-386.exe ./cmd/github-workflow-dashboard 
	env GOOS=darwin GOARCH=amd64 go build -o ./bin/github-workflow-dashboard-darwin-amd64 ./cmd/github-workflow-dashboard 