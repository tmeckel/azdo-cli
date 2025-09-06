package git

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClientCommand(t *testing.T) {
	tests := []struct {
		name     string
		repoDir  string
		gitPath  string
		wantExe  string
		wantArgs []string
	}{
		{
			name:     "creates command",
			gitPath:  "path/to/git",
			wantExe:  "path/to/git",
			wantArgs: []string{"path/to/git", "ref-log"},
		},
		{
			name:     "adds repo directory configuration",
			repoDir:  "path/to/repo",
			gitPath:  "path/to/git",
			wantExe:  "path/to/git",
			wantArgs: []string{"path/to/git", "-C", "path/to/repo", "ref-log"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in, out, errOut := &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{}
			client := client{
				Stdin:   in,
				Stdout:  out,
				Stderr:  errOut,
				RepoDir: tt.repoDir,
				GitPath: tt.gitPath,
			}
			cmd, err := client.Command(context.Background(), "ref-log")
			require.NoError(t, err)
			assert.Equal(t, tt.wantExe, cmd.Path)
			assert.Equal(t, tt.wantArgs, cmd.Args)
			assert.Equal(t, in, cmd.Stdin)
			assert.Equal(t, out, cmd.Stdout)
			assert.Equal(t, errOut, cmd.Stderr)
		})
	}
}

func TestClientAuthenticatedCommand(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		wantArgs []string
	}{
		{
			name:     "adds credential helper config options",
			path:     "path/to/azdo",
			wantArgs: []string{"path/to/git", "-c", "credential.useHttpPath=true", "-c", `credential.helper=!"path/to/azdo" auth git-credential`, "fetch"},
		},
		{
			name:     "fallback when AzDoPath is not set",
			wantArgs: []string{"path/to/git", "-c", "credential.useHttpPath=true", "-c", `credential.helper=!"azdo" auth git-credential`, "fetch"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := client{
				AzDoPath: tt.path,
				GitPath:  "path/to/git",
			}
			cmd, err := client.AuthenticatedCommand(context.Background(), "fetch")
			require.NoError(t, err)
			assert.Equal(t, tt.wantArgs, cmd.Args)
		})
	}
}

func TestClientAuthenticatedCloneCommand(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		wantArgs []string
	}{
		{
			name:     "adds credential helper config options",
			path:     "path/to/azdo",
			wantArgs: []string{"path/to/git", "-c", "credential.useHttpPath=true", "-c", `credential.helper=!"path/to/azdo" auth git-credential`, "clone"},
		},
		{
			name:     "fallback when AzDoPath is not set",
			wantArgs: []string{"path/to/git", "-c", "credential.useHttpPath=true", "-c", `credential.helper=!"azdo" auth git-credential`, "clone"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := client{
				AzDoPath: tt.path,
				GitPath:  "path/to/git",
			}
			cmd, err := client.AuthenticatedCommand(context.Background(), "clone")
			require.NoError(t, err)
			assert.Equal(t, tt.wantArgs, cmd.Args)
		})
	}
}

func TestClientRemotes(t *testing.T) {
	tempDir := t.TempDir()
	initRepo(t, tempDir)
	gitDir := filepath.Join(tempDir, ".git")
	remoteFile := filepath.Join(gitDir, "config")
	remotes := `
[remote "origin"]
	url = git@example.com:v3/monalisa/origin.git
[remote "test"]
	url = git://dev.azure.com/hubot/repo/test.git
	azdo-resolved = other
[remote "upstream"]
	url = https://dev.azure.com/monalisa/project/_git/repo/upstream.git
	azdo-resolved = default
[remote "github"]
	url = git@github.com:v3/hubot/github.git
`
	f, err := os.OpenFile(remoteFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o755)
	require.NoError(t, err)
	_, err = f.Write([]byte(remotes))
	require.NoError(t, err)
	err = f.Close()
	require.NoError(t, err)
	client := client{
		RepoDir: tempDir,
	}
	rs, err := client.Remotes(context.Background())
	require.NoError(t, err)
	assert.Len(t, rs, 4)
	assert.Equal(t, "upstream", rs[0].Name)
	assert.Equal(t, "default", rs[0].Resolved)
	assert.Equal(t, "github", rs[1].Name)
	assert.Equal(t, "", rs[1].Resolved)
	assert.Equal(t, "origin", rs[2].Name)
	assert.Equal(t, "", rs[2].Resolved)
	assert.Equal(t, "test", rs[3].Name)
	assert.Equal(t, "other", rs[3].Resolved)
}

func TestClientRemotes_no_resolved_remote(t *testing.T) {
	tempDir := t.TempDir()
	initRepo(t, tempDir)
	gitDir := filepath.Join(tempDir, ".git")
	remoteFile := filepath.Join(gitDir, "config")
	remotes := `
[remote "origin"]
	url = git@dev.azure.com:v3/org/monalisa/origin
[remote "test"]
	url = git://dev.azure.com:8443/org/hubot/test
[remote "upstream"]
	url = https://dev.azure.com/org/monalisa/_git/upstream
[remote "no-azdo"]
	url = https://example.com/monalisa/upstream
`
	f, err := os.OpenFile(remoteFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o755)
	require.NoError(t, err)
	_, err = f.Write([]byte(remotes))
	require.NoError(t, err)
	err = f.Close()
	require.NoError(t, err)
	client := client{
		RepoDir: tempDir,
	}
	rs, err := client.Remotes(context.Background())
	require.NoError(t, err)
	assert.Len(t, rs, 4)
	assert.Equal(t, "origin", rs[1].Name)
	assert.Equal(t, "test", rs[3].Name)
	assert.Equal(t, "upstream", rs[0].Name)
	assert.Equal(t, "no-azdo", rs[2].Name)
}

