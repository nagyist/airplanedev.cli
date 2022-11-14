package parser

import _ "embed"

//go:embed node/parser.js
var NodeParserScript string

//go:embed python/parser.py
var PythonParserScript string
