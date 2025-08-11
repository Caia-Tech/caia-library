package gql

import (
	"fmt"
	"strings"
	"time"
)

// Query represents a parsed GQL query
type Query struct {
	Type       QueryType
	Filters    []Filter
	Timeframe  *TimeRange
	Limit      int
	OrderBy    string
	Descending bool
}

// QueryType defines the type of query
type QueryType string

const (
	QueryDocuments   QueryType = "documents"
	QueryAuthors     QueryType = "authors"
	QuerySources     QueryType = "sources"
	QueryAttribution QueryType = "attribution"
)

// Filter represents a query filter
type Filter struct {
	Field    string
	Operator Operator
	Value    interface{}
}

// Operator defines filter operators
type Operator string

const (
	OpEquals      Operator = "="
	OpNotEquals   Operator = "!="
	OpContains    Operator = "~"
	OpGreater     Operator = ">"
	OpLess        Operator = "<"
	OpIn          Operator = "in"
	OpNotIn       Operator = "not in"
	OpExists      Operator = "exists"
	OpNotExists   Operator = "not exists"
)

// TimeRange represents a time-based filter
type TimeRange struct {
	Start time.Time
	End   time.Time
}

// Parser parses GQL queries
type Parser struct {
	tokens []token
	pos    int
}

// token represents a lexical token
type token struct {
	typ   tokenType
	value string
}

type tokenType int

const (
	tokenEOF tokenType = iota
	tokenKeyword
	tokenIdentifier
	tokenOperator
	tokenString
	tokenNumber
	tokenLeftParen
	tokenRightParen
	tokenComma
)

// NewParser creates a new GQL parser
func NewParser() *Parser {
	return &Parser{}
}

// Parse parses a GQL query string
func (p *Parser) Parse(query string) (*Query, error) {
	// Tokenize the query
	tokens, err := p.tokenize(query)
	if err != nil {
		return nil, fmt.Errorf("tokenization error: %w", err)
	}

	p.tokens = tokens
	p.pos = 0

	// Parse the query
	return p.parseQuery()
}

// tokenize breaks the query into tokens
func (p *Parser) tokenize(query string) ([]token, error) {
	var tokens []token
	query = strings.TrimSpace(query)
	
	keywords := map[string]bool{
		"SELECT": true, "FROM": true, "WHERE": true, "AND": true, "OR": true,
		"ORDER": true, "BY": true, "DESC": true, "ASC": true, "LIMIT": true,
		"BETWEEN": true, "IN": true, "NOT": true, "EXISTS": true,
		"DOCUMENTS": true, "AUTHORS": true, "SOURCES": true, "ATTRIBUTION": true,
	}

	i := 0
	for i < len(query) {
		// Skip whitespace
		if query[i] == ' ' || query[i] == '\t' || query[i] == '\n' {
			i++
			continue
		}

		// String literals
		if query[i] == '"' || query[i] == '\'' {
			quote := query[i]
			j := i + 1
			for j < len(query) && query[j] != quote {
				if query[j] == '\\' {
					j++ // Skip escaped character
				}
				j++
			}
			if j >= len(query) {
				return nil, fmt.Errorf("unterminated string at position %d", i)
			}
			tokens = append(tokens, token{typ: tokenString, value: query[i+1 : j]})
			i = j + 1
			continue
		}

		// Numbers
		if query[i] >= '0' && query[i] <= '9' {
			j := i
			for j < len(query) && ((query[j] >= '0' && query[j] <= '9') || query[j] == '.') {
				j++
			}
			tokens = append(tokens, token{typ: tokenNumber, value: query[i:j]})
			i = j
			continue
		}

		// Identifiers and keywords
		if (query[i] >= 'a' && query[i] <= 'z') || (query[i] >= 'A' && query[i] <= 'Z') || query[i] == '_' {
			j := i
			for j < len(query) && ((query[j] >= 'a' && query[j] <= 'z') || 
				(query[j] >= 'A' && query[j] <= 'Z') || 
				(query[j] >= '0' && query[j] <= '9') || 
				query[j] == '_' || query[j] == '.') {
				j++
			}
			word := query[i:j]
			if keywords[strings.ToUpper(word)] {
				tokens = append(tokens, token{typ: tokenKeyword, value: strings.ToUpper(word)})
			} else {
				tokens = append(tokens, token{typ: tokenIdentifier, value: word})
			}
			i = j
			continue
		}

		// Operators
		if i+1 < len(query) {
			twoChar := query[i : i+2]
			if twoChar == "!=" || twoChar == ">=" || twoChar == "<=" {
				tokens = append(tokens, token{typ: tokenOperator, value: twoChar})
				i += 2
				continue
			}
		}

		// Single character tokens
		switch query[i] {
		case '=', '>', '<', '~':
			tokens = append(tokens, token{typ: tokenOperator, value: string(query[i])})
		case '(':
			tokens = append(tokens, token{typ: tokenLeftParen, value: "("})
		case ')':
			tokens = append(tokens, token{typ: tokenRightParen, value: ")"})
		case ',':
			tokens = append(tokens, token{typ: tokenComma, value: ","})
		default:
			return nil, fmt.Errorf("unexpected character '%c' at position %d", query[i], i)
		}
		i++
	}

	tokens = append(tokens, token{typ: tokenEOF, value: ""})
	return tokens, nil
}

