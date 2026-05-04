package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"

	MO "github.com/Tusamarco/mysqloperatorcalculator/src/mysqloperatorcalculator"
)

func main() {
	var moc MO.MysqlOperatorCalculator

	testSupportedJson(moc.GetSupportedLayouts(), moc)
	testGetconfiguration(moc)
}

func testSupportedJson(supported MO.Configuration, calculator MO.MysqlOperatorCalculator) {
	output, err := json.MarshalIndent(&supported, "", "  ")
	if err != nil {
		// Used log instead of built-in print for better error formatting
		log.Printf("Failed to marshal JSON: %v\n", err)
		return // Exit early if there's an error
	}
	fmt.Println(string(output))
}

// Get the whole set of parameters plus message as the following objects hierarchy
// Families -> Groups -> Parameters
// THE VALID value in parameter to be considered as CURRENT is !!Value!!!
func testGetconfiguration(moc MO.MysqlOperatorCalculator) {
	var myRequest MO.ConfigurationRequest
	var conf MO.Configuration
	var err error

	myRequest.LoadType = MO.LoadType{Id: MO.LoadTypeSomeWrites}

	// Setting the dimension using literal values
	myRequest.Dimension = MO.Dimension{Id: MO.DimensionOpen, Cpu: 4000, Memory: "2.5G"}

	// Convert and validate standards
	myRequest.Dimension.MemoryBytes, err = myRequest.Dimension.ConvertMemoryToBytes(myRequest.Dimension.Memory)
	if err != nil {
		// Use log.Fatalf instead of syscall.Exit(1) for idiomatic Go process exiting
		log.Fatalf("Memory conversion error: %v\n", err)
	}

	myRequest.DBType = MO.DbTypeGroupReplication  // "pxc" or "group_replication"
	myRequest.Output = MO.ResultOutputFormatHuman // "human" or "json"
	myRequest.Connections = 3000
	myRequest.Mysqlversion = MO.Version{Major: 8, Minor: 4, Patch: 8}
	myRequest.ProviderCostPct = 0.12

	conf.Init()
	moc.Init(myRequest, conf)

	calcErr, responseMessage, families := moc.GetCalculate()
	if calcErr != nil {
		log.Printf("Calculation error: %v\n", calcErr)
	}

	if responseMessage.MType > 0 {
		log.Printf("Message %d: %s %s\n", responseMessage.MType, responseMessage.MName, responseMessage.MText)
	}

	if len(families) > 0 {
		//----------------------------------------------------------
		// 1. Parsing families and Groups one by one
		//----------------------------------------------------------

		// Parsing MySQL
		if mysqlFamily, err := moc.GetFamily(MO.FamilyTypeMysql); err == nil {
			printFamilyGroup(mysqlFamily, MO.GroupNameMySQLd, " ", "mysql configuration")
			printFamilyGroup(mysqlFamily, MO.GroupNameProbes, " ", "mysql probes")
			printFamilyGroup(mysqlFamily, MO.GroupNameResources, " ", "mysql resources")
		} else {
			log.Printf("Error retrieving MySQL family: %v\n", err)
		}

		// Parsing Proxy
		if proxyFamily, err := moc.GetFamily(MO.FamilyTypeProxy); err == nil {
			printFamilyGroup(proxyFamily, MO.GroupNameHAProxy, "  ", "haproxy configuration")
			printFamilyGroup(proxyFamily, MO.GroupNameProbes, "  ", "haproxy probes")
			printFamilyGroup(proxyFamily, MO.GroupNameResources, "  ", "haproxy resources")
		} else {
			log.Printf("Error retrieving Proxy family: %v\n", err)
		}

		// Parsing Monitoring
		if monitorFamily, err := moc.GetFamily(MO.FamilyTypeMonitor); err == nil {
			printFamilyGroup(monitorFamily, MO.GroupNameProbes, "  ", "monitor probes")
			printFamilyGroup(monitorFamily, MO.GroupNameResources, "  ", "monitor resources")
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

// Helper interface & function to DRY up repetitive parsing and error checking code
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
