package main

import (
	"bytes"
	"fmt"
	log "github.com/sirupsen/logrus"
	"math"
	o "mysqloperatorcalculator/src/Objects"
	"strconv"
)

type Configurator struct {
	request        o.ConfigurationRequest
	families       map[string]o.Family
	providerParams map[string]o.ProviderParam
	reference      *references
}

// This structure is used to keep information that is needed while calculating the parameters
type references struct {
	memory          float64 //total memory available
	cpus            int     //total cpus
	gcache          int64   // assigned gcache dimension
	gcacheFootprint int64   // expected file footprint in memory
	gcacheLoad      float64 // gcache load adj factor base on type of load

	memoryLeftover     int64   // memory free after all calculation
	innodbRedoLogDim   int64   // total redolog dimension
	innoDBbpSize       int64   // Calculated BP to apply
	loadAdjustment     float32 // load adjustment indicator based on CPU weight against connections
	loadAdjustmentMax  int     // Upper limit given optimal condition between CPU resources and connections using as minimal connections=50
	loadFactor         float32 // Load factor for calculation based on loadAdjustment
	loadID             int     // loadID coming from request
	dimension          int     // Dimension Id coming from request
	connections        int     // raw number of connections
	tmpTableFootprint  int64   // tempTable expected footprint in memory
	connBuffersMemTot  int64   // Total mem use for all connection buffers + temp table
	idealBufferPoolDIm int64   // Theoretical ideal BP dimension (rule of the thumb)
	innoDBBPInstances  int     //  assigned number of BP
	cpusPmm            float64 // cpu assigned to pmm
	cpusProxy          float64 // cpu assigned to proxy
	cpusMySQL          float64 // cpu assigned to mysql
	memoryMySQL        float64 // memory assigned to MySQL
	memoryProxy        float64 // memory assigned to proxy
	memoryPmm          float64 // memory assigned to pmm
	gcscache           int64   // assigned GR GCScache dimension
	gcscacheFootprint  int64   // GR GCScache expected file footprint in memory
	gcscacheLoad       float64 // GR GCScache load adj factor base on memory available

}

// GetAllGaleraProviderOptionsAsString return all provider option considered as single string for the parameter value
func (c *Configurator) GetAllGaleraProviderOptionsAsString() bytes.Buffer {

	var b bytes.Buffer
	//b.WriteString(`"`)

	for key, param := range c.providerParams {
		b.WriteString(key)
		b.WriteString(`=`)
		if param.Value >= 0 {
			b.WriteString(fmt.Sprintf(param.Literal, strconv.FormatInt(param.Value, 10)))

		} else {
			b.WriteString(param.Literal)
		}
		b.WriteString(";")
	}
	//b.WriteString(`"`)
	return b
}

func (c *Configurator) init(r o.ConfigurationRequest, fam map[string]o.Family, conf o.Configuration, message o.ResponseMessage) (o.ResponseMessage, bool) {

	//if dimension is custom we take it from request otherwise from Configuration
	var dim o.Dimension
	if r.Dimension.Id != 999 {
		dim = conf.GetDimensionByID(r.Dimension.Id)
	} else {
		dim = r.Dimension
	}
	load := conf.GetLoadByID(r.LoadType.Id)
	if load.Id == 0 || dim.Id == 0 {
		log.Warning(fmt.Sprintf("Invalid load %d or Dimension %d detected ", load.Id, dim.Id))
	}
	connections := r.Connections
	if connections < 50 {
		connections = 50
	}

	ref := references{
		((dim.Memory * 1024) * 1024) * 1024, // convert to bytes
		dim.Cpu,
		0,
		0,
		1,
		0,
		0,
		0.0,
		0,
		0.0,
		0,
		load.Id,
		dim.Id,
		connections,
		0,
		0,
		0,
		0,
		float64(dim.PmmCpu),
		float64(dim.ProxyCpu),
		float64(dim.MysqlCpu),
		((dim.MysqlMemory * 1024) * 1024) * 1024,
		((dim.ProxyMemory * 1024) * 1024) * 1024,
		((dim.PmmMemory * 1024) * 1024) * 1024,
		0,
		0,
		1,
	}

	c.reference = &ref

	// set load factors based on the incoming request
	loadConnectionFactor := float32(dim.MysqlCpu) / float32(c.reference.connections)
	if loadConnectionFactor < 1 {
		message.MType = o.OverutilizingI
		return message, true
	}
	c.reference.loadAdjustmentMax = dim.MysqlCpu / 50
	c.reference.loadAdjustment = c.getAdjFactor(loadConnectionFactor)
	c.reference.loadFactor = 1 - c.reference.loadAdjustment
	c.reference.idealBufferPoolDIm = int64(float64(c.reference.memoryMySQL) * 0.65)
	c.reference.gcacheLoad = c.getGcacheLoad()

	var p o.ProviderParam
	c.families = fam
	c.request = r
	c.providerParams = p.Init()

	return message, false
}

