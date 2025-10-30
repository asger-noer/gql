# GQL

A command line tool with utilities for working with GraphQL APIs, schemas and documents.

## Installation

```bash
go install github.com/asger-noer/gql@latest
```

## Usage

The `gql` command provides several subcommands for different utilities. Below are some examples of how to use these subcommands.

### Complexity analysis

Compute the complexity of GraphQL operations in your documents based on a given schema.

```bash
gql complexity -s 'schema.graphqls' -docs '**/*.graphql'
# File:                   Operation:  Complexity:  Flattened Complexity:
# documents/test.graphql  GetTask     21           8
```
