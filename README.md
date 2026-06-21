# MySQL Operator Calculator

## 📖 Overview

The **MySQL Operator Calculator** is a robust Go library designed to dynamically calculate the optimal configurations, Kubernetes resource requests, and limits for MySQL deployments. Built specifically with Kubernetes Operators in mind, it analyzes target hardware dimensions, expected connections, and load profiles to generate highly tuned configurations for **MySQL**, **HAProxy (Proxy)**, and **Percona Monitoring and Management (Monitor)**.

It supports both **Percona XtraDB Cluster (Galera / PXC)** and **MySQL Group Replication**, taking into account version-specific parameters and the hidden memory footprints of various MySQL internal structures (like GCache, GCS Cache, connection buffers, and temporary tables).

### What It Does
Given a set of total available resources (CPU/memory), a workload pattern, target connection count, and MySQL version, it generates:
- **Per‑buffer tuning** (`sort_buffer_size`, `join_buffer_size`, etc.)
- **Resource distribution** among the MySQL, proxy, and monitoring (PMM) containers
- **Kubernetes liveness/readiness probes** and **resource limits/requests**
- **InnoDB‑specific settings** (buffer pool, log file size, etc.)
- **Binary log / replication cache sizing**

### Why It Is Useful
On traditional servers, MySQL tuning follows a well‑known “add more RAM, increase buffer pool” pattern. In Kubernetes, however:
- The shared environment (sidecars, monitoring, logging, service meshes) consumes part of the total node resources.
- You cannot simply assign all available memory to InnoDB; you must account for proxy, PMM, and OS overhead.
- Kubernetes **probes** must be tuned to avoid killing a busy but healthy database pod.
- Resource limits are mandatory—exceeding them causes an immediate **OOM kill**.

This calculator automates the otherwise tedious and error‑prone manual tuning for containerized MySQL.

---

## ⚙️ Parameters (The Input)

All parameters are passed as a JSON payload to the `/calculator` endpoint or via the Go module API.

| Parameter | Type | Required | Description |
|:---|:---|:---:|:---|
| `output` | `string` | **Yes** | `"json"` (structured) or `"human"` (my.cnf‑like text) |
| `dbtype` | `string` | **Yes** | `"pxc"` or `"group_replication"` |
| `dimension.id` | `int` | **Yes** | Pre‑defined ID (`1`…`n`), `998` (auto‑dimension by connections), or `999` (open request) |
| `dimension.cpu` | `int` | *Cond.* | Required if `id=999`. Total CPU in millicores (e.g., `4000` = 4 full cores) |
| `dimension.memory` | `string` | *Cond.* | Required if `id=999`. Total memory (e.g., `"2.5G"`, `"4096Mi"`, `"4GB"`) |
| `loadtype.id` | `int` | **Yes** | `1` (Mainly Reads), `2` (Light OLTP), `3` (Heavy OLTP) |
| `connections` | `int` | **Yes** | Number of connections (min `50`). Pass `0` to auto‑calculate max supported. |
| `mysqlversion.major` | `int` | **Yes** | MySQL major version (currently only `8`) |
| `mysqlversion.minor` | `int` | **Yes** | MySQL minor version (`0` … `4`) |
| `mysqlversion.patch` | `int` | **Yes** | Patch version |
| `providercostpct` | `float` | No | Platform overhead (e.g., `0.15` = 15%). Default `0`. |

> **💡 Important Notes:**
> - Connection values below **50** are automatically raised to `50`.
> - If `connections` is set to `0`, the calculator iteratively increments the connection count until resources become saturated, returning the highest viable value.
> - If `dimension.id` is set to `998`, the request is **connection‑driven**. The calculator will automatically pick the smallest pre‑defined dimension that can comfortably handle the requested connection count.

---

## 📤 Output Structure

When `output = "json"`, the response contains three top‑level sections:

### 1. `message` (Diagnostic Information)
```json
"message": {
  "type": 7001,
  "name": "All resources have been recalculated to match the requested connections",
  "text": "Request ok, resources details: ..."
}
```

