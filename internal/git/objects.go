package git

import (
	"fmt"
	"net/url"
	"strings"

	"go.uber.org/zap/zapcore"
)

// RemoteSet is a slice of git remotes.
type RemoteSet []*Remote

func (r RemoteSet) Len() int      { return len(r) }
func (r RemoteSet) Swap(i, j int) { r[i], r[j] = r[j], r[i] }
func (r RemoteSet) Less(i, j int) bool {
	return remoteNameSortScore(r[i].Name) > remoteNameSortScore(r[j].Name)
}

func (r RemoteSet) MarshalLogArray(enc zapcore.ArrayEncoder) error {
	for _, remote := range r {
		enc.AppendObject(remote)
	}
	return nil
}

func remoteNameSortScore(name string) int {
	switch strings.ToLower(name) {
	case "upstream":
		return 3
	case "azdo":
		return 2
	case "origin":
		return 1
	default:
		return 0
	}
}

// Remote is a parsed git remote.
type Remote struct {
	Name     string
	Resolved string
	FetchURL *url.URL
	PushURL  *url.URL
}

func (r *Remote) String() string {
	return r.Name
}

func (r *Remote) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("Name", r.Name)
	enc.AddString("Resolved", r.Resolved)
	if r.FetchURL != nil {
		enc.AddString("FetchURL", r.FetchURL.String())
	} else {
		enc.AddString("FetchURL", "")
	}
	if r.PushURL != nil {
		enc.AddString("PushURL", r.PushURL.String())
	} else {
		enc.AddString("PushURL", "")
	}
	return nil
}

func NewRemote(name string, u string) (*Remote, error) {
	pu, err := url.Parse(u)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL %q: %w", u, err)
	}
	return &Remote{
		Name:     name,
		FetchURL: pu,
		PushURL:  pu,
	}, nil
}

// Ref represents a git commit reference.
type Ref struct {
	Hash string
	Name string
}

// TrackingRef represents a ref for a remote tracking branch.
type TrackingRef struct {
	RemoteName string
	BranchName string
}

func (r TrackingRef) String() string {
	return "refs/remotes/" + r.RemoteName + "/" + r.BranchName
}

type Commit struct {
	Sha   string
	Title string
	Body  string
}

type BranchConfig struct {
	RemoteName string
	RemoteURL  *url.URL
	MergeRef   string
}
