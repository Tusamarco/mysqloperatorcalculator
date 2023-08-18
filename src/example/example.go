package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	MO "github.com/Tusamarco/mysqloperatorcalculator/src/mysqloperatorcalculator"
	"strconv"
)

func main() {
	var my MO.MysqlOperatorCalculator

	testSupportedJson(my.GetSupportedLayouts(), my)

	testGetconfiguration(my)

}

//get all the supported platforms and dimensions as an object that you can transform or not:
//type Configuration struct {
//	DBType        []string      `json:"dbtype"`
//	Dimension     []Dimension   `json:"dimension"`
//	LoadType      []LoadType    `json:"loadtype"`
//	Connections   []int         `json:"connections"`
//	Output        []string      `json:"output"`
//	Mysqlversions MySQLVersions `json:"mysqlversions"`
//}

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

	myRequest.LoadType = MO.LoadType{Id: 2}
	myRequest.Dimension = MO.Dimension{Id: 999, Cpu: 4000, Memory: 2.5}
	myRequest.DBType = "group_replication" //"pxc"
	myRequest.Output = "human"             //"human"
	myRequest.Connections = 70
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