**Message Types:**
* `1001`: Execution successful, resources match the request perfectly.
* `2001`: Successful, but resources are **close to saturation**.
* `3001`: Resources **not enough** to cover the load (no answer returned).
* `6001`: The number of connections was recalculated to fit available resources.
* `7001`: Resources were scaled up/recalculated to match the requested connections (`dimension.id = 998`).

### 2. `incoming` (Request Echo)
Contains the exact payload you sent, plus the internal resource distribution the calculator ultimately decided to use.

### 3. `answer` (Configuration Families)
Three families are always present: `monitor`, `mysql`, and `proxy`. Each family contains one or more **groups**, and each group contains multiple **parameters**.

**Example: MySQL `configuration_connection` group**
```json
"configuration_connection": {
  "name": "connections",
  "parameters": {
    "binlog_cache_size": { "name": "binlog_cache_size", "value": "131072" },
    "join_buffer_size": { "name": "join_buffer_size", "value": "524288" }
  }
}
```
*Note: The `value` field is the calculated number you should use in your deployments.*

> **⚠️ Critical Warning on Probes and Limits:**
> The `livenessProbe`, `readinessProbe`, and `resources` (CPU/Memory limits) groups are **not optional**. Ignoring the generated resource limits or probe timings will almost certainly cause unnecessary pod restarts or OOM kills under load.

---

## ⚖️ PXC vs. Group Replication

The calculator treats PXC and Group Replication differently because their internal caches have distinct memory consumption patterns.

| Aspect | PXC (Galera) | Group Replication |
|:---|:---|:---|
| **InnoDB Buffer Pool** | Up to **80%** of MySQL memory | Up to **68%** of MySQL memory |
| **Reasoning** | Galera’s GCache footprint is relatively small and stable. | GR’s **certification cache** can bloat during long transactions, risking OOM kills. |
| **Min. InnoDB Memory** | `0.45` (45% of MySQL memory) | `0.40` (40% of MySQL memory) |
| **Tuning Constants** | `GcacheFootPrintFactorRead = 0.5`, etc. | Additional `gcscacheFootprint` reserved. |

These constraints are mapped directly in the code:
```go
InnoDBPctValuePXC  = 0.80
InnoDBPctValueGR   = 0.68
MinLimitPXC        = 0.45
MinLimitGR         = 0.40
```

---

## 🚀 Running as a Server

### Compilation & Installation
Clone the repository and build the binary:
```bash
git clone https://github.com/Tusamarco/mysqloperatorcalculator
cd mysqloperatorcalculator
go build -o mysqloperatorcalculator ./src
```

### Command‑Line Flags
| Flag | Default | Description |
|:---|:---|:---|
| `-address` | `0.0.0.0` | IP address to bind to |
| `-port` | `8080` | Listening port |
| `--help` | – | Show usage |
| `--version` | – | Show version |

### API Endpoints
* **`GET /supported`**: Returns all pre‑defined dimensions, load types, supported MySQL versions, and possible output formats.
* **`POST /calculator`** (also accepts `GET`): Takes a JSON payload and returns the calculated configuration.

**Example Request:**
```bash
curl -i -X POST -H "Content-Type: application/json" -d '{
  "output": "json",
  "dbtype": "pxc",
  "dimension": { "id": 2 },
  "loadtype": { "id": 2 },
  "connections": 400,
  "mysqlversion": { "major": 8, "minor": 0, "patch": 30 }
}' http://127.0.0.1:8080/calculator
```

**Auto-Dimensioning by Connections (`id: 998`):**
If you don't know what hardware you need, tell the calculator your connection requirements, and it will pick the right hardware profile:
```bash
curl -i -X POST -H "Content-Type: application/json" -d '{
  "output": "json",
  "dbtype": "pxc",
  "dimension": { "id": 998 },
  "loadtype": { "id": 2 },
  "connections": 600,
  "mysqlversion": { "major": 8, "minor": 0, "patch": 33 }
}' http://127.0.0.1:8080/calculator
```

