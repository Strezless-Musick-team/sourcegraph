package enqueuer

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/time/rate"

	store "github.com/sourcegraph/sourcegraph/enterprise/internal/codeintel/stores/dbstore"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/repoupdater/protocol"
	"github.com/sourcegraph/sourcegraph/lib/codeintel/precise"
)

var testConfig = Config{
	MaximumRepositoriesInspectedPerSecond:    rate.Inf,
	MaximumIndexJobsPerInferredConfiguration: 50,
}

func TestQueueIndexesForRepositoryInDatabase(t *testing.T) {
	indexConfiguration := store.IndexConfiguration{
		ID:           1,
		RepositoryID: 42,
		Data: []byte(`{
			"shared_steps": [
				{
					"root": "/",
					"image": "node:12",
					"commands": [
						"yarn install --frozen-lockfile --non-interactive",
					],
				}
			],
			"index_jobs": [
				{
					"steps": [
						{
							// Comments are the future
							"image": "go:latest",
							"commands": ["go mod vendor"],
						}
					],
					"indexer": "lsif-go",
					"indexer_args": ["--no-animation"],
				},
				{
					"root": "web/",
					"indexer": "lsif-tsc",
					"indexer_args": ["-p", "."],
					"outfile": "lsif.dump",
				},
			]
		}`),
	}

	mockDBStore := NewMockDBStore()
	mockDBStore.TransactFunc.SetDefaultReturn(mockDBStore, nil)
	mockDBStore.DoneFunc.SetDefaultHook(func(err error) error { return err })
	mockDBStore.GetRepositoriesWithIndexConfigurationFunc.SetDefaultReturn([]int{42}, nil)
	mockDBStore.GetIndexConfigurationByRepositoryIDFunc.SetDefaultReturn(indexConfiguration, true, nil)

	mockGitserverClient := NewMockGitserverClient()
	mockGitserverClient.ResolveRevisionFunc.SetDefaultHook(func(ctx context.Context, repositoryID int, rev string) (api.CommitID, error) {
		return api.CommitID(fmt.Sprintf("c%d", repositoryID)), nil
	})

	scheduler := NewIndexEnqueuer(mockDBStore, mockGitserverClient, nil, &testConfig, &observation.TestContext)
	_ = scheduler.QueueIndexesForRepository(context.Background(), 42)

	if len(mockDBStore.GetIndexConfigurationByRepositoryIDFunc.History()) != 1 {
		t.Errorf("unexpected number of calls to GetIndexConfigurationByRepositoryID. want=%d have=%d", 1, len(mockDBStore.GetIndexConfigurationByRepositoryIDFunc.History()))
	} else {
		var repositoryIDs []int
		for _, call := range mockDBStore.GetIndexConfigurationByRepositoryIDFunc.History() {
			repositoryIDs = append(repositoryIDs, call.Arg1)
		}
		sort.Ints(repositoryIDs)

		if diff := cmp.Diff([]int{42}, repositoryIDs); diff != "" {
			t.Errorf("unexpected repository identifiers (-want +got):\n%s", diff)
		}
	}

	if len(mockDBStore.IsQueuedFunc.History()) != 1 {
		t.Errorf("unexpected number of calls to IsQueued. want=%d have=%d", 1, len(mockDBStore.IsQueuedFunc.History()))
	} else {
		var commits []string
		for _, call := range mockDBStore.IsQueuedFunc.History() {
			commits = append(commits, call.Arg2)
		}
		sort.Strings(commits)

		if diff := cmp.Diff([]string{"c42"}, commits); diff != "" {
			t.Errorf("unexpected commits (-want +got):\n%s", diff)
		}
	}

	if len(mockDBStore.InsertIndexFunc.History()) != 2 {
		t.Errorf("unexpected number of calls to InsertIndex. want=%d have=%d", 2, len(mockDBStore.InsertIndexFunc.History()))
	} else {
		var indexes []store.Index
		for _, call := range mockDBStore.InsertIndexFunc.History() {
			indexes = append(indexes, call.Arg1)
		}

		expectedIndexes := []store.Index{
			{
				RepositoryID: 42,
				Commit:       "c42",
				State:        "queued",
				DockerSteps: []store.DockerStep{
					{
						Root:     "/",
						Image:    "node:12",
						Commands: []string{"yarn install --frozen-lockfile --non-interactive"},
					},
					{
						Image:    "go:latest",
						Commands: []string{"go mod vendor"},
					},
				},
				Indexer:     "lsif-go",
				IndexerArgs: []string{"--no-animation"},
			},
			{
				RepositoryID: 42,
				Commit:       "c42",
				State:        "queued",
				DockerSteps: []store.DockerStep{
					{
						Root:     "/",
						Image:    "node:12",
						Commands: []string{"yarn install --frozen-lockfile --non-interactive"},
					},
				},
				Root:        "web/",
				Indexer:     "lsif-tsc",
				IndexerArgs: []string{"-p", "."},
				Outfile:     "lsif.dump",
			},
		}
		if diff := cmp.Diff(expectedIndexes, indexes); diff != "" {
			t.Errorf("unexpected indexes (-want +got):\n%s", diff)
		}
	}
}

