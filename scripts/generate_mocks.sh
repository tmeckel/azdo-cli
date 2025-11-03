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

echo "Generating Azure DevOps Core client mock..."
mockgen \
  -package=mocks -destination internal/mocks/core_client_mock.go \
  -mock_names Client=MockCoreClient \
  github.com/microsoft/azure-devops-go-api/azuredevops/v7/core Client

echo "Generating Azure DevOps Graph client mock..."
mockgen \
  -package=mocks -destination internal/mocks/graph_client_mock.go \
  -mock_names Client=MockGraphClient \
  github.com/microsoft/azure-devops-go-api/azuredevops/v7/graph Client

echo "Generating Azure DevOps Operations client mock..."
mockgen \
  -package=mocks -destination internal/mocks/operations_client_mock.go \
  -mock_names Client=MockOperationsClient \
  github.com/microsoft/azure-devops-go-api/azuredevops/v7/operations Client

echo "Generating Azure DevOps Identity client mock..."
mockgen \
  -package=mocks -destination internal/mocks/identity_client_mock.go \
  -mock_names Client=MockIdentityClient \
  github.com/microsoft/azure-devops-go-api/azuredevops/v7/identity Client

echo "Generating Azure DevOps Security client mock..."
mockgen \
  -package=mocks -destination internal/mocks/security_client_mock.go \
  -mock_names Client=MockSecurityClient \
  github.com/microsoft/azure-devops-go-api/azuredevops/v7/security Client

echo "Generating Repository mock..."
mockgen -source internal/azdo/repo.go \
  -package=mocks -destination internal/mocks/repository_mock.go

echo "Generating Git Client mock..."
mockgen -source internal/git/client.go \
  -mock_names Client=MockGitCmdClient \
  -package=mocks -destination internal/mocks/git_command_mock.go

echo "Generating Config mock..."
mockgen -source internal/config/config.go \
  -mock_names Client=MockConfig \
  -package=mocks -destination internal/mocks/config_mock.go

echo "Generating Alias Config mock..."
mockgen -source internal/config/alias_config.go \
  -mock_names Client=MockConfigAlias \
  -package=mocks -destination internal/mocks/config_alias.go

echo "Generating Auth Config mock..."
mockgen -source internal/config/auth_config.go \
  -mock_names Client=MockAuthConfig \
  -package=mocks -destination internal/mocks/auth_config.go

echo "Generating Printer mock..."
mockgen -source internal/printer/printer.go \
  -mock_names Client=MockPrinter \
  -package=mocks -destination internal/mocks/printer.go

echo "Generating Prompter mock..."
mockgen -source internal/prompter/prompter.go \
  -package=mocks -destination internal/mocks/prompter_mock.go \
  -mock_names Prompter=MockPrompter

echo "Done."
