docker-build:
	docker build -t github-workflow-dashboard .

docker-run:
	docker run -it --rm -p 8080:8080 -e WORKFLOW_SERVER_MOD=true -e WORKFLOW_OWNER="Azure" -e WORKFLOW_REPO="k8s-deploy" -e WORKFLOW_CSV="Create release PR,Tag and create release draft" github-workflow-dashboard