var yamlIndexConfiguration = []byte(`
shared_steps:
  - root: /
    image: node:12
    commands:
      - yarn install --frozen-lockfile --non-interactive

index_jobs:
  -
    steps:
      - image: go:latest
        commands:
          - go mod vendor
    indexer: lsif-go
    indexer_args:
      - --no-animation
  -
    root: web/
    indexer: lsif-tsc
    indexer_args: ['-p', '.']
    outfile: lsif.dump
`)

func TestQueueIndexesForRepositoryInRepository(t *testing.T) {
	mockDBStore := NewMockDBStore()
	mockDBStore.TransactFunc.SetDefaultReturn(mockDBStore, nil)
	mockDBStore.DoneFunc.SetDefaultHook(func(err error) error { return err })
	mockDBStore.GetRepositoriesWithIndexConfigurationFunc.SetDefaultReturn([]int{42}, nil)

	mockGitserverClient := NewMockGitserverClient()
	mockGitserverClient.ResolveRevisionFunc.SetDefaultHook(func(ctx context.Context, repositoryID int, rev string) (api.CommitID, error) {
		return api.CommitID(fmt.Sprintf("c%d", repositoryID)), nil
	})
	mockGitserverClient.FileExistsFunc.SetDefaultHook(func(ctx context.Context, repositoryID int, commit, file string) (bool, error) {
		return file == "sourcegraph.yaml", nil
	})
	mockGitserverClient.RawContentsFunc.SetDefaultReturn(yamlIndexConfiguration, nil)

	scheduler := NewIndexEnqueuer(mockDBStore, mockGitserverClient, nil, &testConfig, &observation.TestContext)

	if err := scheduler.QueueIndexesForRepository(context.Background(), 42); err != nil {
		t.Fatalf("unexpected error performing update: %s", err)
	}

	if len(mockDBStore.IsQueuedFunc.History()) != 1 {
		t.Errorf("unexpected number of calls to IsQueued. want=%d have=%d", 1, len(mockDBStore.IsQueuedFunc.History()))
	} else {
		var commits []string
		for _, call := range mockDBStore.IsQueuedFunc.History() {
			commits = append(commits, call.Arg2)
		}
		sort.Strings(commits)

		if diff := cmp.Diff([]string{"c42"}, commits); diff != "" {
			t.Errorf("unexpected commits (-want +got):\n%s", diff)
		}
	}

	if len(mockDBStore.InsertIndexFunc.History()) != 2 {
		t.Errorf("unexpected number of calls to InsertIndex. want=%d have=%d", 2, len(mockDBStore.InsertIndexFunc.History()))
	} else {
		var indexes []store.Index
		for _, call := range mockDBStore.InsertIndexFunc.History() {
			indexes = append(indexes, call.Arg1)
		}

		expectedIndexes := []store.Index{
			{
				RepositoryID: 42,
				Commit:       "c42",
				State:        "queued",
				DockerSteps: []store.DockerStep{
					{
						Root:     "/",
						Image:    "node:12",
						Commands: []string{"yarn install --frozen-lockfile --non-interactive"},
					},
					{
						Image:    "go:latest",
						Commands: []string{"go mod vendor"},
					},
				},
				Indexer:     "lsif-go",
				IndexerArgs: []string{"--no-animation"},
			},
			{
				RepositoryID: 42,
				Commit:       "c42",
				State:        "queued",
				DockerSteps: []store.DockerStep{
					{
						Root:     "/",
						Image:    "node:12",
						Commands: []string{"yarn install --frozen-lockfile --non-interactive"},
					},
				},
				Root:        "web/",
				Indexer:     "lsif-tsc",
				IndexerArgs: []string{"-p", "."},
				Outfile:     "lsif.dump",
			},
		}
		if diff := cmp.Diff(expectedIndexes, indexes); diff != "" {
			t.Errorf("unexpected indexes (-want +got):\n%s", diff)
		}
	}
}

