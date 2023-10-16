# MySQL Calculator for Operator
## Why 
With the advent of Kubernets (k8s), had become incresingly common to deploy RBDMS on K8s supported platforms. 
However the way MySQL and also the other components should be set and tune is very different from what is the "standard" way. 
To facilitate the setup and configuration of MYSQL and related, I have wrote this small tool that works as a simple service and that can be query directly 
from your application.

## How 
The tools is a simple service that will listen wherever you run it. 
The calculation is done considering many different parameters combinations. 
The Parameters are:
- Dimensions (CPU/Memory)
- Kind of load (simple reads with very minimal writes say less than 5%; still reads but higher writes less 20%; kind of 50/50% load in reads and writes).
- Number of connections

While the fisrst two are fix and passed by the tool, the number of connection is an open variable, and you can set it to any number considering the minum as _50 connections_. 
It doesn't make too much sense to have a RDBMS with less than that, don't you think? 


### What I should do 
Ok, so what should I do to run it?
After compilation run it as
`./mysqloperatorcalculator -address=<ip> -port=<port>`

if you omit IP it will listen on all available IP, if you omit the port it will use 8080.

The first action is to discover what is currently supported dimensions.
To test it you can do :
` curl -i -X GET  http://<ip>:<port>/supported`
IE
` curl -i -X GET  http://127.0.0.1:8080/supported`

The result you will get is in Json formatted to make it easier also for humans. 
```
  curl -i -X GET  http://127.0.0.1:8080/supported
HTTP/1.1 200 OK
Date: Mon, 09 Jan 2023 15:12:53 GMT
Content-Type: text/plain; charset=utf-8
Transfer-Encoding: chunked

{
  "dbtype": [
    "group_replication",
    "pxc"
  ],
  "dimension": [
    {
      "id": 1,
      "name": "XSmall",
      "cpu": 1000,
      "memory": 2,
      "mysqlCpu": 600,
      "proxyCpu": 200,
      "pmmCpu": 100,
      "mysqlMemory": 1.7,
      "proxyMemory": 0.2,
      "pmmMemory": 0.1
    },
    ...
      "loadtype": [
    {
      "id": 1,
      "name": "Mainly Reads",
      "example": "Blogs ~2% Writes 95% Reads"
    },
    {
      "id": 2,
      "name": "Light OLTP",
      "example": "Shops online  up to 20% Writes "
    },
    {
      "id": 3,
      "name": "Heavy OLTP",
      "example": "Intense analitics, telephony, gaming. 50/50% Reads and Writes"
    }
  ],
  "connections": [
    50,
    100,
    200,
    500,
    1000,
    2000
  ],
  "output": [
    "human",
    "json"
  ],
  "mysqlversions": {
    "min": {
      "major": 8,
      "minor": 0,
      "patch": 32
    },
    "max": {
      "major": 8,
      "minor": 1,
      "patch": 0
    }
  }

}
```
From version `1.1.0` we support also open requests, this means you can pass the values for memory and cpu in open forms.
When retrieving the supported dimensions you will notice a special group `999`:
```json
   {
      "id": 999,
      "name": "Open request",
      "cpu": 0,
      "memory": 0,
      "mysqlCpu": 0,
      "proxyCpu": 0,
      "pmmCpu": 0,
      "mysqlMemory": 0,
      "proxyMemory": 0,
      "pmmMemory": 0
    }
```
This is the ID you should use for your request, plus the values for CPU and Memory ie:
` curl -i -X GET -H "Content-Type: application/json" -d '{"output":"human","dbtype":"pxc", "dimension":  {"id": 999,"cpu":4000,"memory":2.5}, "loadtype":  {"id": 2}, "connections": 100,"mysqlversion":{"major":8,"minor":0,"patch":33}}' http://127.0.0.1:8080/calculator`

