package sm

import "github.com/aws/aws-sdk-go/aws/session"

type AwsSmStore struct {
	SmStore
	Region         string `json:"region" eru:"required"`
	SmName         string `json:"smName" eru:"required"`
	Authentication string `json:"authentication" eru:"required"`
	Key            string `json:"key" eru:"required"`
	Secret         string `json:"secret" eru:"required"`
	session        *session.Session
}
