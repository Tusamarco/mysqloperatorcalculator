package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	MO "github.com/Tusamarco/mysqloperatorcalculator/src/mysqloperatorcalculator"
	log "github.com/sirupsen/logrus"

	"net/http"
	"os"
	"strconv"
)

func main() {

	var (
		//port    int
		//ip      string
		helpB   bool
		version bool
		help    HelpText
	)
	port := flag.Int("port", 8080, "Port to serve")
	ip := flag.String("address", "0.0.0.0", "Ip address")
	flag.BoolVar(&helpB, "help", false, "for help")
	flag.BoolVar(&version, "version", false, "to get product version")
	flag.Parse()

	var versionS = "1.2.1"
	//initialize help

	//just check if we need to pass version or help
	if version {
		fmt.Println("MySQL calculator for Operator Version: ", versionS)
		exitWithCode(0)
	} else if helpB {
		flag.PrintDefaults()
		fmt.Fprintf(os.Stdout, "%s\n", help.GetHelpText())
		exitWithCode(0)
	}

	//set log level
	log.SetLevel(log.DebugLevel)

	//set server address (need to come from configuration parameter)
	server := http.Server{Addr: *ip + ":" + strconv.Itoa(*port)}

	//define API handlers
	http.HandleFunc("/calculator", handleRequestCalculator)
	http.HandleFunc("/supported", handleRequestSupported)
	server.ListenAndServe()
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

func handleGetSupported(writer http.ResponseWriter, request *http.Request) error {
	var conf MO.Configuration
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

// here we return a configuration answering to a request like:
// { "dimension":  {"id": 3, "name": "XSmall",  "cpu": 1000}, "loadtype":  {"id": 2, "name": "Mainly Reads"}, "connections": 50}

func handleGetCalculate(writer http.ResponseWriter, request *http.Request) error {
	// message object to pass back
	var responseMsg MO.ResponseMessage
	var family MO.Family
	var conf MO.Configuration
	var families map[string]MO.Family
	var ConfRequest MO.ConfigurationRequest

	//if we do not have a real request we return a message with the info
	len := request.ContentLength
	if len <= 0 {
		err := returnErrorMessage(writer, request, ConfRequest, responseMsg, families, "Empty request")
		if err != nil {
			return err
		}
		return nil
	}
	body := make([]byte, len)
	request.Body.Read(body)

	// we need to process the request and get the values
	json.Unmarshal(body, &ConfRequest)

	// Before going to the configurator we check the incoming request and IF is not ok we return an error message
	if ConfRequest.Dimension.Id == 0 || ConfRequest.LoadType.Id == 0 {
		err := returnErrorMessage(writer, request, ConfRequest, responseMsg, families, "Possible Malformed request "+string(body[:]))
		if err != nil {
			return err
		}
		return nil
	} else if ConfRequest.Dimension.Id == 999 && (ConfRequest.Dimension.Cpu == 0 || ConfRequest.Dimension.Memory == 0) {
		err := returnErrorMessage(writer, request, ConfRequest, responseMsg, families, "Open dimension request missing CPU OR Memory value "+string(body[:]))
		if err != nil {
			return err
		}
		return nil

	}

	// create and init all the different params organized by families
	conf.Init()
	ConfRequest = getConfForConfRequest(ConfRequest, conf)
	families = family.Init(ConfRequest.DBType)

	// initialize the configurator (where all the things happens)
	var c MO.Configurator
	responseMsg, connectionsOverload := c.Init(ConfRequest, families, conf, responseMsg)

	if connectionsOverload {
		responseMsg.MName = "Resources Overload"
		responseMsg.MText = "Too many connections for the chose dimension. Resource Overload, decrease number of connections OR choose higher CPUs number"
		families = make(map[string]MO.Family)
	} else {
		// here is the calculation step
		overUtilizing := false
		c.ProcessRequest()
		responseMsg, overUtilizing = c.EvaluateResources(responseMsg)
		//if request overutilize resources WE DO NOT pass params but message
		if overUtilizing {
			families = make(map[string]MO.Family)
		}
	}
	err := ReturnResponse(writer, request, ConfRequest, responseMsg, families)
	if err != nil {
		return err
	}

	return nil

	//fmt.Fprintf(os.Stdout, "\n%s\n", body)

}

// we loop the arrays to get all the info we may need for the operation using the ID as reference
func getConfForConfRequest(request MO.ConfigurationRequest, conf MO.Configuration) MO.ConfigurationRequest {

	if request.Dimension.Id != 999 {
		for i := 0; i < len(conf.Dimension); i++ {

			if request.Dimension.Id == conf.Dimension[i].Id {
				request.Dimension = conf.Dimension[i]
				break
			}
		}
	} else {
		//We need to calibrate the dimension on the base of an open request
		request.Dimension = conf.CalculateOpenDimension(request.Dimension)
	}

	for i := 0; i < len(conf.LoadType); i++ {

		if request.Dimension.Id == conf.LoadType[i].Id {
			request.LoadType = conf.LoadType[i]
			break
		}
	}

	return request
}

func exitWithCode(errorCode int) {
	log.Debug("Exiting execution with code ", errorCode)
	os.Exit(errorCode)
}

func returnErrorMessage(writer http.ResponseWriter, request *http.Request, ConfRequest MO.ConfigurationRequest, message MO.ResponseMessage, families map[string]MO.Family, errorMessage string) error {
	message.MType = MO.ErrorexecI
	message.MName = "Invalid incoming request"
	message.MText = fmt.Sprintf(message.GetMessageText(message.MType), errorMessage)
	err := ReturnResponse(writer, request, ConfRequest, message, families)
	if err != nil {
		return err
	}

	return nil
}

func ReturnResponse(writer http.ResponseWriter, request *http.Request, ConfRequest MO.ConfigurationRequest, message MO.ResponseMessage, families map[string]MO.Family) error {
	var b bytes.Buffer
	var err error
	var moc MO.MysqlOperatorCalculator
	if ConfRequest.Output == "json" {
		b, err = moc.GetJSONOutput(message, ConfRequest, families)
	} else {
		b, err = moc.GetHumanOutput(message, ConfRequest, families)
	}
	if err != nil {
		return err
	}

	// Return the information
	writer.Header().Set("Content/Type", "application/json")
	writer.Write(b.Bytes())
	return nil
}