func (c *Configurator) ProcessRequest() map[string]o.Family {

	//Start to perform calculation
	// flow:
	// 1 get connections
	// redolog
	// gcache
	//Innodb Bufferpool + params
	// server
	// galera provider

	//probes
	//pxc
	//haproxy

	// connection buffers
	c.getConnectionBuffers()

	// let us do a simple check to see if the number of connections is consuming too many resources.
	conWeight := float64(c.reference.connBuffersMemTot) / c.reference.memoryMySQL
	if conWeight < 0.40 {

		// Innodb Redolog
		c.getInnodbRedolog()
		if c.request.DBType == "pxc" {
			// Gcache
			c.getGcache()
		}

		if c.request.DBType == "group_replication" {
			// GCS cache
			c.getGCScache()
		}

		// Innodb BP and Params
		c.getInnodbParameters()

		// set Server params
		c.getServerParameters()

		if c.request.DBType == "pxc" {
			// set galera provider options
			c.getGaleraParameters()
		}

		// set Probes timeouts
		// MySQL
		// Proxy
		c.getProbesAndResources("mysql")
		c.getProbesAndResources("proxy")
		c.getProbesAndResources("monitor")
	}
	return c.families

}

// calculate gcache effects on memory (estimation)
func (c *Configurator) getGcache() {
	c.reference.gcache = int64(float64(c.reference.innodbRedoLogDim) * c.reference.gcacheLoad)
	c.reference.gcacheFootprint = int64(math.Ceil(float64(c.reference.gcache) * 0.3))
	c.reference.memoryLeftover -= c.reference.gcacheFootprint
}

// TODO WARNING need to add the weight here
func (c *Configurator) getAdjFactor(loadConnectionFactor float32) float32 {
	impedance := loadConnectionFactor / float32(c.reference.loadAdjustmentMax)

	switch c.reference.loadID {
	case 1:
		return impedance
	case 2:
		return impedance
	case 3:
		return impedance
	case 4:
		return impedance
	default:
		return float32(c.reference.loadAdjustmentMax / 1)

	}

}

// processing per connections first
func (c *Configurator) getConnectionBuffers() {

	group := c.families["mysql"].Groups["configuration_connection"]
	group.Parameters["binlog_cache_size"] = c.paramBinlogCacheSize(group.Parameters["binlog_cache_size"])
	group.Parameters["binlog_stmt_cache_size"] = c.paramBinlogCacheSize(group.Parameters["binlog_stmt_cache_size"])
	group.Parameters["join_buffer_size"] = c.paramJoinBuffer(group.Parameters["join_buffer_size"])
	group.Parameters["read_rnd_buffer_size"] = c.paramReadRndBuffer(group.Parameters["read_rnd_buffer_size"])
	group.Parameters["sort_buffer_size"] = c.paramSortBuffer(group.Parameters["sort_buffer_size"])

	c.calculateTmpTableFootprint(group.Parameters["tmp_table_size"])

	// calculate totals and store in references then pass back new values to stored objects
	c.sumConnectionBuffers(group.Parameters)
	c.families["mysql"].Groups["configuration_connection"] = group
}

