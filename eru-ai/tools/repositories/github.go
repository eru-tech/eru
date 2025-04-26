package github

import (
	"context"
	"encoding/json"
	"errors"

	tools "github.com/eru-tech/eru/eru-ai/tools"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	repos "github.com/eru-tech/eru/eru-repos/repos"
)

type GithubTool struct {
	tools.Tool
	Repo repos.GithubRepo `json:"repo"`
}

func (ghTool *GithubTool) GetSpec() tools.Tooling {
	return ghTool
}

func (ghTool *GithubTool) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("MakeFromJson - Start")
	err := json.Unmarshal(*rj, &ghTool)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}

func (ghTool *GithubTool) Execute(ctx context.Context, actionName string, params map[string]interface{}) (toolResult map[string]interface{}, err error) {
	logs.WithContext(ctx).Debug("PlaywrightTool Execute - Start")
	logs.WithContext(ctx).Info("ghTool.Executed")
	contents := ""
	fileName := ""
	if contentI, contentIOk := params["contents"]; contentIOk {
		contents = contentI.(string)
	} else {
		err = errors.New("contents attribute not found")
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}
	if fileNameI, fileNameIOk := params["file_name"]; fileNameIOk {
		fileName = fileNameI.(string)
	} else {
		err = errors.New("file_name attribute not found")
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}

	err = ghTool.Repo.Commit(ctx, []byte(contents), fileName)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}
	return map[string]interface{}{"content": "Commit successful"}, nil
}
