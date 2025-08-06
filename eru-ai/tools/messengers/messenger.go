package messengers

const (
	SendMessage         = "send_message"
	SubscribeWebhooks   = "subscribe_webhooks"
	GetMessageStatus    = "get_message_status"
	UploadMedia         = "upload_media"
	GetBusinessProfile  = "get_business_profile"
	GetMessageTemplates = "get_message_templates"
	Callback            = "callback"

	// Slack-specific actions
	ListChannels    = "list_channels"
	ListUsers       = "list_users"
	CreateChannel   = "create_channel"
	InviteToChannel = "invite_to_channel"

	Login     = "login"
	GetSsoUrl = "get_sso_url"
)

type MessengerAccount struct {
	PhoneNumberId      string `json:"phone_number_id"`
	BusinessAccountId  string `json:"business_account_id"`
	AccessToken        string `json:"-"`
	WebhookVerifyToken string `json:"-"`
	WebhookUrl         string `json:"webhook_url"`
	ApiVersion         string `json:"api_version"`
}

type messengerAccountWithToken struct {
	PhoneNumberId      string `json:"phone_number_id"`
	BusinessAccountId  string `json:"business_account_id"`
	AccessToken        string `json:"access_token"`
	WebhookVerifyToken string `json:"webhook_verify_token"`
	WebhookUrl         string `json:"webhook_url"`
	ApiVersion         string `json:"api_version"`
}