---

## 📦 Using as a Go Module

The module is fully importable and exposes a clean API for direct integration into your Go applications or custom Kubernetes Operators.

### 1. Import the Module
```go
import MO "github.com/Tusamarco/mysqloperatorcalculator/src/mysqloperatorcalculator"
```

### 2. Supported Layouts (Metadata)
Instead of hitting the `/supported` HTTP endpoint, fetch the layouts programmatically:
```go
var calculator MO.MysqlOperatorCalculator
supportedConf := calculator.GetSupportedLayouts()

// Optional: Print to JSON
output, _ := json.MarshalIndent(&supportedConf, "", " ")
fmt.Println(string(output))
```

### 3. Build a Request and Calculate
Here is a complete, working example of how to configure a request, convert memory units, and retrieve the configurations.

```go
package main

import (
    "bytes"
    "fmt"
    "log"
    MO "github.com/Tusamarco/mysqloperatorcalculator/src/mysqloperatorcalculator"
)

func main() {
    var moc MO.MysqlOperatorCalculator
    var myRequest MO.ConfigurationRequest
    var conf MO.Configuration
    var err error

    // 1. Setup Request Parameters
    myRequest.LoadType = MO.LoadType{Id: MO.LoadTypeSomeWrites}
    myRequest.DBType = MO.DbTypeGroupReplication 
    myRequest.Output = MO.ResultOutputFormatHuman
    myRequest.Connections = 3000
    myRequest.Mysqlversion = MO.Version{Major: 8, Minor: 4, Patch: 8}
    myRequest.ProviderCostPct = 0.12

    // Using an "Open" dimension requires explicit CPU and Memory
    myRequest.Dimension = MO.Dimension{Id: MO.DimensionOpen, Cpu: 4000, Memory: "2.5G"}

    // Convert string memory ("2.5G") to Bytes (Required for internal calculations)
    myRequest.Dimension.MemoryBytes, err = myRequest.Dimension.ConvertMemoryToBytes(myRequest.Dimension.Memory)
    if err != nil {
       log.Fatalf("Memory conversion error: %v\n", err)
    }

    // 2. Initialize and Calculate
    conf.Init()
    moc.Init(myRequest, conf)

    calcErr, responseMessage, families := moc.GetCalculate()
    if calcErr != nil {
       log.Fatalf("Calculation error: %v\n", calcErr)
    }

    if responseMessage.MType > 0 {
       log.Printf("Status Message [%d]: %s - %s\n", responseMessage.MType, responseMessage.MName, responseMessage.MText)
    }

    // 3. Extract Specific Configurations
    if len(families) > 0 {
        // Example: Grabbing just the MySQL family
        if mysqlFamily, err := moc.GetFamily(MO.FamilyTypeMysql); err == nil {
            // Retrieve specific blocks
            mysqlConf, _ := mysqlFamily.ParseFamilyGroup(MO.GroupNameMySQLd, "   ")
            fmt.Println("[MySQL Configuration]\n", mysqlConf.String())
            
            mysqlProbes, _ := mysqlFamily.ParseFamilyGroup(MO.GroupNameProbes, "   ")
            fmt.Println("[MySQL Probes]\n", mysqlProbes.String())
        }
        
        // 4. Or grab everything at once formatted according to myRequest.Output ("human" or "json")
        var b bytes.Buffer
        if myRequest.Output == "json" {
            b, err = moc.GetJSONOutput(responseMessage, myRequest, families)
        } else {
            b, err = moc.GetHumanOutput(responseMessage, myRequest, families)
        }
        
        if err == nil {
            fmt.Println("\n--- FULL OUTPUT ---")
            fmt.Println(b.String())
        }
    }
}
```

> **💡 Pro Tip on Memory Parsing:** If you already know the bytes, you can skip `ConvertMemoryToBytes` and assign it directly: `myRequest.Dimension.MemoryBytes = 2684354560`.

*For a ready-to-run file, view `src/example/example.go` in the GitHub repository.*

---

