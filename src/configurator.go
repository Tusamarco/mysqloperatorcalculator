package main

import (
	"bytes"
	"fmt"
	log "github.com/sirupsen/logrus"
	"math"
	o "pxccalculator/src/Objects"
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
	memory             int64   //total memory available
	cpus               int     //total cpus
	gcache             int64   // assigned gcache dimension
	gcacheFootprint    int64   // expected file footprint in memory
	gcacheLoad         float64 // gcache load adj factor base don type of load
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
	innoDBBPInstances  int     //  assignied number of BP
}

// return all provider option considered as single string for the parameter value
func (c *Configurator) GetAllGaleraProviderOptionsAsString() bytes.Buffer {

	var b bytes.Buffer
	b.WriteString(`"`)

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
	b.WriteString(`"`)
	return b
}

func (c *Configurator) init(r o.ConfigurationRequest, fam map[string]o.Family, conf o.Configuration) {

	dim := conf.GetDimensionByID(r.Dimension.Id)
	load := conf.GetLoadByID(r.LoadType.Id)
	if load.Id == 0 || dim.Id == 0 {
		log.Warning(fmt.Sprintf("Invalid load %d or Dimension %d detected ", load.Id, dim.Id))
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
		r.Connections,
		0,
		0,
		0,
		0}

	c.reference = &ref

	// set load factors based on the incoming request
	c.reference.loadAdjustmentMax = (dim.Cpu / r.Connections)
	c.reference.loadAdjustment = c.getAdjFactor()
	c.reference.loadFactor = (1 - (c.reference.loadAdjustment / float32(c.reference.loadAdjustmentMax)))
	c.reference.idealBufferPoolDIm = int64(float64(c.reference.memory) * 0.65)
	c.reference.gcacheLoad = c.getGcacheLoad()

	var p o.ProviderParam
	c.families = fam
	c.request = r
	c.providerParams = p.Init()

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

	// connection buffers
	c.getConnectionBuffers()

	// Innodb Redolog
	c.getInnodbRedolog()

	// Gcache
	c.reference.gcache = int64(float64(c.reference.innodbRedoLogDim) * c.reference.gcacheLoad)
	c.reference.gcacheFootprint = int64(math.Ceil(float64(c.reference.gcache) * 0.3))

	// Innodb BP and Params
	c.getInnodbParmameters()

	// set Server params
	c.getServerParameters()

	b := c.GetAllGaleraProviderOptionsAsString()

	print(b.String())

	return c.families

}

func (c *Configurator) getAdjFactor() float32 {
	switch c.reference.loadID {
	case 1:
		return float32(c.reference.loadAdjustmentMax / 1)
	case 2:
		return float32(c.reference.loadAdjustmentMax / 2)
	case 3:
		return float32(c.reference.loadAdjustmentMax / 3)
	case 4:
		return (float32(c.reference.loadAdjustmentMax) / 3.4)
	default:
		return float32(c.reference.loadAdjustmentMax / 1)

	}

}

func (c *Configurator) checkValidity() bool {

	return false
}

//processing per connections first
func (c *Configurator) getConnectionBuffers() {

	group := c.families["pxc"].Groups["configuration_connection"]
	group.Parameters["binlog_cache_size"] = c.paramBinlogCacheSize(group.Parameters["binlog_cache_size"])
	group.Parameters["binlog_stmt_cache_size"] = c.paramBinlogCacheSize(group.Parameters["binlog_stmt_cache_size"])
	group.Parameters["join_buffer_size"] = c.paramJoinBuffer(group.Parameters["join_buffer_size"])
	group.Parameters["read_rnd_buffer_size"] = c.paramReadRndBuffer(group.Parameters["read_rnd_buffer_size"])
	group.Parameters["sort_buffer_size"] = c.paramSortBuffer(group.Parameters["sort_buffer_size"])

	c.calculateTmpTableFootprint(group.Parameters["tmp_table_size"])

	// calculate totals and store in references then pass back new values to stored objects
	c.sumConnectionBuffers(group.Parameters)
	c.families["pxc"].Groups["configuration_connection"] = group
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
	c.reference.connBuffersMemTot = (totMemory + c.reference.tmpTableFootprint) * int64(c.reference.connections)

	//update available memory in the references
	c.reference.memoryLeftover = (c.reference.memory - c.reference.connBuffersMemTot)
	log.Debug(fmt.Sprintf("Total memory: %d ;  connections memory : %d ; memory leftover: %d", c.reference.memory, c.reference.connBuffersMemTot, c.reference.memoryLeftover))
}

// define global dimension for redolog

