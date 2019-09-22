package main

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

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

func getUsersFromFile() ([]User, error) {
	xmlFile, err := os.Open("dataset.xml")
	if err != nil {
		// send 500
		return nil, err
	}
	defer xmlFile.Close()

	byteValue, err := ioutil.ReadAll(xmlFile)
	if err != nil {
		// send 500
		return nil, err
	}

	var xmlUsers XMLUsers
	err = xml.Unmarshal(byteValue, &xmlUsers)
	if err != nil {
		// send 500
		return nil, err
	}

	users := make([]User, 0, len(xmlUsers.Users))
	for _, xmlUser := range xmlUsers.Users {
		user := User{
			Id:     xmlUser.Id,
			Name:   xmlUser.FirstName + " " + xmlUser.LastName,
			Age:    xmlUser.Age,
			About:  xmlUser.About,
			Gender: xmlUser.Gender,
		}
		users = append(users, user)
	}

	return users, nil
}

func filterUsers(users []User, params *SearchRequest) []User {
	if params.Query == "" {
		return users
	}

	filteredUsers := make([]User, 0, 10)

	isLegit := func(user User) bool {
		about := strings.ToLower(user.About)
		name := strings.ToLower(user.Name)
		query := strings.ToLower(params.Query)
		if strings.Contains(about, query) || strings.Contains(name, query) {
			return true
		}
		return false
	}

	for _, user := range users {
		if isLegit(user) {
			filteredUsers = append(filteredUsers, user)
		}
	}

	return filteredUsers
}

func sortUsers(users []User, params *SearchRequest) []User {
	if params.OrderBy == -1 || params.OrderBy == 1 {
		less := func() func(i, j int) bool {
			if params.OrderField == "id" {
				if params.OrderBy == -1 {
					return func(i, j int) bool {
						return users[i].Id > users[j].Id
					}
				}
				return func(i, j int) bool {
					return users[i].Id < users[j].Id
				}
			}
			if params.OrderField == "age" {
				if params.OrderBy == -1 {
					return func(i, j int) bool {
						return users[i].Age > users[j].Age
					}
				}
				return func(i, j int) bool {
					return users[i].Age < users[j].Age
				}
			}
			if params.OrderField == "name" || params.OrderField == "" {
				if params.OrderBy == -1 {
					return func(i, j int) bool {
						return users[i].Name > users[j].Name
					}
				}
				return func(i, j int) bool {
					return users[i].Name < users[j].Name
				}
			}
			// TODO: should be explicit error
			return func(i, j int) bool {
				return true
			}
		}()

		sort.Slice(users, less)
	}

	return users
}

func offsetUsers(users []User, params *SearchRequest) []User {
	if params.Offset >= len(users) {
		return []User{}
	}
	return users[params.Offset:]

}

func limitUsers(users []User, params *SearchRequest) []User {
	if params.Limit > len(users) {
		return users
	}
	return users[0:params.Limit]
}

func getSearchParams(w http.ResponseWriter, r *http.Request) (*SearchRequest, error) {
	qp := r.URL.Query()

	limit, err := strconv.Atoi(qp.Get("limit"))
	if err != nil {
		http.Error(w, "invalid params", http.StatusInternalServerError)
		return nil, err
	}

	offset, err := strconv.Atoi(qp.Get("offset"))
	if err != nil {
		http.Error(w, "invalid params", http.StatusInternalServerError)
		return nil, err
	}

	orderBy, err := strconv.Atoi(qp.Get("order_by"))
	if err != nil {
		http.Error(w, "invalid params", http.StatusInternalServerError)
		return nil, err
	}
	if orderBy != -1 && orderBy != 0 && orderBy != 1 {
		searchError := SearchErrorResponse{Error: "ErrorBadOrderBy"}
		searchErrorJson, err := json.Marshal(searchError)
		if err != nil {
			return nil, err
		}
		w.WriteHeader(http.StatusBadRequest)
		w.Write(searchErrorJson)
		return nil, errors.New("Invalid order by")
	}

	orderField := qp.Get("order_field")

	if orderField != "id" && orderField != "age" && orderField != "name" && orderField != "" {
		searchError := SearchErrorResponse{Error: "ErrorBadOrderField"}
		searchErrorJson, err := json.Marshal(searchError)
		if err != nil {
			return nil, err
		}
		w.WriteHeader(http.StatusBadRequest)
		w.Write(searchErrorJson)
		return nil, errors.New("Invalid order field")
	}

	params := &SearchRequest{
		Limit:      limit,
		Offset:     offset,
		Query:      qp.Get("query"),
		OrderField: orderField,
		OrderBy:    orderBy,
	}

	return params, nil
}

