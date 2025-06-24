package messengers

type WhatsAppWebhookPayload struct {
	Object string `json:"object"`
	Entry  []struct {
		Id      string `json:"id"`
		Changes []struct {
			Value struct {
				MessagingProduct string `json:"messaging_product"`
				Metadata         struct {
					DisplayPhoneNumber string `json:"display_phone_number"`
					PhoneNumberId      string `json:"phone_number_id"`
				} `json:"metadata"`
				Contacts []struct {
					Profile struct {
						Name string `json:"name"`
					} `json:"profile"`
					WaId string `json:"wa_id"`
				} `json:"contacts,omitempty"`
				Messages []struct {
					From      string `json:"from"`
					Id        string `json:"id"`
					Timestamp string `json:"timestamp"`
					Type      string `json:"type"`
					Text      struct {
						Body string `json:"body"`
					} `json:"text,omitempty"`
					Image struct {
						Caption string `json:"caption,omitempty"`
						MimeType string `json:"mime_type"`
						Sha256  string `json:"sha256"`
						Id      string `json:"id"`
					} `json:"image,omitempty"`
					Audio struct {
						Id       string `json:"id"`
						MimeType string `json:"mime_type"`
					} `json:"audio,omitempty"`
					Video struct {
						Caption  string `json:"caption,omitempty"`
						Filename string `json:"filename,omitempty"`
						Id       string `json:"id"`
						MimeType string `json:"mime_type"`
					} `json:"video,omitempty"`
					Document struct {
						Caption  string `json:"caption,omitempty"`
						Filename string `json:"filename"`
						Id       string `json:"id"`
						MimeType string `json:"mime_type"`
					} `json:"document,omitempty"`
					Location struct {
						Latitude  float64 `json:"latitude"`
						Longitude float64 `json:"longitude"`
						Name      string  `json:"name,omitempty"`
						Address   string  `json:"address,omitempty"`
					} `json:"location,omitempty"`
					Contacts []struct {
						Name struct {
							FormattedName string `json:"formatted_name"`
							FirstName     string `json:"first_name,omitempty"`
							LastName      string `json:"last_name,omitempty"`
						} `json:"name"`
						Phones []struct {
							Phone string `json:"phone"`
							Type  string `json:"type,omitempty"`
						} `json:"phones,omitempty"`
					} `json:"contacts,omitempty"`
					Interactive struct {
						Type         string `json:"type"`
						ButtonReply struct {
							Id    string `json:"id"`
							Title string `json:"title"`
						} `json:"button_reply,omitempty"`
						ListReply struct {
							Id          string `json:"id"`
							Title       string `json:"title"`
							Description string `json:"description,omitempty"`
						} `json:"list_reply,omitempty"`
					} `json:"interactive,omitempty"`
					Reaction struct {
						MessageId string `json:"message_id"`
						Emoji     string `json:"emoji"`
					} `json:"reaction,omitempty"`
				} `json:"messages,omitempty"`
				Statuses []struct {
					Id           string `json:"id"`
					Status       string `json:"status"`
					Timestamp    string `json:"timestamp"`
					RecipientId  string `json:"recipient_id"`
					Conversation struct {
						Id                  string `json:"id"`
						ExpirationTimestamp string `json:"expiration_timestamp,omitempty"`
						Origin struct {
							Type string `json:"type"`
						} `json:"origin"`
					} `json:"conversation,omitempty"`
					Pricing struct {
						Billable     bool   `json:"billable"`
						PricingModel string `json:"pricing_model"`
						Category     string `json:"category"`
					} `json:"pricing,omitempty"`
				} `json:"statuses,omitempty"`
			} `json:"value"`
			Field string `json:"field"`
		} `json:"changes"`
	} `json:"entry"`
}

type WhatsAppMessageResponse struct {
	MessagingProduct string `json:"messaging_product"`
	Contacts         []struct {
		Input string `json:"input"`
		WaId  string `json:"wa_id"`
	} `json:"contacts"`
	Messages []struct {
		Id string `json:"id"`
	} `json:"messages"`
}

type WhatsAppErrorResponse struct {
	Error struct {
		Message   string `json:"message"`
		Type      string `json:"type"`
		Code      int    `json:"code"`
		ErrorData struct {
			MessagingProduct string `json:"messaging_product"`
			Details          string `json:"details"`
		} `json:"error_data,omitempty"`
		ErrorSubcode int    `json:"error_subcode,omitempty"`
		FbtraceId    string `json:"fbtrace_id,omitempty"`
	} `json:"error"`
}

type MediaUploadResponse struct {
	Id string `json:"id"`
}

type BusinessProfile struct {
	Id          string `json:"id"`
	Name        string `json:"name"`
	Category    string `json:"category"`
	Description string `json:"description,omitempty"`
	Email       string `json:"email,omitempty"`
	Address     string `json:"address,omitempty"`
	Website     []string `json:"website,omitempty"`
}