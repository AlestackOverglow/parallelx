package parallel

import (
	"errors"
	"strings"
	"testing"
)

func TestFilterErrors(t *testing.T) {
	results := []interface{}{
		1,
		errors.New("error 1"),
		2,
		errors.New("error 2"),
		3,
	}

	validResults, errs := FilterErrors(results)

	if len(validResults) != 3 {
		t.Errorf("Expected 3 valid results, got %d", len(validResults))
	}

	if len(errs) != 2 {
		t.Errorf("Expected 2 errors, got %d", len(errs))
	}

	for i, result := range validResults {
		expected := i*1 + 1
		if result != expected {
			t.Errorf("Expected result %d to be %d, got %v", i, expected, result)
		}
	}
}

func TestCombineErrors(t *testing.T) {
	errs := []error{
		errors.New("error 1"),
		errors.New("error 2"),
	}

	combined := CombineErrors(errs)
	if combined == nil {
		t.Error("Expected non-nil error")
	}

	errStr := combined.Error()
	for _, substr := range []string{"error 1", "error 2"} {
		if !strings.Contains(errStr, substr) {
			t.Errorf("Expected error to contain %q, got %q", substr, errStr)
		}
	}

	// Test empty errors
	if CombineErrors(nil) != nil {
		t.Error("Expected nil error for empty error slice")
	}
}

func TestMapResults(t *testing.T) {
	results := []interface{}{
		1,
		errors.New("error 1"),
		2,
		errors.New("error 2"),
		3,
	}

	transformed, errs := MapResults[int, string](results, func(i int) string {
		return string(rune('A' - 1 + i))
	})

	if len(transformed) != 3 {
		t.Errorf("Expected 3 transformed results, got %d", len(transformed))
	}

	if len(errs) != 2 {
		t.Errorf("Expected 2 errors, got %d", len(errs))
	}

	expected := []string{"A", "B", "C"}
	for i, result := range transformed {
		if result != expected[i] {
			t.Errorf("Expected result %d to be %q, got %q", i, expected[i], result)
		}
	}
}