func TestQueueIndexesForRepositoryInferred(t *testing.T) {
	mockDBStore := NewMockDBStore()
	mockDBStore.TransactFunc.SetDefaultReturn(mockDBStore, nil)
	mockDBStore.DoneFunc.SetDefaultHook(func(err error) error { return err })

	mockGitserverClient := NewMockGitserverClient()
	mockGitserverClient.ResolveRevisionFunc.SetDefaultHook(func(ctx context.Context, repositoryID int, rev string) (api.CommitID, error) {
		return api.CommitID(fmt.Sprintf("c%d", repositoryID)), nil
	})
	mockGitserverClient.ListFilesFunc.SetDefaultHook(func(ctx context.Context, repositoryID int, commit string, pattern *regexp.Regexp) ([]string, error) {
		switch repositoryID {
		case 42:
			return []string{"go.mod"}, nil
		case 44:
			return []string{"a/go.mod", "b/go.mod"}, nil
		default:
			return nil, nil
		}
	})

	scheduler := NewIndexEnqueuer(mockDBStore, mockGitserverClient, nil, &testConfig, &observation.TestContext)

	for _, id := range []int{41, 42, 43, 44} {
		if err := scheduler.QueueIndexesForRepository(context.Background(), id); err != nil {
			t.Fatalf("unexpected error performing update: %s", err)
		}
	}

	if len(mockDBStore.InsertIndexFunc.History()) != 3 {
		t.Errorf("unexpected number of calls to InsertIndex. want=%d have=%d", 3, len(mockDBStore.InsertIndexFunc.History()))
	} else {
		indexRoots := map[int][]string{}
		for _, call := range mockDBStore.InsertIndexFunc.History() {
			indexRoots[call.Arg1.RepositoryID] = append(indexRoots[call.Arg1.RepositoryID], call.Arg1.Root)
		}

		expectedIndexRoots := map[int][]string{
			42: {""},
			44: {"a", "b"},
		}
		if diff := cmp.Diff(expectedIndexRoots, indexRoots); diff != "" {
			t.Errorf("unexpected indexes (-want +got):\n%s", diff)
		}
	}

	if len(mockDBStore.IsQueuedFunc.History()) != 4 {
		t.Errorf("unexpected number of calls to IsQueued. want=%d have=%d", 4, len(mockDBStore.IsQueuedFunc.History()))
	} else {
		var commits []string
		for _, call := range mockDBStore.IsQueuedFunc.History() {
			commits = append(commits, call.Arg2)
		}
		sort.Strings(commits)

		if diff := cmp.Diff([]string{"c41", "c42", "c43", "c44"}, commits); diff != "" {
			t.Errorf("unexpected commits (-want +got):\n%s", diff)
		}
	}
}

func TestQueueIndexesForRepositoryInferredTooLarge(t *testing.T) {
	mockDBStore := NewMockDBStore()
	mockDBStore.TransactFunc.SetDefaultReturn(mockDBStore, nil)
	mockDBStore.DoneFunc.SetDefaultHook(func(err error) error { return err })

	var paths []string
	for i := 0; i < 25; i++ {
		paths = append(paths, fmt.Sprintf("s%d/go.mod", i+1))
	}

	mockGitserverClient := NewMockGitserverClient()
	mockGitserverClient.ResolveRevisionFunc.SetDefaultHook(func(ctx context.Context, repositoryID int, rev string) (api.CommitID, error) {
		return api.CommitID(fmt.Sprintf("c%d", repositoryID)), nil
	})
	mockGitserverClient.ListFilesFunc.SetDefaultHook(func(ctx context.Context, repositoryID int, commit string, pattern *regexp.Regexp) ([]string, error) {
		if repositoryID == 42 {
			return paths, nil
		}

		return nil, nil
	})

	config := testConfig
	config.MaximumIndexJobsPerInferredConfiguration = 20
	scheduler := NewIndexEnqueuer(mockDBStore, mockGitserverClient, nil, &config, &observation.TestContext)

	if err := scheduler.QueueIndexesForRepository(context.Background(), 42); err != nil {
		t.Fatalf("unexpected error performing update: %s", err)
	}

	if len(mockDBStore.InsertIndexFunc.History()) != 0 {
		t.Errorf("unexpected number of calls to InsertIndex. want=%d have=%d", 0, len(mockDBStore.InsertIndexFunc.History()))
	}
}