func (c *Configurator) paramBinlogCacheSize(inParameter o.Parameter) o.Parameter {

	switch c.reference.loadID {
	case 1:
		inParameter.Value = strconv.FormatInt(32768, 10)
	case 2:
		inParameter.Value = strconv.FormatInt(131072, 10)
	case 3:
		inParameter.Value = strconv.FormatInt(262144, 10)
	case 4:
		inParameter.Value = strconv.FormatInt(358400, 10)

	}

	return inParameter
}

func (c *Configurator) paramJoinBuffer(inParameter o.Parameter) o.Parameter {

	switch c.reference.loadID {
	case 1:
		inParameter.Value = strconv.FormatInt(262144, 10)
	case 2:
		inParameter.Value = strconv.FormatInt(524288, 10)
	case 3:
		inParameter.Value = strconv.FormatInt(1048576, 10)
	case 4:
		inParameter.Value = strconv.FormatInt(1048576, 10)

	}

	return inParameter
}

func (c *Configurator) paramReadRndBuffer(inParameter o.Parameter) o.Parameter {
	switch c.reference.loadID {
	case 1:
		inParameter.Value = strconv.FormatInt(262144, 10)
	case 2:
		inParameter.Value = strconv.FormatInt(393216, 10)
	case 3:
		inParameter.Value = strconv.FormatInt(707788, 10)
	case 4:
		inParameter.Value = strconv.FormatInt(707788, 10)

	}

	return inParameter
}

func (c *Configurator) paramSortBuffer(inParameter o.Parameter) o.Parameter {
	switch c.reference.loadID {
	case 1:
		inParameter.Value = strconv.FormatInt(262144, 10)
	case 2:
		inParameter.Value = strconv.FormatInt(524288, 10)
	case 3:
		inParameter.Value = strconv.FormatInt(1572864, 10)
	case 4:
		inParameter.Value = strconv.FormatInt(2097152, 10)

	}

	return inParameter
}

func (c *Configurator) calculateTmpTableFootprint(inParameter o.Parameter) int64 {
	var footPrint = 0
	c.reference.tmpTableFootprint, _ = strconv.ParseInt(inParameter.Value, 10, 64)

	switch c.reference.loadID {
	case 1:
		c.reference.tmpTableFootprint = int64(float64(c.reference.tmpTableFootprint) * 0.03)
	case 2:
		c.reference.tmpTableFootprint = int64(float64(c.reference.tmpTableFootprint) * 0.01)
	case 3:
		c.reference.tmpTableFootprint = int64(float64(c.reference.tmpTableFootprint) * 0.04)
	case 4:
		c.reference.tmpTableFootprint = int64(float64(c.reference.tmpTableFootprint) * 0.05)
	}

	return int64(footPrint)

}

// sum of the memory utilized  by the connections and the estimated cost of temp table
func (c *Configurator) sumConnectionBuffers(params map[string]o.Parameter) {

	totMemory := int64(0)
	for key, param := range params {
		if key != "tmp_table_size" && key != "max_heap_table_size" {
			v, _ := strconv.ParseInt(param.Value, 10, 64)
			totMemory += v
		}
	}

	//once we have the total buffer allocation we calculate the total estimation of the temp table based on the load factor to adjust the connection load
	possibleConnectionTmp := float64(c.reference.connections) * float64(c.reference.loadFactor)
	possibleTmpMemPressure := int64(math.Floor(possibleConnectionTmp)) * c.reference.tmpTableFootprint

	c.reference.connBuffersMemTot = totMemory * int64(possibleConnectionTmp)
	c.reference.connBuffersMemTot += possibleTmpMemPressure

	//update available memory in the references
	c.reference.memoryLeftover = int64(c.reference.memoryMySQL) - c.reference.connBuffersMemTot
	//log.Debug(fmt.Sprintf("Total memory: %d ;  connections memory : %d ; memory leftover: %d", c.reference.memory, c.reference.connBuffersMemTot, c.reference.memoryLeftover))
}

