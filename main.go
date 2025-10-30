package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"text/tabwriter"

	"github.com/asger-noer/gql/complexity"
	"github.com/urfave/cli/v3"
)

const (
	ComplexityCommandName        = "complexity"
	ComplexityCommandUsage       = "Analyze GraphQL query complexity"
	ComplexityCommandDescription = `Analyze the complexity of GraphQL operations based on the provided schema.

The complexity is calculated using the folling rules from gqlgen:
- Each field has a base complexity of 1.
- Interfaces have the complexity of their most complex implementing type.`
)

func main() {
	ctx := context.Background()

	cmd := &cli.Command{
		Name:  "gql",
		Usage: "GraphQL utilities",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "schema",
				Aliases: []string{"s"},
				Usage:   "Glob pattern to search for graphql schema files",
				Value:   "*.graphqls",
			},
		},
		Commands: []*cli.Command{
			{
				Name:        ComplexityCommandName,
				Usage:       ComplexityCommandUsage,
				Description: ComplexityCommandDescription,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "docs",
						Usage: "Glob pattern to search for graphql files",
						Value: "*.graphql",
					},
				},
				Action: func(ctx context.Context, c *cli.Command) error {
					var (
						schemaFind = c.String("schema")
						docFind    = c.String("docs")
					)

					result, err := complexity.RunAnalysis(ctx, schemaFind, docFind)
					if err != nil {
						return cli.Exit("Unable to calculate complexity", 1)
					}

					w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
					fmt.Fprintf(w, "File:\tOperation:\tComplexity:\tFlattened Complexity:\n")
					defer w.Flush()

					for _, r := range result {
						fmt.Fprintf(w, "%s\t%s\t%d\t%d\n", r.Path, r.OperationName, r.Complexity, r.FlattenedComplexity)
						if err := w.Flush(); err != nil {
							return cli.Exit("Unable to flush writer", 1)
						}
					}

					return nil
				},
			},
		},
	}

	if err := cmd.Run(ctx, os.Args); err != nil {
		log.Fatal(err)
	}
}
