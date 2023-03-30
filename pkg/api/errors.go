package api

import "fmt"

const (
	DefaultAppURL = "https://app.airplane.dev"
)

func getAppURL(appURL string) string {
	if appURL != "" {
		return appURL
	}
	return DefaultAppURL
}

func linkToCreatePage(pageName string, url string) string {
	return fmt.Sprintf("Follow the URL below to create the %s:\n%s", pageName, url)
}

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
	url := getAppURL(err.AppURL) + "/tasks/new"
	return linkToCreatePage("task", url)
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
	url := getAppURL(err.AppURL) + "/views/new"
	return linkToCreatePage("view", url)
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
	url := getAppURL(err.AppURL) + "/settings/resources/new"
	return linkToCreatePage("resource", url)
}
