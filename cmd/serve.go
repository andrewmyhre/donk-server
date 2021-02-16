/*
Copyright © 2021 NAME HERE <EMAIL ADDRESS>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"encoding/json"
	"github.com/andrewmyhre/donk-server/pkg/instance"
	"github.com/andrewmyhre/donk-server/pkg/session"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	_ "image/jpeg"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/spf13/cobra"
)

var defaultInstance = &instance.Instance {
	ID: uuid.Nil,
	SourceImageWidth: 5544,
	SourceImageHeight: 3744,
	StepCountX: 6,
	StepCountY: 6,
	StepSizeX: 924,
	StepSizeY: 624,
}

// serveCmd represents the serve command
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start serving API requests",
	Long: `Runs the web service which serves API requests for Donk`,
	Run: func(cmd *cobra.Command, args []string) {
		err := defaultInstance.EnsurePath()
		if err != nil {
			log.Fatal(err)
		}

		r := mux.NewRouter()
		r.HandleFunc("/", HomeHandler)
		r.HandleFunc("/v1/composite", CompositeHandler)
		r.HandleFunc("/v1/instance/{instanceID}/composite", CompositeHandler)
		r.HandleFunc("/v1/instance/{instanceID}", InstanceInfoHandler)
		r.HandleFunc("/v1/session/new/{x:[0-9]+}/{y:[0-9]+}", NewSessionHandler).Methods(http.MethodPost,http.MethodOptions)
		r.HandleFunc("/v1/session/{sessionID}", SessionInfoHandler)
		r.HandleFunc("/v1/session/{sessionID}/background", SessionBackgroundImageHandler)
		r.HandleFunc("/v1/session/{sessionID}/save", SessionSaveImageHandler).Methods(http.MethodPost,http.MethodOptions)
		r.HandleFunc("/v1/instance/{instanceID}/session/{sessionID}/background", SessionBackgroundImageHandler)
		r.HandleFunc("/v1/instance/{instanceID}/session/{sessionID}/save", SessionSaveImageHandler).Methods(http.MethodPost,http.MethodOptions)
		http.Handle("/v1", r)

		srv := &http.Server{
			Handler:      r,
			Addr:         "127.0.0.1:8000",
			// Good practice: enforce timeouts for servers you create!
			WriteTimeout: 15 * time.Second,
			ReadTimeout:  15 * time.Second,
		}

		r.Use(mux.CORSMethodMiddleware(r))

		log.Fatal(srv.ListenAndServe())
	},
}

func HomeHandler(w http.ResponseWriter, r *http.Request) {
	log.Info("home")
}

func InstanceInfoHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if r.Method == http.MethodOptions {
		return
	}
	json, err := json.Marshal(defaultInstance)
	if err != nil {
		log.Error(errors.Wrap(err, "Failed to marshall instance data"))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(json)
}

func SessionInfoHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if r.Method == http.MethodOptions {
		return
	}
	vars := mux.Vars(r)
	session, err := session.Find(vars["sessionID"])
	if err != nil {
		log.Error(errors.Wrap(err, "Failed to find session"))
	}

	json, err := json.Marshal(session)
	if err != nil {
		log.Error(errors.Wrap(err, "Failed to marshall session data"))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(json)
}

func CompositeHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if r.Method == http.MethodOptions {
		return
	}
	image, err := defaultInstance.GetStitchedImage()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Error(errors.Wrap(err, "Couldn't provide instance composite image"))
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(image)
}

func NewSessionHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if r.Method == http.MethodOptions {
		return
	}
	vars := mux.Vars(r)
	x,err := strconv.Atoi(vars["x"])
	if err != nil {
		w.Write([]byte("X argument must be a number"))
	}
	y,err := strconv.Atoi(vars["y"])
	if err != nil {
		w.Write([]byte("Y argument must be a number"))
	}
	session, err := session.NewSession(defaultInstance, x,y)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		log.Error(err)
		return
	}
	log.Infof("Created session %v", session.ID)

	data, err := json.Marshal(session)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Error(err)
		return
	}

	w.Write(data)
}

func SessionBackgroundImageHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if r.Method == http.MethodOptions {
		return
	}
	vars := mux.Vars(r)
	session, err := session.Open(defaultInstance, vars["sessionID"])
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		log.Error(err)
	}
	imageData, err := session.ReadBackgroundImage()
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		log.Error(err)
	}
	_, err = w.Write(imageData)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		log.Error(err)
	}
}

func SessionSaveImageHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if r.Method == http.MethodOptions {
		return
	}

	vars := mux.Vars(r)
	session, err := session.Open(defaultInstance, vars["sessionID"])
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		log.Error(err)
		return
	}

	defer r.Body.Close()
	bodyData, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		log.Error(err)
		return
	}

	err = session.UpdateBackgroundImage(bodyData)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		log.Error(err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func init() {
	rootCmd.AddCommand(serveCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// serveCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// serveCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
