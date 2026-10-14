package kernel

const RememberPath = "remember"

type RememberParams[Req, F any] struct {
	Fields      []FieldDef[Req, F]
	Label       string
	Description string
}

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