Here is the reviewed and optimized version of your "How-To" guide. I have fixed the broken code blocks (specifically the text incorrectly placed inside the Go block in section 2.3), merged the fragmented code segments into cohesive, copy-pasteable examples, and streamlined the formatting for better scannability.

---

## 📝 How to Use – Practical Examples

This section provides concrete examples for implementing the `mysqloperatorcalculator`. The tool is designed to be used in two primary ways:

* **As a standalone service:** Submit JSON requests via HTTP `POST`/`GET` endpoints.
* **As an embedded Go module:** Integrate the calculator directly into your Go application or Kubernetes Operator.

---

### 1. Standalone Service

After building and starting the binary with `./mysqloperatorcalculator -address=127.0.0.1 -port=8080`, you can interact with the service using standard HTTP clients like `curl`.

#### 1.1 Discover Supported Parameters

The `/supported` endpoint returns all built‑in dimensions, load types, connection presets, output formats, and supported MySQL versions.

```bash
curl -X GET http://127.0.0.1:8080/supported

```

**Example Response:**

```json
{
  "dbtype": [ "group_replication", "pxc" ],
  "dimension": [
    { "id": 1, "name": "XSmall", "cpu": 1000, "memory": 2 },
    ...
  ],
  "loadtype": [
    { "id": 1, "name": "Mainly Reads", "example": "Blogs ~2% Writes 95% Reads" },
    { "id": 2, "name": "Light OLTP", "example": "Shops online up to 20% Writes" },
    { "id": 3, "name": "Heavy OLTP", "example": "Intense analytics... 50/50% Reads and Writes" }
  ],
  "connections": [ 50, 100, 200, 500, 1000, 2000 ],
  "output": [ "human", "json" ],
  "mysqlversions": { "min": { "major": 8, "minor": 0, "patch": 32 }, "max": { "major": 8, "minor": 1, "patch": 0 } }
}

```

#### 1.2 Request a Configuration

The `/calculator` endpoint accepts a JSON payload detailing your requirements.

**Example A: Pre‑defined Dimension ID**

```bash
curl -X POST -H "Content-Type: application/json" -d '{
  "output": "json",
  "dbtype": "pxc",
  "dimension": { "id": 2 },
  "loadtype": { "id": 2 },
  "connections": 400,
  "mysqlversion": { "major": 8, "minor": 0, "patch": 30 }
}' http://127.0.0.1:8080/calculator

```

**Example B: Open Request (Custom CPU/Memory)**
Setting `dimension.id` to `999` indicates an *open request*. You must explicitly provide the total `cpu` (in millicores) and `memory` (e.g., `"2.5GB"`, `"4096Mi"`, or `"4G"`). The service will distribute these resources among the MySQL container, proxy, and monitoring sidecars.

```bash
curl -X POST -H "Content-Type: application/json" -d '{
  "output": "json",
  "dbtype": "group_replication",
  "dimension": { "id": 999, "cpu": 4000, "memory": "2.5GB" },
  "loadtype": { "id": 2 },
  "connections": 70,
  "mysqlversion": { "major": 8, "minor": 0, "patch": 33 }
}' http://127.0.0.1:8080/calculator

```

---

### 2. Using the Go Module

Importing the calculator as a Go module is the recommended approach for embedding it into custom operators.

#### 2.1 Import and Supported Layouts

First, import the module. You can retrieve the supported configurations programmatically instead of relying on the HTTP endpoint.

```go
import (
    "bytes"
    "encoding/json"
    "fmt"
    "log"
    MO "github.com/Tusamarco/mysqloperatorcalculator/src/mysqloperatorcalculator"
)

// Retrieve supported layouts
var my MO.MysqlOperatorCalculator
supportedConf := my.GetSupportedLayouts()

// Optional: Marshal to JSON for inspection
output, _ := json.MarshalIndent(&supportedConf, "", " ")
fmt.Println(string(output))

```

#### 2.2 Complete Example: Build, Calculate, and Retrieve

