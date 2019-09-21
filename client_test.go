package main

import (
	//"fmt"
	//"log"
	"net/http"
	"net/http/httptest"
	"testing"
	"encoding/json"
	"reflect"
)

func TestFindUsers(t *testing.T) {
	users := []User{
		User{
			Id:     0,
			Name:   "yanis",
			Age:    25,
			About:  "my name is yanis",
			Gender: "male",
		},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		result, _ := json.Marshal(users)
		w.Write(result)
	}))

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
	if !reflect.DeepEqual(res.Users, users) {
		t.Errorf("Users should be equals")
	}
	if res.NextPage {
		t.Errorf("There should be no next page")
	}
}
