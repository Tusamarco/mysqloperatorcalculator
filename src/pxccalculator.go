package main

import (
	"encoding/json"
	"errors"
	"fmt"
	log "github.com/sirupsen/logrus"
	"net/http"
	"os"
	"pxccalculator/src/Global"
	"pxccalculator/src/Objects"
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
	http.HandleFunc("/calculator", handleRequestCalculator)
	http.HandleFunc("/supported", handleRequestSupported)
	server.ListenAndServe()
}

func handleRequestCalculator(writer http.ResponseWriter, request *http.Request) {
	var err error
	switch request.Method {
	case "GET":
		err = handleGetCalculate(writer, request)

	}
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		log.Error(err)
	}
}

func handleRequestSupported(writer http.ResponseWriter, request *http.Request) {
	var err error
	switch request.Method {
	case "GET":
		err = handleGetSupported(writer, request)
	}
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		log.Error(err)
	}
}

func handleGetCalculate(writer http.ResponseWriter, request *http.Request) error {
	len := request.ContentLength

	if len <= 0 {
		return errors.New(fmt.Sprintf("Empty request"))
	}
	body := make([]byte, len)
	request.Body.Read(body)

	fmt.Fprintf(os.Stdout, "\n%s\n", body)

	return nil
}

func handleGetSupported(writer http.ResponseWriter, request *http.Request) error {
	var conf Objects.Configuration
	conf.Init()
	//output, err := json.Marshal(&conf)
	output, err := json.MarshalIndent(&conf, "", "\t")

	if err != nil {
		return err
	}

	writer.Header().Set("Content/Type", "application/json")
	writer.Write(output)
	return nil
}

func exitWithCode(errorCode int) {
	log.Debug("Exiting execution with code ", errorCode)
	os.Exit(errorCode)
}