func TestParseRemotes(t *testing.T) {
	remoteList := []string{
		"mona\tgit@dev.azure.com:v3/monalisa/project/myfork.git (fetch)",
		"origin\thttps://github.com/monalisa/repo/_git/octo-cat.git (fetch)",
		"origin\thttps://github.com/monalisa/repo/_git/octo-cat-push.git (push)",
		"upstream\thttps://example.com/organization/project/_git/nowhere (fetch)",
		"upstream\thttps://dev.azure.com/hubot/tools/_git/nowhere (push)",
		"zardoz\thttps://example.com/zed.git (push)",
		"koke\tgit://dev.azure.com/koke/project/_git/grit.git (fetch)",
		"koke\tgit://dev.azure.com/koke/project/_git/grit.git (push)",
	}

	r := parseRemotes(remoteList)
	assert.Len(t, r, 5)

	assert.Equal(t, "mona", r[0].Name)
	assert.Equal(t, "ssh://git@dev.azure.com/v3/monalisa/project/myfork.git", r[0].FetchURL.String())
	assert.Nil(t, r[0].PushURL)

	assert.Equal(t, "origin", r[1].Name)
	assert.Equal(t, "/monalisa/repo/_git/octo-cat.git", r[1].FetchURL.Path)
	assert.Equal(t, "/monalisa/repo/_git/octo-cat-push.git", r[1].PushURL.Path)

	assert.Equal(t, "upstream", r[2].Name)
	assert.Equal(t, "example.com", r[2].FetchURL.Host)
	assert.Equal(t, "dev.azure.com", r[2].PushURL.Host)

	assert.Equal(t, "zardoz", r[3].Name)
	assert.Nil(t, r[3].FetchURL)
	assert.Equal(t, "https://example.com/zed.git", r[3].PushURL.String())

	assert.Equal(t, "koke", r[4].Name)
	assert.Equal(t, "/koke/project/_git/grit.git", r[4].FetchURL.Path)
	assert.Equal(t, "/koke/project/_git/grit.git", r[4].PushURL.Path)
}

func TestClientUpdateRemoteURL(t *testing.T) {
	tests := []struct {
		name          string
		cmdExitStatus int
		cmdStdout     string
		cmdStderr     string
		wantCmdArgs   string
		wantErrorMsg  string
	}{
		{
			name:        "update remote url",
			wantCmdArgs: `path/to/git remote set-url test https://test.com`,
		},
		{
			name:          "git error",
			cmdExitStatus: 1,
			cmdStderr:     "git error message",
			wantCmdArgs:   `path/to/git remote set-url test https://test.com`,
			wantErrorMsg:  "failed to run git: git error message",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, cmdCtx := createCommandContext(t, tt.cmdExitStatus, tt.cmdStdout, tt.cmdStderr)
			client := client{
				GitPath:        "path/to/git",
				commandContext: cmdCtx,
			}
			err := client.UpdateRemoteURL(context.Background(), "test", "https://test.com")
			assert.Equal(t, tt.wantCmdArgs, strings.Join(cmd.Args[3:], " "))
			if tt.wantErrorMsg == "" {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tt.wantErrorMsg)
			}
		})
	}
}

func TestClientSetRemoteResolution(t *testing.T) {
	tests := []struct {
		name          string
		cmdExitStatus int
		cmdStdout     string
		cmdStderr     string
		wantCmdArgs   string
		wantErrorMsg  string
	}{
		{
			name:        "set remote resolution",
			wantCmdArgs: `path/to/git config --add remote.origin.azdo-resolved base`,
		},
		{
			name:          "git error",
			cmdExitStatus: 1,
			cmdStderr:     "git error message",
			wantCmdArgs:   `path/to/git config --add remote.origin.azdo-resolved base`,
			wantErrorMsg:  "failed to run git: git error message",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, cmdCtx := createCommandContext(t, tt.cmdExitStatus, tt.cmdStdout, tt.cmdStderr)
			client := client{
				GitPath:        "path/to/git",
				commandContext: cmdCtx,
			}
			err := client.SetRemoteResolution(context.Background(), "origin", "base")
			assert.Equal(t, tt.wantCmdArgs, strings.Join(cmd.Args[3:], " "))
			if tt.wantErrorMsg == "" {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tt.wantErrorMsg)
			}
		})
	}
}

func TestClientCurrentBranch(t *testing.T) {
	tests := []struct {
		name          string
		cmdExitStatus int
		cmdStdout     string
		cmdStderr     string
		wantCmdArgs   string
		wantErrorMsg  string
		wantBranch    string
	}{
		{
			name:        "branch name",
			cmdStdout:   "branch-name\n",
			wantCmdArgs: `path/to/git symbolic-ref --quiet HEAD`,
			wantBranch:  "branch-name",
		},
		{
			name:        "ref",
			cmdStdout:   "refs/heads/branch-name\n",
			wantCmdArgs: `path/to/git symbolic-ref --quiet HEAD`,
			wantBranch:  "branch-name",
		},
		{
			name:        "escaped ref",
			cmdStdout:   "refs/heads/branch\u00A0with\u00A0non\u00A0breaking\u00A0space\n",
			wantCmdArgs: `path/to/git symbolic-ref --quiet HEAD`,
			wantBranch:  "branch\u00A0with\u00A0non\u00A0breaking\u00A0space",
		},
		{
			name:          "detached head",
			cmdExitStatus: 1,
			wantCmdArgs:   `path/to/git symbolic-ref --quiet HEAD`,
			wantErrorMsg:  "failed to run git: not on any branch",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, cmdCtx := createCommandContext(t, tt.cmdExitStatus, tt.cmdStdout, tt.cmdStderr)
			client := client{
				GitPath:        "path/to/git",
				commandContext: cmdCtx,
			}
			branch, err := client.CurrentBranch(context.Background())
			assert.Equal(t, tt.wantCmdArgs, strings.Join(cmd.Args[3:], " "))
			if tt.wantErrorMsg == "" {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tt.wantErrorMsg)
			}
			assert.Equal(t, tt.wantBranch, branch)
		})
	}
}

