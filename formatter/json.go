package formatter

import (
	"encoding/json"

	"github.com/NewestUser/github-workflow-dashboard/github"
)


func ToJson(runs []*github.WorkflowRun) (string, error) {
	bytes, err := json.Marshal(runs)
	if err != nil {
		return "", err
	}

	return string(bytes), nil
}
