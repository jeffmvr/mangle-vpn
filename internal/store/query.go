package store

import "fmt"

// Query describes a listing: an optional case insensitive search term and an
// optional page of results. A zero Size means every match is returned.
type Query struct {
	Search string
	Page   int
	Size   int
}

// Paginated reports whether the query asks for a single page.
func (q Query) Paginated() bool { return q.Size > 0 }

// limitOffset renders the LIMIT clause for the query, or an empty string when
// the query is unpaginated.
func (q Query) limitOffset() string {
	if !q.Paginated() {
		return ""
	}
	page := max(q.Page, 1)
	return fmt.Sprintf(" LIMIT %d OFFSET %d", q.Size, (page-1)*q.Size)
}

// Page is one page of a listing together with the total number of matches.
type Page[T any] struct {
	Items []T
	Total int
}
