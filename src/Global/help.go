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

package Global

import "fmt"

type HelpText struct {
	inParams  [2]string
	license   string
	helpShort string
}

//func (help *HelpText) Init() {
//	help.inParams = [2]string{"configfile", "configPath"}
//}
func (help *HelpText) PrintLicense() {
	fmt.Println(help.GetHelpText())
}

func (help *HelpText) GetHelpText() string {
	helpText := `PXC Calculator for Percona Operator

Parameters for the executable --configfile <file name> --configpath <full path> --help


Parameters in the config file:
Global:
`
	return helpText
}
