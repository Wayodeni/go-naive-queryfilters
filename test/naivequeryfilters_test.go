package naivequeryfilters_test

import (
	"net/url"
	"testing"

	naivequeryfilters "github.com/Wayodeni/go-naive-queryfilters"
	"github.com/stretchr/testify/assert"
)

type caseTestData struct {
	ColumnsWhitelist      naivequeryfilters.AllowedColumns
	QueryParams           url.Values
	ExpectedFilter        naivequeryfilters.Filter
	ExpectedInvalidParams url.Values
}

var whitelist = naivequeryfilters.AllowedColumns{
	"col1": func(s string) string { return s },
	"col2": func(s string) string { return s },
}

var testData = []caseTestData{
	{
		ColumnsWhitelist: whitelist,
		QueryParams: url.Values{
			"col1": []string{"1", "2", "3"},
			"col2": []string{"1"},
			"col3": []string{"val"},
		},
		ExpectedFilter: naivequeryfilters.Filter{
			SqlFilters:        `col1 IN (?,?,?) AND col2 = ?`,
			PlaceholderValues: []any{"1", "2", "3", "1"},
		},
		ExpectedInvalidParams: url.Values{
			"col3": []string{"val"},
		},
	},
	{
		ColumnsWhitelist: whitelist,
		QueryParams: url.Values{
			"col1__in": []string{"1", "2", "3"},
			"col2__in": []string{"1"},
			"col3__in": []string{"val"},
		},
		ExpectedFilter: naivequeryfilters.Filter{
			SqlFilters:        `col1 IN (?,?,?) AND col2 = ?`,
			PlaceholderValues: []any{"1", "2", "3", "1"},
		},
		ExpectedInvalidParams: url.Values{
			"col3__in": []string{"val"},
		},
	},
	{
		ColumnsWhitelist: whitelist,
		QueryParams: url.Values{
			"col1__not_in": []string{"1", "2", "3"},
			"col2__not_in": []string{"1"},
			"col3__not_in": []string{"val"},
		},
		ExpectedFilter: naivequeryfilters.Filter{
			SqlFilters:        `col1 NOT IN (?,?,?) AND col2 <> ?`,
			PlaceholderValues: []any{"1", "2", "3", "1"},
		},
		ExpectedInvalidParams: url.Values{
			"col3__not_in": []string{"val"},
		},
	},
	{
		ColumnsWhitelist: whitelist,
		QueryParams: url.Values{
			"col1__in":     []string{"1"},
			"col2__not_in": []string{"1"},
			"col3":         []string{"val"},
		},
		ExpectedFilter: naivequeryfilters.Filter{
			SqlFilters:        `col1 = ? AND col2 <> ?`,
			PlaceholderValues: []any{"1", "1"},
		},
		ExpectedInvalidParams: url.Values{"col3": []string{"val"}},
	},
	{
		ColumnsWhitelist: whitelist,
		QueryParams: url.Values{
			"col1__like": []string{"foo"},
		},
		ExpectedFilter: naivequeryfilters.Filter{
			SqlFilters:        `LOWER(col1) LIKE CONCAT('%', ?, '%')`,
			PlaceholderValues: []any{"foo"},
		},
		ExpectedInvalidParams: nil,
	},
	{ // this test case throws an error but we don't check it here
		ColumnsWhitelist: whitelist,
		QueryParams: url.Values{
			"col1__like": []string{"foo", "bar"},
		},
		ExpectedFilter: naivequeryfilters.Filter{
			SqlFilters:        ``,
			PlaceholderValues: nil,
		},
		ExpectedInvalidParams: make(url.Values),
	},
}

func TestBuildQuery(t *testing.T) {
	for _, expected := range testData {
		filter, invalidParams, _ := naivequeryfilters.Build(expected.ColumnsWhitelist, expected.QueryParams)
		assert.Equal(t, expected.ExpectedFilter.SqlFilters, filter.SqlFilters, "sql in filter does not equal")
		assert.ElementsMatch(t, expected.ExpectedFilter.PlaceholderValues, filter.PlaceholderValues, "placeholder values in filter does not equal")
		assert.Equal(t, expected.ExpectedInvalidParams, invalidParams, "got not equal invalidParams map")
	}
}
