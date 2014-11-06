package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
)

var configuration Configuration
var Verbose bool

func main() {

	// Define flags
	flag.BoolVar(&Verbose, "verbose", false, "Turn on verbose logging.")
	flag.Parse()

	// Read in Config
	file, err := os.Open("conf.json")
	check(err)
	decoder := json.NewDecoder(file)
	configuration = Configuration{}
	err = decoder.Decode(&configuration)
	check(err)

	// For Each endpoint make API call	
	for _, resource := range configuration.Resources {
		ok := checkResource(resource)
		if ok {
			fmt.Printf("Resource %s is ok\n", resource.Name)
		} else {
			fmt.Printf("Resource %s is not ok\n", resource.Name)
		}
	}
	// 
}

func checkResource(resource Resource) bool {

	req, err := http.NewRequest("GET", resource.Url, nil)
	check(err)

	if resource.OauthProtected {
		token := GetToken()
		req.Header.Add("Authorization", "Bearer " + token)
	} else {
		// TODO ..
	}

	res, err := http.DefaultClient.Do(req)
	check(err)
	fmt.Printf("Status code: %d\n", res.StatusCode)
	return res.StatusCode == 200

}

func GetToken() string {
	// If this is changed from script to long running program will need to keep track of token tll
	// if len(AccessToken) > 0 {
	// 	return AccessToken
	// }

	postBody := "grant_type=" + configuration.Oauth.GrantType
	req, err := http.NewRequest("POST", configuration.Oauth.Url, strings.NewReader(postBody))
	if err != nil {
		fmt.Println("1. error retrieving token...", err)
	}
	req.SetBasicAuth(configuration.Oauth.Username, configuration.Oauth.Password)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println("2. error retrieving token...", err)
	}
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	var token OAuthToken
	err = json.Unmarshal(body, &token)
	if err != nil {
		fmt.Println("3. error retrieving token...", err, string(body))
	}

	// AccessToken = token.AccessToken
	return token.AccessToken
}

func check(err error) { 
	if err != nil {
		fmt.Println("Panicking: ", err) 
		panic(err) 
	} 
}

type Configuration struct {
	Resources []Resource
	Oauth struct {
		Url string
		GrantType string
		Username string
		Password string
	}
	RunOnce bool
	Period int
}

type Resource struct {
	Name string
	Url string
	Key string
	KeyLocation string
	OauthProtected bool
	Oauth struct {
		Url string
		GrantType string
		Username string
		Password string
	}
}

// Generisize this
type OAuthToken struct {
	TokenType string		`json:"token_type"`
	Mapi string				`json:"mapi"`
	AccessToken string		`json:"access_token"`
	ExpiresIn int 			`json:"expires_in"`
	RefreshToken string		`json:"refresh_token"`
	Scope string			`json:"scope"`
}