The calculator will automatically adjust the memory for MySQL, Proxy and Pmm monitoring in relation to what you are passing.
From version `1.5.0` we also support the auto calculation of the maximum number of supported connections. 
To trigger it just pass 0 as the connection value when using the Open Reuest options ie:
` curl -i -X GET -H "Content-Type: application/json" -d '{"output":"human","dbtype":"pxc", "dimension":  {"id": 999,"cpu":4000,"memory":2.5}, "loadtype":  {"id": 2}, "connections": 0,"mysqlversion":{"major":8,"minor":0,"patch":33}}' http://127.0.0.1:8080/calculator``

Let see each section one by one.
#### Dimension
- id : is what you will use to ASK the calculation
- name : just a human reference, to make easier for us 
- cpu : the TOTAL maximum available cpu dimension we will have with this solution, to share with all pods
- memory : same as CPU but for memory
- <Resource>[cpu/memory] : the segment that will be associated to the resources. 

#### LoadType
- id : again what you will use to ask for the calculation
- name : Human reference
- example : well ... just to better clarify 

### Connections
Here I just report some example, however connections can be any number from 50 up. If you pass less than 50, th evalue will be adjusted to 50, period. 

### Output
- json : well it is json you can use in your application 
- human : will give you some kindish of my.cnf output plus more information on top. You can use to easily check the output and/or cut and paste in a my.cnf

### MySQL Version
MySQL versions report the range of supported version by configurator. Inside that window the parameters settings and/or presence may change.
This is it, you may have a different value given the version of the MySQL or a parameter can be fully removed. 


## Getting the calculation back
Once you have it running and have decided what to pick, is time to get the calculation back.

To get the "results" you need to query a different entry point `/calculator` instead the previously used `/supported`.
to test it you can do something like:
`curl -i -X GET -H "Content-Type: application/json" -d '{"output":"json","dbtype":"pxc", "dimension":  {"id": 2}, "loadtype":  {"id": 2}, "connections": 400, "mysqlversion": {"major":8,"minor":
0, "patch": 30}}' http://127.0.0.1:8080/calculator` 

From version `1.1.0` we support also open requests, this means you can pass the values for memory and cpu in open forms.
When retrieving the supported dimensions you will notice a special group `999`:
```json
   {
      "id": 999,
      "name": "Open request",
      "cpu": 0,
      "memory": 0,
      "mysqlCpu": 0,
      "proxyCpu": 0,
      "pmmCpu": 0,
      "mysqlMemory": 0,
      "proxyMemory": 0,
      "pmmMemory": 0
    }
```
This is the ID you should use for your request, plus the values for CPU and Memory ie:
` curl -i -X GET -H "Content-Type: application/json" -d '{"output":"human","dbtype":"pxc", "dimension":  {"id": 999,"cpu":4000,"memory":2.5}, "loadtype":  {"id": 2}, "connections": 100,"mysqlversion":{"major":8,"minor":0,"patch":33}}' http://127.0.0.1:8080/calculator`

The calculator will automatically adjust the memory for MySQL, Proxy and Pmm monitoring in relation to what you are passing.

