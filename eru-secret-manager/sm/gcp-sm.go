package sm

type GcpSmStore struct {
	SmStore
	SmName string `json:"smName" eru:"required"`
}
