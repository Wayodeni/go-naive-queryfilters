// Django like (django-filter, DRF) queryparam filters to SQL parser.
//
// Supported features:
//
//   - IN and NOT IN for list (or not) of get parameters: `.../path?column__in=1&column__in=2&column__in=3“
//   - exact equality and inequality (using not_in): `.../path?column__not_in=1 = column <> 1`
//   - LIKE using __like operator: col1__like=foo = `LOWER(col1) LIKE CONCAT('%', foo, '%')`
//
// Planned features:
//
//   - support for operators: BETWEEN, IS NULL, gt, gte, lt, lte, not
//   - OR support
//   - ordering support (parentheses in resulting SQL)
package naivequeryfilters

import (
	"fmt"
	"maps"
	"net/url"
	"slices"
	"strings"

	"github.com/kirill-scherba/omap"
)

/*
These constants define operator names in get parameters keys.
E.g. ".../path?column__not_in=val" == fmt.Sprintf(".../path?column%s%s=val", OPERATOR_SEPARATOR, OPERATOR_NOT_IN)
*/
const (
	OPERATOR_SEPARATOR = "__"
	OPERATOR_IN        = "in"
	OPERATOR_NOT_IN    = "not_in"
	OPERATOR_LIKE      = "like"
)

/*
These constants define SQL query tokens in which operators will be transformed.
*/
const (
	QUERY_TOKEN_IN         = "IN"
	QUERY_TOKEN_NOT_IN     = "NOT IN"
	QUERY_TOKEN_EQUALS     = "="
	QUERY_TOKEN_NOT_EQUALS = "<>"
	QUERY_TOKEN_LIKE       = "LIKE"
)

/*
This map stores "<get param operator>: <sql query token equivalent>" pairs.
*/
var OPERATORS = map[string]string{
	OPERATOR_IN:     QUERY_TOKEN_IN,
	OPERATOR_NOT_IN: QUERY_TOKEN_NOT_IN,
	OPERATOR_LIKE:   QUERY_TOKEN_LIKE,
}

/*
Struct holding SQL filtering query with placeholders and placeholder values.
*/
type Filter struct {
	// This field contains resulting SQL filters for WHERE part e.g. `col1 IN (?,?,?) AND col2 = ?`.
	SqlFilters string

	// List of placeholder values for SQL query.
	PlaceholderValues []interface{}
}

/*
Map storing "<db column name>: <db column name changing function>".

Matching every column name to function which will optionally return transformed (or completely new) name instead of passed.
Primarily used for transforming of aliases like this: "SELECT table.col_name AS new_col_name" into
column names like this: "table_name.column_name" to be used in resulting SQL query.

We need that because inside SQL we can't use column aliases inside WHERE conditions:
https://stackoverflow.com/questions/13031013/how-do-i-use-alias-in-where-clause
*/
type AllowedColumns map[string]func(string) string

/*
filterParam holds query parameter name with query token in separate fields also with
query param values field.
Used for SQL code generation for single query parameter using filterParam.Sql().
E.g. "col = ?" or "col <> ?" or "col IN (?, ?, ?)"
*/
type filterParam struct {
	// Holds column name instead of query parameter key.
	// E.g. "col" instead of "col__not_in"
	Name string

	// Holds SQL query token corresponding to query operator.
	// E.g. "IN", "NOT_IN", "<>"
	QueryToken string

	// Holds query param values when array is passed inside URL.
	//E.g. ".../path?column__in=1&column__in=2&column__in=3"
	Values []string
}

/*
Construct filterParam based on column name (without filter operator), operator and values (from url) array.
*/
func newFilterParam(name, operator string, values []string) (filterParam, error) {
	if len(values) == 0 {
		return filterParam{}, fmt.Errorf("passed empty values to filterParam")
	}
	if operator == "" {
		return filterParam{
			Name:       name,
			QueryToken: QUERY_TOKEN_IN,
			Values:     values,
		}, nil
	} else if operator == OPERATOR_LIKE && len(values) > 1 {
		return filterParam{}, fmt.Errorf("like operator only supports single value")
	}
	queryToken, ok := OPERATORS[operator]
	if !ok {
		return filterParam{}, ErrWrongOperator{Operator: operator, ParamName: name}
	}
	return filterParam{
		Name:       name,
		QueryToken: queryToken,
		Values:     values,
	}, nil
}

/*
Get SQL query code for filterParam.
*/
func (fp *filterParam) Sql() string {
	if len(fp.Values) == 1 && fp.QueryToken == QUERY_TOKEN_IN {
		return fmt.Sprintf("%s %s ?", fp.Name, QUERY_TOKEN_EQUALS)
	}
	if len(fp.Values) == 1 && fp.QueryToken == QUERY_TOKEN_NOT_IN {
		return fmt.Sprintf("%s %s ?", fp.Name, QUERY_TOKEN_NOT_EQUALS)
	}
	if fp.QueryToken == QUERY_TOKEN_LIKE {
		return fmt.Sprintf("LOWER(%s) %s CONCAT('%%', ?, '%%')", fp.Name, QUERY_TOKEN_LIKE)
	}
	return fp.buildPhrase(fp.Name, fp.QueryToken, fp.Values)
}

