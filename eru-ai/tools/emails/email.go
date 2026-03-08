package emails

type EmailAccount struct {
	DisplayName string `json:"display_name"`
	//SecretName  string `json:"secret_name"`
	AccessToken                    string `json:"-"`
	RefreshToken                   string `json:"-"`
	SubscriptionId                 string `json:"subscription_id"`
	SubscriptionExpirationDateTime string `json:"subscription_expiration_date_time"`
	TokenExpirationDateTime        string `json:"token_expiration_date_time"`
}

type emailAccountWithToken struct {
	DisplayName                    string `json:"display_name"`
	AccessToken                    string `json:"access_token"`
	RefreshToken                   string `json:"refresh_token"`
	SubscriptionId                 string `json:"subscription_id"`
	SubscriptionExpirationDateTime string `json:"subscription_expiration_date_time"`
	TokenExpirationDateTime        string `json:"token_expiration_date_time"`
}

const (
	SendEmail         = "send_email"
	ReadEmail         = "read_email"
	ReadConversation  = "read_conversation"
	SubscribeEmail    = "subscribe_email"
	ReadMessage       = "read_message"
	Callback          = "callback"
	Login             = "login"
	RenewToken        = "renew_token"
	GetSsoUrl         = "get_sso_url"
	RenewSubscription = "renew_subscription"
	StopAutoRenew     = "stop_auto_renew"
	StopSubscription  = "stop_subscription"

	ListCalendarEvents     = "list_calendar_events"
	GetCalendarEvent       = "get_calendar_event"
	CreateCalendarEvent    = "create_calendar_event"
	UpdateCalendarEvent    = "update_calendar_event"
	DeleteCalendarEvent    = "delete_calendar_event"
	AcceptCalendarEvent    = "accept_calendar_event"
	DeclineCalendarEvent   = "decline_calendar_event"
	TentativeCalendarEvent = "tentative_calendar_event"
	CancelCalendarEvent    = "cancel_calendar_event"
	ListCalendars          = "list_calendars"
	SubscribeCalendar      = "subscribe_calendar"
)
