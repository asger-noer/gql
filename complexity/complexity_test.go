package complexity_test

import (
	"testing"

	"github.com/asger-noer/gql/complexity"
	"github.com/google/go-cmp/cmp"
	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/parser"
)

const (
	schema = `type Query {
		user(id: ID!): User
	}

	type User {
		id: ID!
		name: String!
	}
	`

	fragmentedQuery = `query GetOrder($id: ID!) {
		user(id: $id) {
			...HeaderFragment
			...UserFragment
		}
	}

	fragment HeaderFragment on User {
		id
		name
	}

	fragment UserFragment on User {
		id
		name
	}`
)

var (
	schemaSource = ast.Source{
		Name:    "schema.graphql",
		Input:   schema,
		BuiltIn: false,
	}
	fragmentedQuerySource = ast.Source{
		Name:    "fragmentedQuery.graphql",
		Input:   fragmentedQuery,
		BuiltIn: false,
	}
)

func TestAnalyseDocument(t *testing.T) {
	schemaDoc, err := gqlparser.LoadSchema(&schemaSource)
	if err != nil {
		t.Fatalf("failed to load schema: %v", err)
	}

	queryDoc, err := parser.ParseQuery(&fragmentedQuerySource)
	if err != nil {
		t.Fatalf("failed to parse query: %v", err)
	}

	result, err := complexity.AnalyseDocument(t.Context(), schemaDoc, queryDoc)
	if err != nil {
		t.Fatalf("failed to analyse document: %v", err)
	}

	expected := []complexity.DocumentAnalysis{
		{
			OperationName:       "GetOrder",
			Complexity:          5,
			FlattenedComplexity: 3,
		},
	}

	if diff := cmp.Diff(expected, result); diff != "" {
		t.Errorf("AnalyseDocument() mismatch (-want +got):\n%s", diff)
	}
}
