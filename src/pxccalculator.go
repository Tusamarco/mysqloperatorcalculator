package main

import (
	"errors"
	"fmt"
	log "github.com/sirupsen/logrus"
	"net/http"
	"os"
	"pxccalculator/src/Global"
)

func main() {

	var version = "0.0.1"
	//initialize help
	help := new(Global.HelpText)

	if len(os.Args) > 1 &&
		os.Args[1] == "--version" {
		fmt.Println("PXC calculator for Operator Version: ", version)
		exitWithCode(0)
	} else if len(os.Args) > 1 &&
		os.Args[1] == "--help" {
		fmt.Fprintf(os.Stdout, "\n%s\n", help.GetHelpText())
		exitWithCode(0)
	}

	server := http.Server{Addr: "0.0.0.0:8080"}
	http.HandleFunc("/post/", handleRequest)
	server.ListenAndServe()
}

func handleRequest(writer http.ResponseWriter, request *http.Request) {
	var err error
	switch request.Method {
	case "GET":
		err = handleGet(writer, request)

	}
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		log.Error(err)
	}
}

func handleGet(writer http.ResponseWriter, request *http.Request) error {
	len := request.ContentLength

	if len <= 0 {
		return errors.New(fmt.Sprintf("Empty request"))
	}
	body := make([]byte, len)
	request.Body.Read(body)

	fmt.Fprintf(os.Stdout, "\n%s\n", body)

	return nil
}

func exitWithCode(errorCode int) {
	log.Debug("Exiting execution with code ", errorCode)
	os.Exit(errorCode)
}
