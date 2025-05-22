package naivequeryfilters_test

import (
	"fmt"
	"net/url"

	naivequeryfilters "github.com/Wayodeni/go-naive-queryfilters"
)

func Example() {
	var whitelist = naivequeryfilters.AllowedColumns{
		"col1": func(s string) string { return "table_name.col1" },
		"col2": func(s string) string { return s },
	}
	filter, invalidParams, err := naivequeryfilters.Build(whitelist, url.Values{
		"col1__in":     []string{"1", "2"},
		"col2__not_in": []string{"1"},
		"col3":         []string{"val"},
	})
	if err != nil {
		panic(err)
	}
	if len(invalidParams) != 0 {
		fmt.Printf("passed invalid column names in get parameters: %s\n", invalidParams)
	}
	fmt.Printf("SQL query filters part for WHERE clause: '%s'\nSQL query placeholder values: %v", filter.SqlFilters, filter.PlaceholderValues)
	// Output:
	// passed invalid column names in get parameters: map[col3:[val]]
	// SQL query filters part for WHERE clause: 'table_name.col1 IN (?,?) AND col2 <> ?'
	// SQL query placeholder values: [1 2 1]
}
