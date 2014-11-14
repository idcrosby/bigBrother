package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"text/template"
	"time"
)

var configuration Configuration
var Verbose bool
var Update bool

// Templates
var statusTemplateFile = "statusHtml.txt"

func main() {

	// Define flags
	flag.BoolVar(&Verbose, "verbose", false, "Turn on verbose logging.")
	flag.BoolVar(&Update, "update", false, "Update Confluence Page.")
	flag.Parse()

	// Read in Config
	file, err := os.Open("conf.json")
	check(err)
	decoder := json.NewDecoder(file)
	configuration = Configuration{}
	err = decoder.Decode(&configuration)
	check(err)

	var statusMap map[Resource]int

	if configuration.RunOnce {
		statusMap = checkAllResouces()
	} else {
		ticker := time.NewTicker(time.Duration(configuration.Period) * time.Second)
		quit := make(chan struct{})
		go func(){
			for {
				select {
				case <- ticker.C:
					statusMap = checkAllResouces()
				case <- quit:
					ticker.Stop()
					return
				}
			}
		}()
	}

	// 
	if Update {
		updateConfluencePage(statusMap)
	}
}

func checkAllResouces() (statusMap map[Resource]int) {

	statusMap = make(map[Resource]int)
	// For Each endpoint make API call	
	for _, resource := range configuration.Resources {
		status := checkResource(resource)
		statusMap[resource] = status
	}
	return statusMap
}

func checkResource(resource Resource) int {

	req, err := http.NewRequest("GET", resource.Url, nil)
	check(err)

	if resource.OauthProtected {
		var token string
		if len(resource.Oauth.Url) > 0 {
			token = GetToken(resource.Oauth)
		} else {
			token = GetToken(configuration.Oauth)
		}
		req.Header.Add("Authorization", "Bearer " + token)
	} else {

		switch resource.KeyLocation {
		case "Header":
			req.Header.Add(resource.KeyParamName, resource.Key)
		case "Query":
			req.URL.Query().Add(resource.KeyParamName, resource.Key)
		}
		// TODO ..
	}

	res, err := http.DefaultClient.Do(req)
	check(err)
	fmt.Printf("Status code: %d\n", res.StatusCode)
	if resource.ExpectedStatus == 0 {
		resource.ExpectedStatus = 200
	}
	
	if res.StatusCode == resource.ExpectedStatus {
			fmt.Printf("Resource %s is ok\n", resource.Name)
	} else {
			fmt.Printf("Resource %s is not ok\n", resource.Name)
	}

	// TODO add additional verification steps

	return res.StatusCode

}

func updateConfluencePage(statusMap map[Resource]int) {

	// TODO first check if exists
	var method string
	pageUrl := configuration.Confluence.Url + "content/"
	page := ConfluencePage{Type: "page"}

	oldPage := fetchPageById(configuration.Confluence.PageId)

	// if len(oldPage.Id) > 0 {
	// Update
	method = "PUT"
	pageUrl += configuration.Confluence.PageId
	page.Id = oldPage.Id
	page.Title = oldPage.Title
	page.Version.Number = oldPage.Version.Number + 1
	// } else {
	// 	method = "POST"
	// }

	page.Space.Key = configuration.Confluence.SpaceKey
	page.Body.Storage.Representation = "storage"
	var pageBuffer bytes.Buffer

	const layout = "Jan 2, 2006 at 3:04pm (MST)"
	pageBuffer.WriteString("<h2>*This page is auto-generated. Do not modify*</h2>")
	pageBuffer.WriteString("<p>Created: " + time.Now().Format(layout) + "</p>")
	statusTemplate, err := template.ParseFiles(statusTemplateFile)
	check(err)

	endpoints := make([]Endpoint, len(statusMap))
	count := 0
	for key, val := range statusMap {
		fmt.Println("Adding endpoint...")
		endpoints[count] = Endpoint{Name: key.Name, Url: key.Url, Status: val}
		count += 1
	}
	statusTemplate.Execute(&pageBuffer, endpoints)

	page.Body.Storage.Value = pageBuffer.String()
	
	// Send Confluence API Create
	buff, err := json.Marshal(page)
	check(err)

	// fmt.Println("Create page with " + string(buff))
	req, err := http.NewRequest(method, pageUrl, bytes.NewReader(buff))
	check(err)

	req.SetBasicAuth(configuration.Confluence.User, configuration.Confluence.Password)
	req.Header.Add("Content-Type", "application/json")

	// if method == "POST" {
		// fmt.Println("Creating confluence page: " + page.Title + "...")
	// } else {
		fmt.Println("Updating confluence page: " + page.Title + "...")
	// }
	res, err := http.DefaultClient.Do(req)
	check(err)

	// Check status
	if res.StatusCode != 200 {
		defer res.Body.Close()
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println("Error creating confluence page: " + page.Title)
		fmt.Println("Response: ", string(body))
		fmt.Println("Request: ", string(buff))
	} else {
		fmt.Println("...completed successfully.")
	}
}

