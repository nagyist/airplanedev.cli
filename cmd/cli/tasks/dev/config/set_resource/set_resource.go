package set_resource

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"

	"github.com/MakeNowJust/heredoc"
	"github.com/airplanedev/cli/pkg/cli"
	"github.com/airplanedev/cli/pkg/cli/prompts"
	"github.com/airplanedev/cli/pkg/cli/resources"
	"github.com/airplanedev/cli/pkg/cli/resources/kinds"
	"github.com/airplanedev/cli/pkg/utils/logger"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type config struct {
	devCLI      *cli.DevCLI
	name        string
	kind        string
	slug        string
	useDefaults bool
}

func New(c *cli.DevCLI) *cobra.Command {
	var cfg = config{devCLI: c}
	cmd := &cobra.Command{
		Use:   "set-resource",
		Short: "Sets a resource in the dev config file",
		Example: heredoc.Doc(`
			airplane dev config set-resource --kind <kind> <slug>
			airplane dev config set-resource
			airplane dev config set-resource --kind postgres db
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				cfg.slug = args[0]
			} else {
				if err := c.Prompter.Input(
					"What's the slug of your resource? This should match the <alias>: <slug> entry in your resource attachments.",
					&cfg.slug,
					prompts.WithRequired(),
				); err != nil {
					return err
				}
			}

			if cfg.name == "" {
				if err := c.Prompter.Input(
					"What's the name of your resource? This should be a human-readable name for your resource.",
					&cfg.name,
					prompts.WithRequired(),
				); err != nil {
					return err
				}
			}

			if cfg.kind == "" {
				resourceKinds := make([]resources.ResourceKind, 0, len(resources.ResourceFactories))
				for kind := range resources.ResourceFactories {
					resourceKinds = append(resourceKinds, kind)
				}

				sort.Slice(resourceKinds, func(i, j int) bool {
					return resourceKinds[i] < resourceKinds[j]
				})

				message := "What's your resource kind? Available kinds:"
				for _, kind := range resourceKinds {
					message += fmt.Sprintf("\n - %s", kind)
				}
				message += "\n>"

				if err := c.Prompter.Input(
					message,
					&cfg.kind,
					prompts.WithRequired(),
				); err != nil {
					return err
				}
			}

			return run(cmd.Root().Context(), cfg)
		},
	}

	cmd.Flags().StringVarP(&cfg.kind, "kind", "k", "", "Kind of resource")
	cmd.Flags().StringVarP(&cfg.name, "name", "n", "", "Name of resource")
	cmd.Flags().BoolVar(&cfg.useDefaults, "use-defaults", false, "If set, create an entry in the dev config file for the user to fill in")

	return cmd
}

func run(ctx context.Context, cfg config) error {
	serializedResource := map[string]interface{}{}
	kind := resources.ResourceKind(cfg.kind)
	emptyResource, err := resources.GetResource(kind, serializedResource)
	if err != nil {
		return err
	}

	// REST and SMTP resources have slice/map/struct fields - it is awkward to support these using prompts, and so we
	// direct the user to modify the files directly.
	// TODO: Support the resource kinds below
	switch kind {
	case kinds.ResourceKindREST, kinds.ResourceKindSMTP:
		return errors.Errorf("We do not currently support adding resources of kind %s through the CLI, please create the resource through the previewer.", kind)
	}

	// Set base resource fields
	serializedResource["kind"] = cfg.kind
	serializedResource["slug"] = cfg.slug
	serializedResource["name"] = cfg.name

	// Iterate over resource struct fields and dynamically prompt user for input.
	// Resource factories return a pointer to an external resource struct, and so we dereference by calling Elem().
	v := reflect.ValueOf(emptyResource).Elem()
	for i := 0; i < v.NumField(); i++ {
		field := v.Type().Field(i)
		fieldName := field.Tag.Get("json")
		if fieldName == "" {
			continue
		}
		var value string
		if !cfg.useDefaults {
			if err := cfg.devCLI.Prompter.Input(
				fmt.Sprintf("Enter %s resource `%s`:", cfg.kind, fieldName),
				&value,
			); err != nil {
				return err
			}
		}
		serializedResource[fieldName] = value
	}

	devConfig := cfg.devCLI.DevConfig
	resource, err := resources.GetResource(kind, serializedResource)
	if err != nil {
		return errors.Wrap(err, "unable to convert resource map to resource")
	}
	if err := devConfig.SetResource(cfg.slug, resource); err != nil {
		return errors.Wrap(err, "setting resource in dev config file")
	}

	encodedResource, err := json.MarshalIndent(serializedResource, "", "  ")
	if err != nil {
		return err
	}
	logger.Log(string(encodedResource))
	return nil
}