Below is a consolidated example showing how to construct a `ConfigurationRequest`, initialize the calculator, execute the calculation, and extract specific configuration groups (like `mysqld` settings or Kubernetes probes).

```go
func testGetconfiguration(moc MO.MysqlOperatorCalculator) {
    var myRequest MO.ConfigurationRequest
    var conf MO.Configuration
    var err error

    // 1. Build the Request
    myRequest.LoadType = MO.LoadType{Id: MO.LoadTypeSomeWrites}
    myRequest.DBType = MO.DbTypeGroupReplication
    myRequest.Output = MO.ResultOutputFormatHuman
    myRequest.Connections = 3000
    myRequest.Mysqlversion = MO.Version{Major: 8, Minor: 4, Patch: 8}
    myRequest.ProviderCostPct = 0.12

    // Setting an open dimension using literal values
    myRequest.Dimension = MO.Dimension{Id: MO.DimensionOpen, Cpu: 4000, Memory: "2.5G"}

    // Convert string memory to bytes (Mandatory for open requests)
    myRequest.Dimension.MemoryBytes, err = myRequest.Dimension.ConvertMemoryToBytes(myRequest.Dimension.Memory)
    if err != nil {
       log.Fatalf("Memory conversion error: %v\n", err)
    }

    // 2. Initialize and Calculate
    conf.Init()
    moc.Init(myRequest, conf)

    calcErr, responseMessage, families := moc.GetCalculate()
    if calcErr != nil {
       log.Printf("Calculation error: %v\n", calcErr)
       return
    }

    if responseMessage.MType > 0 {
       log.Printf("Message %d: %s - %s\n", responseMessage.MType, responseMessage.MName, responseMessage.MText)
    }

    // 3. Extract and Print Configurations by Group
    if len(families) > 0 {
       // Example: Parsing the MySQL Family
       if mysqlFamily, err := moc.GetFamily(MO.FamilyTypeMysql); err == nil {
          printFamilyGroup(mysqlFamily, MO.GroupNameMySQLd, " ", "MySQL Configuration")
          printFamilyGroup(mysqlFamily, MO.GroupNameProbes, " ", "MySQL Probes")
          printFamilyGroup(mysqlFamily, MO.GroupNameResources, " ", "MySQL Resources")
       } else {
          log.Printf("Error retrieving MySQL family: %v\n", err)
       }

       // 4. Alternatively, grab everything at once (based on Output format)
       var b bytes.Buffer
       if myRequest.Output == "json" {
          b, err = moc.GetJSONOutput(responseMessage, myRequest, families)
       } else {
          b, err = moc.GetHumanOutput(responseMessage, myRequest, families)
       }

       if err == nil {
          fmt.Println("\n--- FULL OUTPUT ---")
          fmt.Println(b.String())
       }
    }
}

// Helper interface & function to DRY up repetitive parsing
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

```

#### 2.3 Best Practices for Memory Values

When utilizing the module, memory must ultimately be evaluated in bytes. You can either assign it directly or use the built-in converter:

```go
// Method 1: Direct assignment (Recommended if you know the exact byte count)
myRequest.Dimension = MO.Dimension{Id: MO.DimensionOpen, Cpu: 4000, MemoryBytes: 2684354560}

// Method 2: Literal conversion
myRequest.Dimension = MO.Dimension{Id: MO.DimensionOpen, Cpu: 4000, Memory: "2.5G"}
myRequest.Dimension.MemoryBytes, err = myRequest.Dimension.ConvertMemoryToBytes(myRequest.Dimension.Memory)

```

> **Note:** For a fully runnable working example, refer to `src/example/example.go` in the official repository.

---

### 3. Summary of the Workflow

| Step | Action | Description |
| --- | --- | --- |
| **1** | Start / Initialize | Run the standalone server (`-port=8080`) or initialize the Go module. |
| **2** | Discover *(Optional)* | Query `/supported` to see available dimensions and parameters. |
| **3** | Build Request | Construct a JSON payload or `ConfigurationRequest` struct with target CPU, Memory, and DB constraints. |
| **4** | Calculate | Submit to `/calculator` or call `GetCalculate()`. |
| **5** | Apply | Use the returned variables, Kubernetes resource limits, and probe timings in your deployment. |

