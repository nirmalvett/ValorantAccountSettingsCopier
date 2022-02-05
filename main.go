package main

import (
	"bufio"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-resty/resty/v2"
	"net"
	"net/http"
	"os"
	"regexp"
	"strings"
)

type ValAccount struct {
	client *resty.Client
}

func NewValAccount() *ValAccount {
	account := new(ValAccount)
	customClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	account.client = resty.NewWithClient(customClient)
	account.client.SetHeaders(map[string]string {
		"Content-Type": "application/json",
		"Accept-Encoding": "gzip, deflate, br",
		"Host": "auth.riotgames.com",
		"User-Agent": "RiotClient/43.0.1.4195386.4190634 rso-auth (Windows;10;;Professional, x64)",
	})
	return account
}

func (v *ValAccount) Authenticate(username string, password string) error {
	authConn, err := net.Dial("tcp", "auth.riotgames.com:443")
	if err != nil {
		return err
	}

	authDomain := fmt.Sprintf("https://%s", authConn.RemoteAddr().String())

	resp, err := v.client.R().
		SetBody(map[string]interface{}{
			"client_id": "play-valorant-web-prod",
			"nonce": "1",
			"redirect_uri": "https://playvalorant.com/opt_in",
			"response_type": "token id_token",
		}).
		Post(authDomain + "/api/v1/authorization")

	if err != nil {
		return err
	}

	resp, err = v.client.R().
		SetBody(map[string]interface{}{
			"type": "auth",
			"username": username,
			"password": password,
		}).
		SetCookies(resp.Cookies()).
		Put(authDomain + "/api/v1/authorization")

	if err != nil {
		return err
	}

	var authResp map[string]interface{}
	err = json.Unmarshal(resp.Body(), &authResp)
	if err != nil {
		return err
	}

	if _, ok := authResp["error"]; ok {
		return errors.New("failed to authenticate")
	}

	response := authResp["response"].(map[string]interface{})
	parameters := response["parameters"].(map[string]interface{})
	uri := parameters["uri"].(string)
	r := regexp.MustCompile("access_token=((?:[a-zA-Z]|\\d|\\.|-|_)*).*id_token=((?:[a-zA-Z]|\\d|\\.|-|_)*).*expires_in=(\\d*)")

	matched := r.FindStringSubmatch(uri)
	if len(matched) != 4 {
		return errors.New("unable to match access token")
	}

	accessToken := matched[1]
	v.client.SetHeader("Authorization", "Bearer " + accessToken)

	entConn, err := net.Dial("tcp", "entitlements.auth.riotgames.com:443")
	if err != nil {
		return err
	}

	entDomain := fmt.Sprintf("https://%s", entConn.RemoteAddr().String())

	resp, err = v.client.R().
		SetHeaders(map[string]string {
			"Content-Type": "application/json",
			"Accept-Encoding": "gzip, deflate, br",
			"Host": "entitlements.auth.riotgames.com",
			"User-Agent": "RiotClient/43.0.1.4195386.4190634 rso-auth (Windows;10;;Professional, x64)",
		}).
		SetCookies(resp.Cookies()).
		Post(entDomain + "/api/token/v1")
	if err != nil {
		return err
	}

	var entitlementsResp map[string]interface{}
	err = json.Unmarshal(resp.Body(), &entitlementsResp)
	if err != nil {
		return err
	}

	entitlementsToken := entitlementsResp["entitlements_token"].(string)
	v.client.SetHeader("X-Riot-Entitlements-JWT", entitlementsToken)
	return nil
}

func (v *ValAccount) GetSettings() (string, error) {
	resp, err := v.client.R().
		SetHeaders(map[string]string {
			"Content-Type": "application/json",
			"Accept-Encoding": "gzip, deflate, br",
			"Host": "playerpreferences.riotgames.com",
			"User-Agent": "RiotClient/43.0.1.4195386.4190634 rso-auth (Windows;10;;Professional, x64)",
		}).
		Get("https://playerpreferences.riotgames.com/playerPref/v3/getPreference/Ares.PlayerSettings")
	if err != nil {
		return "", err
	}

	var settingsResp map[string]interface{}
	err = json.Unmarshal(resp.Body(), &settingsResp)
	if err != nil {
		return "", err
	}

	return settingsResp["data"].(string), nil
}

func (v *ValAccount) SetSettings(settings string) (string, error) {
	resp, err := v.client.R().
		SetHeaders(map[string]string {
			"Content-Type": "application/json",
			"Accept-Encoding": "gzip, deflate, br",
			"Host": "playerpreferences.riotgames.com",
			"User-Agent": "RiotClient/43.0.1.4195386.4190634 rso-auth (Windows;10;;Professional, x64)",
		}).
		SetBody(map[string]interface{}{
			"data": settings,
			"type": "Ares.PlayerSettings",
		}).
		Put("https://playerpreferences.riotgames.com/playerPref/v3/savePreference")
	if err != nil {
		return "", err
	}

	var settingsResp map[string]interface{}
	err = json.Unmarshal(resp.Body(), &settingsResp)
	if err != nil {
		return "", err
	}

	return settingsResp["data"].(string), nil
}

func ReadString(reader *bufio.Reader) string {
	input, _ := reader.ReadString('\n')
	input = strings.TrimRight(input, "\n")
	input = strings.TrimRight(input, "\r")
	return input
}

func run() {
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("Valorant Account Settings Copier")
	fmt.Println("--------------------------------")

	fmt.Print("From account login name: ")
	fromUser := ReadString(reader)
	fmt.Print("From account login password: ")
	fromPassword := ReadString(reader)

	fmt.Print("To account login name: ")
	toUser := ReadString(reader)
	fmt.Print("To account login password: ")
	toPassword := ReadString(reader)

	from := NewValAccount()
	err := from.Authenticate(fromUser, fromPassword)
	if err != nil {
		fmt.Println(err)
		return
	}

	to := NewValAccount()
	err = to.Authenticate(toUser, toPassword)
	if err != nil {
		fmt.Println(err)
		return
	}

	fromSettings, err := from.GetSettings()
	if err != nil {
		fmt.Println(err)
		return
	}

	toSettings, err := to.SetSettings(fromSettings)
	if err != nil {
		fmt.Println(err)
		return
	}

	if fromSettings == toSettings {
		fmt.Println("Account settings transferred successfully")
	} else {
		fmt.Println("Failed to transfer settings")
	}
}

func main() {
	run()

	fmt.Print("Press 'Enter' to close...")
	_, err := fmt.Scanln()
	if err != nil {
		fmt.Println(err)
	}
}