package emails

type EmailAccount struct {
	DisplayName string `json:"display_name"`
	//SecretName  string `json:"secret_name"`
	AccessToken  string `json:"-"`
	RefreshToken string `json:"-"`
}

type emailAccountWithToken struct {
	DisplayName string `json:"display_name"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

const (
	SendEmail         = "send_email"
	ReadEmail         = "read_email"
	SubscribeEmail    = "subscribe_email"
	ReadMessage       = "read_message"
	Callback          = "callback"
	Login             = "login"
	RenewToken        = "renew_token"
	GetSsoUrl         = "get_sso_url"
	RenewSubscription = "renew_subscription"
)
