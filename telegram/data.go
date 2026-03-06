package telegram

type CommandHandler func() (string, error)

type update struct {
	UpdateID int      `json:"update-id"`
	Message  *message `json:"message"`
}

type message struct {
	Chat chat   `json:"chat"`
	Text string `json:"text"`
}

type chat struct {
	ID int64 `json:"id"`
}

type getUpdatesResponse struct {
	OK     bool     `json:"ok"`
	Result []update `json:"result"`
}
