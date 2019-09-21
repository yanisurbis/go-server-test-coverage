package main

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strconv"
	"strings"

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

	filteredUsers := make([]User, 0, params.Limit)

	isLegit := func(user User) bool {
		if strings.Contains(user.About, params.Query) || strings.Contains(user.Name, params.Query) {
			return true
		}
		return false
	}

	for _, user := range users {
		if isLegit(user) {
			filteredUsers = append(filteredUsers, user)
			if len(filteredUsers) == params.Limit {
				break
			}
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

func getSearchParams(r *http.Request) (*SearchRequest, error) {
	qp := r.URL.Query()

	limit, err1 := strconv.Atoi(qp.Get("limit"))
	offset, err2 := strconv.Atoi(qp.Get("offset"))
	orderBy, err3 := strconv.Atoi(qp.Get("order_by"))
	orderField := qp.Get("order_field")

	if err1 != nil || err2 != nil || err3 != nil {
		return nil, errors.New("Invalid params")
	}

	if orderField != "id" && orderField != "age" && orderField != "name" {
		return nil, errors.New("Invalid params")
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
	for _, user := range users {
		fmt.Println(user.Name)
	}
}

func SearchServer(w http.ResponseWriter, r *http.Request) {
	users, err := getUsersFromFile()
	if err != nil {
		http.Error(w, "Error connecting to the DB.", http.StatusInternalServerError)
		return
	}

	params, err := getSearchParams(r)
	if err != nil {
		http.Error(w, "Invalid params", http.StatusBadRequest)
		return
	}

	filteredUsers := filterUsers(users, params)
	orderedUsers := sortUsers(filteredUsers, params)
	offsetUsers := offsetUsers(orderedUsers, params)

	printUsers(offsetUsers)

	w.WriteHeader(200)
	result, _ := json.Marshal(offsetUsers)
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
			OrderField: "id",
			OrderBy:    0,
		}

		res, err := searchClient.FindUsers(req)

		if err != nil {
			fmt.Println(err)
			t.Errorf("Should not have error")
			return
		}
		if res.NextPage {
			t.Errorf("There should be no next page")
		}
		if len(res.Users) != limit {
			t.Errorf("The length should be 5")
		}
		//if !reflect.DeepEqual(res.Users, users) {
		//	t.Errorf("Users should be equals")
		//}
		//fmt.Println(res.Users)
	})

	t.Run("incorrect limit in search request", func(t *testing.T) {
		req := SearchRequest{
			Limit:      -1,
			Offset:     0,
			Query:      "",
			OrderField: "id",
			OrderBy:    0,
		}

		_, err := searchClient.FindUsers(req)

		if err == nil {
			fmt.Println(err)
			t.Errorf("Should trigger error")
			return
		}
	})

}