func TestClientShowRefs(t *testing.T) {
	tests := []struct {
		name          string
		cmdExitStatus int
		cmdStdout     string
		cmdStderr     string
		wantCmdArgs   string
		wantRefs      []Ref
		wantErrorMsg  string
	}{
		{
			name:          "show refs with one valid ref and one invalid ref",
			cmdExitStatus: 128,
			cmdStdout:     "9ea76237a557015e73446d33268569a114c0649c refs/heads/valid",
			cmdStderr:     "fatal: 'refs/heads/invalid' - not a valid ref",
			wantCmdArgs:   `path/to/git show-ref --verify -- refs/heads/valid refs/heads/invalid`,
			wantRefs: []Ref{{
				Hash: "9ea76237a557015e73446d33268569a114c0649c",
				Name: "refs/heads/valid",
			}},
			wantErrorMsg: "failed to run git: fatal: 'refs/heads/invalid' - not a valid ref",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, cmdCtx := createCommandContext(t, tt.cmdExitStatus, tt.cmdStdout, tt.cmdStderr)
			client := client{
				GitPath:        "path/to/git",
				commandContext: cmdCtx,
			}
			refs, err := client.ShowRefs(context.Background(), []string{"refs/heads/valid", "refs/heads/invalid"})
			assert.Equal(t, tt.wantCmdArgs, strings.Join(cmd.Args[3:], " "))
			require.EqualError(t, err, tt.wantErrorMsg)
			assert.Equal(t, tt.wantRefs, refs)
		})
	}
}

func TestClientConfig(t *testing.T) {
	tests := []struct {
		name          string
		cmdExitStatus int
		cmdStdout     string
		cmdStderr     string
		wantCmdArgs   string
		wantOut       string
		wantErrorMsg  string
	}{
		{
			name:        "get config key",
			cmdStdout:   "test",
			wantCmdArgs: `path/to/git config credential.helper`,
			wantOut:     "test",
		},
		{
			name:          "get unknown config key",
			cmdExitStatus: 1,
			cmdStderr:     "git error message",
			wantCmdArgs:   `path/to/git config credential.helper`,
			wantErrorMsg:  "failed to run git: unknown config key credential.helper",
		},
		{
			name:          "git error",
			cmdExitStatus: 2,
			cmdStderr:     "git error message",
			wantCmdArgs:   `path/to/git config credential.helper`,
			wantErrorMsg:  "failed to run git: git error message",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, cmdCtx := createCommandContext(t, tt.cmdExitStatus, tt.cmdStdout, tt.cmdStderr)
			client := client{
				GitPath:        "path/to/git",
				commandContext: cmdCtx,
			}
			out, err := client.GetConfig(context.Background(), "credential.helper")
			assert.Equal(t, tt.wantCmdArgs, strings.Join(cmd.Args[3:], " "))
			if tt.wantErrorMsg == "" {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tt.wantErrorMsg)
			}
			assert.Equal(t, tt.wantOut, out)
		})
	}
}

func TestClientUncommittedChangeCount(t *testing.T) {
	tests := []struct {
		name            string
		cmdExitStatus   int
		cmdStdout       string
		cmdStderr       string
		wantCmdArgs     string
		wantChangeCount int
	}{
		{
			name:            "no changes",
			wantCmdArgs:     `path/to/git status --porcelain`,
			wantChangeCount: 0,
		},
		{
			name:            "one change",
			cmdStdout:       " M poem.txt",
			wantCmdArgs:     `path/to/git status --porcelain`,
			wantChangeCount: 1,
		},
		{
			name:            "untracked file",
			cmdStdout:       " M poem.txt\n?? new.txt",
			wantCmdArgs:     `path/to/git status --porcelain`,
			wantChangeCount: 2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, cmdCtx := createCommandContext(t, tt.cmdExitStatus, tt.cmdStdout, tt.cmdStderr)
			client := client{
				GitPath:        "path/to/git",
				commandContext: cmdCtx,
			}
			ucc, err := client.UncommittedChangeCount(context.Background())
			assert.Equal(t, tt.wantCmdArgs, strings.Join(cmd.Args[3:], " "))
			require.NoError(t, err)
			assert.Equal(t, tt.wantChangeCount, ucc)
		})
	}
}

type stubbedCommit struct {
	Sha   string
	Title string
	Body  string
}

type stubbedCommitsCommandData struct {
	ExitStatus int

	ErrMsg string

	Commits []stubbedCommit
}

