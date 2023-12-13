package sm

type SmStore struct {
	SmStoreType string `json:"smStoreType" eru:"required"`
}

type SmStoreI interface {
	//SaveSm(ctx context.Context) (docId string, err error)
}

func GetSm(storageType string) SmStoreI {
	switch storageType {
	case "AWS":
		return new(AwsSmStore)
	case "GCP":
		return new(GcpSmStore)

	default:
		return nil
	}
	return nil
}
