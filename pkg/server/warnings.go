package server

type UnsupportedApp struct {
	Name   string
	Kind   AppKind
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
