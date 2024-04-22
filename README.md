# Omnirepo

![test workflow](https://github.com/mitchelldw01/omnirepo/actions/workflows/test.yaml/badge.svg)
![lint workflow](https://github.com/mitchelldw01/omnirepo/actions/workflows/lint.yaml/badge.svg)

Inspired by monorepo tools like [Turborepo](https://turbo.build/repo) and [Bazel](https://bazel.build), Omnirepo is task runner that executes tasks in topological order according to the dependencies between them. It is flexible enough to be used with any programming language(s), and uses caching to prevent rerunning tasks when nothing has changed. In addition to local caching with the user's file system, remote caching is possible with [AWS S3](https://aws.amazon.com/s3/). Remote caching enables shared caches between different environments such as CI pipelines.