// define global dimension for redolog

func (c *Configurator) getInnodbRedolog() {

	parameter := c.families["mysql"].Groups["configuration_innodb"].Parameters["innodb_log_file_size"]

	c.families["mysql"].Groups["configuration_innodb"].Parameters["innodb_log_file_size"] = c.getRedologDimensionTot(parameter)
}

func (c *Configurator) getRedologDimensionTot(inParameter o.Parameter) o.Parameter {

	var redologTotDimension int64

	switch c.reference.loadID {
	case 1:
		redologTotDimension = int64(float32(c.reference.idealBufferPoolDIm) * (0.15 + (0.15 * c.reference.loadFactor)))
	case 2:
		redologTotDimension = int64(float32(c.reference.idealBufferPoolDIm) * (0.2 + (0.2 * c.reference.loadFactor)))
	case 3:
		redologTotDimension = int64(float32(c.reference.idealBufferPoolDIm) * (0.3 + (0.3 * c.reference.loadFactor)))
	default:
		redologTotDimension = int64(float32(c.reference.idealBufferPoolDIm) * (0.15 + (0.15 * c.reference.loadFactor)))
	}
	// Store in reference the total redolog dimension
	c.reference.innodbRedoLogDim = redologTotDimension

	//Calculate the number of file base on the dimension
	parameter := c.families["mysql"].Groups["configuration_innodb"].Parameters["innodb_log_files_in_group"]
	parameter = c.getRedologfilesNumber(redologTotDimension, parameter)
	c.families["mysql"].Groups["configuration_innodb"].Parameters["innodb_log_files_in_group"] = parameter

	// Calculate the dimension per redolog file base on dimension and number
	a, _ := strconv.ParseInt(parameter.Value, 10, 64)
	inParameter.Value = strconv.FormatInt(redologTotDimension/a, 10)

	return inParameter

}

// calculate the number of file for redolog
func (c *Configurator) getRedologfilesNumber(dimension int64, parameter o.Parameter) o.Parameter {

	// transform redolog dimension into MB
	dimension = int64(math.Ceil((float64(dimension) / 1025) / 1025))

	switch {
	case dimension < 500:
		parameter.Value = "2"
	case dimension > 500 && dimension < 1000:
		if c.reference.loadID == 1 {

			parameter.Value = strconv.FormatFloat(math.Floor(3.0*0.7), 'f', 0, 64)
		} else {
			parameter.Value = "3"
		}

	case dimension > 1001 && dimension < 2000:
		if c.reference.loadID == 1 {

			parameter.Value = strconv.FormatFloat(math.Floor(5.0*0.7), 'f', 0, 64)
		} else {
			parameter.Value = "5"
		}
	case dimension > 2001 && dimension < 4000:
		if c.reference.loadID == 1 {

			parameter.Value = strconv.FormatFloat(math.Floor(8.0*0.7), 'f', 0, 64)
		} else {
			parameter.Value = "8"
		}

	case dimension > 4000:
		parameter.Value = strconv.FormatFloat(math.Floor(float64(dimension)/400), 'f', 0, 64)
	}

	return parameter

}

// adjust the gcache dimension based on the type of load
func (c *Configurator) getGcacheLoad() float64 {
	switch c.reference.loadID {
	case 1:
		return 1
	case 2:
		return 1.15
	case 3:
		return 1.2
	default:
		return 1
	}
}

