package voice

type ControlMessage struct {
	Type     string `json:"type"`
	Name     string `json:"name,omitempty"`
	ID       string `json:"id,omitempty"`
	T        int64  `json:"t,omitempty"`
	Muted    *bool  `json:"muted,omitempty"`
	Deafened *bool  `json:"deafened,omitempty"`
	Error    string `json:"error,omitempty"`
	Users    []User `json:"users,omitempty"`
}

type User struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Muted    bool   `json:"muted"`
	Deafened bool   `json:"deafened"`
}
