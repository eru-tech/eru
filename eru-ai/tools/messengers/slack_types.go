package messengers

type SlackEventPayload struct {
	Token     string `json:"token"`
	TeamId    string `json:"team_id"`
	ApiAppId  string `json:"api_app_id"`
	Event     SlackEvent `json:"event"`
	Type      string `json:"type"`
	EventId   string `json:"event_id"`
	EventTime int64  `json:"event_time"`
}

type SlackEvent struct {
	Type      string `json:"type"`
	Channel   string `json:"channel,omitempty"`
	User      string `json:"user,omitempty"`
	Text      string `json:"text,omitempty"`
	Ts        string `json:"ts,omitempty"`
	EventTs   string `json:"event_ts,omitempty"`
	ChannelType string `json:"channel_type,omitempty"`
	
	// Message-specific fields
	ClientMsgId string `json:"client_msg_id,omitempty"`
	Thread_ts   string `json:"thread_ts,omitempty"`
	
	// Reaction fields
	Reaction string `json:"reaction,omitempty"`
	Item     struct {
		Type    string `json:"type"`
		Channel string `json:"channel"`
		Ts      string `json:"ts"`
	} `json:"item,omitempty"`
	
	// File fields
	FileId string `json:"file_id,omitempty"`
	File   struct {
		Id   string `json:"id"`
		Name string `json:"name"`
	} `json:"file,omitempty"`
	
	// App mention fields
	Blocks []interface{} `json:"blocks,omitempty"`
}

type SlackMessageResponse struct {
	Ok      bool   `json:"ok"`
	Channel string `json:"channel,omitempty"`
	Ts      string `json:"ts,omitempty"`
	Message struct {
		Type    string `json:"type"`
		Subtype string `json:"subtype,omitempty"`
		Text    string `json:"text"`
		User    string `json:"user"`
		Ts      string `json:"ts"`
		BotId   string `json:"bot_id,omitempty"`
	} `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
	Warning string `json:"warning,omitempty"`
}

type SlackErrorResponse struct {
	Ok    bool   `json:"ok"`
	Error string `json:"error"`
}

type SlackChannel struct {
	Id             string   `json:"id"`
	Name           string   `json:"name"`
	IsChannel      bool     `json:"is_channel"`
	IsGroup        bool     `json:"is_group"`
	IsIm           bool     `json:"is_im"`
	Created        int64    `json:"created"`
	IsArchived     bool     `json:"is_archived"`
	IsGeneral      bool     `json:"is_general"`
	Unlinked       int      `json:"unlinked"`
	NameNormalized string   `json:"name_normalized"`
	IsShared       bool     `json:"is_shared"`
	IsPrivate      bool     `json:"is_private"`
	IsMember       bool     `json:"is_member"`
	IsOrgShared    bool     `json:"is_org_shared"`
	Creator        string   `json:"creator"`
	Topic          struct {
		Value   string `json:"value"`
		Creator string `json:"creator"`
		LastSet int64  `json:"last_set"`
	} `json:"topic"`
	Purpose struct {
		Value   string `json:"value"`
		Creator string `json:"creator"`
		LastSet int64  `json:"last_set"`
	} `json:"purpose"`
	Members []string `json:"members,omitempty"`
}

type SlackChannelsListResponse struct {
	Ok       bool          `json:"ok"`
	Channels []SlackChannel `json:"channels"`
	Error    string        `json:"error,omitempty"`
}

type SlackUser struct {
	Id       string `json:"id"`
	TeamId   string `json:"team_id"`
	Name     string `json:"name"`
	Deleted  bool   `json:"deleted"`
	Color    string `json:"color"`
	RealName string `json:"real_name"`
	Tz       string `json:"tz"`
	TzLabel  string `json:"tz_label"`
	TzOffset int    `json:"tz_offset"`
	Profile  struct {
		AvatarHash            string `json:"avatar_hash"`
		StatusText            string `json:"status_text"`
		StatusEmoji           string `json:"status_emoji"`
		RealName              string `json:"real_name"`
		DisplayName           string `json:"display_name"`
		RealNameNormalized    string `json:"real_name_normalized"`
		DisplayNameNormalized string `json:"display_name_normalized"`
		Email                 string `json:"email"`
		Image24               string `json:"image_24"`
		Image32               string `json:"image_32"`
		Image48               string `json:"image_48"`
		Image72               string `json:"image_72"`
		Image192              string `json:"image_192"`
		Image512              string `json:"image_512"`
	} `json:"profile"`
	IsAdmin           bool `json:"is_admin"`
	IsOwner           bool `json:"is_owner"`
	IsPrimaryOwner    bool `json:"is_primary_owner"`
	IsRestricted      bool `json:"is_restricted"`
	IsUltraRestricted bool `json:"is_ultra_restricted"`
	IsBot             bool `json:"is_bot"`
	Updated           int64 `json:"updated"`
	IsAppUser         bool `json:"is_app_user"`
}

type SlackUsersListResponse struct {
	Ok      bool        `json:"ok"`
	Members []SlackUser `json:"members"`
	Error   string      `json:"error,omitempty"`
}

type SlackFileUploadResponse struct {
	Ok   bool `json:"ok"`
	File struct {
		Id   string `json:"id"`
		Name string `json:"name"`
		Size int    `json:"size"`
	} `json:"file"`
	Error string `json:"error,omitempty"`
}