func (c *Configurator) getInnodbParameters() {
	group := c.families["mysql"].Groups["configuration_innodb"]
	group.Parameters["innodb_adaptive_hash_index"] = c.paramInnoDBAdaptiveHashIndex(group.Parameters["innodb_adaptive_hash_index"])
	group.Parameters["innodb_buffer_pool_size"] = c.paramInnoDBBufferPool(group.Parameters["innodb_buffer_pool_size"])
	group.Parameters["innodb_buffer_pool_instances"] = c.paramInnoDBBufferPoolInstances(group.Parameters["innodb_buffer_pool_instances"])
	group.Parameters["innodb_page_cleaners"] = c.paramInnoDBBufferPoolCleaners(group.Parameters["innodb_buffer_pool_instances"])
	group.Parameters["innodb_purge_threads"] = c.paramInnoDPurgeThreads(group.Parameters["innodb_purge_threads"])
	group.Parameters["innodb_io_capacity_max"] = c.paramInnoDBIOCapacityMax(group.Parameters["innodb_io_capacity_max"])

	group.Parameters["innodb_parallel_read_threads"] = c.paramInnoDBinnodb_parallel_read_threads(group.Parameters["innodb_parallel_read_threads"])

	c.families["mysql"].Groups["configuration_innodb"] = group
}

func (c *Configurator) paramInnoDBAdaptiveHashIndex(parameter o.Parameter) o.Parameter {
	switch c.reference.loadID {
	case 1:
		parameter.Value = "True"
		return parameter
	case 2:
		parameter.Value = "True"
		return parameter
	case 3:
		parameter.Value = "False"
		return parameter
	default:
		parameter.Value = "True"
		return parameter
	}

}

// calculate BP removing from available memory the connections buffers, gcache memory footprint and give a % of additional space
func (c *Configurator) paramInnoDBBufferPool(parameter o.Parameter) o.Parameter {

	var bufferPool int64
	bufferPool = int64(math.Floor(float64(c.reference.memoryLeftover) * 0.95))
	parameter.Value = strconv.FormatInt(bufferPool, 10)
	c.reference.innoDBbpSize = bufferPool
	c.reference.memoryLeftover -= bufferPool
	return parameter
}

// number of instance can only be more than 1 when we have mor ethan 1 core and BP size will allow it
// to avoid too many bp we should not go below 500m dimension
func (c *Configurator) paramInnoDBBufferPoolInstances(parameter o.Parameter) o.Parameter {
	instances := 1
	if c.reference.cpus > 2000 {
		bpSize := float64(((c.reference.innoDBbpSize / 1024) / 1024) / 1024)
		maxCpus := float64(c.reference.cpusMySQL / 1000)

		factor := bpSize / maxCpus

		if factor > 1 {
			instances = int(maxCpus * 2)
		} else if factor < 1 && factor > 0.4 {
			instances = int(maxCpus)
		} else {
			instances = int(math.Ceil(maxCpus / 2))
		}

		parameter.Value = strconv.FormatInt(int64(instances), 10)
	} else {
		parameter.Value = "1"
	}
	c.reference.innoDBBPInstances = instances
	return parameter
}

func (c *Configurator) paramInnoDBBufferPoolCleaners(parameter o.Parameter) o.Parameter {
	parameter.Value = strconv.Itoa(c.reference.innoDBBPInstances)

	return parameter
}

// purge threads should be set on the base of the table involved in parallel DML, here we assume that a load with intense OLTP has more parallel tables involved than the others
// the g cache load factor is the one use to tune
func (c *Configurator) paramInnoDPurgeThreads(parameter o.Parameter) o.Parameter {

	threads := 4
	if (c.reference.cpus / 1000) > 4 {
		valore := float64(c.reference.cpusMySQL/1000) * c.reference.gcacheLoad
		threads = int(math.Ceil(valore))
	}

	if threads > 32 {
		threads = 32
	}

	parameter.Value = strconv.Itoa(threads)

	return parameter
}

// TODO  this must reflect the storage class used which at the moment is not implemented yet
// so what we will do is just to stay out of it and keep  it base of the load
// Advisor thing
func (c *Configurator) paramInnoDBIOCapacityMax(parameter o.Parameter) o.Parameter {
	switch c.reference.loadID {
	case 1:
		parameter.Value = "1400"
		return parameter
	case 2:
		parameter.Value = "1800"
		return parameter
	case 3:
		parameter.Value = "2000"
		return parameter
	default:
		parameter.Value = "1400"
		return parameter
	}

}