func TestClientCommits(t *testing.T) {
	tests := []struct {
		name         string
		testData     stubbedCommitsCommandData
		wantCmdArgs  string
		wantCommits  []*Commit
		wantErrorMsg string
	}{
		{
			name: "single commit no body",
			testData: stubbedCommitsCommandData{
				Commits: []stubbedCommit{
					{
						Sha:   "6a6872b918c601a0e730710ad8473938a7516d30",
						Title: "testing testability test",
						Body:  "",
					},
				},
			},
			wantCmdArgs: `path/to/git -c log.ShowSignature=false log --pretty=format:%H%x00%s%x00%b%x00 --cherry SHA1...SHA2`,
			wantCommits: []*Commit{{
				Sha:   "6a6872b918c601a0e730710ad8473938a7516d30",
				Title: "testing testability test",
			}},
		},
		{
			name: "single commit with body",
			testData: stubbedCommitsCommandData{
				Commits: []stubbedCommit{
					{
						Sha:   "6a6872b918c601a0e730710ad8473938a7516d30",
						Title: "testing testability test",
						Body:  "This is the body",
					},
				},
			},
			wantCmdArgs: `path/to/git -c log.ShowSignature=false log --pretty=format:%H%x00%s%x00%b%x00 --cherry SHA1...SHA2`,
			wantCommits: []*Commit{{
				Sha:   "6a6872b918c601a0e730710ad8473938a7516d30",
				Title: "testing testability test",
				Body:  "This is the body",
			}},
		},
		{
			name: "multiple commits with bodies",
			testData: stubbedCommitsCommandData{
				Commits: []stubbedCommit{
					{
						Sha:   "6a6872b918c601a0e730710ad8473938a7516d30",
						Title: "testing testability test",
						Body:  "This is the body",
					},
					{
						Sha:   "7a6872b918c601a0e730710ad8473938a7516d31",
						Title: "testing testability test 2",
						Body:  "This is the body 2",
					},
				},
			},
			wantCmdArgs: `path/to/git -c log.ShowSignature=false log --pretty=format:%H%x00%s%x00%b%x00 --cherry SHA1...SHA2`,
			wantCommits: []*Commit{
				{
					Sha:   "6a6872b918c601a0e730710ad8473938a7516d30",
					Title: "testing testability test",
					Body:  "This is the body",
				},
				{
					Sha:   "7a6872b918c601a0e730710ad8473938a7516d31",
					Title: "testing testability test 2",
					Body:  "This is the body 2",
				},
			},
		},
		{
			name: "multiple commits mixed bodies",
			testData: stubbedCommitsCommandData{
				Commits: []stubbedCommit{
					{
						Sha:   "6a6872b918c601a0e730710ad8473938a7516d30",
						Title: "testing testability test",
					},
					{
						Sha:   "7a6872b918c601a0e730710ad8473938a7516d31",
						Title: "testing testability test 2",
						Body:  "This is the body 2",
					},
				},
			},
			wantCmdArgs: `path/to/git -c log.ShowSignature=false log --pretty=format:%H%x00%s%x00%b%x00 --cherry SHA1...SHA2`,
			wantCommits: []*Commit{
				{
					Sha:   "6a6872b918c601a0e730710ad8473938a7516d30",
					Title: "testing testability test",
				},
				{
					Sha:   "7a6872b918c601a0e730710ad8473938a7516d31",
					Title: "testing testability test 2",
					Body:  "This is the body 2",
				},
			},
		},
		{
			name: "multiple commits newlines in bodies",
			testData: stubbedCommitsCommandData{
				Commits: []stubbedCommit{
					{
						Sha:   "6a6872b918c601a0e730710ad8473938a7516d30",
						Title: "testing testability test",
						Body:  "This is the body\nwith a newline",
					},
					{
						Sha:   "7a6872b918c601a0e730710ad8473938a7516d31",
						Title: "testing testability test 2",
						Body:  "This is the body 2",
					},
				},
			},
			wantCmdArgs: `path/to/git -c log.ShowSignature=false log --pretty=format:%H%x00%s%x00%b%x00 --cherry SHA1...SHA2`,
			wantCommits: []*Commit{
				{
					Sha:   "6a6872b918c601a0e730710ad8473938a7516d30",
					Title: "testing testability test",
					Body:  "This is the body\nwith a newline",
				},
				{
					Sha:   "7a6872b918c601a0e730710ad8473938a7516d31",
					Title: "testing testability test 2",
					Body:  "This is the body 2",
				},
			},
		},
		{
			name: "no commits between SHAs",
			testData: stubbedCommitsCommandData{
				Commits: []stubbedCommit{},
			},
			wantCmdArgs:  `path/to/git -c log.ShowSignature=false log --pretty=format:%H%x00%s%x00%b%x00 --cherry SHA1...SHA2`,
			wantErrorMsg: "could not find any commits between SHA1 and SHA2",
		},
		{
			name: "git error",
			testData: stubbedCommitsCommandData{
				ErrMsg:     "git error message",
				ExitStatus: 1,
			},
			wantCmdArgs:  `path/to/git -c log.ShowSignature=false log --pretty=format:%H%x00%s%x00%b%x00 --cherry SHA1...SHA2`,
			wantErrorMsg: "failed to run git: git error message",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, cmdCtx := createCommitsCommandContext(t, tt.testData)
			client := client{
				GitPath:        "path/to/git",
				commandContext: cmdCtx,
			}
			commits, err := client.Commits(context.Background(), "SHA1", "SHA2")
			require.Equal(t, tt.wantCmdArgs, strings.Join(cmd.Args[3:], " "))
			if tt.wantErrorMsg != "" {
				require.EqualError(t, err, tt.wantErrorMsg)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.wantCommits, commits)
		})
	}
}

func TestCommitsHelperProcess(t *testing.T) {
	if os.Getenv("GH_WANT_HELPER_PROCESS") != "1" {
		return
	}

	var td stubbedCommitsCommandData
	_ = json.Unmarshal([]byte(os.Getenv("GH_COMMITS_TEST_DATA")), &td)

	if td.ErrMsg != "" {
		fmt.Fprint(os.Stderr, td.ErrMsg)
	} else {
		var sb strings.Builder
		for _, commit := range td.Commits {
			sb.WriteString(commit.Sha)
			sb.WriteString("\u0000")
			sb.WriteString(commit.Title)
			sb.WriteString("\u0000")
			sb.WriteString(commit.Body)
			sb.WriteString("\u0000")
			sb.WriteString("\n")
		}
		fmt.Fprint(os.Stdout, sb.String())
	}

	os.Exit(td.ExitStatus)
}

func createCommitsCommandContext(t *testing.T, testData stubbedCommitsCommandData) (*exec.Cmd, commandCtx) {
	t.Helper()

	b, err := json.Marshal(testData)
	require.NoError(t, err)

	cmd := exec.CommandContext(context.Background(), os.Args[0], "-test.run=TestCommitsHelperProcess", "--")
	cmd.Env = []string{
		"GH_WANT_HELPER_PROCESS=1",
		"GH_COMMITS_TEST_DATA=" + string(b),
	}
	return cmd, func(ctx context.Context, exe string, args ...string) *exec.Cmd {
		cmd.Args = append(cmd.Args, exe)
		cmd.Args = append(cmd.Args, args...)
		return cmd
	}
}

func TestClientLastCommit(t *testing.T) {
	client := client{
		RepoDir: "./fixtures/simple.git",
	}
	c, err := client.LastCommit(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "6f1a2405cace1633d89a79c74c65f22fe78f9659", c.Sha)
	assert.Equal(t, "Second commit", c.Title)
}

