package main

import (
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"os"

	"encoding/json"
	//"fmt"
	//"log"
	"net/http"
	"net/http/httptest"
	"testing"
)

type XMLUsers struct {
	XMLName xml.Name  `xml:"root"`
	Users   []XMLUser `xml:"row"`
}

type XMLUser struct {
	XMLName   xml.Name `xml:"row"`
	Id        int      `xml:"id"`
	FirstName string   `xml:"first_name"`
	LastName  string   `xml:"last_name"`
	Age       int      `xml:"age"`
	About     string   `xml:"about"`
	Gender    string   `xml:"gender"`
}

var users = []User{
	User{
		Id:     0,
		Name:   "yanis",
		Age:    25,
		About:  "my name is yanis",
		Gender: "male",
	},
}

func SearchServer(w http.ResponseWriter, r *http.Request) {
	xmlFile, err := os.Open("dataset.xml")
	if err != nil {
		// send 500
		fmt.Println(err)
	}
	defer xmlFile.Close()

	byteValue, err := ioutil.ReadAll(xmlFile)
	if err != nil {
		// send 500
		fmt.Println(err)
	}

	// we initialize our Users array
	var xmlUsers XMLUsers
	// we unmarshal our byteArray which contains our
	// xmlFiles content into 'users' which we defined above
	err = xml.Unmarshal(byteValue, &xmlUsers)
	if err != nil {
		// send 500
		fmt.Println(err)
	}

	fmt.Println(xmlUsers.Users[0].About)

	w.WriteHeader(200)
	result, _ := json.Marshal(users)
	w.Write(result)
}

func TestFindUsers(t *testing.T) {

	ts := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer ts.Close()

	searchClient := &SearchClient{
		AccessToken: "password",
		URL:         ts.URL,
	}

	req := SearchRequest{
		Limit:      5,
		Offset:     0,
		Query:      "",
		OrderField: "",
		OrderBy:    0,
	}

	res, err := searchClient.FindUsers(req)

	if err != nil {
		t.Errorf("Should not have error")
	}
	//if !reflect.DeepEqual(res.Users, users) {
	//	t.Errorf("Users should be equals")
	//}
	if res.NextPage {
		t.Errorf("There should be no next page")
	}
	fmt.Println(res.Users)
}