func printUsers(users []User) {
	fmt.Println(".............")
	for _, user := range users {
		fmt.Println(user.Name)
	}
	fmt.Println(",,,")
}

func SearchServer(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("AccessToken") == "" {
		http.Error(w, "Error connecting to the DB.", http.StatusUnauthorized)
		return
	}

	users, err := getUsersFromFile()
	if err != nil {
		http.Error(w, "Error connecting to the DB.", http.StatusInternalServerError)
		return
	}

	params, err := getSearchParams(w, r)
	if err != nil {
		return
	}

	filteredUsers := filterUsers(users, params)
	orderedUsers := sortUsers(filteredUsers, params)
	offsetUsers := offsetUsers(orderedUsers, params)
	limitedUsers := limitUsers(offsetUsers, params)

	w.WriteHeader(200)
	result, _ := json.Marshal(limitedUsers)
	w.Write(result)
}

func TestFindUsers(t *testing.T) {

	ts := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer ts.Close()

	searchClient := &SearchClient{
		AccessToken: "password",
		URL:         ts.URL,
	}

	t.Run("basic search", func(t *testing.T) {
		limit := 5
		req := SearchRequest{
			Limit:      limit,
			Offset:     0,
			Query:      "",
			OrderField: "",
			OrderBy:    0,
		}

		res, _ := searchClient.FindUsers(req)

		if len(res.Users) != limit {
			t.Errorf("The length should be 5")
		}
		if !res.NextPage {
			t.Errorf("There should be no next page")
		}

		//if !reflect.DeepEqual(res.Users, users) {
		//	t.Errorf("Users should be equals")
		//}
		//printUsers(res.Users)
	})

	t.Run("incorrect limit in search request", func(t *testing.T) {
		req := SearchRequest{
			Limit:      -1,
			Offset:     0,
			Query:      "",
			OrderField: "",
			OrderBy:    0,
		}

		_, err := searchClient.FindUsers(req)

		if err == nil {
			t.Errorf("Should trigger error")
			return
		}
	})

	t.Run("incorrect offset in search request", func(t *testing.T) {
		req := SearchRequest{
			Limit:      5,
			Offset:     -1,
			Query:      "",
			OrderField: "",
			OrderBy:    0,
		}

		_, err := searchClient.FindUsers(req)

		if err == nil {
			t.Errorf("Should trigger error")
			return
		}
	})

	t.Run("big limit in search request", func(t *testing.T) {
		req := SearchRequest{
			Limit:      45,
			Offset:     0,
			Query:      "",
			OrderField: "",
			OrderBy:    0,
		}

		res, _ := searchClient.FindUsers(req)

		// TODO: should be global variable
		if len(res.Users) != 25 {
			t.Errorf("The length should be 25")
		}
	})

	t.Run("offset works", func(t *testing.T) {
		req1 := SearchRequest{
			Limit:      5,
			Offset:     0,
			Query:      "",
			OrderField: "",
			OrderBy:    0,
		}
		res1, _ := searchClient.FindUsers(req1)

		req2 := SearchRequest{
			Limit:      3,
			Offset:     2,
			Query:      "",
			OrderField: "",
			OrderBy:    0,
		}
		res2, _ := searchClient.FindUsers(req2)

		if !reflect.DeepEqual(res1.Users[2:], res2.Users) {
			t.Errorf("Should be equal")
		}
	})

	t.Run("test next page", func(t *testing.T) {
		req := SearchRequest{
			Limit:      25,
			Offset:     0,
			Query:      "nn",
			OrderField: "",
			OrderBy:    0,
		}

		res, _ := searchClient.FindUsers(req)

		if res.NextPage {
			t.Errorf("Should not have next page")
		}
	})

	t.Run("test bad order_field", func(t *testing.T) {
		req := SearchRequest{
			Limit:      5,
			Offset:     0,
			Query:      "",
			OrderField: "xxx",
			OrderBy:    0,
		}

		_, err := searchClient.FindUsers(req)

		if err == nil {
			t.Errorf("Should result in error")
		}
	})

	t.Run("test bad order_by", func(t *testing.T) {
		req := SearchRequest{
			Limit:      5,
			Offset:     0,
			Query:      "",
			OrderField: "",
			OrderBy:    2,
		}

		_, err := searchClient.FindUsers(req)

		if err == nil {
			t.Errorf("Should result in error")
		}
	})

	t.Run("test access token", func(t *testing.T) {
		searchClient := &SearchClient{
			AccessToken: "",
			URL:         ts.URL,
		}

		req := SearchRequest{
			Limit:      3,
			Offset:     2,
			Query:      "nn",
			OrderField: "name",
			OrderBy:    1,
		}

		_, err := searchClient.FindUsers(req)

		if err == nil {
			t.Errorf("Should result in error")
		}
	})

	t.Run("test url", func(t *testing.T) {
		searchClient := &SearchClient{
			AccessToken: "password",
			URL:         "",
		}

		req := SearchRequest{
			Limit:      3,
			Offset:     2,
			Query:      "nn",
			OrderField: "name",
			OrderBy:    1,
		}

		_, err := searchClient.FindUsers(req)

		if err == nil {
			t.Errorf("Should result in error")
		}
	})

	t.Run("test not json response", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(200)
			w.Write([]byte("Hello"))
		}))
		defer ts.Close()

		searchClient := &SearchClient{
			AccessToken: "password",
			URL:         ts.URL,
		}

		req := SearchRequest{
			Limit:      3,
			Offset:     2,
			Query:      "nn",
			OrderField: "name",
			OrderBy:    1,
		}

		_, err := searchClient.FindUsers(req)

		if err == nil {
			t.Errorf("Should result in error")
		}
	})

	t.Run("test fatal response", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Something happened"))
		}))
		defer ts.Close()

		searchClient := &SearchClient{
			AccessToken: "password",
			URL:         ts.URL,
		}

		req := SearchRequest{
			Limit:      3,
			Offset:     2,
			Query:      "nn",
			OrderField: "name",
			OrderBy:    1,
		}

		_, err := searchClient.FindUsers(req)

		if err == nil {
			t.Errorf("Should result in error")
		}
	})

	t.Run("test invalid params error", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "invalid params", http.StatusBadRequest)
		}))
		defer ts.Close()

		searchClient := &SearchClient{
			AccessToken: "password",
			URL:         ts.URL,
		}

		req := SearchRequest{
			Limit:      3,
			Offset:     0,
			Query:      "",
			OrderField: "",
			OrderBy:    0,
		}

		_, err := searchClient.FindUsers(req)

		if err == nil {
			t.Errorf("Should result in error")
		}
	})

	t.Run("check timeout", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(200)
			time.Sleep(2000 * time.Millisecond)
		}))
		defer ts.Close()

		searchClient := &SearchClient{
			AccessToken: "password",
			URL:         ts.URL,
		}

		req := SearchRequest{
			Limit:      3,
			Offset:     0,
			Query:      "",
			OrderField: "",
			OrderBy:    0,
		}

		_, err := searchClient.FindUsers(req)

		if err == nil {
			t.Errorf("Should result in error")
		}
	})
}