func TestQueueIndexesForPackage(t *testing.T) {
	mockDBStore := NewMockDBStore()
	mockDBStore.TransactFunc.SetDefaultReturn(mockDBStore, nil)
	mockDBStore.DoneFunc.SetDefaultHook(func(err error) error { return err })
	mockDBStore.IsQueuedFunc.SetDefaultReturn(false, nil)

	mockGitserverClient := NewMockGitserverClient()
	mockGitserverClient.ResolveRevisionFunc.SetDefaultHook(func(ctx context.Context, repoID int, versionString string) (api.CommitID, error) {
		if repoID != 42 || versionString != "4e7eeb0f8a96" {

			t.Errorf("unexpected (repoID, versionString) (%v, %v) supplied to EnqueueRepoUpdate", repoID, versionString)
		}
		return "c42", nil
	})
	mockGitserverClient.ListFilesFunc.SetDefaultReturn([]string{"go.mod"}, nil)

	mockRepoUpdater := NewMockRepoUpdaterClient()
	mockRepoUpdater.EnqueueRepoUpdateFunc.SetDefaultHook(func(ctx context.Context, repoName api.RepoName) (*protocol.RepoUpdateResponse, error) {
		if repoName != "github.com/sourcegraph/sourcegraph" {
			t.Errorf("unexpected repo %v supplied to EnqueueRepoUpdate", repoName)
		}
		return &protocol.RepoUpdateResponse{ID: 42}, nil
	})

	scheduler := NewIndexEnqueuer(mockDBStore, mockGitserverClient, mockRepoUpdater, &testConfig, &observation.TestContext)

	_ = scheduler.QueueIndexesForPackage(context.Background(), precise.Package{
		Scheme:  "gomod",
		Name:    "https://github.com/sourcegraph/sourcegraph",
		Version: "v3.26.0-4e7eeb0f8a96",
	})

	if len(mockDBStore.IsQueuedFunc.History()) != 1 {
		t.Errorf("unexpected number of calls to IsQueued. want=%d have=%d", 1, len(mockDBStore.IsQueuedFunc.History()))
	} else {
		var commits []string
		for _, call := range mockDBStore.IsQueuedFunc.History() {
			commits = append(commits, call.Arg2)
		}
		sort.Strings(commits)

		if diff := cmp.Diff([]string{"c42"}, commits); diff != "" {
			t.Errorf("unexpected commits (-want +got):\n%s", diff)
		}
	}

	if len(mockDBStore.InsertIndexFunc.History()) != 1 {
		t.Errorf("unexpected number of calls to InsertIndex. want=%d have=%d", 1, len(mockDBStore.InsertIndexFunc.History()))
	} else {
		var indexes []store.Index
		for _, call := range mockDBStore.InsertIndexFunc.History() {
			indexes = append(indexes, call.Arg1)
		}

		expectedIndexes := []store.Index{
			{
				RepositoryID: 42,
				Commit:       "c42",
				State:        "queued",
				DockerSteps: []store.DockerStep{
					{
						Image:    "sourcegraph/lsif-go:latest",
						Commands: []string{"go mod download"},
					},
				},
				Indexer:     "sourcegraph/lsif-go:latest",
				IndexerArgs: []string{"lsif-go", "--no-animation"},
			},
		}
		if diff := cmp.Diff(expectedIndexes, indexes); diff != "" {
			t.Errorf("unexpected indexes (-want +got):\n%s", diff)
		}
	}
}
