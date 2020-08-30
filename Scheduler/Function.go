package Scheduler

import (
	"cloud.google.com/go/firestore"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
)

type TwitchTokens struct {
	AccessToken  string   `json:"access_token"`
	RefreshToken string   `json:"refresh_token"`
	ExpiresIn    int      `json:"expires_in"`
	Scope        []string `json:"scope"`
	TokenType    string   `json:"token_type"`
}

func GetClientToken() (*TwitchTokens, error) {
	oauthUrl, _ := url.Parse("https://id.twitch.tv/oauth2/token")
	query, _ := url.ParseQuery(oauthUrl.RawQuery)
	query.Add("client_id", os.Getenv("clientid"))
	query.Add("client_secret", os.Getenv("clientsecret"))
	query.Add("grant_type", "client_credentials")
	oauthUrl.RawQuery = query.Encode()
	resp, err := http.Post(oauthUrl.String(), "application/json", strings.NewReader(""))
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 300 {
		fmt.Println(ioutil.ReadAll(resp.Body))
		fmt.Printf("%+v\n", resp)
		return nil, errors.New("failed to get token")
	}

	var d TwitchTokens
	if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
		return nil, err
	}

	return &d, nil
}

type TwitchUser struct {
	BroadcasterType string `json:"broadcaster_type"`
	Description     string `json:"description"`
	DisplayName     string `json:"display_name"`
	Email           string `json:"email"`
	Id              string `json:"id"`
	Login           string `json:"login"`
	OfflineImageUrl string `json:"offline_image_url"`
	ProfileImageUrl string `json:"profile_image_url"`
	UserType        string `json:"type"`
	ViewCount       int    `json:"view_count"`
}

type TwitchUserResponse struct {
	Data []TwitchUser `json:"data"`
}

type DiscordRole struct {
	Id             string `json:"id"`
	Name           string `json:"name"`
	Color          int    `json:"color"`
	Hoist          bool   `json:"hoist"`
	Position       int    `json:"position"`
	Permissions    int    `json:"permissions"`
	PermissionsNew string `json:"permissions_new"`
	Managed        bool   `json:"managed"`
	Mentionable    bool   `json:"mentionable"`
}

func GetDiscordGuildRoles(guildId string) ([]DiscordRole, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("https://discord.com/api/v6/guilds/%s/roles", guildId), strings.NewReader(""))
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bot %s", os.Getenv("discordtoken")))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		body, err := ioutil.ReadAll(resp.Body)
		fmt.Println(string(body))
		fmt.Print(err)
		return nil, errors.New("failed to fetch roles for guild")
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var roles []DiscordRole
	if err := json.Unmarshal(body, &roles); err != nil {
		return nil, err
	}

	return roles, nil
}

type DiscordUser struct {
	Id            string `json:"id"`
	Username      string `json:"username"`
	Discriminator string `json:"discriminator"`
	Avatar        string `json:"avatar"`
	Bot           bool   `json:"bot"`
	System        bool   `json:"system"`
	MfaEnabled    bool   `json:"mfa_enabled"`
	Locale        string `json:"locale"`
	Verified      bool   `json:"verified"`
	Email         string `json:"email"`
	Flags         int    `json:"flags"`
	PremiumType   int    `json:"premium_type"`
	PublicFlags   int    `json:"public_flags"`
}

type DiscordMember struct {
	User         DiscordUser `json:"user"`
	Nick         string      `json:"nick"`
	Roles        []string    `json:"roles"`
	JoinedAt     string       `json:"joined_at"`
	PremiumSince string       `json:"premium_since"`
	Deaf         bool        `json:"deaf"`
	Mute         bool        `json:"mute"`
}

func GetDiscordGuildMembers(guildId string) ([]DiscordMember, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("https://discord.com/api/v6/guilds/%s/members?limit=1000", guildId), strings.NewReader(""))
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bot %s", os.Getenv("discordtoken")))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		body, err := ioutil.ReadAll(resp.Body)
		fmt.Println(string(body))
		fmt.Print(err)
		return nil, errors.New("failed to fetch members for guild")
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var members []DiscordMember
	if err := json.Unmarshal(body, &members); err != nil {
		return nil, err
	}

	return members, nil
}

