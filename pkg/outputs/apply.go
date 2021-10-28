package outputs

import (
	"github.com/airplanedev/ojson"
	"github.com/airplanedev/path"
	"github.com/pkg/errors"
)

func ApplyOutputCommand(cmd *ParsedLine, o *ojson.Value) error {
	switch cmd.Command {
	case "":
		if err := applyLegacy(cmd.Name, cmd.Value.V, o); err != nil {
			return err
		}

	case "set":
		if err := applySet(cmd.JsonPath, cmd.Value.V, o); err != nil {
			return err
		}

	case "append":
		if err := applyAppend(cmd.JsonPath, cmd.Value.V, o); err != nil {
			return err
		}

	default:
		return errors.New("unknown command")
	}

	return nil
}

func applyLegacy(name string, v interface{}, o *ojson.Value) error {
	if o.V == nil {
		o.V = ojson.NewObject()
	}

	obj, ok := o.V.(*ojson.Object)
	if !ok {
		return errors.New("expected json object at top level")
	}

	target, ok := obj.Get(name)
	if !ok {
		target = []interface{}{}
	}

	arr, ok := target.([]interface{})
	if !ok {
		return errors.New("expected array")
	}

	obj.Set(name, append(arr, v))
	return nil
}

func applySet(jsPath string, v interface{}, o *ojson.Value) error {
	p, err := path.FromJS(jsPath)
	if err != nil {
		return err
	}

	// Setting whole object.
	if p.Len() == 0 {
		o.V = v
		return nil
	}

	var cur interface{}
	cur = o.V
	for i, component := range p.Components() {
		switch c := component.(type) {
		case string:
			obj, ok := cur.(*ojson.Object)
			if !ok {
				return errors.New("expected *ojson.Object")
			}
			if i == p.Len()-1 {
				obj.Set(c, v)
			} else {
				var ok bool
				cur, ok = obj.Get(c)
				if !ok {
					return errors.New("could not find value in path")
				}
			}

		case int:
			arr, ok := cur.([]interface{})
			if !ok {
				return errors.New("expected array")
			}
			if c >= len(arr) {
				return errors.New("array had too few elements")
			}
			if i == p.Len()-1 {
				arr[c] = v
			} else {
				cur = arr[c]
			}

		default:
			return errors.New("unexpected component type")
		}
	}
	return nil
}

func applyAppend(jsPath string, v interface{}, o *ojson.Value) error {
	p, err := path.FromJS(jsPath)
	if err != nil {
		return err
	}

	// Appending on root.
	if p.Len() == 0 {
		if o.V == nil {
			o.V = []interface{}{}
		}
		arr, ok := o.V.([]interface{})
		if !ok {
			return errors.New("expected array at root")
		}
		o.V = append(arr, v)
		return nil
	}

	var cur interface{}
	cur = o.V
	for i, component := range p.Components() {
		switch c := component.(type) {
		case string:
			obj, ok := cur.(*ojson.Object)
			if !ok {
				return errors.New("expected *ojson.Object")
			}
			if i == p.Len()-1 {
				childArrV, ok := obj.Get(c)
				if !ok || childArrV == nil {
					childArrV = []interface{}{}
				}
				childArr, ok := childArrV.([]interface{})
				if !ok {
					return errors.New("expected array at append point")
				}
				obj.Set(c, append(childArr, v))
			} else {
				var ok bool
				cur, ok = obj.Get(c)
				if !ok {
					return errors.New("could not find value in path")
				}
			}

		case int:
			arr, ok := cur.([]interface{})
			if !ok {
				return errors.New("expected array")
			}
			if c >= len(arr) {
				return errors.New("array had too few elements")
			}
			if i == p.Len()-1 {
				if arr[c] == nil {
					arr[c] = []interface{}{}
				}
				childArr, ok := arr[c].([]interface{})
				if !ok {
					return errors.New("expected array at append point")
				}
				arr[c] = append(childArr, v)
			} else {
				cur = arr[c]
			}

		default:
			return errors.New("unexpected component type")
		}
	}
	return nil
}
