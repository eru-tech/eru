package repos

import (
	"context"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
)

type RepoI interface {
	Commit(ctx context.Context, repoData map[string]map[string]interface{}, repoFileName string) (err error)
	GetAttribute(attrName string) (attrValue interface{})
	GetBranch(ctx context.Context) (branch interface{}, err error)
}

func (repo *Repo) Commit(ctx context.Context, repoData map[string]map[string]interface{}, repoFileName string) (err error) {
	logs.WithContext(ctx).Info("Commit not implemented")
	return
}

func (repo *Repo) GetAttribute(attrName string) (attrValue interface{}) {
	switch attrName {
	case "branchName":
		return repo.BranchName
	case "repoName":
		return repo.RepoName
	case "repoType":
		return repo.RepoType
	case "authKey":
		return repo.AuthKey
	case "autoCommit":
		return repo.AutoCommit
	case "authMode":
		return repo.AuthMode
	default:
		return nil
	}
}

type Repo struct {
	RepoType   string `json:"repoType"`
	RepoName   string `json:"repoName"`
	BranchName string `json:"branchName"`
	AuthMode   string `json:"authMode"`
	AuthKey    string `json:"authKey"`
	AutoCommit bool   `json:"autoCommit"`
}

func GetRepo(repoType string, repo Repo) RepoI {
	switch repoType {
	case "GITHUB":
		gr := new(GithubRepo)
		gr.BranchName = repo.GetAttribute("branchName").(string)
		gr.RepoName = repo.GetAttribute("repoName").(string)
		gr.RepoType = repo.GetAttribute("repoType").(string)
		gr.AuthKey = repo.GetAttribute("authKey").(string)
		gr.AuthMode = repo.GetAttribute("authMode").(string)
		gr.AutoCommit = repo.GetAttribute("autoCommit").(bool)
		return gr
	default:
		return nil
	}
	return nil
}
