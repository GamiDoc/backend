package mailer

type Message struct {
	FromEmail string
	FromName  string
	To        []string
	Subject   string
	Text      string
	HTML      string
}