*The calculator automatically scales up resource distribution and connection limits if your request risks saturation, ensuring safety bounds. It will also enforce a hard minimum of 50 connections.*

---
## 📊 Output Examples

Depending on the `output` parameter provided in your request, the calculator returns either a highly structured JSON payload (ideal for automation and Kubernetes Operators) or a human-readable text format (ideal for direct inspection and `.cnf` files).

Below are abbreviated examples of both output types for a request asking for **400 connections** on a **Percona XtraDB Cluster (PXC)**.

### 1. JSON Output (`"output": "json"`)

The JSON response is divided into three main blocks: `message` (diagnostic status), `incoming` (echo of the resolved request and limits), and `answer` (the calculated families and groups).

```json
{
  "message": {
    "type": 1001,
    "name": "Success",
    "text": "Execution successful, resources match the request perfectly."
  },
  "incoming": {
    "dbtype": "pxc",
    "connections": 400,
    "dimension": {
      "id": 2,
      "cpu": 2000,
      "memory": "8G"
    },
    "mysqlversion": {
      "major": 8,
      "minor": 0,
      "patch": 30
    }
  },
  "answer": {
    "mysql": {
      "configuration_connection": {
        "name": "connections",
        "parameters": {
          "max_connections": {
            "name": "max_connections",
            "value": "400"
          },
          "join_buffer_size": {
            "name": "join_buffer_size",
            "value": "262144"
          }
        }
      },
      "resources": {
        "name": "resources",
        "parameters": {
          "limit_memory": {
            "name": "limit_memory",
            "value": "6442450944"
          },
          "request_cpu": {
            "name": "request_cpu",
            "value": "1800"
          }
        }
      },
      "probes": {
        "name": "probes",
        "parameters": {
          "liveness_timeoutSeconds": {
            "name": "liveness_timeoutSeconds",
            "value": "5"
          }
        }
      }
    },
    "proxy": {
      "...": "..."
    },
    "monitor": {
      "...": "..."
    }
  }
}

```

### 2. Human-Readable Output (`"output": "human"`)

The human-readable format strips away the structural metadata, outputting flat, INI-style blocks. This is particularly useful for quickly pasting into a `my.cnf` file or reviewing the raw numbers.

```ini
--- MESSAGE ---
Message 1001: Success - Execution successful, resources match the request perfectly.

--- MYSQL ---

max_connections = 400
join_buffer_size = 262144
sort_buffer_size = 262144
innodb_buffer_pool_size = 4294967296
innodb_log_file_size = 1073741824
binlog_cache_size = 131072

[mysql resources]
request_cpu = 1800
limit_cpu = 1800
request_memory = 6442450944
limit_memory = 6442450944

[mysql probes]
liveness_timeoutSeconds = 5
liveness_periodSeconds = 10
readiness_timeoutSeconds = 3
readiness_periodSeconds = 5

--- PROXY ---
[haproxy configuration]
maxconn = 1200
timeout_client = 28800s

[haproxy resources]
request_cpu = 150
limit_cpu = 150
request_memory = 268435456
limit_memory = 268435456

```

## 📝 Final Notes & Best Practices

1. **Versioning:** The tool currently supports MySQL **8.0.32 through 10.x**. Certain parameters are removed, adjusted, or deprecated outside this range.
2. **Testing:** Always **test** the generated configurations in a non‑production Kubernetes environment before rolling them out to a critical production cluster.
3. **Customization:** The source code relies on several sensible internal constants (e.g., `CPUIncrement`, `MemoryIncrement`, `GcacheFootPrintFactorRead`). If your specific environment diverges heavily from standard cloud workloads, these can be adjusted in the Go code.

*For bug reports, feature requests, or contributions, please open an issue on the [GitHub Repository](https://github.com/Tusamarco/mysqloperatorcalculator).*