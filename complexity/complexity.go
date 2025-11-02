// Package complexity provides functionality to analyze the complexity of GraphQL operations
package complexity

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"os"

	"github.com/99designs/gqlgen/complexity"
	"github.com/99designs/gqlgen/graphql"
	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/parser"
	"github.com/vektah/gqlparser/v2/validator"
	"github.com/vektah/gqlparser/v2/validator/rules"
)

// ComplexityAnalysis holds the complexity analysis result for a single operation
type ComplexityAnalysis struct {
	Path                string
	OperationName       string
	Complexity          int
	FlattenedComplexity int
}

func RunAnalysis(ctx context.Context, schema, docs string) ([]ComplexityAnalysis, error) {
	schemas, err := fs.Glob(os.DirFS("."), schema)
	if err != nil {
		return nil, fmt.Errorf("globbing schema files: %w", err)
	}

	var inputs []*ast.Source
	for _, schemaPath := range schemas {
		fileBytes, err := os.ReadFile(schemaPath)
		if err != nil {
			return nil, fmt.Errorf("reading schema file %s: %w", schemaPath, err)
		}

		inputs = append(inputs, &ast.Source{Input: string(fileBytes), Name: schemaPath, BuiltIn: false})
	}

	schemaDoc, err := gqlparser.LoadSchema(inputs...)
	if err != nil {
		return nil, fmt.Errorf("loading schema: %w", err)
	}

	matches, err := fs.Glob(os.DirFS("."), docs)
	if err != nil {
		return nil, fmt.Errorf("globbing documents files: %w", err)
	}

	var results []ComplexityAnalysis
	for _, match := range matches {
		fileBytes, err := os.ReadFile(match)
		if err != nil {
			slog.Warn("Reading query file", "file", match, "error", err)
			continue
		}

		source := ast.Source{Input: string(fileBytes), Name: match, BuiltIn: false}

		queryDoc, err := parser.ParseQuery(&source)
		if err != nil {
			slog.Warn("Parsing query", "file", match, "error", err)
			continue
		}

		analysis, err := AnalyseDocument(ctx, schemaDoc, queryDoc)
		if err != nil {
			slog.Warn("Analysing document", "file", match, "error", err)
			continue
		}

		for _, res := range analysis {
			results = append(results, ComplexityAnalysis{
				Path:                match,
				OperationName:       res.OperationName,
				Complexity:          res.Complexity,
				FlattenedComplexity: res.FlattenedComplexity,
			})
		}
	}

	return results, nil
}

type DocumentAnalysis struct {
	OperationName       string
	Complexity          int
	FlattenedComplexity int
}

func AnalyseDocument(ctx context.Context, schemaDoc *ast.Schema, queryDoc *ast.QueryDocument) ([]DocumentAnalysis, error) {
	if err := validator.ValidateWithRules(schemaDoc, queryDoc, rules.NewDefaultRules()); err != nil {
		return nil, fmt.Errorf("validating query document: %w", err)
	}

	s := graphql.ExecutableSchemaMock{
		ComplexityFunc: func(ctx context.Context, typeName string, fieldName string, childComplexity int, args map[string]any) (int, bool) {
			return childComplexity + 1, true
		},
		ExecFunc:   func(ctx context.Context) graphql.ResponseHandler { return nil },
		SchemaFunc: func() *ast.Schema { return schemaDoc },
	}

	var documentResults []DocumentAnalysis
	for _, op := range queryDoc.Operations {
		flatOp := flatten(queryDoc, op)

		documentResults = append(documentResults, DocumentAnalysis{
			OperationName:       op.Name,
			Complexity:          complexity.Calculate(ctx, &s, op, nil),
			FlattenedComplexity: complexity.Calculate(ctx, &s, flatOp, nil),
		})
	}
	return documentResults, nil
}