func TestClientCommitBody(t *testing.T) {
	client := client{
		RepoDir: "./fixtures/simple.git",
	}
	body, err := client.CommitBody(context.Background(), "6f1a2405cace1633d89a79c74c65f22fe78f9659")
	require.NoError(t, err)
	assert.Equal(t, "I'm starting to get the hang of things\n", body)
}

func TestClientReadBranchConfig(t *testing.T) {
	tests := []struct {
		name             string
		cmdExitStatus    int
		cmdStdout        string
		cmdStderr        string
		wantCmdArgs      string
		wantBranchConfig BranchConfig
	}{
		{
			name:             "read branch config",
			cmdStdout:        "branch.trunk.remote origin\nbranch.trunk.merge refs/heads/trunk",
			wantCmdArgs:      `path/to/git config --get-regexp ^branch\.trunk\.(remote|merge)$`,
			wantBranchConfig: BranchConfig{RemoteName: "origin", MergeRef: "refs/heads/trunk"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, cmdCtx := createCommandContext(t, tt.cmdExitStatus, tt.cmdStdout, tt.cmdStderr)
			client := client{
				GitPath:        "path/to/git",
				commandContext: cmdCtx,
			}
			branchConfig := client.ReadBranchConfig(context.Background(), "trunk")
			assert.Equal(t, tt.wantCmdArgs, strings.Join(cmd.Args[3:], " "))
			assert.Equal(t, tt.wantBranchConfig, branchConfig)
		})
	}
}

func TestClientDeleteLocalTag(t *testing.T) {
	tests := []struct {
		name          string
		cmdExitStatus int
		cmdStdout     string
		cmdStderr     string
		wantCmdArgs   string
		wantErrorMsg  string
	}{
		{
			name:        "delete local tag",
			wantCmdArgs: `path/to/git tag -d v1.0`,
		},
		{
			name:          "git error",
			cmdExitStatus: 1,
			cmdStderr:     "git error message",
			wantCmdArgs:   `path/to/git tag -d v1.0`,
			wantErrorMsg:  "failed to run git: git error message",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, cmdCtx := createCommandContext(t, tt.cmdExitStatus, tt.cmdStdout, tt.cmdStderr)
			client := client{
				GitPath:        "path/to/git",
				commandContext: cmdCtx,
			}
			err := client.DeleteLocalTag(context.Background(), "v1.0")
			assert.Equal(t, tt.wantCmdArgs, strings.Join(cmd.Args[3:], " "))
			if tt.wantErrorMsg == "" {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tt.wantErrorMsg)
			}
		})
	}
}

func TestClientDeleteLocalBranch(t *testing.T) {
	tests := []struct {
		name          string
		cmdExitStatus int
		cmdStdout     string
		cmdStderr     string
		wantCmdArgs   string
		wantErrorMsg  string
	}{
		{
			name:        "delete local branch",
			wantCmdArgs: `path/to/git branch -D trunk`,
		},
		{
			name:          "git error",
			cmdExitStatus: 1,
			cmdStderr:     "git error message",
			wantCmdArgs:   `path/to/git branch -D trunk`,
			wantErrorMsg:  "failed to run git: git error message",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, cmdCtx := createCommandContext(t, tt.cmdExitStatus, tt.cmdStdout, tt.cmdStderr)
			client := client{
				GitPath:        "path/to/git",
				commandContext: cmdCtx,
			}
			err := client.DeleteLocalBranch(context.Background(), "trunk")
			assert.Equal(t, tt.wantCmdArgs, strings.Join(cmd.Args[3:], " "))
			if tt.wantErrorMsg == "" {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tt.wantErrorMsg)
			}
		})
	}
}

func TestClientHasLocalBranch(t *testing.T) {
	tests := []struct {
		name          string
		cmdExitStatus int
		cmdStdout     string
		cmdStderr     string
		wantCmdArgs   string
		wantOut       bool
	}{
		{
			name:        "has local branch",
			wantCmdArgs: `path/to/git rev-parse --verify refs/heads/trunk`,
			wantOut:     true,
		},
		{
			name:          "does not have local branch",
			cmdExitStatus: 1,
			wantCmdArgs:   `path/to/git rev-parse --verify refs/heads/trunk`,
			wantOut:       false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, cmdCtx := createCommandContext(t, tt.cmdExitStatus, tt.cmdStdout, tt.cmdStderr)
			client := client{
				GitPath:        "path/to/git",
				commandContext: cmdCtx,
			}
			out := client.HasLocalBranch(context.Background(), "trunk")
			assert.Equal(t, tt.wantCmdArgs, strings.Join(cmd.Args[3:], " "))
			assert.Equal(t, tt.wantOut, out)
		})
	}
}

func TestClientCheckoutBranch(t *testing.T) {
	tests := []struct {
		name          string
		cmdExitStatus int
		cmdStdout     string
		cmdStderr     string
		wantCmdArgs   string
		wantErrorMsg  string
	}{
		{
			name:        "checkout branch",
			wantCmdArgs: `path/to/git checkout trunk`,
		},
		{
			name:          "git error",
			cmdExitStatus: 1,
			cmdStderr:     "git error message",
			wantCmdArgs:   `path/to/git checkout trunk`,
			wantErrorMsg:  "failed to run git: git error message",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, cmdCtx := createCommandContext(t, tt.cmdExitStatus, tt.cmdStdout, tt.cmdStderr)
			client := client{
				GitPath:        "path/to/git",
				commandContext: cmdCtx,
			}
			err := client.CheckoutBranch(context.Background(), "trunk")
			assert.Equal(t, tt.wantCmdArgs, strings.Join(cmd.Args[3:], " "))
			if tt.wantErrorMsg == "" {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tt.wantErrorMsg)
			}
		})
	}
}

