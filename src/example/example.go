package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	MO "github.com/Tusamarco/mysqloperatorcalculator/src/mysqloperatorcalculator"
	"strconv"
	"syscall"
)

func main() {
	var moc MO.MysqlOperatorCalculator

	testSupportedJson(moc.GetSupportedLayouts(), moc)

	testGetconfiguration(moc)

}

func testSupportedJson(supported MO.Configuration, calculator MO.MysqlOperatorCalculator) {
	output, err := json.MarshalIndent(&supported, "", "  ")
	if err != nil {
		print(err.Error())
	}
	fmt.Println(string(output))

}

//Get the whole set of parameters plus message as the following objects hierarchy
//  Families->
//          Groups -->
//                   Parameters
// THE VALID value in parameter to be consider as CURRENT is !!Value!!!

func testGetconfiguration(moc MO.MysqlOperatorCalculator) {
	var b bytes.Buffer
	var myRequest MO.ConfigurationRequest
	var err error

	myRequest.LoadType = MO.LoadType{Id: MO.LoadTypeSomeWrites}
	// Memory resource can be set as bytes using MemoryBytes ...
	myRequest.Dimension = MO.Dimension{Id: MO.DimensionOpen, Cpu: 4000, MemoryBytes: 2684354560}
	//OR in literal using M G GB etc with Memory
	// We can assign the value...
	myRequest.Dimension = MO.Dimension{Id: MO.DimensionOpen, Cpu: 4000, Memory: "2.5G"}
	// Then convert and validate it if it follows the standards:
	var errConv error
	myRequest.Dimension.MemoryBytes, errConv = myRequest.Dimension.ConvertMemoryToBytes(myRequest.Dimension.Memory)
	// If any error then do what you want ...
	if errConv != nil {
		println(errConv.Error())
		syscall.Exit(1)
	}

	myRequest.DBType = MO.DbTypeGroupReplication  //"pxc"
	myRequest.Output = MO.ResultOutputFormatHuman //"human"
	myRequest.Connections = 0
	myRequest.Mysqlversion = MO.Version{8, 0, 33}

	moc.Init(myRequest)
	error, responseMessage, families := moc.GetCalculate()
	if error != nil {
		print(error.Error())
	}

	if responseMessage.MType > 0 {
		fmt.Errorf(strconv.Itoa(responseMessage.MType) + ": " + responseMessage.MName + " " + responseMessage.MText)
	}
	if len(families) > 0 {

		// IF Using HUMAN than:
		//  we can use the by group parsing option:
		//----------------------------------------------------------
		//1 Parsing  families and Groups one by one
		//----------------------------------------------------------

		// Parsing MySQL
		MySQLfamily, err1 := moc.GetFamily(MO.FamilyTypeMysql)
		if err1 != nil {
			print(err1.Error())
		}
		mysqlStBuffer, err1 := MySQLfamily.ParseFamilyGroup(MO.GroupNameMySQLd, " ")
		probesStBuffer, err1 := MySQLfamily.ParseFamilyGroup(MO.GroupNameProbes, " ")
		resourcesStBuffer, err1 := MySQLfamily.ParseFamilyGroup(MO.GroupNameResources, " ")

		if err1 == nil {
			println("[mysql configuration]")
			println(mysqlStBuffer.String())
			println("[mysql probes]")
			println(probesStBuffer.String())
			println("[mysql resources]")
			println(resourcesStBuffer.String())
		} else {
			println(err1.Error())
		}

		//Parsing Proxy
		proxyFamily, err1 := moc.GetFamily(MO.FamilyTypeProxy)
		if err1 != nil {
			print(err1.Error())
		}
		proxyStBuffer, err1 := proxyFamily.ParseFamilyGroup(MO.GroupNameHAProxy, "  ")
		proxyProbesStBuffer, err1 := proxyFamily.ParseFamilyGroup(MO.GroupNameProbes, "  ")
		proxyResourcesStBuffer, err1 := proxyFamily.ParseFamilyGroup(MO.GroupNameResources, "  ")
		if err1 == nil {
			println("[haproxy configuration]")
			println(proxyStBuffer.String())
			println("[haproxy probes]")
			println(proxyProbesStBuffer.String())
			println("[haproxy resources]")
			println(proxyResourcesStBuffer.String())

		} else {
			println(err1.Error())
		}

		//Parsing Monitoring
		monitorFamily, err1 := moc.GetFamily(MO.FamilyTypeMonitor)
		if err1 != nil {
			print(err1.Error())
		}
		monitorProbesStBuffer, err1 := monitorFamily.ParseFamilyGroup(MO.GroupNameProbes, "  ")
		monitorResourcesStBuffer, err1 := monitorFamily.ParseFamilyGroup(MO.GroupNameResources, "  ")
		if err1 == nil {
			println("[monitor probes]")
			println(monitorProbesStBuffer.String())
			println("[monitor resources]")
			println(monitorResourcesStBuffer.String())

		} else {
			println(err1.Error())
		}

		//----------------------------------------------------------
		//2 Parsing  All in one shot (mainly for Json output)
		//----------------------------------------------------------

		if myRequest.Output == "json" {
			b, err = moc.GetJSONOutput(responseMessage, myRequest, families)
		} else {
			b, err = moc.GetHumanOutput(responseMessage, myRequest, families)
		}
		if err != nil {
			print(err.Error())
			return
		}

		println(b.String())

	}

}
