package wtm

type Warning struct {
	Code    string `json:"code"`
	Params  Params `json:"params,omitempty"`
	Message string `json:"message"`
}
