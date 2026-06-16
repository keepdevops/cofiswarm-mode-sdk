package mode

type Envelope struct {
	Mode   string                 `json:"mode"`
	Agents map[string]string      `json:"agents"`
	Final  *string                `json:"final"`
	Meta   map[string]interface{} `json:"meta"`
}

func StubEnvelope(modeName, prompt string) Envelope {
	final := prompt
	return Envelope{
		Mode:   modeName,
		Agents: map[string]string{"architect": prompt},
		Final:  &final,
		Meta:   map[string]interface{}{"stub": true},
	}
}