func (c *Configurator) getServerParameters() {

	group := c.families["mysql"].Groups["configuration_server"]
	group.Parameters["max_connections"] = c.paramServerMaxConnections(group.Parameters["max_connections"])
	group.Parameters["thread_pool_size"] = c.paramServerThreadPool(group.Parameters["thread_pool_size"])
	group.Parameters["table_definition_cache"] = c.paramServerTableDefinitionCache(group.Parameters["table_definition_cache"])
	group.Parameters["table_open_cache"] = c.paramServerTableOpenCache(group.Parameters["table_open_cache"])
	group.Parameters["thread_stack"] = c.paramServerThreadStack(group.Parameters["thread_stack"])
	group.Parameters["table_open_cache_instances"] = c.paramServerTableOpenCacheInstances(group.Parameters["table_open_cache_instances"])

	c.families["mysql"].Groups["configuration_server"] = group

}

// set max connection + 2 for admin
func (c *Configurator) paramServerMaxConnections(parameter o.Parameter) o.Parameter {

	parameter.Value = strconv.Itoa(c.reference.connections + 2)

	return parameter

}

// about thread pool the default is the number of CPU, but we will try to push a bit more doubling them but never going over the double of the dimension threads
func (c *Configurator) paramServerThreadPool(parameter o.Parameter) o.Parameter {
	threads := 4
	cpus := c.reference.cpusMySQL / 1000

	// we just set some limits to the cpu range
	if cpus > 2 && cpus < 256 {
		threads = int(cpus) * 2
	}

	parameter.Value = strconv.Itoa(threads)

	return parameter
}

// TODO  not supported yet need to be tuned on the base of the schema this can be an advisor thing
func (c *Configurator) paramServerTableDefinitionCache(parameter o.Parameter) o.Parameter {

	return parameter
}

// TODO not supported yet need to be tuned on the base of the schema this can be an advisor thing
func (c *Configurator) paramServerTableOpenCache(parameter o.Parameter) o.Parameter {

	return parameter
}

// TODO not supported yet need to be tuned on the base of the schema this can be an advisor thing
func (c *Configurator) paramServerThreadStack(parameter o.Parameter) o.Parameter {

	return parameter
}

// default is 16, but we have seen that this value is crazy high and create memory overload and a lot of fragmentation Advisor to tune
func (c *Configurator) paramServerTableOpenCacheInstances(parameter o.Parameter) o.Parameter {
	parameter.Value = strconv.Itoa(4)
	return parameter
}

func (c *Configurator) getGaleraProvider(inParameter o.Parameter) o.Parameter {
	for key, param := range c.providerParams {
		if param.Value >= 0 {
			if key != "evs.stats_report_period" {
				param.Value = int64(float32(param.RMax) * c.reference.loadFactor)
			} else {
				param.Value = 1
			}
			c.providerParams[key] = param
		}

	}
	asString := c.GetAllGaleraProviderOptionsAsString()
	inParameter.Value = asString.String()

	return inParameter
}

func (c *Configurator) getGaleraParameters() {
	group := c.families["mysql"].Groups["configuration_galera"]
	group.Parameters["wsrep-provider-options"] = c.getGaleraProvider(group.Parameters["wsrep-provider-options"])
	group.Parameters["wsrep_sync_wait"] = c.getGaleraSyncWait(group.Parameters["wsrep_sync_wait"])
	group.Parameters["wsrep_slave_threads"] = c.getGaleraSlaveThreads(group.Parameters["wsrep_slave_threads"])
	group.Parameters["wsrep_trx_fragment_size"] = c.getGaleraFragmentSize(group.Parameters["wsrep_trx_fragment_size"])

	c.families["mysql"].Groups["configuration_galera"] = group
}

func (c *Configurator) getGaleraSyncWait(parameter o.Parameter) o.Parameter {
	switch c.reference.loadID {
	case 1:
		parameter.Value = "0"
		return parameter
	case 2:
		parameter.Value = "3"
		return parameter
	case 3:
		parameter.Value = "3"
		return parameter
	default:
		parameter.Value = "0"
		return parameter
	}

}

