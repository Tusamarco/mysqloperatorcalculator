package main

import (
	"bytes"
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

	//just check if we need to pass version or help
	if len(os.Args) > 1 &&
		os.Args[1] == "--version" {
		fmt.Println("PXC calculator for Operator Version: ", version)
		exitWithCode(0)
	} else if len(os.Args) > 1 &&
		os.Args[1] == "--help" {
		fmt.Fprintf(os.Stdout, "\n%s\n", help.GetHelpText())
		exitWithCode(0)
	}

	//set log level
	log.SetLevel(log.DebugLevel)

	//set server address (need to come from configuration parameter)
	server := http.Server{Addr: "0.0.0.0:8080"}

	//define API handlers
	http.HandleFunc("/calculator", handleRequestCalculator)
	http.HandleFunc("/supported", handleRequestSupported)
	server.ListenAndServe()
}

// When need to calculate we (for the moment) just catch the request and pass over
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

// here we return a configuration answering to a request like:
// { "dimension":  {"id": 3, "name": "XSmall",  "cpu": 1000}, "loadtype":  {"id": 2, "name": "Mainly Reads"}, "connections": 50}

func handleGetCalculate(writer http.ResponseWriter, request *http.Request) error {
	len := request.ContentLength

	if len <= 0 {
		return errors.New(fmt.Sprintf("Empty request"))
	}
	body := make([]byte, len)
	request.Body.Read(body)

	// we need to process the request and get the values
	var ConfRequest Objects.ConfigurationRequest
	json.Unmarshal(body, &ConfRequest)

	// create and init all the different params organized by families
	var family Objects.Family
	families := family.Init()

	var conf Objects.Configuration
	conf.Init()

	//output, err := json.Marshal(&conf)
	// we store incoming request for reference when passing back the configuration
	processedRequest, err := json.MarshalIndent(&ConfRequest, "", "  ")

	// initialize the configurator (where all the things happens)
	var c Configurator
	c.init(ConfRequest, families, conf)

	// here is the calculation step
	c.ProcessRequest()

	// We transform to Json all the calculated params
	output, err := json.MarshalIndent(&families, "", "  ")

	// Concatenate all into a single output
	var b bytes.Buffer
	b.WriteString(`{"request":{"incoming":`)
	b.Write(processedRequest)
	b.WriteString(`,"answer":`)
	b.Write(output)
	b.WriteString("}}")

	if err != nil {
		return err
	}

	// Return the information
	writer.Header().Set("Content/Type", "application/json")
	writer.Write(b.Bytes())
	return nil

	//fmt.Fprintf(os.Stdout, "\n%s\n", body)

}

func handleGetSupported(writer http.ResponseWriter, request *http.Request) error {
	var conf Objects.Configuration
	conf.Init()
	//output, err := json.Marshal(&conf)
	output, err := json.MarshalIndent(&conf, "", "  ")

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
