package server

import "github.com/airplanedev/cli/pkg/server/apidev"

type UnsupportedApp struct {
	Name   string
	Kind   apidev.AppKind
	Reason string
}

type UnattachedResource struct {
	TaskName      string
	ResourceSlugs []string
}

type RegistrationWarnings struct {
	UnsupportedApps     []UnsupportedApp
	UnattachedResources []UnattachedResource
}