func (c *Configurator) getGaleraSlaveThreads(parameter o.Parameter) o.Parameter {

	cpus := int(math.Floor(float64(c.reference.cpusMySQL / 1000)))

	if cpus <= 1 {
		cpus = 1
	} else {
		cpus = cpus / 2
	}
	parameter.Value = strconv.Itoa(cpus)

	return parameter
}

// TODO this is something to tune with advisors for now let us set a default of 1MB period
func (c *Configurator) getGaleraFragmentSize(parameter o.Parameter) o.Parameter {
	return parameter
}

func (c *Configurator) getProbesAndResources(family string) {
	val := 0

	group := c.families[family].Groups["resources"]
	cpus, memory := c.getResourcesByFamily(family)
	group = c.setResources(group, cpus, memory)
	c.families[family].Groups["resources"] = group

	// setting readiness and liveness
	group = c.families[family].Groups["readinessProbe"]
	parameter := group.Parameters["timeoutSeconds"]
	val = int(math.Ceil(float64(float32(parameter.Max) * c.reference.loadFactor)))
	if val < 1 {
		val = int(parameter.Min)
	}
	parameter.Value = strconv.Itoa(val)
	group.Parameters["timeoutSeconds"] = parameter
	c.families[family].Groups["readinessProbe"] = group

	group = c.families[family].Groups["livenessProbe"]
	val = int(math.Ceil(float64(float32(parameter.Max) * c.reference.loadFactor)))
	if val < 1 {
		val = int(parameter.Min)
	}
	parameter.Value = strconv.Itoa(val)
	group.Parameters["timeoutSeconds"] = parameter
	c.families[family].Groups["livenessProbe"] = group

}

func (c *Configurator) setResources(group o.GroupObj, cpus float64, memory float64) o.GroupObj {
	// we set the memory request as 95% of the available memory and set Limit as 100%
	parameter := group.Parameters["request_memory"]
	parameter.Value = strconv.FormatFloat(float64(memory)*0.95, 'f', 0, 64)
	group.Parameters["request_memory"] = parameter

	parameter = group.Parameters["limit_memory"]
	parameter.Value = strconv.FormatFloat(memory, 'f', 0, 64)
	group.Parameters["limit_memory"] = parameter

	parameter = group.Parameters["request_cpu"]
	parameter.Value = strconv.FormatFloat(float64(cpus)*0.95, 'f', 0, 64)
	group.Parameters["request_cpu"] = parameter

	parameter = group.Parameters["limit_cpu"]
	parameter.Value = strconv.FormatFloat(cpus, 'f', 0, 64)
	group.Parameters["limit_cpu"] = parameter

	return group
}

