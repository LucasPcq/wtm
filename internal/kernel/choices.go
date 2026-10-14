package kernel

type Badge struct {
	Code   Code   `json:"code"`
	Params Params `json:"params,omitempty"`
}

type Option struct {
	Value   string      `json:"value"`
	Label   string      `json:"label"`
	Danger  bool        `json:"danger,omitempty"`
	Refusal *FieldError `json:"refusal,omitempty"`
	Badges  []Badge     `json:"badges,omitempty"`
}

type Choices struct {
	Options []Option `json:"options"`
	Pinned  string   `json:"pinned,omitempty"`
	Banner  *Warning `json:"banner,omitempty"`
}
