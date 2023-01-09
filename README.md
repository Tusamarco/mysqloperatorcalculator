# MySQL Calculator for Operator
## Why 
With the advent of Kubernets (k8), had become incresingly common to deploy RBDMS on K8 supported platforms. 
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
  ]
}
```
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


## Getting the calculation back
Once you have it running and have decide what to pick, is time to get the calculation back.

To get the "results" you need to query a different entry point `/calculator` instead the previously used `/supported`.
to test it you can do something like:
`curl -i -X GET -H "Content-Type: application/json" -d '{"output":"json","dbtype":"pxc", "dimension":  {"id": 2}, "loadtype":  {"id": 2}, "connections": 400}' http://127.0.0.1:8080/calculator` 

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
            "max": 0
          },
          "binlog_stmt_cache_size": {
            "name": "binlog_stmt_cache_size",
            "section": "configuration",
            "group": "connection",
            "value": "131072",
            "default": "32768",
            "min": 32768,
            "max": 0
          },
          "join_buffer_size": {
            "name": "join_buffer_size",
            "section": "configuration",
            "group": "connection",
            "value": "524288",
            "default": "262144",
            "min": 262144,
            "max": 0
          },
          "max_heap_table_size": {
            "name": "max_heap_table_size",
            "section": "configuration",
            "group": "connection",
            "value": "16777216",
            "default": "16777216",
            "min": 16777216,
            "max": 0
          },
          "read_rnd_buffer_size": {
            "name": "read_rnd_buffer_size",
            "section": "configuration",
            "group": "connection",
            "value": "393216",
            "default": "262144",
            "min": 262144,
            "max": 0
          },
          "sort_buffer_size": {
            "name": "sort_buffer_size",
            "section": "configuration",
            "group": "connection",
            "value": "524288",
            "default": "524288",
            "min": 524288,
            "max": 0
          },
          "tmp_table_size": {
            "name": "tmp_table_size",
            "section": "configuration",
            "group": "connection",
            "value": "16777216",
            "default": "16777216",
            "min": 16777216,
            "max": 0
          }
        }
      },
      "configuration_galera": {
        "name": "galera",
        "parameters": {
          "wsrep-provider-options": {
            "name": "wsrep-provider-options",
            "section": "configuration",
            "group": "galera",
            "value": "gcache.recover=yes;evs.delay_margin=PT26S;evs.user_send_window=PT896S;evs.inactive_check_period=PT4S;evs.join_retrans_period=PT4S;evs.stats_report_period=PT1M;evs.delayed_keep_period=PT552S;evs.send_window=PT896S;evs.suspect_timeout=PT52S;pc.announce_timeout=PT52S;gcache.size=0;evs.inactive_timeout=PT105S;gmcast.peer_timeout=PT13S;gmcast.time_wait=PT15S;pc.linger=PT52S;pc.recovery=true;gcs.fc_limit=PT112S;gcs.max_packet_size=114688;evs.max_install_timeouts=4;",
            "default": "",
            "min": 0,
            "max": 0
          },
          "wsrep_slave_threads": {
            "name": "wsrep_slave_threads",
            "section": "configuration",
            "group": "galera",
            "value": "1",
            "default": "1",
            "min": 1,
            "max": 0
          },
          "wsrep_sync_wait": {
            "name": "wsrep_sync_wait",
            "section": "configuration",
            "group": "galera",
            "value": "3",
            "default": "0",
            "min": 0,
            "max": 8
          },
          "wsrep_trx_fragment_size": {
            "name": "wsrep_trx_fragment_size",
            "section": "configuration",
            "group": "galera",
            "value": "1048576",
            "default": "0",
            "min": 0,
            "max": 0
          },
          "wsrep_trx_fragment_unit": {
            "name": "wsrep_trx_fragment_unit",
            "section": "configuration",
            "group": "galera",
            "value": "bytes",
            "default": "bytes",
            "min": -1,
            "max": -1
          }
        }
      },
      "configuration_innodb": {
        "name": "innodb",
        "parameters": {
          "innodb_adaptive_hash_index": {
            "name": "innodb_adaptive_hash_index",
            "section": "configuration",
            "group": "innodb",
            "value": "True",
            "default": "1",
            "min": 0,
            "max": 1
          },
          "innodb_buffer_pool_chunk_size": {
            "name": "innodb_buffer_pool_chunk_size",
            "section": "configuration",
            "group": "innodb",
            "value": "2097152",
            "default": "134217728",
            "min": 1048576,
            "max": 0
          },
          "innodb_buffer_pool_instances": {
            "name": "innodb_buffer_pool_instances",
            "section": "configuration",
            "group": "innodb",
            "value": "1",
            "default": "8",
            "min": 1,
            "max": 64
          },
          "innodb_buffer_pool_size": {
            "name": "innodb_buffer_pool_size",
            "section": "configuration",
            "group": "innodb",
            "value": "2647617845",
            "default": "134217728",
            "min": 5242880,
            "max": 0
          },
          "innodb_flush_log_at_trx_commit": {
            "name": "innodb_flush_log_at_trx_commit",
            "section": "configuration",
            "group": "innodb",
            "value": "2",
            "default": "1",
            "min": 0,
            "max": 2
          },
          "innodb_flush_method": {
            "name": "innodb_flush_method",
            "section": "configuration",
            "group": "innodb",
            "value": "O_DIRECT",
            "default": "O_DIRECT",
            "min": 0,
            "max": 0
          },
          "innodb_io_capacity_max": {
            "name": "innodb_io_capacity_max",
            "section": "configuration",
            "group": "innodb",
            "value": "1800",
            "default": "1400",
            "min": 100,
            "max": 0
          },
          "innodb_log_file_size": {
            "name": "innodb_log_file_size",
            "section": "configuration",
            "group": "innodb",
            "value": "305345344",
            "default": "50331648",
            "min": 4194304,
            "max": 0
          },
          "innodb_log_files_in_group": {
            "name": "innodb_log_files_in_group",
            "section": "configuration",
            "group": "innodb",
            "value": "3",
            "default": "2",
            "min": 2,
            "max": 100
          },
          "innodb_page_cleaners": {
            "name": "innodb_buffer_pool_instances",
            "section": "configuration",
            "group": "innodb",
            "value": "1",
            "default": "8",
            "min": 1,
            "max": 64
          },
          "innodb_purge_threads": {
            "name": "innodb_purge_threads",
            "section": "configuration",
            "group": "innodb",
            "value": "4",
            "default": "4",
            "min": 1,
            "max": 32
          }
        }
      },
      "configuration_server": {
        "name": "server",
        "parameters": {
          "max_connections": {
            "name": "max_connections",
            "section": "configuration",
            "group": "server",
            "value": "402",
            "default": "2",
            "min": 2,
            "max": 65536
          },
          "table_definition_cache": {
            "name": "table_definition_cache",
            "section": "configuration",
            "group": "server",
            "value": "4096",
            "default": "4096",
            "min": 400,
            "max": 524288
          },
          "table_open_cache": {
            "name": "table_open_cache",
            "section": "configuration",
            "group": "server",
            "value": "4096",
            "default": "4096",
            "min": 400,
            "max": 524288
          },
          "table_open_cache_instances": {
            "name": "table_open_cache_instances",
            "section": "configuration",
            "group": "server",
            "value": "4",
            "default": "16",
            "min": 1,
            "max": 64
          },
          "tablespace_definition_cache": {
            "name": "tablespace_definition_cache",
            "section": "configuration",
            "group": "server",
            "value": "512",
            "default": "256",
            "min": 256,
            "max": 524288
          },
          "thread_pool_size": {
            "name": "thread_pool_size",
            "section": "configuration",
            "group": "server",
            "value": "4",
            "default": "2",
            "min": 2,
            "max": 64
          },
          "thread_stack": {
            "name": "thread_stack",
            "section": "configuration",
            "group": "server",
            "value": "1024",
            "default": "1024",
            "min": 125,
            "max": 1048576
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
            "value": "525",
            "default": "15",
            "min": 15,
            "max": 600
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
            "value": "525",
            "default": "15",
            "min": 15,
            "max": 600
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
            "value": "2000",
            "default": "1000",
            "min": 1000,
            "max": 8500
          },
          "limit_memory": {
            "name": "memory",
            "section": "limit",
            "group": "resources",
            "value": "3758096384",
            "default": "2",
            "min": 2,
            "max": 32
          },
          "request_cpu": {
            "name": "cpu",
            "section": "request",
            "group": "resources",
            "value": "1900",
            "default": "1000",
            "min": 1000,
            "max": 8500
          },
          "request_memory": {
            "name": "memory",
            "section": "request",
            "group": "resources",
            "value": "3570191565",
            "default": "2",
            "min": 2,
            "max": 32
          }
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
  "output": "json"
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
  
# Final... 
The toool is there and it needs testing and real evaluation, so I reccomand you to test, test, test whatever configuration you will get. 
Notihing is perfect, so let me know if you find things that make no sense or not workign as expected. 
  
Last thing ... 
you can use:
  --version to get the version  
  --help to get basic help at command line

Thank you   