func TestClientCheckoutNewBranch(t *testing.T) {
	tests := []struct {
		name          string
		cmdExitStatus int
		cmdStdout     string
		cmdStderr     string
		wantCmdArgs   string
		wantErrorMsg  string
	}{
		{
			name:        "checkout new branch",
			wantCmdArgs: `path/to/git checkout -b trunk --track origin/trunk`,
		},
		{
			name:          "git error",
			cmdExitStatus: 1,
			cmdStderr:     "git error message",
			wantCmdArgs:   `path/to/git checkout -b trunk --track origin/trunk`,
			wantErrorMsg:  "failed to run git: git error message",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, cmdCtx := createCommandContext(t, tt.cmdExitStatus, tt.cmdStdout, tt.cmdStderr)
			client := client{
				GitPath:        "path/to/git",
				commandContext: cmdCtx,
			}
			err := client.CheckoutNewBranch(context.Background(), "origin", "trunk")
			assert.Equal(t, tt.wantCmdArgs, strings.Join(cmd.Args[3:], " "))
			if tt.wantErrorMsg == "" {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tt.wantErrorMsg)
			}
		})
	}
}

func TestClientToplevelDir(t *testing.T) {
	tests := []struct {
		name          string
		cmdExitStatus int
		cmdStdout     string
		cmdStderr     string
		wantCmdArgs   string
		wantDir       string
		wantErrorMsg  string
	}{
		{
			name:        "top level dir",
			cmdStdout:   "/path/to/repo",
			wantCmdArgs: `path/to/git rev-parse --show-toplevel`,
			wantDir:     "/path/to/repo",
		},
		{
			name:          "git error",
			cmdExitStatus: 1,
			cmdStderr:     "git error message",
			wantCmdArgs:   `path/to/git rev-parse --show-toplevel`,
			wantErrorMsg:  "failed to run git: git error message",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, cmdCtx := createCommandContext(t, tt.cmdExitStatus, tt.cmdStdout, tt.cmdStderr)
			client := client{
				GitPath:        "path/to/git",
				commandContext: cmdCtx,
			}
			dir, err := client.ToplevelDir(context.Background())
			assert.Equal(t, tt.wantCmdArgs, strings.Join(cmd.Args[3:], " "))
			if tt.wantErrorMsg == "" {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tt.wantErrorMsg)
			}
			assert.Equal(t, tt.wantDir, dir)
		})
	}
}

func TestClientGitDir(t *testing.T) {
	tests := []struct {
		name          string
		cmdExitStatus int
		cmdStdout     string
		cmdStderr     string
		wantCmdArgs   string
		wantDir       string
		wantErrorMsg  string
	}{
		{
			name:        "git dir",
			cmdStdout:   "/path/to/repo/.git",
			wantCmdArgs: `path/to/git rev-parse --git-dir`,
			wantDir:     "/path/to/repo/.git",
		},
		{
			name:          "git error",
			cmdExitStatus: 1,
			cmdStderr:     "git error message",
			wantCmdArgs:   `path/to/git rev-parse --git-dir`,
			wantErrorMsg:  "failed to run git: git error message",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, cmdCtx := createCommandContext(t, tt.cmdExitStatus, tt.cmdStdout, tt.cmdStderr)
			client := client{
				GitPath:        "path/to/git",
				commandContext: cmdCtx,
			}
			dir, err := client.GitDir(context.Background())
			assert.Equal(t, tt.wantCmdArgs, strings.Join(cmd.Args[3:], " "))
			if tt.wantErrorMsg == "" {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tt.wantErrorMsg)
			}
			assert.Equal(t, tt.wantDir, dir)
		})
	}
}

func TestClientPathFromRoot(t *testing.T) {
	tests := []struct {
		name          string
		cmdExitStatus int
		cmdStdout     string
		cmdStderr     string
		wantCmdArgs   string
		wantErrorMsg  string
		wantDir       string
	}{
		{
			name:        "current path from root",
			cmdStdout:   "some/path/",
			wantCmdArgs: `path/to/git rev-parse --show-prefix`,
			wantDir:     "some/path",
		},
		{
			name:          "git error",
			cmdExitStatus: 1,
			cmdStderr:     "git error message",
			wantCmdArgs:   `path/to/git rev-parse --show-prefix`,
			wantDir:       "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, cmdCtx := createCommandContext(t, tt.cmdExitStatus, tt.cmdStdout, tt.cmdStderr)
			client := client{
				GitPath:        "path/to/git",
				commandContext: cmdCtx,
			}
			dir := client.PathFromRoot(context.Background())
			assert.Equal(t, tt.wantCmdArgs, strings.Join(cmd.Args[3:], " "))
			assert.Equal(t, tt.wantDir, dir)
		})
	}
}

func TestClientUnsetRemoteResolution(t *testing.T) {
	tests := []struct {
		name          string
		cmdExitStatus int
		cmdStdout     string
		cmdStderr     string
		wantCmdArgs   string
		wantErrorMsg  string
	}{
		{
			name:        "unset remote resolution",
			wantCmdArgs: `path/to/git config --unset remote.origin.azdo-resolved`,
		},
		{
			name:          "git error",
			cmdExitStatus: 1,
			cmdStderr:     "git error message",
			wantCmdArgs:   `path/to/git config --unset remote.origin.azdo-resolved`,
			wantErrorMsg:  "failed to run git: git error message",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, cmdCtx := createCommandContext(t, tt.cmdExitStatus, tt.cmdStdout, tt.cmdStderr)
			client := client{
				GitPath:        "path/to/git",
				commandContext: cmdCtx,
			}
			err := client.UnsetRemoteResolution(context.Background(), "origin")
			assert.Equal(t, tt.wantCmdArgs, strings.Join(cmd.Args[3:], " "))
			if tt.wantErrorMsg == "" {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tt.wantErrorMsg)
			}
		})
	}
}

