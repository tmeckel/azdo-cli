#!/usr/bin/env bash
set -euo pipefail

# Regenerate mocks with mockgen. Requires mockgen installed:
#   go install go.uber.org/mock/mockgen@latest

echo "Generating ConnectionFactory mock..."
mockgen -source internal/azdo/connection.go \
  -package=mocks -destination internal/mocks/connection_factory_mock.go

echo "Generating Extensions mock..."
mockgen -source internal/azdo/extensions/extension.go \
  -package=mocks -destination internal/mocks/extension_mock.go \
  -mock_names Client=MockAzDOExtension

echo "Generating CmdContext/RepoContext mocks..."
mockgen -source internal/cmd/util/cmd_context.go \
  -package=mocks -destination internal/mocks/cmd_context_mock.go

echo "Generating Azure DevOps Git client mock..."
mockgen  \
  -package=mocks -destination internal/mocks/azdogit_client_mock.go \
  -mock_names Client=MockAzDOGitClient \
  github.com/microsoft/azure-devops-go-api/azuredevops/v7/git Client

echo "Generating Azure DevOps Identity client mock..."
mockgen \
  -package=mocks -destination internal/mocks/identity_client_mock.go \
  -mock_names Client=MockIdentityClient \
  github.com/microsoft/azure-devops-go-api/azuredevops/v7/identity Client

echo "Generating Repository mock..."
mockgen -source internal/azdo/repo.go \
  -package=mocks -destination internal/mocks/repository_mock.go

echo "Generating Git Client mock..."
mockgen -source internal/git/client.go \
  -mock_names Client=MockGitCmdClient \
  -package=mocks -destination internal/mocks/git_command_mock.go

echo "Done."