func GetToken(oauthConf OauthData) string {
	// If this is changed from script to long running program will need to keep track of token ttl
	// if len(AccessToken) > 0 {
	// 	return AccessToken
	// }

	postBody := "grant_type=" + oauthConf.GrantType
	req, err := http.NewRequest("POST", oauthConf.Url, strings.NewReader(postBody))
	if err != nil {
		fmt.Println("1. error retrieving token...", err)
	}
	req.SetBasicAuth(oauthConf.Username, oauthConf.Password)
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

func fetchPageById(id string) ConfluencePage {
	req, err := http.NewRequest("GET", configuration.Confluence.Url + "content/" + id, nil)
	check(err)
	req.SetBasicAuth(configuration.Confluence.User, configuration.Confluence.Password)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println("Error fetching confluence page ", err)
		return ConfluencePage{}
	}
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	var result ConfluencePage
	err = json.Unmarshal(body, &result)
	check(err)

	return result
}

func check(err error) { 
	if err != nil {
		fmt.Println("Panicking: ", err) 
		panic(err) 
	} 
}

type Configuration struct {
	Resources []Resource
	Oauth OauthData
	RunOnce bool
	Period int
	Confluence struct {
		User string
		Password string
		SpaceKey string
		PageId string
	}
}

type Resource struct {
	Name string
	Url string
	Key string
	KeyLocation string
	KeyParamName string
	ExpectedStatus int
	OauthProtected bool
	Oauth OauthData
}

type OauthData struct {
	Url string
	GrantType string
	Username string
	Password string
}

type Endpoint struct {
	Name string
	Url string
	Status int
}

// Genericize this
type OAuthToken struct {
	TokenType string		`json:"token_type"`
	Mapi string				`json:"mapi"`
	AccessToken string		`json:"access_token"`
	ExpiresIn int 			`json:"expires_in"`
	RefreshToken string		`json:"refresh_token"`
	Scope string			`json:"scope"`
}

// Confluence Types

type ConfluencePage struct {
	Id string                        `json:"id,omitempty"`
	Type string                      `json:"type"`
	Title string                     `json:"title,omitempty"`
	Space struct {
		Key string                   `json:"key"`
		}                            `json:"space"`
	Body struct {
		Storage struct {
			Value string             `json:"value"`
			Representation string    `json:"representation"`
		}                            `json:"storage"`
	}                                `json:"body"`
	Version struct {
		// By struct {
		// 	Type string              `json:"type"`
		// 	Username string          `json:"username"`
		// 	DisplayName string       `json:"displayName"`
		// 	UserKey string           `json:"userKey"`
		// }                            `json:"by"`
		// When string                  `json:"when"`
		// Message string               `json:"message"`
		Number int                   `json:"number"`
		// MinorEdit bool               `json:"minorEdit"`
	}                                `json:"version"`
}