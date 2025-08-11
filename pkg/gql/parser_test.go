package gql

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParser_Parse(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name    string
		query   string
		want    *Query
		wantErr bool
	}{
		{
			name:  "simple document query",
			query: `SELECT FROM documents LIMIT 10`,
			want: &Query{
				Type:  QueryDocuments,
				Limit: 10,
			},
			wantErr: false,
		},
		{
			name:  "document query with filter",
			query: `SELECT FROM documents WHERE source = "arXiv"`,
			want: &Query{
				Type: QueryDocuments,
				Filters: []Filter{
					{Field: "source", Operator: OpEquals, Value: "arXiv"},
				},
				Limit: 100,
			},
			wantErr: false,
		},
		{
			name:  "query with multiple filters",
			query: `SELECT FROM documents WHERE source = "arXiv" AND title ~ "neural"`,
			want: &Query{
				Type: QueryDocuments,
				Filters: []Filter{
					{Field: "source", Operator: OpEquals, Value: "arXiv"},
					{Field: "title", Operator: OpContains, Value: "neural"},
				},
				Limit: 100,
			},
			wantErr: false,
		},
		{
			name:  "query with order by",
			query: `SELECT FROM documents ORDER BY created_at DESC`,
			want: &Query{
				Type:       QueryDocuments,
				OrderBy:    "created_at",
				Descending: true,
				Limit:      100,
			},
			wantErr: false,
		},
		{
			name:  "attribution query",
			query: `SELECT FROM attribution WHERE caia_attribution = true`,
			want: &Query{
				Type: QueryAttribution,
				Filters: []Filter{
					{Field: "caia_attribution", Operator: OpEquals, Value: true},
				},
				Limit: 100,
			},
			wantErr: false,
		},
		{
			name:  "sources query",
			query: `SELECT FROM sources`,
			want: &Query{
				Type:  QuerySources,
				Limit: 100,
			},
			wantErr: false,
		},
		{
			name:  "authors query with limit",
			query: `SELECT FROM authors ORDER BY count DESC LIMIT 20`,
			want: &Query{
				Type:       QueryAuthors,
				OrderBy:    "count",
				Descending: true,
				Limit:      20,
			},
			wantErr: false,
		},
		{
			name:    "missing SELECT",
			query:   `FROM documents`,
			wantErr: true,
		},
		{
			name:    "missing FROM",
			query:   `SELECT documents`,
			wantErr: true,
		},
		{
			name:    "invalid query type",
			query:   `SELECT FROM invalid_type`,
			wantErr: true,
		},
		{
			name:    "empty query",
			query:   ``,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parser.Parse(tt.query)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want.Type, got.Type)
			assert.Equal(t, tt.want.Limit, got.Limit)
			assert.Equal(t, tt.want.OrderBy, got.OrderBy)
			assert.Equal(t, tt.want.Descending, got.Descending)
			assert.Equal(t, len(tt.want.Filters), len(got.Filters))
			
			for i, filter := range tt.want.Filters {
				assert.Equal(t, filter.Field, got.Filters[i].Field)
				assert.Equal(t, filter.Operator, got.Filters[i].Operator)
				assert.Equal(t, filter.Value, got.Filters[i].Value)
			}
		})
	}
}