func TestClientSetRemoteBranches(t *testing.T) {
	tests := []struct {
		name          string
		cmdExitStatus int
		cmdStdout     string
		cmdStderr     string
		wantCmdArgs   string
		wantErrorMsg  string
	}{
		{
			name:        "set remote branches",
			wantCmdArgs: `path/to/git remote set-branches origin trunk`,
		},
		{
			name:          "git error",
			cmdExitStatus: 1,
			cmdStderr:     "git error message",
			wantCmdArgs:   `path/to/git remote set-branches origin trunk`,
			wantErrorMsg:  "failed to run git: git error message",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, cmdCtx := createCommandContext(t, tt.cmdExitStatus, tt.cmdStdout, tt.cmdStderr)
			client := client{
				GitPath:        "path/to/git",
				commandContext: cmdCtx,
			}
			err := client.SetRemoteBranches(context.Background(), "origin", "trunk")
			assert.Equal(t, tt.wantCmdArgs, strings.Join(cmd.Args[3:], " "))
			if tt.wantErrorMsg == "" {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tt.wantErrorMsg)
			}
		})
	}
}

func TestClientFetch(t *testing.T) {
	tests := []struct {
		name          string
		mods          []CommandModifier
		cmdExitStatus int
		cmdStdout     string
		cmdStderr     string
		wantCmdArgs   string
		wantErrorMsg  string
	}{
		{
			name:        "fetch",
			wantCmdArgs: `path/to/git -c credential.useHttpPath=true -c credential.helper=!"azdo" auth git-credential fetch origin trunk`,
		},
		{
			name:        "accepts command modifiers",
			mods:        []CommandModifier{WithRepoDir("/path/to/repo")},
			wantCmdArgs: `path/to/git -C /path/to/repo -c credential.useHttpPath=true -c credential.helper=!"azdo" auth git-credential fetch origin trunk`,
		},
		{
			name:          "git error",
			cmdExitStatus: 1,
			cmdStderr:     "git error message",
			wantCmdArgs:   `path/to/git -c credential.useHttpPath=true -c credential.helper=!"azdo" auth git-credential fetch origin trunk`,
			wantErrorMsg:  "failed to run git: git error message",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, cmdCtx := createCommandContext(t, tt.cmdExitStatus, tt.cmdStdout, tt.cmdStderr)
			client := client{
				GitPath:        "path/to/git",
				commandContext: cmdCtx,
			}
			err := client.Fetch(context.Background(), "origin", "trunk", tt.mods...)
			assert.Equal(t, tt.wantCmdArgs, strings.Join(cmd.Args[3:], " "))
			if tt.wantErrorMsg == "" {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tt.wantErrorMsg)
			}
		})
	}
}

func TestClientPull(t *testing.T) {
	tests := []struct {
		name          string
		mods          []CommandModifier
		cmdExitStatus int
		cmdStdout     string
		cmdStderr     string
		wantCmdArgs   string
		wantErrorMsg  string
	}{
		{
			name:        "pull",
			wantCmdArgs: `path/to/git -c credential.useHttpPath=true -c credential.helper=!"azdo" auth git-credential pull --ff-only origin trunk`,
		},
		{
			name:        "accepts command modifiers",
			mods:        []CommandModifier{WithRepoDir("/path/to/repo")},
			wantCmdArgs: `path/to/git -C /path/to/repo -c credential.useHttpPath=true -c credential.helper=!"azdo" auth git-credential pull --ff-only origin trunk`,
		},
		{
			name:          "git error",
			cmdExitStatus: 1,
			cmdStderr:     "git error message",
			wantCmdArgs:   `path/to/git -c credential.useHttpPath=true -c credential.helper=!"azdo" auth git-credential pull --ff-only origin trunk`,
			wantErrorMsg:  "failed to run git: git error message",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, cmdCtx := createCommandContext(t, tt.cmdExitStatus, tt.cmdStdout, tt.cmdStderr)
			client := client{
				GitPath:        "path/to/git",
				commandContext: cmdCtx,
			}
			err := client.Pull(context.Background(), "origin", "trunk", tt.mods...)
			assert.Equal(t, tt.wantCmdArgs, strings.Join(cmd.Args[3:], " "))
			if tt.wantErrorMsg == "" {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tt.wantErrorMsg)
			}
		})
	}
}

func TestClientPush(t *testing.T) {
	tests := []struct {
		name          string
		mods          []CommandModifier
		cmdExitStatus int
		cmdStdout     string
		cmdStderr     string
		wantCmdArgs   string
		wantErrorMsg  string
	}{
		{
			name:        "push",
			wantCmdArgs: `path/to/git -c credential.useHttpPath=true -c credential.helper=!"azdo" auth git-credential push --set-upstream origin trunk`,
		},
		{
			name:        "accepts command modifiers",
			mods:        []CommandModifier{WithRepoDir("/path/to/repo")},
			wantCmdArgs: `path/to/git -C /path/to/repo -c credential.useHttpPath=true -c credential.helper=!"azdo" auth git-credential push --set-upstream origin trunk`,
		},
		{
			name:          "git error",
			cmdExitStatus: 1,
			cmdStderr:     "git error message",
			wantCmdArgs:   `path/to/git -c credential.useHttpPath=true -c credential.helper=!"azdo" auth git-credential push --set-upstream origin trunk`,
			wantErrorMsg:  "failed to run git: git error message",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, cmdCtx := createCommandContext(t, tt.cmdExitStatus, tt.cmdStdout, tt.cmdStderr)
			client := client{
				GitPath:        "path/to/git",
				commandContext: cmdCtx,
			}
			err := client.Push(context.Background(), "origin", "trunk", tt.mods...)
			assert.Equal(t, tt.wantCmdArgs, strings.Join(cmd.Args[3:], " "))
			if tt.wantErrorMsg == "" {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tt.wantErrorMsg)
			}
		})
	}
}