// EvaluateResources here we give a basic check about the resources and if is over we just set the message as overload and remove the families details
func (c *Configurator) EvaluateResources(responseMsg o.ResponseMessage) (o.ResponseMessage, bool) {
	totMeme := c.reference.memory
	reqConnections := c.reference.connections
	reqCpu := c.reference.cpus

	gcacheFootPrint := c.reference.gcacheFootprint
	temTableFootprint := c.reference.tmpTableFootprint
	connectionMem := c.reference.connBuffersMemTot
	memLeftOver := c.reference.memoryLeftover

	var b bytes.Buffer
	b.WriteString("\n\nTot Memory          = " + strconv.FormatFloat(totMeme, 'f', 0, 64) + "\n")
	b.WriteString("Tot CPU                 = " + strconv.Itoa(reqCpu) + "\n")
	b.WriteString("Tot Connections         = " + strconv.Itoa(reqConnections) + "\n")
	b.WriteString("\n")
	b.WriteString("memory assign to mysql  = " + strconv.FormatFloat(c.reference.memoryMySQL, 'f', 0, 64) + "\n")
	b.WriteString("memory assign to Proxy  = " + strconv.FormatFloat(c.reference.memoryProxy, 'f', 0, 64) + "\n")
	b.WriteString("memory assign to Monitor= " + strconv.FormatFloat(c.reference.memoryPmm, 'f', 0, 64) + "\n")
	b.WriteString("cpus assign to mysql  = " + strconv.FormatFloat(c.reference.cpusMySQL, 'f', 0, 64) + "\n")
	b.WriteString("cpus assign to Proxy  = " + strconv.FormatFloat(c.reference.cpusProxy, 'f', 0, 64) + "\n")
	b.WriteString("cpus assign to Monitor= " + strconv.FormatFloat(c.reference.cpusPmm, 'f', 0, 64) + "\n")
	b.WriteString("\n")
	if c.request.DBType == "pxc" {
		b.WriteString("Gcache mem on disk      = " + strconv.FormatInt(c.reference.gcache, 10) + "\n")
		b.WriteString("Gcache mem Footprint    = " + strconv.FormatInt(gcacheFootPrint, 10) + "\n")
		b.WriteString("\n")
	}
	b.WriteString("Tmp Table mem Footprint = " + strconv.FormatInt(temTableFootprint, 10) + "\n")
	b.WriteString("By connection mem tot   = " + strconv.FormatInt(connectionMem, 10) + "\n")
	b.WriteString("\n")
	b.WriteString("Innodb Bufferpool       = " + strconv.FormatInt(c.reference.innoDBbpSize, 10) + "\n")
	bpPct := float64(c.reference.innoDBbpSize) / totMeme
	b.WriteString("% BP over av memory     = " + strconv.FormatFloat(bpPct, 'f', 2, 64) + "\n")
	b.WriteString("\n")
	b.WriteString("memory leftover         = " + strconv.FormatInt(memLeftOver, 10) + "\n")
	b.WriteString("\n")

	return fillResponseMessage(bpPct, responseMsg, b)

}

func (c *Configurator) getResourcesByFamily(family string) (float64, float64) {
	cpus := 0.0
	memory := 0.0

	switch family {
	case "mysql":
		cpus = c.reference.cpusMySQL
		memory = c.reference.memoryMySQL
	case "proxy":
		cpus = c.reference.cpusPmm
		memory = c.reference.memoryProxy
	case "monitor":
		cpus = c.reference.cpusPmm
		memory = c.reference.memoryPmm
	}

	return cpus, memory
}

// We assign value for parallel read of clustered index equal to the number of virtual cpu available for MySQL
func (c *Configurator) paramInnoDBinnodb_parallel_read_threads(parameter o.Parameter) o.Parameter {
	threads := 1
	cpus := c.reference.cpusMySQL / 1000

	// we just set some limits to the cpu range
	if cpus > 2 && cpus < 256 {
		threads = int(cpus)
	}

	parameter.Value = strconv.Itoa(threads)

	return parameter
}

// We calculate the dimension of the GCS keeping it as low as possible to prevent OOM Kill
func (c *Configurator) getGCScache() {
	//c.reference.gcache = int64(float64(c.reference.innodbRedoLogDim) * c.reference.gcacheLoad)
	//c.reference.gcacheFootprint = int64(math.Ceil(float64(c.reference.gcache) * 0.3))
	//c.reference.memoryLeftover -= c.reference.gcacheFootprint

}

func fillResponseMessage(pct float64, msg o.ResponseMessage, b bytes.Buffer) (o.ResponseMessage, bool) {
	overUtilizing := false
	if pct < 0.50 {
		msg.MType = o.OverutilizingI
		msg.MText = "Request cancelled not enough resources details: " + b.String()
		msg.MName = msg.GetMessageText(msg.MType)
		overUtilizing = true
	} else if pct > 0.50 && pct <= 0.65 {
		msg.MType = o.ClosetolimitI
		msg.MText = "Request processed however not optimal details: " + b.String()
		msg.MName = msg.GetMessageText(msg.MType)
	} else if pct > 0.66 {
		msg.MType = o.OkI
		msg.MText = "Request ok, resources details: " + b.String()
		msg.MName = msg.GetMessageText(msg.MType)
	}

	return msg, overUtilizing
}
