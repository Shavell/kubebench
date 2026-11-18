package reason

// Message is an explanation on its own, outside an error response: the
// English text with its reason code and parameters, if it has them, as
// {"error": "…", "reason": "…", "params": {…}}. Streams end with one (a
// log stream's "event: error", a watch stream's "event: end", a
// terminal's error frame), and a Message can be the parameter of another
// reason ({{cause}} in "the session was closed: {{cause}}"): the UI then
// translates it in place, and the English text renders it by Text.
type Message struct {
	Text   string `json:"error"`
	Code   string `json:"reason,omitempty"`
	Params Params `json:"params,omitempty"`
}

// String is the English text, so that Render fills a placeholder with it.
func (m Message) String() string { return m.Text }

// MessageOf explains err: its reason (Of) when it has one, else its
// message alone, which the UI shows as it is.
func MessageOf(err error) Message {
	m := Message{Text: err.Error()}
	if code, params, ok := Of(err); ok {
		m.Code, m.Params = code.String(), params
	}
	return m
}
