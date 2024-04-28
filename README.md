# Omnirepo

![test workflow](https://github.com/mitchelldw01/omnirepo/actions/workflows/test.yaml/badge.svg)
![release workflow](https://github.com/mitchelldw01/omnirepo/actions/workflows/release.yaml/badge.svg)
![lint workflow](https://github.com/mitchelldw01/omnirepo/actions/workflows/lint.yaml/badge.svg)

Omnirepo is a task runner that executes tasks in topological order, parallelizing them when possible, according to the dependencies between them. It was inspired by monorepo tools like [Turborepo](https://turbo.build/repo) and [Bazel](https://bazel.build), but is flexible enough to be used with any programming language(s).

It uses caching to prevent rerunning tasks when nothing has changed, and to restore artifacts produced by tasks. In addition to local caching with the user's file system, remote caching is possible with AWS [S3](https://aws.amazon.com/s3/) and [DynamoDB](https://aws.amazon.com/dynamodb/). Remote caching enables shared caches between different environments such as CI pipelines.

![example](./example.svg)

## Installation

Users on UNIX-based systems can use the following command to install Omnirepo:

```sh
curl -sSL https://raw.githubusercontent.com/mitchelldw01/omnirepo/main/install.sh | sh
```

Go developers can build from source:

```sh
go install github.com/mitchelldw01/omnirepo@latest
```

For Windows users, you can download the appropriate tarball from the [releases](https://github.com/mitchelldw01/omnirepo/releases) page. Extract the contents and move the binary executable somewhere in your path.

## Getting Started

This section covers a basic usage of Omnirepo. See [Configuration](#configuration) for more configuration options.

Clone the example repository for a working example:

```sh
git clone https://github.com/mitchelldw01/omnirepo-example
```

Omnirepo assumes a monorepo-like project structure, where the root, or "workspace" directory contains subdirectories for the apps, libraries, etc. in the project. These subdirectories are called "target" directories.

The example repository contains three directories: foo, bar, and baz. The `targets` property in the `omni-workspace.yaml` indicates that these three directories are target directories. Each target directory contains an `omni-target.yaml` that defines the tasks for the directory and the dependencies between them.

The `dependencies` property in `foo/omni-target.yaml` indicates that `foo` depends on both `bar` and `baz`. The `dependencies` property alone does not indicate dependencies between tasks, it just potentiates them. The pipeline in `foo/omni-target.yaml` contains a `test` task, where the `dependsOn` property indicates that the task depends on all `test` tasks from targets that the current target (`foo`) depends on (`bar` and `baz`).

Now that we understand the dependency structure of the project, use the following command to run the `test` tasks:

```
omni test
```

On the initial run, each task will be executed and a cache will be populated. On subsequent runs, assuming no changes have been made, the cached outputs will be replayed without rerunning the tasks.

Making changes to the files defined in `workspaceAssets` or `inputs` will invalidate the cache. For example, if you were to make a change to `bar/omni-target.yaml`, that would invalidate the cache for both `bar:test` and `foo:test`, resulting in both tasks being rerun. You can test this by adding a comment to `bar/omni-target.yaml` and rerunning the test tasks.

Sometimes you want to run a task for a subset of target directories. The `-t, --target` option lets you specify a target directory to start from. For example, the following command will run the `test` task for the `bar` directory:

```sh
omni --target bar test
```

For more command line options, see [Command Line Reference](#command-line-reference) or run the `omni --help` command.

## Configuration

### Workspace Configuration

Configuration options for the workspace are defined in an `omni-workspace.yaml` file located in the root of the project.

- `name`: The name of the project. This is only required when remote caching is enabled.
- `targets`: Paths to the target directories in the workspace.
- `remoteCache`: Remote cache configuration options.
    - `enabled`: Indicates whether remote caching is enabled.
    - `bucket`: The name of the S3 bucket to use for caching. Omnirepo will not create this bucket for you.
    - `table`: The name of the DynamoDB table to use for cache locking. Omnirepo will not create this table for you.
    - `region`: The AWS region to use. You can omit this property to use the default region.

> The DynamoDB table for remote caching should be configured with a string partition key named `WorkspaceName`.

```yaml
# omni-workspace.yaml
name: sample-project
targets:
    - foo
    - bar
remoteCache:
    bucket: my-bucket
    table: my-table
    region: us-east-1
```

### Target Configuration

Configuration options for target directories are defined in an `omni-target.yaml` file located in the root of each target directory.

- `dependencies`: Paths to other target directories that this target depends on (relative to workspace root).
- `workspaceAssets`: Patterns matching files outside of target directories that should be included in the cache for this target (relative to the workspace root).
- `pipeline`: Map from task names to their configuration options.
    - `command`: The shell command to run for this task. PowerShell is used for windows, otherwise Bash is used.
    - `dependsOn`: List of tasks that this task depends on. The `^` prefix indicates a dependency on tasks from other target directories, while the absence of the prefix indicates a dependency on a task from this target directory.
    - `includes`: Patterns matching files to be included in the cache for this task (relative to the target root).
    - `excludes`: Patterns matching files to be excluded from the cache for this task (relative to the target root). This property takes priority over `includes`.
    - `outputs`: Patterns matching files that this task produces.

```yaml
# omni-target.yaml
dependencies:
    - bar
workspaceAssets:
    - omni-workspace.yaml
pipeline:
    build:
        command: "echo 'running build command'"
        dependsOn:
            - ^build
        includes:
            - "src/**/*"
        excludes:
            - "src/**/*.test"
```

#### Pattern Behavior

This section details the behavior of patterns in configuration files via [doublestar](https://github.com/bmatcuk/doublestar).

Special Terms | Meaning
------------- | -------
`*`           | matches any sequence of non-path-separators
`/**/`        | matches zero or more directories
`?`           | matches any single non-path-separator character
`[class]`     | matches any single non-path-separator character against a class of characters ([see "character classes"])
`{alt1,...}`  | matches a sequence of characters if one of the comma-separated alternatives matches

Any character with a special meaning can be escaped with a backslash (`\`).

A doublestar (`**`) should appear surrounded by path separators such as `/**/`.
A mid-pattern doublestar (`**`) behaves like bash's globstar option: a pattern
such as `path/to/**.txt` would return the same results as `path/to/*.txt`. The
pattern you're looking for is `path/to/**/*.txt`.

Character classes support the following:

Class      | Meaning
---------- | -------
`[abc]`    | matches any single character within the set
`[a-z]`    | matches any single character in the range
`[^class]` | matches any single character which does *not* match the class
`[!class]` | same as `^`: negates the class

## Command Line Reference

Options must be provided before the command and/or arguments. Otherwise, the options will be not be parsed correctly.

### Commands

- `unlock`: Forcefully unlock the cache. This command should only be used when you're positive that the cache lock was not freed properly.
- `tree` Show the dependency graph as JSON. This command can be useful for debugging or visualizing a complicated dependency tree.
- `run` Run tasks (default). The only time this needs to be used explicilty is when you want to run a task that's name conflicts with another command.

### Options

- `-h, --help`: Show help
- `--no-cache`: Invalidate the cache before running tasks
- `--no-color`: Disable color output
- `-r, --remote`: Use remote cache
- `-t, --target <PATH>`: Load tasks from a specific target directory
- `-v, --version`: Show version
