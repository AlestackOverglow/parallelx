package parallel

import "errors"

// Result represents a result from a parallel task execution
type Result struct {
	Value interface{}
	Error error
}

// FilterErrors returns only the successful results and aggregates errors
func FilterErrors(results []interface{}) ([]interface{}, []error) {
	var (
		validResults []interface{}
		errs         []error
	)

	for _, result := range results {
		if err, ok := result.(error); ok {
			errs = append(errs, err)
		} else {
			validResults = append(validResults, result)
		}
	}

	return validResults, errs
}

// CombineErrors combines multiple errors into a single error
func CombineErrors(errs []error) error {
	if len(errs) == 0 {
		return nil
	}

	errStr := "multiple errors occurred:"
	for _, err := range errs {
		errStr += "\n - " + err.Error()
	}
	return errors.New(errStr)
}

// MapResults applies a transformation function to successful results
func MapResults[T, R any](results []interface{}, mapper func(T) R) ([]R, []error) {
	var (
		transformed []R
		errs        []error
	)

	for _, result := range results {
		if err, ok := result.(error); ok {
			errs = append(errs, err)
		} else if val, ok := result.(T); ok {
			transformed = append(transformed, mapper(val))
		}
	}

	return transformed, errs
}
