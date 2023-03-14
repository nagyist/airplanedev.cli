package definitions

import (
	"encoding/json"

	"github.com/airplanedev/lib/pkg/api"
	"github.com/airplanedev/lib/pkg/builtins"
	"github.com/pkg/errors"
)

// Track TaskBuiltinPlugins by definition key + function specification. There shouldn't be any key
// collisions for either of these..
var builtinTaskPluginsByDefinitionKey = map[string]TaskBuiltinPlugin{}
var builtinTaskPluginsByFunctionSpecification = map[builtins.FunctionKey]TaskBuiltinPlugin{}

func registerBuiltinTaskPlugin(plugin TaskBuiltinPlugin) error {
	for _, fs := range plugin.GetFunctionSpecifications() {
		if _, ok := builtinTaskPluginsByFunctionSpecification[fs.Key()]; ok {
			return errors.Errorf("Already registered a builtin task for %s", fs.Key())
		}
		builtinTaskPluginsByFunctionSpecification[fs.Key()] = plugin
	}

	defKey := plugin.GetDefinitionKey()
	if _, ok := builtinTaskPluginsByDefinitionKey[defKey]; ok {
		return errors.Errorf("Already registered a builtin task for %s", defKey)
	}
	builtinTaskPluginsByDefinitionKey[defKey] = plugin

	return nil
}

func newTaskBuiltinPlugin(fs []builtins.FunctionSpecification, defKey string, defGenerator func() BuiltinTaskDef) TaskBuiltinPlugin {
	return TaskBuiltinPlugin{
		functionSpecifications: fs,
		definitionKey:          defKey,
		defGenerator:           defGenerator,
	}
}

type TaskBuiltinPlugin struct {
	// Indicates the function specifications that this plugin can handle.
	functionSpecifications []builtins.FunctionSpecification
	// The key that this definition should be slotted under in the yaml/json definition.
	definitionKey string
	// A function that returns a new BuiltinTaskDef for this plugin.
	defGenerator func() BuiltinTaskDef
}

func (p TaskBuiltinPlugin) GetFunctionSpecifications() []builtins.FunctionSpecification {
	return p.functionSpecifications
}

func (p TaskBuiltinPlugin) GetDefinitionKey() string {
	return p.definitionKey
}

func (p TaskBuiltinPlugin) GetTaskKindDefinition() BuiltinTaskDef {
	return p.defGenerator()
}

type BuiltinTaskDef interface {
	taskKind
	getFunctionSpecification() (builtins.FunctionSpecification, error)
}

// This is purely to override the marshalling code, so we don't have to override the marshalling in
// Definition (which does squirrelly things with field ordering if we were overriding it there).
// The BuiltinTaskContainer is inlined, so we just generate a map of definition key -> definition
// in the marshalling code.
type BuiltinTaskContainer struct {
	def BuiltinTaskDef
}

func (c BuiltinTaskContainer) MarshalJSON() ([]byte, error) {
	marshalled, err := c.MarshalYAML()
	if err != nil {
		return nil, err
	}
	return json.Marshal(marshalled)
}

func (c BuiltinTaskContainer) MarshalYAML() (interface{}, error) {
	fs, err := c.def.getFunctionSpecification()
	if err != nil {
		return nil, err
	}

	plugin, ok := builtinTaskPluginsByFunctionSpecification[fs.Key()]
	if !ok {
		return nil, errors.Errorf("no plugin for function specification %s", fs.Key())
	}

	return map[string]interface{}{
		plugin.GetDefinitionKey(): c.def,
	}, nil
}

// Hydrates a builtin definition from a task.
func hydrateBuiltin(d *Definition, t api.Task, availableResources []api.ResourceMetadata) error {
	fs, err := builtins.GetFunctionSpecificationFromKindOptions(t.KindOptions)
	if err != nil {
		return err
	}

	plugin, ok := builtinTaskPluginsByFunctionSpecification[fs.Key()]
	if !ok {
		return errors.Errorf("unknown function specification: %s", fs.Key())
	}

	def := plugin.GetTaskKindDefinition()
	if err := def.hydrateFromTask(t, availableResources); err != nil {
		return err
	}
	d.Builtin = &BuiltinTaskContainer{def: def}

	return nil
}
