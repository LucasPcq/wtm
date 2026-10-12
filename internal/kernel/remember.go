package kernel

// RememberPath is the field that names the answers to always use: the paths
// of the rememberable fields the user ticked. The form fills it from its
// "always use this answer" boxes; the command's Apply keeps them.
const RememberPath = "remember"

type RememberParams[Req, F any] struct {
	Fields      []FieldDef[Req, F]
	Label       string
	Description string
}

// Remember declares the remember field over fields: a multiselect accepting
// only the paths of the rememberable ones.
func Remember[Req, F any](params RememberParams[Req, F]) FieldDef[Req, F] {
	var paths []string
	for _, field := range params.Fields {
		if field.Spec.Rememberable {
			paths = append(paths, field.Spec.Path)
		}
	}
	return FieldDef[Req, F]{Spec: FieldSpec{
		Path:        RememberPath,
		Type:        FieldMultiSelect,
		Label:       params.Label,
		Description: params.Description,
		Choices:     ChoicesStatic,
		Constraints: Constraints{Enum: paths},
	}}
}
