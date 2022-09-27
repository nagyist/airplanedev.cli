package api

import "fmt"

// TaskMissingError implements an explainable error.
type TaskMissingError struct {
	AppURL string
	Slug   string
}

// Error implementation.
func (err TaskMissingError) Error() string {
	return fmt.Sprintf("task with slug %q does not exist", err.Slug)
}

// ExplainError implementation.
func (err TaskMissingError) ExplainError() string {
	return fmt.Sprintf(
		"Follow the URL below to create the task:\n%s",
		err.AppURL+"/tasks/new",
	)
}

// ViewMissingError implements an explainable error.
type ViewMissingError struct {
	AppURL string
	Slug   string
}

// Error implementation.
func (err ViewMissingError) Error() string {
	return fmt.Sprintf("view with slug %q does not exist", err.Slug)
}

// ExplainError implementation.
func (err ViewMissingError) ExplainError() string {
	return fmt.Sprintf(
		"Follow the URL below to create the view:\n%s",
		err.AppURL+"/apps/new",
	)
}

// ResourceMissingError implements an explainable error.
type ResourceMissingError struct {
	AppURL string
	Slug   string
}

// Error implementation.
func (err ResourceMissingError) Error() string {
	return fmt.Sprintf("resource with slug %q does not exist", err.Slug)
}

// ExplainError implementation.
func (err ResourceMissingError) ExplainError() string {
	return fmt.Sprintf("Follow the URL below to create the resource:\n%s", err.AppURL+"/settings/resources/new")
}
