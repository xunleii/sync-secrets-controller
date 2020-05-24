package kubernetes_feature_context

import (
	"fmt"

	"github.com/cucumber/messages-go/v10"
	"gopkg.in/yaml.v3"
)

type (
	pickleDocString *messages.PickleStepArgument_PickleDocString
	pickleTable     *messages.PickleStepArgument_PickleTable
)

// YamlDocString adds unmarshalling method to easily manage YAML content.
type YamlDocString pickleDocString

// UnmarshalYamlDocString converts a *messages.PickleStepArgument_PickleDocString into
// a map[string]interface{}.
func UnmarshalYamlDocString(doc YamlDocString) (obj map[string]interface{}, err error) {
	return obj, yaml.Unmarshal([]byte(doc.Content), &obj)
}

type (
	// ResourceTable adds unmarshalling method to easily manage Resource table.
	ResourceTable pickleTable
	// ResourceTableRow describes a resource in the ResourceTable.
	ResourceTableRow struct {
		GroupVersion string
		Kind         string
		Namespace    string
		Name         string
	}
)

// UnmarshalResourceTable converts a *messages.PickleStepArgument_PickleTable into a
// list of ResourceTableRow, in which we can easily extract GroupVersionKind and
// NamespacedName values.
func UnmarshalResourceTable(table ResourceTable) (resources []ResourceTableRow, err error) {
	for i, row := range table.Rows {
		if i == 0 && row.Cells[0].GetValue() == "ApiGroupVersion" {
			// Ignore header line if exists
			continue
		}

		if len(row.Cells) != 4 {
			return nil, fmt.Errorf("invalid resource table: it must contains 4 columns (GroupVersion, Kind, Namespace, Name)")
		}

		resources = append(resources, ResourceTableRow{
			GroupVersion: row.Cells[0].GetValue(),
			Kind:         row.Cells[1].GetValue(),
			Namespace:    row.Cells[2].GetValue(),
			Name:         row.Cells[3].GetValue(),
		})
	}
	return resources, nil
}

// GroupVersionKind returns the GroupVersionKind of the current resource.
func (row ResourceTableRow) GroupVersionKind() string {
	return row.GroupVersion + "/" + row.Kind
}

// NamespacedName returns the NamespacedName of the current resource.
func (row ResourceTableRow) NamespacedName() string {
	if row.Namespace == "" {
		return row.Name
	}
	return row.Namespace + "/" + row.Name
}
