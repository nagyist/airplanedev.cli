package gentypes

type templateConfig struct {
	TaskName   string
	TaskParams string
}

var paramTypesTemplate = `export type {{.TaskName}}Params = {
  {{.TaskParams}}
};
`

var paramTypesTemplateNoParams = `export type {{.TaskName}}Params = {};
`

var fileType = `{ __airplaneType: "upload"; id: string; url: string }`

var configType = `{ name: string; value: string }`