// parseQuery parses the main query structure
func (p *Parser) parseQuery() (*Query, error) {
	q := &Query{
		Limit: 100, // Default limit
	}

	// Expect SELECT
	if !p.expectKeyword("SELECT") {
		return nil, fmt.Errorf("expected SELECT keyword")
	}

	// Parse query type
	if err := p.parseQueryType(q); err != nil {
		return nil, err
	}

	// Optional WHERE clause
	if p.matchKeyword("WHERE") {
		filters, err := p.parseFilters()
		if err != nil {
			return nil, err
		}
		q.Filters = filters
	}

	// Optional ORDER BY
	if p.matchKeyword("ORDER") {
		if !p.expectKeyword("BY") {
			return nil, fmt.Errorf("expected BY after ORDER")
		}
		orderBy, err := p.parseIdentifier()
		if err != nil {
			return nil, err
		}
		q.OrderBy = orderBy

		if p.matchKeyword("DESC") {
			q.Descending = true
		} else if p.matchKeyword("ASC") {
			q.Descending = false
		}
	}

	// Optional LIMIT
	if p.matchKeyword("LIMIT") {
		limit, err := p.parseNumber()
		if err != nil {
			return nil, err
		}
		q.Limit = int(limit)
	}

	return q, nil
}

// parseQueryType parses the FROM clause to determine query type
func (p *Parser) parseQueryType(q *Query) error {
	if !p.expectKeyword("FROM") {
		return fmt.Errorf("expected FROM keyword")
	}

	typ := p.current()
	if typ.typ != tokenKeyword {
		return fmt.Errorf("expected query type after FROM")
	}

	switch typ.value {
	case "DOCUMENTS":
		q.Type = QueryDocuments
	case "AUTHORS":
		q.Type = QueryAuthors
	case "SOURCES":
		q.Type = QuerySources
	case "ATTRIBUTION":
		q.Type = QueryAttribution
	default:
		return fmt.Errorf("unknown query type: %s", typ.value)
	}

	p.advance()
	return nil
}

// parseFilters parses WHERE clause filters
func (p *Parser) parseFilters() ([]Filter, error) {
	var filters []Filter

	for {
		filter, err := p.parseFilter()
		if err != nil {
			return nil, err
		}
		filters = append(filters, filter)

		// Check for AND (OR not supported yet for simplicity)
		if !p.matchKeyword("AND") {
			break
		}
	}

	return filters, nil
}

// parseFilter parses a single filter condition
func (p *Parser) parseFilter() (Filter, error) {
	// Get field name
	field, err := p.parseIdentifier()
	if err != nil {
		return Filter{}, fmt.Errorf("expected field name: %w", err)
	}

	// Get operator
	op := p.current()
	if op.typ != tokenOperator {
		return Filter{}, fmt.Errorf("expected operator after field %s", field)
	}
	p.advance()

	var operator Operator
	switch op.value {
	case "=":
		operator = OpEquals
	case "!=":
		operator = OpNotEquals
	case "~":
		operator = OpContains
	case ">":
		operator = OpGreater
	case "<":
		operator = OpLess
	default:
		return Filter{}, fmt.Errorf("unknown operator: %s", op.value)
	}

	// Get value
	value, err := p.parseValue()
	if err != nil {
		return Filter{}, err
	}

	return Filter{
		Field:    field,
		Operator: operator,
		Value:    value,
	}, nil
}

// Helper methods

func (p *Parser) current() token {
	if p.pos >= len(p.tokens) {
		return token{typ: tokenEOF}
	}
	return p.tokens[p.pos]
}

func (p *Parser) advance() {
	if p.pos < len(p.tokens) {
		p.pos++
	}
}

func (p *Parser) expectKeyword(keyword string) bool {
	tok := p.current()
	if tok.typ == tokenKeyword && tok.value == keyword {
		p.advance()
		return true
	}
	return false
}

func (p *Parser) matchKeyword(keyword string) bool {
	tok := p.current()
	if tok.typ == tokenKeyword && tok.value == keyword {
		p.advance()
		return true
	}
	return false
}

func (p *Parser) parseIdentifier() (string, error) {
	tok := p.current()
	if tok.typ != tokenIdentifier {
		return "", fmt.Errorf("expected identifier, got %v", tok.value)
	}
	p.advance()
	return tok.value, nil
}

func (p *Parser) parseNumber() (float64, error) {
	tok := p.current()
	if tok.typ != tokenNumber {
		return 0, fmt.Errorf("expected number, got %v", tok.value)
	}
	p.advance()
	var num float64
	fmt.Sscanf(tok.value, "%f", &num)
	return num, nil
}

func (p *Parser) parseValue() (interface{}, error) {
	tok := p.current()
	switch tok.typ {
	case tokenString:
		p.advance()
		return tok.value, nil
	case tokenNumber:
		p.advance()
		var num float64
		fmt.Sscanf(tok.value, "%f", &num)
		return num, nil
	case tokenIdentifier:
		p.advance()
		// Could be a date or boolean
		if tok.value == "true" || tok.value == "false" {
			return tok.value == "true", nil
		}
		// Try parsing as date
		if t, err := time.Parse("2006-01-02", tok.value); err == nil {
			return t, nil
		}
		return tok.value, nil
	default:
		return nil, fmt.Errorf("expected value, got %v", tok.value)
	}
}