Your (long) output will look like this:
```json
{"request": {,"message":{
  "type": 2001,
  "name": "Execution was successful however resources are close to saturation based on the load requested",
  "text": "Request processed however not optimal details: \n\nTot Memory          = 4294967296\nTot CPU                 = 2500\nTot Connections         = 400\n\nmemory assign to mysql  = 3758096384\nmemory assign to Proxy  = 429496730\nmemory assign to Monitor= 107374182\ncpus assign to mysql  = 2000\ncpus assign to Proxy  = 350\ncpus assign to Monitor= 150\n\nGcache mem on disk      = 1053441436\nGcache mem Footprint    = 316032431\n\nTmp Table mem Footprint = 167772\nBy connection mem tot   = 655097800\n\nInnodb Bufferpool       = 2647617845\n% BP over av memory     = 0.62\n\nmemory leftover         = 139348308\n\n"
},"incoming":{
  "dbtype": "pxc",
  "dimension": {
    "id": 2,
    "name": "Small",
    "cpu": 2500,
    "memory": 4,
    "mysqlCpu": 2000,
    "proxyCpu": 350,
    "pmmCpu": 150,
    "mysqlMemory": 3.5,
    "proxyMemory": 0.4,
    "pmmMemory": 0.1
  },
  "loadtype": {
    "id": 2,
    "name": "Light OLTP",
    "example": "Shops online  up to 20% Writes "
  },
  "connections": 400,
  "output": "json"
},"answer":{
  "monitor": {
    "name": "pmm",
    "groups": {
      "livenessProbe": {
        "name": "livenessProbe",
        "parameters": {
          "timeoutSeconds": {
            "name": "timeoutSeconds",
            "section": "",
            "group": "readinessProbe",
            "value": "27",
            "default": "5",
            "min": 5,
            "max": 30
          }
        }
      },
      "readinessProbe": {
        "name": "redinessProbe",
        "parameters": {
          "timeoutSeconds": {
            "name": "timeoutSeconds",
            "section": "",
            "group": "readinessProbe",
            "value": "27",
            "default": "5",
            "min": 5,
            "max": 30
          }
        }
      },
      "resources": {
        "name": "resources",
        "parameters": {
          "limit_cpu": {
            "name": "cpu",
            "section": "limit",
            "group": "resources",
            "value": "150",
            "default": "1000",
            "min": 100,
            "max": 2000
          },
          "limit_memory": {
            "name": "memory",
            "section": "limit",
            "group": "resources",
            "value": "107374182",
            "default": "!",
            "min": 1,
            "max": 2
          },
          "request_cpu": {
            "name": "cpu",
            "section": "request",
            "group": "resources",
            "value": "142",
            "default": "1000",
            "min": 100,
            "max": 2000
          },
          "request_memory": {
            "name": "memory",
            "section": "request",
            "group": "resources",
            "value": "102005473",
            "default": "1",
            "min": 1,
            "max": 2
          }
        }
      }
    }
  },
  "mysql": {
    "name": "pxc",
    "groups": {
      "configuration_connection": {
        "name": "connections",
        "parameters": {
          "binlog_cache_size": {
            "name": "binlog_cache_size",
            "section": "configuration",
            "group": "connection",
            "value": "131072",
            "default": "32768",
            "min": 32768,
            "max": 0,
            "mysqlversions": {
              "min": {
                "major": 8,
                "minor": 0,
                "patch": 30
              },
              "max": {
                "major": 8,
                "minor": 1,
                "patch": 0
              }
            }
          },
          "binlog_stmt_cache_size": {
            "name": "binlog_stmt_cache_size",
            "section": "configuration",
            "group": "connection",
            "value": "131072",
            "default": "32768",
            "min": 32768,
            "max": 0,
            "mysqlversions": {
              "min": {
                "major": 8,
                "minor": 0,
                "patch": 30
              },
              "max": {
                "major": 8,
                "minor": 1,
                "patch": 0
              }
            }
          },
<snip ...>
        }
      }
    }
  },
  "proxy": {
    "name": "haproxy",
    "groups": {
      "ha_connection_timeout": {
        "name": "ha_connection_timeout",
        "parameters": {
          "timeoutSeconds": {
            "name": "timeoutSeconds",
            "section": "",
            "group": "ha_connection_timeout",
            "value": "5",
            "default": "1000",
            "min": 1000,
            "max": 5000
          }
        }
      },
      "livenessProbe": {
        "name": "livenessProbe",
        "parameters": {
          "timeoutSeconds": {
            "name": "timeoutSeconds",
            "section": "",
            "group": "readinessProbe",
            "value": "27",
            "default": "5",
            "min": 5,
            "max": 30
          }
        }
      },
      "readinessProbe": {
        "name": "redinessProbe",
        "parameters": {
          "timeoutSeconds": {
            "name": "timeoutSeconds",
            "section": "",
            "group": "readinessProbe",
            "value": "27",
            "default": "5",
            "min": 5,
            "max": 30
          }
        }
      },
      "resources": {
        "name": "resources",
        "parameters": {
          "limit_cpu": {
            "name": "cpu",
            "section": "limit",
            "group": "resources",
            "value": "150",
            "default": "1000",
            "min": 1000,
            "max": 2000
          },
          "limit_memory": {
            "name": "memory",
            "section": "limit",
            "group": "resources",
            "value": "429496730",
            "default": "!",
            "min": 1,
            "max": 2
          },
          "request_cpu": {
            "name": "cpu",
            "section": "request",
            "group": "resources",
            "value": "142",
            "default": "1000",
            "min": 1000,
            "max": 2000
          },
          "request_memory": {
            "name": "memory",
            "section": "request",
            "group": "resources",
            "value": "408021893",
            "default": "1",
            "min": 1,
            "max": 2
          }
        }
      }
    }
  }
}}}
```
#### Message
The first section you will see is `message`
```json
"message":{
  "type": 2001,
  "name": "Execution was successful however resources are close to saturation based on the load requested",
  "text": "Request processed however not optimal details: "
},
```
it will provide some information about the results and will tell you if the usage is fully OK, if close to the limit or worse scenario, is not possible 
to use it given resource limitation. 
In this last case toy __will not__ have the other sections. 

#### Incoming
The incoming section is a summary of the request you have sent.
I put it in so you can validate that what the tools is processing is what you have ask for:
```json
"incoming":{
  "dbtype": "pxc",
  "dimension": {
    "id": 2,
    "name": "Small",
    "cpu": 2500,
    "memory": 4,
    "mysqlCpu": 2000,
    "proxyCpu": 350,
    "pmmCpu": 150,
    "mysqlMemory": 3.5,
    "proxyMemory": 0.4,
    "pmmMemory": 0.1
  },
  "loadtype": {
    "id": 2,
    "name": "Light OLTP",
    "example": "Shops online  up to 20% Writes "
  },
  "connections": 400,
  "output": "json",
  "mysqlversion": {
    "major": 8,
    "minor": 0,
    "patch": 30
  }
}
```

