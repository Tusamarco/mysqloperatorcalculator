/*
 * Copyright (c) Marco Tusa 2021 - present
 *                     GNU GENERAL PUBLIC LICENSE
 *                        Version 3, 29 June 2007
 *
 *  Copyright (C) 2007 Free Software Foundation, Inc. <https://fsf.org/>
 *  Everyone is permitted to copy and distribute verbatim copies
 *  of this license document, but changing it is not allowed.
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package main

import "fmt"

type HelpText struct {
	inParams  [2]string
	license   string
	helpShort string
}

//	func (help *HelpText) Init() {
//		help.inParams = [2]string{"configfile", "configPath"}
//	}
func (help *HelpText) PrintLicense() {
	fmt.Println(help.GetHelpText())
}

func (help *HelpText) GetHelpText() string {
	helpText := `PXC Calculator for Percona Operator


To get supported scenarios:
  curl -i -X GET  http://192.168.4.41:8080/supported
	  HTTP/1.1 200 OK
	  Date: Sat, 24 Dec 2022 17:00:33 GMT
	  Content-Length: 183
	  Content-Type: text/plain; charset=utf-8
	
	  {"dimension":{"Large":4,"Medium":3,"Small":2,"XLarge":5,"XSmall":1},"loadtype":{"Intense OLTP (50/50 R/W)":3,"Light OLTP":2,"Mainly Reads":1},"connections":[50,100,200,500,1000,2000]}



to test:
 curl -i -X GET -H "Content-Type: application/json" -d '{"output":"human","dbtype":"pxc", "dimension":  {"id": 2}, "loadtype":  {"id": 2}, "connections": 5}' http://127.0.0.1:8080/calculator



`
	return helpText
}
