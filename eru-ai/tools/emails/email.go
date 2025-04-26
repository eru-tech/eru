package emails

type EmailAccount struct {
	DisplayName string `json:"display_name"`
	SecretName  string `json:"secret_name"`
}

const (
	SendEmail = "send_email"
	ReadEmail = "read_email"
)