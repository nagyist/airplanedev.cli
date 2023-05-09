package apiint

import "github.com/airplanedev/cli/pkg/cli/apiclient/cliapi"

func DefaultUser(userID string) api.User {
	gravatarURL := "https://www.gravatar.com/avatar?d=mp"
	return api.User{
		ID:        userID,
		Email:     "hello@airplane.dev",
		Name:      "Airplane studio",
		AvatarURL: &gravatarURL,
	}
}