#### Answer
this section is what will have the information you are looking for.
it is diveded in three __families__:
- monitor
- mysql
- proxy 

Each family has a variable number of __Groups__, and each Group has multiple __Parameters__ in.
To understand better in the MySQL family we will have a group named __configuration_connection__ which will contains all the Parameters relative to "per connection" buffers such as: sort_buffer_size, join_buffer_size and so on  
  
Each parameter has this structure:
```json
          "innodb_buffer_pool_chunk_size": {
            "name": "innodb_buffer_pool_chunk_size",
            "section": "configuration",
            "group": "innodb",
            "value": "2097152",
            "default": "134217728",
            "min": 1048576,
            "max": 0
          }    
```
It is quite self explanatory, but let us review it:
- name : is the variable name
- section : the name of the section (for future use)
- group : the group to who it belongs, in this case InnoDB configuration
- value : __THIS IS__ what you are interested in. This is the value you should take for your prcessing.
- default/min/max : are used for calculation and reference.
  
### livenessProbe / readinessProbe / resources
These three Groups are __EXTREMELY__ important.
The values for the __probes__, are calculated to help you to prevent Kubernetes to kill a perfectly working but busy Pod.
You must use them and be sure they are correctly set in your CR or all the work done will be useless. 
  
Resources are the cpu/memory dimension you should set. You will always have a LIMIT and a REQUEST for the resources. Keep in mind that whatever will push your pod above the memory limit will IMMEDIATELY trigger the OOM killer :) not a nice thing to have. 

# Module
MySQLOperatorCalculator is also available as module.
This is it you can include it in your code and query it directly getting back objects to browse or Json.
The example directory contains a very simple example of code on how to do it, but mainly you have to:
```go 
import (
	"bytes"
	"encoding/json"
	"fmt"
	MO "github.com/Tusamarco/mysqloperatorcalculator/src/mysqloperatorcalculator"  <----------- Import the module
	"strconv"
)

```
Then when you need it:
```go
func main() {
var my MO.MysqlOperatorCalculator

testSupportedJson(my.GetSupportedLayouts(), my)  <---------------- 

testGetconfiguration(my)

}
func testSupportedJson(supported MO.Configuration, calculator MO.MysqlOperatorCalculator) {
	output, err := json.MarshalIndent(&supported, "", "  ")
	if err != nil {
		print(err.Error())
	}
	fmt.Println(string(output))

}
```
In the example above I get the list of all supported platform in Json format
But the list as objects is already there in one single call:
```go
my.GetSupportedLayouts()
```
To get the full set of parameters we first need to build the request, then pass it and get back a map containing all the settings:
```go
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
```
The first object we need to create is the ConfigurationRequest:
```go
	var myRequest MO.ConfigurationRequest
```
then we can populate it, in this case we DO NOT set a supported environment but an open scope:
```go
	myRequest.LoadType = MO.LoadType{Id: 2}
	myRequest.Dimension = MO.Dimension{Id: 999, Cpu: 4000, Memory: 2.5}
	myRequest.DBType = "group_replication" //"pxc"
	myRequest.Output = "human"             //"human"
	myRequest.Connections = 70
	myRequest.Mysqlversion = MO.Version{8, 0, 33}
```
With DImension Id 999 we declare is going to be an open scope and after we must declare the CPU and memory.
If instead we choose a fix and supported dimension, then the Dimension.Id and MySQL Version, will be enough.

We then need to:
```go
	moc.Init(myRequest)
	error, responseMessage, families := moc.GetCalculate()
```
Where families will be the top container of all our settings.
We can decide if to parse it as a Map or if convert it to other formats like Json, or plain text.

This is it, easy.

From version `1.5.0` mysqloperatorcalculator supports the use of contants and more important, when using _HUMAN_ output you can retrive the parameters by Group.
Please refer to the (example.go)[example/example.go] file. 


# Final... 
The toool is there and it needs testing and real evaluation, so I reccomand you to test, test, test whatever configuration you will get. 
Notihing is perfect, so let me know if you find things that make no sense or not workign as expected. 
  
Last thing ... 
you can use:
  --version to get the version  
  --help to get basic help at command line

Thank you   