func (c *Configurator) getInnodbRedolog() {

	parameter := c.families["pxc"].Groups["configuration_innodb"].Parameters["innodb_log_file_size"]

	c.families["pxc"].Groups["configuration_innodb"].Parameters["innodb_log_file_size"] = c.getRedologDimensionTot(parameter)
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
	parameter := c.families["pxc"].Groups["configuration_innodb"].Parameters["innodb_log_files_in_group"]
	parameter = c.getRedologfilesNumber(redologTotDimension, parameter)
	c.families["pxc"].Groups["configuration_innodb"].Parameters["innodb_log_files_in_group"] = parameter

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

//adjust the gcache dimension based on the type of load
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

func (c *Configurator) getInnodbParmameters() {
	group := c.families["pxc"].Groups["configuration_innodb"]
	group.Parameters["innodb_adaptive_hash_index"] = c.paramInnoDBAdaptiveHashIndex(group.Parameters["innodb_adaptive_hash_index"])
	group.Parameters["innodb_buffer_pool_size"] = c.paramInnoDBBufferPool(group.Parameters["innodb_buffer_pool_size"])
	group.Parameters["innodb_buffer_pool_instances"] = c.paramInnoDBBufferPoolInstances(group.Parameters["innodb_buffer_pool_instances"])
	group.Parameters["innodb_page_cleaners"] = c.paramInnoDBBufferPoolCleaners(group.Parameters["innodb_buffer_pool_instances"])
	group.Parameters["innodb_purge_threads"] = c.paramInnoDBpurgeThreads(group.Parameters["innodb_purge_threads"])
	group.Parameters["innodb_io_capacity_max"] = c.paramInnoDBIOCapacityMax(group.Parameters["innodb_io_capacity_max"])

	c.families["pxc"].Groups["configuration_innodb"] = group
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

	return parameter

}

// calculate BP removing from available memory the connections buffers, gcache memory footprint and give a % of additional space
func (c *Configurator) paramInnoDBBufferPool(parameter o.Parameter) o.Parameter {

	var bufferPool int64
	bufferPool = int64(math.Floor(float64(c.reference.memory-(c.reference.connBuffersMemTot+c.reference.gcacheFootprint)) * 0.9))
	parameter.Value = strconv.FormatInt(bufferPool, 10)
	c.reference.innoDBbpSize = bufferPool
	return parameter
}

// number of instance can only be more than 1 when we have mor ethan 1 core and BP size will allow it
// to avoid too many bp we should not go below 500m dimension
func (c *Configurator) paramInnoDBBufferPoolInstances(parameter o.Parameter) o.Parameter {
	instances := 1
	if c.reference.cpus > 2000 {
		bpSize := float64(((c.reference.innoDBbpSize / 1024) / 1024) / 1024)
		maxCpus := float64(c.reference.cpus / 1000)

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
func (c *Configurator) paramInnoDBpurgeThreads(parameter o.Parameter) o.Parameter {

	threads := 4
	if (c.reference.cpus / 1000) > 4 {
		valore := float64(c.reference.cpus/1000) * float64(c.reference.gcacheLoad)
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

	return parameter

}

func (c *Configurator) getServerParameters() {

	group := c.families["pxc"].Groups["configuration_server"]
	group.Parameters["max_connections"] = c.paramServerMaxConnections(group.Parameters["max_connections"])
	group.Parameters["thread_pool_size"] = c.paramServerThreadPool(group.Parameters["thread_pool_size"])
	group.Parameters["table_definition_cache"] = c.paramServerTableDefinitionCache(group.Parameters["table_definition_cache"])
	group.Parameters["table_open_cache"] = c.paramServerTableOpenCache(group.Parameters["table_open_cache"])
	group.Parameters["thread_stack"] = c.paramServerThreadStack(group.Parameters["thread_stack"])
	group.Parameters["table_open_cache_instances"] = c.paramServerTableOpenCacheInstaces(group.Parameters["table_open_cache_instances"])

	c.families["pxc"].Groups["configuration_server"] = group

}

// set max connection + 2 for admin
func (c *Configurator) paramServerMaxConnections(parameter o.Parameter) o.Parameter {

	parameter.Value = strconv.Itoa(c.reference.connections + 2)

	return parameter

}

// about thread pool the default is the number of CPU but we will try to push a bit more doubling them but never going over the double of the dimension threads
func (c *Configurator) paramServerThreadPool(parameter o.Parameter) o.Parameter {
	threads := 4
	cpus := c.reference.cpus / 1000

	// we just set some limits to the cpu range
	if cpus > 2 && cpus < 256 {
		threads = cpus * 2
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

// default is 16 but we have seen that this value is crazy high and create memory overload and a lot of fragmentation Advisor to tune)
func (c *Configurator) paramServerTableOpenCacheInstaces(parameter o.Parameter) o.Parameter {
	parameter.Value = strconv.Itoa(4)
	return parameter
}
