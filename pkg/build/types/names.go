package types

// TODO: this can merge with TaskKind
type Name string

const (
	NameImage  Name = "image"
	NamePython Name = "python"
	NameNode   Name = "node"
	NameShell  Name = "shell"
	NameView   Name = "view"

	NameSQL     Name = "sql"
	NameREST    Name = "rest"
	NameBuiltin Name = "builtin"
)