/*
Build SQL phrase for multi-value operators. (IN, NOT IN)
*/
func (fp *filterParam) buildPhrase(columnName, operatorName string, values []string) string {
	phrase := fmt.Sprintf("%s %s (", columnName, strings.ToUpper(operatorName))
	for range values {
		phrase += "?,"
	}
	phrase = phrase[:len(phrase)-1] // trim last redundant comma
	return phrase + ")"
}

/*
Main library function.
Accept whitelist of db table column names with get parameters extracted from url.
Returns Filter with SQL query string and placeholder values, wrong column names passed in getParams and error.
*/
func Build(filterAllowedColumnNames AllowedColumns, getParams url.Values) (Filter, url.Values, error) {
	orderedGetParams, err := getOrderedGetParams(getParams)
	if err != nil {
		return Filter{}, nil, err
	}
	validParams, invalidParams, err := getValidQueryParams(filterAllowedColumnNames, orderedGetParams)
	if err != nil {
		return Filter{}, invalidParams, err
	}
	return buildQueryFilters(validParams), invalidParams, nil
}

/*
Get pairs "<get parameter name>: {array of get parameter values}" sorted alphabetically by key.
(using ordered map)
Using this because golang map (which was used before in SQL query generation) can't produce
idempotent query around AND and OR operators.
(operands around AND and OR were placed randomly because of go map random iter order)
*/
func getOrderedGetParams(getParams url.Values) (*omap.Omap[string, []string], error) {
	orderedGetParams, err := omap.New[string, []string]()
	if err != nil {
		return nil, err
	}
	getParamsKeys := slices.Collect(maps.Keys(getParams))
	slices.Sort(getParamsKeys)
	for _, key := range getParamsKeys {
		orderedGetParams.Set(key, getParams[key])
	}
	return orderedGetParams, nil
}

/*
Accepts whitelist of table columns and map of alphabetically sorted get params with values.
Returns alphabetically sorted map of "<db column name>: <filterParam struct>" which contains only valid column names,
wrong get parameters map (map with columns that are not in whitelist) and error.
*/
func getValidQueryParams(filterAllowedColumnNames AllowedColumns, getParams *omap.Omap[string, []string]) (*omap.Omap[string, filterParam], url.Values, error) {
	validParams, err := omap.New[string, filterParam]()
	if err != nil {
		return nil, nil, err
	}
	if getParams.Len() == 0 {
		return validParams, nil, errAttemptedToPassEmptyFilteringParams
	}

	removedParams := make(url.Values)
	for _, querystringPair := range getParams.Pairs() {
		querystringFilterParamName := querystringPair.Key
		filterParamValues := querystringPair.Value
		filterParam, err := splitParamName(querystringFilterParamName, filterParamValues)
		if err != nil {
			return validParams, removedParams, err
		}

		if colNameConverterFunc, isColumnNameValid := filterAllowedColumnNames[filterParam.Name]; isColumnNameValid {
			filterParam.Name = colNameConverterFunc(filterParam.Name)
			validParams.Set(filterParam.Name, filterParam)
		} else {
			removedParams[querystringFilterParamName] = filterParamValues
		}

		getParams.Del(querystringFilterParamName)
	}

	if len(removedParams) == 0 {
		removedParams = nil
	}

	return validParams, removedParams, nil
}

/*
Accepts query param name from url and array of param values.
Returns filterParam with separated column name and SQL query token and error.
*/
func splitParamName(paramName string, paramValues []string) (filterParam, error) {
	res := strings.Split(paramName, OPERATOR_SEPARATOR)
	if !strings.Contains(paramName, OPERATOR_SEPARATOR) {
		return newFilterParam(
			res[0],
			"",
			paramValues,
		)
	}

	if len(res) != 2 {
		return filterParam{}, ErrWrongQueryParamName{ParamName: paramName}
	}

	operator := res[1]
	if _, operatorIsValid := OPERATORS[operator]; !operatorIsValid {
		return filterParam{}, ErrWrongOperator{
			Operator:  operator,
			ParamName: paramName,
		}
	}
	return newFilterParam(res[0], res[1], paramValues)
}

/*
Accepts alphabetically sorted ordered map which contains valid "<db column name>: <filterParam>" pairs.
*/
func buildQueryFilters(validParams *omap.Omap[string, filterParam]) Filter {
	queryFilters := `` // Example value: `col1=? AND col2=?`
	queryParamsValues := []any{}
	filtersCount := 0
	for _, mapPair := range validParams.Pairs() {
		queryFilters += mapPair.Value.Sql()
		filtersCount += 1
		if filtersCount < validParams.Len() {
			queryFilters += " AND "
		}
		queryParamsValues = append(queryParamsValues, func() []any {
			interfaceSlice := make([]any, len(mapPair.Value.Values))
			for i := range interfaceSlice {
				interfaceSlice[i] = mapPair.Value.Values[i]
			}
			return interfaceSlice
		}()...)
	}
	return Filter{
		queryFilters,
		queryParamsValues,
	}
}
