package metadata

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/spi/metadata"
	"github.com/docker/infrakit/pkg/types"
	"github.com/sergi/go-diff/diffmatchpatch"
	"github.com/spf13/cobra"
)

func prettyPrintYAML(any *types.Any) ([]byte, error) {
	return any.MarshalYAML()
}

func prettyPrintJSON(any *types.Any) ([]byte, error) {
	var v interface{}
	err := any.Decode(&v)
	if err != nil {
		return any.MarshalJSON()
	}
	return json.MarshalIndent(v, "", "  ")
}

// Change returns the Change command
func Change(name string, services *cli.Services) *cobra.Command {

	change := &cobra.Command{
		Use:   "change k1=v1 [, k2=v2]+",
		Short: "Update metadata where args are key=value pairs and keys are within namespace of the plugin.",
	}

	printYAML := change.Flags().BoolP("yaml", "y", false, "Show diff in YAML")
	commitChange := change.Flags().BoolP("commit", "c", false, "Commit changes")

	updatablePlugin, err := loadPluginUpdatable(services.Scope.Plugins(), name)
	if err != nil {
		return nil
	}
	cli.MustNotNil(updatablePlugin, "updatable metadata plugin not found", "name", name)

	change.RunE = func(cmd *cobra.Command, args []string) error {

		printer := prettyPrintJSON
		if *printYAML {
			printer = prettyPrintYAML
		}

		// get the changes
		changes, err := changeSet(args)
		if err != nil {
			return err
		}
		current, proposed, cas, err := updatablePlugin.Changes(changes)
		if err != nil {
			return err
		}
		currentBuff, err := printer(current)
		if err != nil {
			return err
		}

		proposedBuff, err := printer(proposed)
		if err != nil {
			return err
		}

		if *commitChange {
			fmt.Printf("Committing %d changes, hash=%s\n", len(changes), cas)
		} else {
			fmt.Printf("Proposing %d changes, hash=%s\n", len(changes), cas)
		}

		// Render the delta
		dmp := diffmatchpatch.New()
		diffs := dmp.DiffMain(string(currentBuff), string(proposedBuff), false)
		fmt.Println(dmp.DiffPrettyText(diffs))

		if *commitChange {
			return updatablePlugin.Commit(proposed, cas)
		}
		return nil
	}
	return change
}

// changeSet returns a set of changes from the input pairs of path / value
func changeSet(kvPairs []string) ([]metadata.Change, error) {
	changes := []metadata.Change{}

	for _, kv := range kvPairs {

		parts := strings.SplitN(kv, "=", 2)
		key := strings.Trim(parts[0], " \t\n")
		value := strings.Trim(parts[1], " \t\n")

		change := metadata.Change{
			Path:  types.PathFromString(key),
			Value: types.AnyYAMLMust([]byte(value)),
		}

		changes = append(changes, change)
	}
	return changes, nil
}
