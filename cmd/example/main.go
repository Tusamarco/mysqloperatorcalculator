package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"

	"github.com/Tusamarco/mysqloperatorcalculator/calculator"
)

func main() {
	var moc calculator.MySQLOperatorCalculator

	testSupportedJson(moc.GetSupportedLayouts(), moc)
	testGetconfiguration(moc)
}

func testSupportedJson(supported calculator.Configuration, calc calculator.MySQLOperatorCalculator) {
	output, err := json.MarshalIndent(&supported, "", "  ")
	if err != nil {
		log.Printf("Failed to marshal JSON: %v\n", err)
		return
	}
	fmt.Println(string(output))
}

// Get the whole set of parameters plus message as the following objects hierarchy
// Families -> Groups -> Parameters
// THE VALID value in parameter to be considered as CURRENT is !!Value!!!
func testGetconfiguration(moc calculator.MySQLOperatorCalculator) {
	var myRequest calculator.ConfigurationRequest
	var conf calculator.Configuration
	var err error

	myRequest.LoadType = calculator.LoadType{ID: calculator.LoadTypeSomeWrites}
	myRequest.MySQLDedicated = true

	myRequest.Dimension = calculator.Dimension{ID: calculator.DimensionOpen, Cpu: 4000, Memory: "2.5G"}

	myRequest.Dimension.MemoryBytes, err = myRequest.Dimension.ConvertMemoryToBytes(myRequest.Dimension.Memory)
	if err != nil {
		log.Fatalf("Memory conversion error: %v\n", err)
	}

	myRequest.DBType = calculator.DBTypeGroupReplication
	myRequest.Output = calculator.ResultOutputFormatJSON
	myRequest.Connections = 3000
	myRequest.MySQLVersion = calculator.Version{Major: 8, Minor: 4, Patch: 8}
	myRequest.ProviderCostPct = 0.12

	conf.Init()
	moc.Init(myRequest, conf)

	calcErr, responseMessage, families := moc.GetCalculate()
	if calcErr != nil {
		log.Printf("Calculation error: %v\n", calcErr)
	}

	if responseMessage.Type > 0 {
		log.Printf("Message %d: %s %s\n", responseMessage.Type, responseMessage.Name, responseMessage.Text)
	}

	if len(families) > 0 {
		//----------------------------------------------------------
		// 1. Parsing families and Groups one by one
		//----------------------------------------------------------

		if mysqlFamily, err := moc.GetFamily(calculator.FamilyTypeMySQL); err == nil {
			printFamilyGroup(mysqlFamily, calculator.GroupNameMySQLd, " ", "mysql configuration")
			printFamilyGroup(mysqlFamily, calculator.GroupNameProbes, " ", "mysql probes")
			printFamilyGroup(mysqlFamily, calculator.GroupNameResources, " ", "mysql resources")
		} else {
			log.Printf("Error retrieving MySQL family: %v\n", err)
		}

		if proxyFamily, err := moc.GetFamily(calculator.FamilyTypeProxy); err == nil {
			printFamilyGroup(proxyFamily, calculator.GroupNameHAProxy, "  ", "haproxy configuration")
			printFamilyGroup(proxyFamily, calculator.GroupNameProbes, "  ", "haproxy probes")
			printFamilyGroup(proxyFamily, calculator.GroupNameResources, "  ", "haproxy resources")
		} else {
			log.Printf("Error retrieving Proxy family: %v\n", err)
		}

		if monitorFamily, err := moc.GetFamily(calculator.FamilyTypeMonitor); err == nil {
			printFamilyGroup(monitorFamily, calculator.GroupNameProbes, "  ", "monitor probes")
			printFamilyGroup(monitorFamily, calculator.GroupNameResources, "  ", "monitor resources")
		} else {
			log.Printf("Error retrieving Monitor family: %v\n", err)
		}

		//----------------------------------------------------------
		// 2. Parsing All in one shot (mainly for Json output)
		//----------------------------------------------------------
		var b bytes.Buffer

		if myRequest.Output == "json" {
			b, err = moc.GetJSONOutput(responseMessage, myRequest, families)
		} else {
			b, err = moc.GetHumanOutput(responseMessage, myRequest, families)
		}

		if err != nil {
			log.Printf("Error generating output: %v\n", err)
			return
		}

		fmt.Println(b.String())
	}
}

type FamilyParser interface {
	ParseFamilyGroup(groupName string, separator string) (bytes.Buffer, error)
}

func printFamilyGroup(family FamilyParser, groupName, separator, header string) {
	buffer, err := family.ParseFamilyGroup(groupName, separator)
	if err != nil {
		log.Printf("Failed to parse [%s]: %v\n", header, err)
		return
	}
	fmt.Printf("[%s]\n%s\n", header, buffer.String())
}
