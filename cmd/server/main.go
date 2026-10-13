package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/Tusamarco/mysqloperatorcalculator/v2/calculator"
	log "github.com/sirupsen/logrus"

	"net/http"
	"os"
	"strconv"
)

func main() {

	var (
		helpB    bool
		version  bool
		help     HelpText
		loglevel string
	)
	port := flag.Int("port", 8080, "Port to serve")
	ip := flag.String("address", "0.0.0.0", "Ip address")
	flag.BoolVar(&helpB, "help", false, "for help")
	flag.BoolVar(&version, "version", false, "to get product version")
	flag.StringVar(&loglevel, "loglevel", "DEBUG", "log level default debug (ERROR|INFO|DEBUG)")
	flag.Parse()

	if version {
		fmt.Println("MySQL calculator for Operator Version: ", calculator.VERSION)
		exitWithCode(0)
	} else if helpB {
		flag.PrintDefaults()
		fmt.Fprintf(os.Stdout, "%s\n", help.GetHelpText())
		exitWithCode(0)
	}

	loglevel = strings.ToUpper(loglevel)
	switch loglevel {
	case "ERROR":
		log.SetLevel(log.ErrorLevel)
	case "INFO":
		log.SetLevel(log.InfoLevel)
	case "DEBUG":
		log.SetLevel(log.DebugLevel)
	default:
		log.SetLevel(log.DebugLevel)
	}

	server := http.Server{Addr: *ip + ":" + strconv.Itoa(*port)}

	http.HandleFunc("/calculator", handleRequestCalculator)
	http.HandleFunc("/supported", handleRequestSupported)
	err := server.ListenAndServe()
	if err != nil {
		log.Error(err)
		os.Exit(1)
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

func handleGetSupported(writer http.ResponseWriter, request *http.Request) error {
	var conf calculator.Configuration
	conf.Init()
	output, err := json.MarshalIndent(&conf, "", "  ")

	if err != nil {
		return err
	}

	writer.Header().Set("Content-Type", "application/json")
	writer.Write(output)
	return nil
}

func handleRequestCalculator(writer http.ResponseWriter, request *http.Request) {
	var err error
	switch request.Method {
	case "GET":
		err = handleGetCalculate(writer, request)
	case "POST":
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
	var responseMsg calculator.ResponseMessage
	var conf calculator.Configuration
	var families map[string]calculator.Family
	var ConfRequest calculator.ConfigurationRequest

	if request.ContentLength == 0 {
		err := returnErrorMessage(writer, request, ConfRequest, responseMsg, families, "Empty request")
		if err != nil {
			return err
		}
		return nil
	}
	body, readErr := io.ReadAll(request.Body)
	if readErr != nil {
		err := returnErrorMessage(writer, request, ConfRequest, responseMsg, families, "Failed to read request body: "+readErr.Error())
		if err != nil {
			return err
		}
		return nil
	}
	if len(body) == 0 {
		err := returnErrorMessage(writer, request, ConfRequest, responseMsg, families, "Empty request body")
		if err != nil {
			return err
		}
		return nil
	}

	err1 := json.Unmarshal(body, &ConfRequest)
	if err1 != nil {
		err := returnErrorMessage(writer, request, ConfRequest, responseMsg, families, "Malformed JSON: "+err1.Error())
		if err != nil {
			return err
		}
		return nil
	}
	if ConfRequest.Dimension.MemoryBytes == 0 && ConfRequest.Dimension.ID != 998 {
		var errConv error
		ConfRequest.Dimension.MemoryBytes, errConv = ConfRequest.Dimension.ConvertMemoryToBytes(ConfRequest.Dimension.Memory)
		if errConv != nil {
			err := returnErrorMessage(writer, request, ConfRequest, responseMsg, families, "Possible Malformed request "+string(body[:])+" "+errConv.Error())
			if err != nil {
				return err
			}
		}
	}

	if ConfRequest.Dimension.ID == 0 || ConfRequest.LoadType.ID == 0 || ConfRequest.MySQLVersion.Major == 0 {
		var message = ""
		if ConfRequest.MySQLVersion.Major == 0 {
			message = "Missing MySQL Version"
		}

		err := returnErrorMessage(writer, request, ConfRequest, responseMsg, families, "Possible Malformed request "+string(body[:])+" "+message)
		if err != nil {
			return err
		}
		return nil
	} else if ConfRequest.Dimension.ID == calculator.DimensionOpen && (ConfRequest.Dimension.Cpu == 0 || ConfRequest.Dimension.MemoryBytes == 0 || ConfRequest.MySQLVersion.Major == 0) {
		err := returnErrorMessage(writer, request, ConfRequest, responseMsg, families, "Open dimension request missing CPU, Memory value or MySQL Version"+string(body[:]))
		if err != nil {
			return err
		}
		return nil
	}

	conf.Init()

	var moc calculator.MySQLOperatorCalculator
	ConfRequest = moc.Init(ConfRequest, conf)

	err1, responseMsg, familiesCalculated := moc.GetCalculate()
	if err1 != nil {
		return err1
	}

	err := ReturnResponse(writer, request, ConfRequest, responseMsg, familiesCalculated)
	if err != nil {
		return err
	}

	return nil
}

func exitWithCode(errorCode int) {
	log.Debug("Error execution with code ", errorCode)
}

func returnErrorMessage(writer http.ResponseWriter, request *http.Request, ConfRequest calculator.ConfigurationRequest, message calculator.ResponseMessage, families map[string]calculator.Family, errorMessage string) error {
	message.Type = calculator.ErrorexecI
	message.Name = "Invalid incoming request"
	message.Text = fmt.Sprintf(message.GetMessageText(message.Type), errorMessage)
	err := ReturnResponse(writer, request, ConfRequest, message, families)
	if err != nil {
		return err
	}

	return nil
}

func ReturnResponse(writer http.ResponseWriter, request *http.Request, ConfRequest calculator.ConfigurationRequest, message calculator.ResponseMessage, families map[string]calculator.Family) error {
	var b bytes.Buffer
	var err error
	var moc calculator.MySQLOperatorCalculator

	if ConfRequest.Output == "json" {
		b, err = moc.GetJSONOutput(message, ConfRequest, families)
	} else {
		b, err = moc.GetHumanOutput(message, ConfRequest, families)
	}
	if err != nil {
		return err
	}

	writer.Header().Set("Content-Type", "application/json")
	writer.Write(b.Bytes())
	return nil
}
