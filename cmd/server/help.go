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
	helpText := `MySQL Operator Calculator — Percona (PXC and Group Replication)

ENDPOINTS
  GET  /supported    Returns valid dimensions, load types, DB types, and MySQL version range.
  POST /calculator   Returns a full MySQL / Kubernetes configuration for the given request.

────────────────────────────────────────────────────────────────
GET /supported
────────────────────────────────────────────────────────────────
  curl http://127.0.0.1:8080/supported

  Response (abbreviated):
  {
    "DBType":  ["group_replication", "pxc"],
    "Output":  ["human", "json"],
    "Dimension": [
      {"Id":1,  "Name":"XSmall",   "Cpu":1000,  "Memory":"2GB"},
      {"Id":2,  "Name":"Small",    "Cpu":2500,  "Memory":"4GB"},
      {"Id":3,  "Name":"Medium",   "Cpu":4500,  "Memory":"8GB"},
      {"Id":4,  "Name":"Large",    "Cpu":6500,  "Memory":"16GB"},
      {"Id":5,  "Name":"2XLarge",  "Cpu":8500,  "Memory":"32GB"},
      {"Id":6,  "Name":"4XLarge",  "Cpu":16000, "Memory":"64GB"},
      {"Id":7,  "Name":"8XLarge",  "Cpu":32000, "Memory":"128GB"},
      {"Id":8,  "Name":"12XLarge", "Cpu":48000, "Memory":"192GB"},
      {"Id":9,  "Name":"16XLarge", "Cpu":64000, "Memory":"256GB"},
      {"Id":10, "Name":"24XLarge", "Cpu":96000, "Memory":"384GB"},
      {"Id":999,"Name":"Open request by resources"},
      {"Id":998,"Name":"Open request by Connection"}
    ],
    "LoadType": [
      {"Id":1,"Name":"Mainly Reads", "Description":"Blogs ~10% Writes 90% Reads"},
      {"Id":2,"Name":"Light OLTP",   "Description":"Shops online up to ~40% Writes"},
      {"Id":3,"Name":"Heavy OLTP",   "Description":"Intense analytics, telephony, gaming. 30/70% Reads and Writes"},
      {"Id":4,"Name":"Mainly write", "Description":"Data load, data ingest. 90% writes"}
    ],
    "Connections":   [50,100,200,500,1000,2000],
    "Mysqlversions": {"Min":{"Major":8,"Minor":0,"Patch":46},"Max":{"Major":11,"Minor":1,"Patch":1}}
  }

────────────────────────────────────────────────────────────────
POST /calculator — predefined dimension
────────────────────────────────────────────────────────────────
  curl -X POST -H "Content-Type: application/json" \
    -d '{
          "output":       "human",
          "dbtype":       "pxc",
          "dimension":    {"id": 2},
          "loadtype":     {"id": 2},
          "connections":  100,
          "mysqlversion": {"major":8,"minor":0,"patch":46}
        }' \
    http://127.0.0.1:8080/calculator

────────────────────────────────────────────────────────────────
POST /calculator — open dimension (id 999, custom CPU + memory)
────────────────────────────────────────────────────────────────
  curl -X POST -H "Content-Type: application/json" \
    -d '{
          "output":       "json",
          "dbtype":       "group_replication",
          "dimension":    {"id": 999, "cpu": 4000, "memory": "8GB"},
          "loadtype":     {"id": 3},
          "connections":  200,
          "mysqlversion": {"major":8,"minor":0,"patch":46}
        }' \
    http://127.0.0.1:8080/calculator

────────────────────────────────────────────────────────────────
POST /calculator — connection-driven sizing (id 998)
────────────────────────────────────────────────────────────────
  The calculator picks the smallest dimension that sustains the
  requested connection count and returns ResourcesRecalculated.

  curl -X POST -H "Content-Type: application/json" \
    -d '{
          "output":       "json",
          "dbtype":       "pxc",
          "dimension":    {"id": 998},
          "loadtype":     {"id": 2},
          "connections":  500,
          "mysqlversion": {"major":8,"minor":0,"patch":46}
        }' \
    http://127.0.0.1:8080/calculator

────────────────────────────────────────────────────────────────
REQUEST FIELDS
────────────────────────────────────────────────────────────────
  output          "human" (INI-style my.cnf) or "json"
  dbtype          "pxc" or "group_replication"
  dimension.id    1–10 predefined  |  998 connection-driven  |  999 custom resources
  loadtype.id     1 Mainly Reads   |  2 Light OLTP  |  3 Heavy OLTP  |  4 Mainly write
  connections     target connection count (0 = auto-discover maximum for the dimension)
  mysqlversion    {"major":M,"minor":m,"patch":p}  minimum supported: 8.0.46
  providerCostPct optional overhead fraction deducted from resources (e.g. 0.12 = 12%)

`
	return helpText
}
