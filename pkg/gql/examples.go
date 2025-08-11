package gql

import "fmt"

// Example queries for Caia Library Git Query Language

const (
	// Document queries
	ExampleAllDocuments = `SELECT FROM documents LIMIT 10`
	
	ExampleArxivDocuments = `SELECT FROM documents WHERE source = "arXiv" ORDER BY created_at DESC`
	
	ExampleRecentDocuments = `SELECT FROM documents WHERE created_at > "2024-03-01" LIMIT 50`
	
	ExampleAIDocuments = `SELECT FROM documents WHERE title ~ "artificial intelligence" OR title ~ "machine learning"`
	
	ExampleDocumentsByAuthor = `SELECT FROM documents WHERE author = "John Doe" OR authors ~ "John Doe"`

	// Attribution queries
	ExampleAttributionCompliance = `SELECT FROM attribution WHERE caia_attribution = true`
	
	ExampleAttributionBySource = `SELECT FROM attribution WHERE source = "arXiv" LIMIT 10`
	
	ExampleMissingAttribution = `SELECT FROM attribution WHERE caia_attribution = false`

	// Source queries
	ExampleAllSources = `SELECT FROM sources`
	
	ExampleActiveSource = `SELECT FROM sources WHERE count > 10`

	// Author queries
	ExampleTopAuthors = `SELECT FROM authors ORDER BY count DESC LIMIT 20`
	
	ExampleProlificAuthors = `SELECT FROM authors WHERE count > 5`
)

// QueryExamples provides example queries with descriptions
var QueryExamples = []struct {
	Name        string
	Query       string
	Description string
}{
	{
		Name:        "All Documents",
		Query:       ExampleAllDocuments,
		Description: "Retrieve the 10 most recent documents",
	},
	{
		Name:        "arXiv Papers",
		Query:       ExampleArxivDocuments,
		Description: "Find all documents from arXiv, ordered by date",
	},
	{
		Name:        "Recent Documents",
		Query:       ExampleRecentDocuments,
		Description: "Documents collected after March 1, 2024",
	},
	{
		Name:        "AI Research",
		Query:       ExampleAIDocuments,
		Description: "Documents about AI or machine learning",
	},
	{
		Name:        "Attribution Compliance",
		Query:       ExampleAttributionCompliance,
		Description: "Check which sources have proper Caia Tech attribution",
	},
	{
		Name:        "Source Statistics",
		Query:       ExampleAllSources,
		Description: "List all document sources with counts",
	},
	{
		Name:        "Top Authors",
		Query:       ExampleTopAuthors,
		Description: "Find the most published authors in the library",
	},
}

// QueryBuilder helps construct GQL queries programmatically
type QueryBuilder struct {
	queryType  QueryType
	filters    []Filter
	orderBy    string
	descending bool
	limit      int
}

// NewQueryBuilder creates a new query builder
func NewQueryBuilder(queryType QueryType) *QueryBuilder {
	return &QueryBuilder{
		queryType: queryType,
		limit:     100,
	}
}

// Where adds a filter condition
func (qb *QueryBuilder) Where(field string, op Operator, value interface{}) *QueryBuilder {
	qb.filters = append(qb.filters, Filter{
		Field:    field,
		Operator: op,
		Value:    value,
	})
	return qb
}

// OrderBy sets the sort field
func (qb *QueryBuilder) OrderBy(field string, desc bool) *QueryBuilder {
	qb.orderBy = field
	qb.descending = desc
	return qb
}

// Limit sets the result limit
func (qb *QueryBuilder) Limit(limit int) *QueryBuilder {
	qb.limit = limit
	return qb
}

// Build constructs the GQL query string
func (qb *QueryBuilder) Build() string {
	query := "SELECT FROM " + string(qb.queryType)

	// Add WHERE clause
	if len(qb.filters) > 0 {
		query += " WHERE "
		for i, filter := range qb.filters {
			if i > 0 {
				query += " AND "
			}
			query += filter.Field + " " + string(filter.Operator) + " "
			
			// Format value
			switch v := filter.Value.(type) {
			case string:
				query += `"` + v + `"`
			case bool:
				if v {
					query += "true"
				} else {
					query += "false"
				}
			default:
				query += fmt.Sprintf("%v", v)
			}
		}
	}

	// Add ORDER BY
	if qb.orderBy != "" {
		query += " ORDER BY " + qb.orderBy
		if qb.descending {
			query += " DESC"
		} else {
			query += " ASC"
		}
	}

	// Add LIMIT
	if qb.limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", qb.limit)
	}

	return query
}