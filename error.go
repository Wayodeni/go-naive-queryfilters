package naivequeryfilters

import (
	"errors"
	"fmt"
)

var (
	// Internal error. Thrown when len of getParams map in getValidQueryParams is 0.
	errAttemptedToPassEmptyFilteringParams error = errors.New("getValidQueryParams not accepting empty filtering values")
)

// This error is returned when query parameter with wrong key passed. e.g col__name__like.
type ErrWrongQueryParamName struct {
	ParamName string
}

func (e ErrWrongQueryParamName) Error() string {
	return fmt.Sprintf("wrong filtering param name: %v", e.ParamName)
}

// This error is thrown when wrong operator is passed in param name. e.g. col__not_listed_op.
type ErrWrongOperator struct {
	Operator  string
	ParamName string
}

func (e ErrWrongOperator) Error() string {
	return fmt.Sprintf("wrong filtering operator name '%v' in parameter '%v' ", e.Operator, e.ParamName)
}