type RootConfig struct {
	Watchlist []string `firestore:"watchlist"`
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

type PubSubMessage struct {
	Data []byte `json:"data"`
}

func SubscribeWebhooks(ctx context.Context, m PubSubMessage) error {
	client, err := firestore.NewClient(ctx, "bitmasher-dev")
	if err != nil {
		return err
	}
	defer client.Close()

	configCol := client.Collection("Configs")
	rootSnap, err := configCol.Doc("root").Get(ctx)
	if err != nil {
		return err
	}

	if !rootSnap.Exists() {
		fmt.Println("no configs exist")
		return nil
	}
	var rootConfig RootConfig
	err = rootSnap.DataTo(&rootConfig)
	if err != nil {
		return err
	}

	guildSnap, err := configCol.Doc("guildconfigs").Get(ctx)
	if err != nil {
		return err
	}

	if !guildSnap.Exists() {
		fmt.Println("no configs exist")
		return nil
	}

	var guildChannels map[string]string
	err = guildSnap.DataTo(&guildChannels)
	if err != nil {
		return err
	}

	tokens, err := GetClientToken()
	if err != nil {
		return err
	}
	for i := range rootConfig.Watchlist {

		// get roles for guild to find id of streamer role
		guildRoles, err := GetDiscordGuildRoles(rootConfig.Watchlist[i])
		if err != nil {
			return err
		}

		// find the streamer role
		streamerRole := ""
		for ir := range guildRoles {
			if strings.EqualFold(guildRoles[ir].Name, "streamer") {
				streamerRole = guildRoles[ir].Id
			}
		}

		// no streamer role? no integration for you!
		if len(streamerRole) == 0 {
			continue
		}

		guildMembers, err := GetDiscordGuildMembers(rootConfig.Watchlist[i])
		if err != nil {
			return err
		}

		for im := range guildMembers {

			if !contains(guildMembers[im].Roles, streamerRole) {
				continue
			}

			streamerName := guildMembers[im].User.Username
			if len(guildMembers[im].Nick) > 0 {
				streamerName = guildMembers[im].Nick
			}

			req, err := http.NewRequest("GET", fmt.Sprintf("https://api.twitch.tv/helix/users?login=%s", streamerName), nil)
			if err != nil {
				return err
			}
			req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", tokens.AccessToken))
			req.Header.Add("Client-ID", os.Getenv("clientid"))
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return err
			}

			if resp.StatusCode >= 300 {
				continue
			}

			var userDets TwitchUserResponse
			if err = json.NewDecoder(resp.Body).Decode(&userDets); err != nil {
				return err
			}

			fmt.Printf("%+v\n", userDets)

			if len(userDets.Data) == 0 {
				continue
			}

			str := fmt.Sprintf("{\"hub.callback\": \"%s?userid=%s\",\"hub.mode\": \"subscribe\",\"hub.topic\":\"https://api.twitch.tv/helix/streams?user_id=%s\",\"hub.lease_seconds\": \"864000\",\"hub.secret\": \"%s\"}", os.Getenv("callbackuri"), userDets.Data[0].Login, userDets.Data[0].Id, os.Getenv("clientsecret"))
			req, err = http.NewRequest("POST", "https://api.twitch.tv/helix/webhooks/hub", strings.NewReader(str))
			if err != nil {
				return err
			}
			req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", tokens.AccessToken))
			req.Header.Add("Client-ID", os.Getenv("clientid"))
			req.Header.Add("Content-Type", "application/json")
			resp, err = http.DefaultClient.Do(req)
			if err != nil {
				return err
			}
			if resp.StatusCode >= 300 {
				fmt.Println(ioutil.ReadAll(resp.Body))
				return errors.New("failed to create webhook")
			}

			userSnap, err := configCol.Doc("usermap").Get(ctx)
			if err != nil {
				return err
			}

			if !userSnap.Exists() {
				fmt.Println("no configs exist")
				return nil
			}
			var userMap map[string][]string
			err = userSnap.DataTo(&userMap)
			if err != nil {
				return err
			}

			if uMap, ok := userMap[userDets.Data[0].Login]; ok {
				if !contains(uMap, guildChannels[rootConfig.Watchlist[i]]) {
					uMap = append(uMap, guildChannels[rootConfig.Watchlist[i]])
					userMap[userDets.Data[0].Login] = uMap
				}
			} else {
				uMap = []string{guildChannels[rootConfig.Watchlist[i]]}
				userMap[userDets.Data[0].Login] = uMap
			}
			_, err = configCol.Doc("usermap").Set(ctx, userMap)
			if err != nil {
				return err
			}
			fmt.Printf("%v\n", userMap)
		}
	}
	return nil
}