func TestParser_Tokenize(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name    string
		query   string
		want    []token
		wantErr bool
	}{
		{
			name:  "simple query",
			query: `SELECT FROM documents`,
			want: []token{
				{typ: tokenKeyword, value: "SELECT"},
				{typ: tokenKeyword, value: "FROM"},
				{typ: tokenKeyword, value: "DOCUMENTS"},
				{typ: tokenEOF, value: ""},
			},
			wantErr: false,
		},
		{
			name:  "query with string",
			query: `WHERE source = "arXiv"`,
			want: []token{
				{typ: tokenKeyword, value: "WHERE"},
				{typ: tokenIdentifier, value: "source"},
				{typ: tokenOperator, value: "="},
				{typ: tokenString, value: "arXiv"},
				{typ: tokenEOF, value: ""},
			},
			wantErr: false,
		},
		{
			name:  "query with number",
			query: `LIMIT 10`,
			want: []token{
				{typ: tokenKeyword, value: "LIMIT"},
				{typ: tokenNumber, value: "10"},
				{typ: tokenEOF, value: ""},
			},
			wantErr: false,
		},
		{
			name:  "operators",
			query: `a = b AND c != d AND e ~ f`,
			want: []token{
				{typ: tokenIdentifier, value: "a"},
				{typ: tokenOperator, value: "="},
				{typ: tokenIdentifier, value: "b"},
				{typ: tokenKeyword, value: "AND"},
				{typ: tokenIdentifier, value: "c"},
				{typ: tokenOperator, value: "!="},
				{typ: tokenIdentifier, value: "d"},
				{typ: tokenKeyword, value: "AND"},
				{typ: tokenIdentifier, value: "e"},
				{typ: tokenOperator, value: "~"},
				{typ: tokenIdentifier, value: "f"},
				{typ: tokenEOF, value: ""},
			},
			wantErr: false,
		},
		{
			name:    "unterminated string",
			query:   `WHERE source = "arXiv`,
			wantErr: true,
		},
		{
			name:    "invalid character",
			query:   `SELECT @ FROM`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parser.tokenize(tt.query)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, len(tt.want), len(got))
			
			for i, token := range tt.want {
				assert.Equal(t, token.typ, got[i].typ, "token %d type mismatch", i)
				assert.Equal(t, token.value, got[i].value, "token %d value mismatch", i)
			}
		})
	}
}

func TestQueryBuilder_Build(t *testing.T) {
	tests := []struct {
		name  string
		build func() string
		want  string
	}{
		{
			name: "simple query",
			build: func() string {
				return NewQueryBuilder(QueryDocuments).Build()
			},
			want: `SELECT FROM documents LIMIT 100`,
		},
		{
			name: "query with filter",
			build: func() string {
				return NewQueryBuilder(QueryDocuments).
					Where("source", OpEquals, "arXiv").
					Build()
			},
			want: `SELECT FROM documents WHERE source = "arXiv" LIMIT 100`,
		},
		{
			name: "query with multiple filters",
			build: func() string {
				return NewQueryBuilder(QueryDocuments).
					Where("source", OpEquals, "arXiv").
					Where("caia_attribution", OpEquals, true).
					Build()
			},
			want: `SELECT FROM documents WHERE source = "arXiv" AND caia_attribution = true LIMIT 100`,
		},
		{
			name: "query with order by",
			build: func() string {
				return NewQueryBuilder(QueryDocuments).
					OrderBy("created_at", true).
					Build()
			},
			want: `SELECT FROM documents ORDER BY created_at DESC LIMIT 100`,
		},
		{
			name: "complex query",
			build: func() string {
				return NewQueryBuilder(QueryDocuments).
					Where("source", OpEquals, "arXiv").
					Where("title", OpContains, "neural").
					OrderBy("created_at", true).
					Limit(20).
					Build()
			},
			want: `SELECT FROM documents WHERE source = "arXiv" AND title ~ "neural" ORDER BY created_at DESC LIMIT 20`,
		},
		{
			name: "attribution query",
			build: func() string {
				return NewQueryBuilder(QueryAttribution).
					Where("caia_attribution", OpEquals, true).
					Build()
			},
			want: `SELECT FROM attribution WHERE caia_attribution = true LIMIT 100`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.build()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestOperators(t *testing.T) {
	// Test all operators are recognized
	operators := []struct {
		op       Operator
		symbol   string
		example  string
	}{
		{OpEquals, "=", `field = "value"`},
		{OpNotEquals, "!=", `field != "value"`},
		{OpContains, "~", `field ~ "substring"`},
		{OpGreater, ">", `count > 10`},
		{OpLess, "<", `count < 10`},
		{OpExists, "exists", `field exists`},
		{OpNotExists, "not exists", `field not exists`},
	}

	for _, op := range operators {
		assert.Equal(t, op.symbol, string(op.op))
	}
}

func TestQueryTypes(t *testing.T) {
	// Test all query types
	types := []QueryType{
		QueryDocuments,
		QueryAttribution,
		QuerySources,
		QueryAuthors,
	}

	expected := []string{
		"documents",
		"attribution",
		"sources",
		"authors",
	}

	for i, typ := range types {
		assert.Equal(t, expected[i], string(typ))
	}
}