func TestClientClone(t *testing.T) {
	tests := []struct {
		name          string
		mods          []CommandModifier
		cmdExitStatus int
		cmdStdout     string
		cmdStderr     string
		wantCmdArgs   string
		wantTarget    string
		wantErrorMsg  string
	}{
		{
			name:        "clone",
			wantCmdArgs: `path/to/git -c credential.useHttpPath=true -c credential.helper=!"azdo" auth git-credential clone github.com/cli/cli`,
			wantTarget:  "cli",
		},
		{
			name:        "accepts command modifiers",
			mods:        []CommandModifier{WithRepoDir("/path/to/repo")},
			wantCmdArgs: `path/to/git -C /path/to/repo -c credential.useHttpPath=true -c credential.helper=!"azdo" auth git-credential clone github.com/cli/cli`,
			wantTarget:  "cli",
		},
		{
			name:          "git error",
			cmdExitStatus: 1,
			cmdStderr:     "git error message",
			wantCmdArgs:   `path/to/git -c credential.useHttpPath=true -c credential.helper=!"azdo" auth git-credential clone github.com/cli/cli`,
			wantErrorMsg:  "failed to run git: git error message",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, cmdCtx := createCommandContext(t, tt.cmdExitStatus, tt.cmdStdout, tt.cmdStderr)
			client := client{
				GitPath:        "path/to/git",
				commandContext: cmdCtx,
			}
			target, err := client.Clone(context.Background(), "github.com/cli/cli", []string{}, tt.mods...)
			assert.Equal(t, tt.wantCmdArgs, strings.Join(cmd.Args[3:], " "))
			if tt.wantErrorMsg == "" {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tt.wantErrorMsg)
			}
			assert.Equal(t, tt.wantTarget, target)
		})
	}
}

func TestParseCloneArgs(t *testing.T) {
	type wanted struct {
		args []string
		dir  string
	}
	tests := []struct {
		name string
		args []string
		want wanted
	}{
		{
			name: "args and target",
			args: []string{"target_directory", "-o", "upstream", "--depth", "1"},
			want: wanted{
				args: []string{"-o", "upstream", "--depth", "1"},
				dir:  "target_directory",
			},
		},
		{
			name: "only args",
			args: []string{"-o", "upstream", "--depth", "1"},
			want: wanted{
				args: []string{"-o", "upstream", "--depth", "1"},
				dir:  "",
			},
		},
		{
			name: "only target",
			args: []string{"target_directory"},
			want: wanted{
				args: []string{},
				dir:  "target_directory",
			},
		},
		{
			name: "no args",
			args: []string{},
			want: wanted{
				args: []string{},
				dir:  "",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args, dir := parseCloneArgs(tt.args)
			got := wanted{args: args, dir: dir}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestClientAddRemote(t *testing.T) {
	tests := []struct {
		title         string
		name          string
		url           string
		branches      []string
		dir           string
		cmdExitStatus int
		cmdStdout     string
		cmdStderr     string
		wantCmdArgs   string
		wantErrorMsg  string
	}{
		{
			title:       "fetch all",
			name:        "test",
			url:         "URL",
			dir:         "DIRECTORY",
			branches:    []string{},
			wantCmdArgs: `path/to/git -C DIRECTORY remote add test URL`,
		},
		{
			title:       "fetch specific branches only",
			name:        "test",
			url:         "URL",
			dir:         "DIRECTORY",
			branches:    []string{"trunk", "dev"},
			wantCmdArgs: `path/to/git -C DIRECTORY remote add -t trunk -t dev test URL`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			cmd, cmdCtx := createCommandContext(t, tt.cmdExitStatus, tt.cmdStdout, tt.cmdStderr)
			client := client{
				GitPath:        "path/to/git",
				RepoDir:        tt.dir,
				commandContext: cmdCtx,
			}
			_, err := client.AddRemote(context.Background(), tt.name, tt.url, tt.branches)
			assert.Equal(t, tt.wantCmdArgs, strings.Join(cmd.Args[3:], " "))
			require.NoError(t, err)
		})
	}
}

func initRepo(t *testing.T, dir string) {
	errBuf := &bytes.Buffer{}
	inBuf := &bytes.Buffer{}
	outBuf := &bytes.Buffer{}
	client := client{
		RepoDir: dir,
		Stderr:  errBuf,
		Stdin:   inBuf,
		Stdout:  outBuf,
	}
	cmd, err := client.Command(context.Background(), []string{"init", "--quiet"}...)
	require.NoError(t, err)
	_, err = cmd.Output()
	require.NoError(t, err)
}

func TestHelperProcess(t *testing.T) {
	if os.Getenv("GH_WANT_HELPER_PROCESS") != "1" {
		return
	}
	if err := func(args []string) error {
		fmt.Fprint(os.Stdout, os.Getenv("GH_HELPER_PROCESS_STDOUT"))
		exitStatus := os.Getenv("GH_HELPER_PROCESS_EXIT_STATUS")
		if exitStatus != "0" {
			return errors.New("error")
		}
		return nil
	}(os.Args[3:]); err != nil {
		fmt.Fprint(os.Stderr, os.Getenv("GH_HELPER_PROCESS_STDERR"))
		exitStatus := os.Getenv("GH_HELPER_PROCESS_EXIT_STATUS")
		i, err := strconv.Atoi(exitStatus)
		if err != nil {
			os.Exit(1)
		}
		os.Exit(i)
	}
	os.Exit(0)
}

func createCommandContext(t *testing.T, exitStatus int, stdout, stderr string) (*exec.Cmd, commandCtx) {
	cmd := exec.CommandContext(context.Background(), os.Args[0], "-test.run=TestHelperProcess", "--")
	cmd.Env = []string{
		"GH_WANT_HELPER_PROCESS=1",
		fmt.Sprintf("GH_HELPER_PROCESS_STDOUT=%s", stdout),
		fmt.Sprintf("GH_HELPER_PROCESS_STDERR=%s", stderr),
		fmt.Sprintf("GH_HELPER_PROCESS_EXIT_STATUS=%v", exitStatus),
	}
	return cmd, func(ctx context.Context, exe string, args ...string) *exec.Cmd {
		cmd.Args = append(cmd.Args, exe)
		cmd.Args = append(cmd.Args, args...)
		return cmd
	}
}