// flatten will flatten the operation by inlining all fragments.
func flatten(doc *ast.QueryDocument, op *ast.OperationDefinition) *ast.OperationDefinition {
	// Create a deep copy of the operation
	flattened := &ast.OperationDefinition{
		Operation:           op.Operation,
		Name:                op.Name,
		VariableDefinitions: make([]*ast.VariableDefinition, len(op.VariableDefinitions)),
		Directives:          make(ast.DirectiveList, len(op.Directives)),
		SelectionSet:        flattenSelectionSet(op.SelectionSet, doc),
		Position:            op.Position,
		Comment:             op.Comment,
	}

	// Copy variable definitions
	copy(flattened.VariableDefinitions, op.VariableDefinitions)

	// Copy directives
	copy(flattened.Directives, op.Directives)

	return flattened
}

// flattenSelectionSet recursively flattens a selection set by inlining fragments
func flattenSelectionSet(selectionSet ast.SelectionSet, doc *ast.QueryDocument) ast.SelectionSet {
	fieldMap := make(map[string]*ast.Field)

	for _, selection := range selectionSet {
		switch sel := selection.(type) {
		case *ast.Field:
			// Create a key for deduplication based on field name and alias
			key := sel.Name
			if sel.Alias != "" {
				key = sel.Alias + ":" + sel.Name
			}

			// If we've seen this field before, merge their selection sets
			if existing, exists := fieldMap[key]; exists {
				// Merge selection sets
				mergedSelectionSet := make(ast.SelectionSet, 0)
				mergedSelectionSet = append(mergedSelectionSet, existing.SelectionSet...)
				mergedSelectionSet = append(mergedSelectionSet, sel.SelectionSet...)

				existing.SelectionSet = flattenSelectionSet(mergedSelectionSet, doc)
				continue
			}

			// For fields, recursively flatten their selection sets
			flattenedField := &ast.Field{
				Alias:            sel.Alias,
				Name:             sel.Name,
				Arguments:        sel.Arguments,
				Directives:       sel.Directives,
				SelectionSet:     flattenSelectionSet(sel.SelectionSet, doc),
				Position:         sel.Position,
				Comment:          sel.Comment,
				Definition:       sel.Definition,
				ObjectDefinition: sel.ObjectDefinition,
			}
			fieldMap[key] = flattenedField

		case *ast.InlineFragment:
			// For inline fragments, flatten their selection sets and merge them directly
			fragmentSelections := flattenSelectionSet(sel.SelectionSet, doc)
			for _, fragSel := range fragmentSelections {
				if field, ok := fragSel.(*ast.Field); ok {
					key := field.Name
					if field.Alias != "" {
						key = field.Alias + ":" + field.Name
					}

					if existing, exists := fieldMap[key]; exists {
						// Merge selection sets
						mergedSelectionSet := make(ast.SelectionSet, 0)
						mergedSelectionSet = append(mergedSelectionSet, existing.SelectionSet...)
						mergedSelectionSet = append(mergedSelectionSet, field.SelectionSet...)

						existing.SelectionSet = flattenSelectionSet(mergedSelectionSet, doc)

						continue
					}

					fieldMap[key] = field
				}
			}

		case *ast.FragmentSpread:
			// For fragment spreads, find the fragment definition and inline its selections
			if fragDef := findFragmentDefinition(doc, sel.Name); fragDef != nil {
				fragmentSelections := flattenSelectionSet(fragDef.SelectionSet, doc)
				for _, fragSel := range fragmentSelections {
					if field, ok := fragSel.(*ast.Field); ok {
						key := field.Name
						if field.Alias != "" {
							key = field.Alias + ":" + field.Name
						}

						if existing, exists := fieldMap[key]; exists {
							// Merge selection sets
							mergedSelectionSet := make(ast.SelectionSet, 0)
							mergedSelectionSet = append(mergedSelectionSet, existing.SelectionSet...)
							mergedSelectionSet = append(mergedSelectionSet, field.SelectionSet...)

							existing.SelectionSet = flattenSelectionSet(mergedSelectionSet, doc)
							continue
						}

						fieldMap[key] = field
					}
				}
			}
		}
	}

	// Convert map back to selection set
	var flattened ast.SelectionSet
	for _, field := range fieldMap {
		flattened = append(flattened, field)
	}

	return flattened
}

// findFragmentDefinition finds a fragment definition by name in the document
func findFragmentDefinition(doc *ast.QueryDocument, name string) *ast.FragmentDefinition {
	for _, frag := range doc.Fragments {
		if frag.Name == name {
			return frag
		}
	}
	return nil
}
