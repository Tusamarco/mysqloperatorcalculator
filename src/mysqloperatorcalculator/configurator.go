package mysqloperatorcalculator

import (
	"bytes"
	"fmt"
	"github.com/hashicorp/go-version"
	log "github.com/sirupsen/logrus"
	"math"
	"strconv"
)

type Configurator struct {
	request            ConfigurationRequest
	families           map[string]Family
	providerParams     map[string]ProviderParam
	reference          *references
	connectionResearch bool
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

func (c *Configurator) Init(r ConfigurationRequest, fam map[string]Family, conf Configuration, message ResponseMessage) (ResponseMessage, bool) {

	//if dimension is custom we take it from request otherwise from Configuration
	var dim Dimension
	if r.Dimension.Id != DimensionOpen {
		dim = conf.GetDimensionByID(r.Dimension.Id)
	} else {
		dim = r.Dimension
	}
	load := conf.GetLoadByID(r.LoadType.Id)
	if load.Id == 0 || dim.Id == 0 {
		log.Warning(fmt.Sprintf("Invalid load %d or Dimension %d detected ", load.Id, dim.Id))
	}
	connections := r.Connections
	if connections < MinConnectionNumber && connections != 0 {
		connections = MinConnectionNumber
	}

	ref := references{
		dim.MemoryBytes,
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
		dim.MysqlMemory,
		dim.ProxyMemory,
		dim.PmmMemory,
		0,
		0,
		1,
	}

	c.reference = &ref

	// set load factors based on the incoming request
	// we first decide how many cycles want by cpu and then calculate the pressure
	c.reference.loadAdjustmentMax = dim.MysqlCpu / CpuConncetionMillFactor
	loadConnectionFactor := float32(c.reference.connections) / float32(c.reference.loadAdjustmentMax)
	if loadConnectionFactor >= 1 {
		message.MType = OverutilizingI
		return message, true
	}

	//c.reference.loadAdjustment = c.getAdjFactor(loadConnectionFactor)
	//c.reference.loadFactor = 1 - c.reference.loadAdjustment
	//c.reference.loadAdjustment = loadConnectionFactor
	c.reference.loadFactor = loadConnectionFactor
	c.reference.idealBufferPoolDIm = int64(float64(c.reference.memoryMySQL) * 0.65)
	c.reference.gcacheLoad = c.getGcacheLoad()

	var p ProviderParam
	c.families = fam
	c.request = r
	c.providerParams = p.Init()

	return message, false
}

func (c *Configurator) ProcessRequest() map[string]Family {

	//Start to perform calculation
	// flow:
	// 1 get connections
	// redolog
	// gcache or GCS Cache
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
	if conWeight < ConnectionWeighPctLimit {

		// Innodb Redolog
		c.getInnodbRedolog()
		if c.request.DBType == "pxc" {
			// Gcache
			c.getGcache()
		}

		if c.request.DBType == "group_replication" {
			// GCS cache
			group := c.families["mysql"].Groups["configuration_groupReplication"]
			group.Parameters["loose_group_replication_message_cache_size"] = c.getGCScache(group.Parameters["loose_group_replication_message_cache_size"])
			c.families["mysql"].Groups["configuration_groupReplication"] = group
		}

		// Innodb BP and Params
		c.getInnodbParameters()

		// set Server params
		c.getServerParameters()

		if c.request.DBType == "pxc" {
			// set galera provider options
			c.getGaleraParameters()
		}

		//We set Group replication after we have ste the MySQL values, to tune them better
		if c.request.DBType == "group_replication" {
			c.getGroupReplicationParameters()
		}

		// set Probes timeouts
		// MySQL
		// Proxy
		c.getProbesAndResources(FamilyTypeMysql)
		c.getProbesAndResources(FamilyTypeProxy)
		c.getProbesAndResources(FamilyTypeMonitor)
	}
	return c.filterByMySQLVersion()

}

// Filter out the parameter based on mysql version
func (c *Configurator) filterByMySQLVersion() map[string]Family {

	incomingV, _ := version.NewVersion(strconv.Itoa(c.request.Mysqlversion.Major) +
		"." + strconv.Itoa(c.request.Mysqlversion.Minor) +
		"." + strconv.Itoa(c.request.Mysqlversion.Patch))

	//iterate cross all parameters
	for _, l1Val := range c.families {
		//print("Processing L1 " + l1Key)
		for _, l2Val := range l1Val.Groups {
			//println("Processing L2" + l2Key)
			for l3Key, l3Val := range l2Val.Parameters {
				paramVmin, _ := version.NewVersion(strconv.Itoa(l3Val.Mysqlversions.Min.Major) +
					"." + strconv.Itoa(l3Val.Mysqlversions.Min.Minor) +
					"." + strconv.Itoa(l3Val.Mysqlversions.Min.Patch))
				paramVmax, _ := version.NewVersion(strconv.Itoa(l3Val.Mysqlversions.Max.Major) +
					"." + strconv.Itoa(l3Val.Mysqlversions.Max.Minor) +
					"." + strconv.Itoa(l3Val.Mysqlversions.Max.Patch))

				//We identify the parameters that have a valid mysql version, only them will be processed
				if l3Val.Mysqlversions.Min.Major > 0 {
					if incomingV.GreaterThanOrEqual(paramVmin) && incomingV.LessThanOrEqual(paramVmax) {
						//println(l3Val.Name + " = " + l3Val.Value)
					} else {
						//if the version do not fits in the window define the parameter is removed from the list of the returned
						delete(l2Val.Parameters, l3Key)
					}

				}
			}
		}
	}

	return c.families
}

// calculate gcache effects on memory (estimation)
func (c *Configurator) getGcache() {
	c.reference.gcache = int64(float64(c.reference.innodbRedoLogDim) * c.reference.gcacheLoad)
	c.reference.gcacheFootprint = int64(math.Ceil(float64(c.reference.gcache) * 0.3))
	c.reference.memoryLeftover -= c.reference.gcacheFootprint
}

// TODO Thinking...
// For the moment i have disabled this global adjustment method and preferred to apply the tuning by case.
// this because we cannot use the same weight in case of READ operation or Write operation in the different moment of the execution
// What I mean here is that a write can be less expensive than complex read and as such the ADJ factor based on the available cpu cycles needs to take that in consideration
// Unfortunately from were we are this is not possible to do. However it can become a dynamic parameter tuned by observation using tools such as Advisors.
func (c *Configurator) getAdjFactor(loadConnectionFactor float32) float32 {
	impedance := loadConnectionFactor / float32(c.reference.loadAdjustmentMax)

	switch c.reference.loadID {
	case LoadTypeMostlyReads:
		return impedance
	case LoadTypeSomeWrites:
		return impedance
	case LoadTypeEqualReadsWrites:
		return impedance
	case LoadTypeHeavyWrites:
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

func (c *Configurator) paramBinlogCacheSize(inParameter Parameter) Parameter {

	switch c.reference.loadID {
	case LoadTypeMostlyReads:
		inParameter.Value = strconv.FormatInt(32768, 10)
	case LoadTypeSomeWrites:
		inParameter.Value = strconv.FormatInt(131072, 10)
	case LoadTypeEqualReadsWrites:
		inParameter.Value = strconv.FormatInt(262144, 10)
	case LoadTypeHeavyWrites:
		inParameter.Value = strconv.FormatInt(358400, 10)

	}

	return inParameter
}

func (c *Configurator) paramJoinBuffer(inParameter Parameter) Parameter {

	switch c.reference.loadID {
	case LoadTypeMostlyReads:
		inParameter.Value = strconv.FormatInt(262144, 10)
	case LoadTypeSomeWrites:
		inParameter.Value = strconv.FormatInt(524288, 10)
	case LoadTypeEqualReadsWrites:
		inParameter.Value = strconv.FormatInt(1048576, 10)
	case LoadTypeHeavyWrites:
		inParameter.Value = strconv.FormatInt(1048576, 10)

	}

	return inParameter
}

func (c *Configurator) paramReadRndBuffer(inParameter Parameter) Parameter {
	switch c.reference.loadID {
	case LoadTypeMostlyReads:
		inParameter.Value = strconv.FormatInt(262144, 10)
	case LoadTypeSomeWrites:
		inParameter.Value = strconv.FormatInt(393216, 10)
	case LoadTypeEqualReadsWrites:
		inParameter.Value = strconv.FormatInt(707788, 10)
	case LoadTypeHeavyWrites:
		inParameter.Value = strconv.FormatInt(707788, 10)

	}

	return inParameter
}

func (c *Configurator) paramSortBuffer(inParameter Parameter) Parameter {
	switch c.reference.loadID {
	case LoadTypeMostlyReads:
		inParameter.Value = strconv.FormatInt(262144, 10)
	case LoadTypeSomeWrites:
		inParameter.Value = strconv.FormatInt(524288, 10)
	case LoadTypeEqualReadsWrites:
		inParameter.Value = strconv.FormatInt(1572864, 10)
	case LoadTypeHeavyWrites:
		inParameter.Value = strconv.FormatInt(2097152, 10)

	}

	return inParameter
}

func (c *Configurator) calculateTmpTableFootprint(inParameter Parameter) int64 {
	var footPrint = 0
	c.reference.tmpTableFootprint, _ = strconv.ParseInt(inParameter.Value, 10, 64)

	switch c.reference.loadID {
	case LoadTypeMostlyReads:
		c.reference.tmpTableFootprint = int64(float64(c.reference.tmpTableFootprint) * 0.03)
	case LoadTypeSomeWrites:
		c.reference.tmpTableFootprint = int64(float64(c.reference.tmpTableFootprint) * 0.01)
	case LoadTypeEqualReadsWrites:
		c.reference.tmpTableFootprint = int64(float64(c.reference.tmpTableFootprint) * 0.04)
	case LoadTypeHeavyWrites:
		c.reference.tmpTableFootprint = int64(float64(c.reference.tmpTableFootprint) * 0.05)
	}

	return int64(footPrint)

}

// sum of the memory utilized  by the connections and the estimated cost of temp table
func (c *Configurator) sumConnectionBuffers(params map[string]Parameter) {

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

func (c *Configurator) getRedologDimensionTot(inParameter Parameter) Parameter {

	var redologTotDimension int64

	switch c.reference.loadID {
	case LoadTypeMostlyReads:
		redologTotDimension = int64(float32(c.reference.idealBufferPoolDIm) * (0.15 + (0.15 * c.reference.loadFactor)))
	case LoadTypeSomeWrites:
		redologTotDimension = int64(float32(c.reference.idealBufferPoolDIm) * (0.2 + (0.2 * c.reference.loadFactor)))
	case LoadTypeEqualReadsWrites:
		redologTotDimension = int64(float32(c.reference.idealBufferPoolDIm) * (0.3 + (0.3 * c.reference.loadFactor)))
	default:
		redologTotDimension = int64(float32(c.reference.idealBufferPoolDIm) * (0.15 + (0.15 * c.reference.loadFactor)))
	}
	// Store in reference the total redolog dimension
	c.reference.innodbRedoLogDim = redologTotDimension
	// we set the redolog capacity
	parameterIbC := c.families["mysql"].Groups["configuration_innodb"].Parameters["innodb_redo_log_capacity"]
	parameterIbC.Value = strconv.FormatInt(c.reference.innodbRedoLogDim, 10)
	c.families["mysql"].Groups["configuration_innodb"].Parameters["innodb_redo_log_capacity"] = parameterIbC

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
func (c *Configurator) getRedologfilesNumber(dimension int64, parameter Parameter) Parameter {

	// transform redolog dimension into MB
	dimension = int64(math.Ceil((float64(dimension) / 1024) / 1024))

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
	case LoadTypeMostlyReads:
		return 1
	case LoadTypeSomeWrites:
		return 1.15
	case LoadTypeEqualReadsWrites:
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

func (c *Configurator) getGroupReplicationParameters() {
	group := c.families["mysql"].Groups["configuration_groupReplication"]
	group.Parameters["loose_group_replication_member_expel_timeout"] = c.paramGroupReplicationMemberExpelTimeout(group.Parameters["loose_group_replication_member_expel_timeout"])
	group.Parameters["loose_group_replication_autorejoin_tries"] = c.paramGroupReplicationAutorejoinTries(group.Parameters["loose_group_replication_autorejoin_tries"])
	group.Parameters["loose_group_replication_communication_max_message_size"] = c.paramGroupReplicationMessageCacheSize(group.Parameters["loose_group_replication_communication_max_message_size"])
	//	group.Parameters["loose_group_replication_unreachable_majority_timeout"] = c.paramGroupReplicationUnreachableMajorityTimeout(group.Parameters["loose_group_replication_unreachable_majority_timeout"])
	group.Parameters["loose_group_replication_poll_spin_loops"] = c.paramGroupReplicationPollSpinLoops(group.Parameters["loose_group_replication_poll_spin_loops"])
	//group.Parameters["loose_group_replication_compression_threshold"] = c.paramGroupReplicationCompressionThreshold(group.Parameters["loose_group_replication_compression_threshold"])

	c.families["mysql"].Groups["configuration_groupReplication"] = group
}

func (c *Configurator) paramInnoDBAdaptiveHashIndex(parameter Parameter) Parameter {
	switch c.reference.loadID {
	case LoadTypeMostlyReads:
		parameter.Value = "True"
		return parameter
	case LoadTypeSomeWrites:
		parameter.Value = "True"
		return parameter
	case LoadTypeEqualReadsWrites:
		parameter.Value = "False"
		return parameter
	default:
		parameter.Value = "True"
		return parameter
	}

}

// calculate BP removing from available memory the connections buffers, gcache memory footprint and give a % of additional space
func (c *Configurator) paramInnoDBBufferPool(parameter Parameter) Parameter {

	var bufferPool int64
	bufferPool = int64(math.Floor(float64(c.reference.memoryLeftover) * 0.95))
	parameter.Value = strconv.FormatInt(bufferPool, 10)
	c.reference.innoDBbpSize = bufferPool
	c.reference.memoryLeftover -= bufferPool
	return parameter
}

// number of instance can only be more than 1 when we have mor ethan 1 core and BP size will allow it
// to avoid too many bp we should not go below 500m dimension
func (c *Configurator) paramInnoDBBufferPoolInstances(parameter Parameter) Parameter {
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

func (c *Configurator) paramInnoDBBufferPoolCleaners(parameter Parameter) Parameter {
	parameter.Value = strconv.Itoa(c.reference.innoDBBPInstances)

	return parameter
}

// purge threads should be set on the base of the table involved in parallel DML, here we assume that a load with intense OLTP has more parallel tables involved than the others
// the g cache load factor is the one use to tune
func (c *Configurator) paramInnoDPurgeThreads(parameter Parameter) Parameter {

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
func (c *Configurator) paramInnoDBIOCapacityMax(parameter Parameter) Parameter {
	switch c.reference.loadID {
	case LoadTypeMostlyReads:
		parameter.Value = "1400"
		return parameter
	case LoadTypeSomeWrites:
		parameter.Value = "1800"
		return parameter
	case LoadTypeEqualReadsWrites:
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
	group.Parameters["thread_cache_size"] = c.paramServerThreadCacheSize(group.Parameters["thread_cache_size"])

	c.families["mysql"].Groups["configuration_server"] = group

}

// set max connection + 2 for admin
func (c *Configurator) paramServerMaxConnections(parameter Parameter) Parameter {

	parameter.Value = strconv.Itoa(c.reference.connections + 2)

	return parameter

}

// about thread pool the default is the number of CPU, but we will try to push a bit more doubling them but never going over the double of the dimension threads
func (c *Configurator) paramServerThreadPool(parameter Parameter) Parameter {
	threads := 4
	cpus := c.reference.cpusMySQL / 1000

	// we just set some limits to the cpu range
	if cpus > 2 && cpus <= 256 {
		threads = int(cpus) * 2
	}

	parameter.Value = strconv.Itoa(threads)

	return parameter
}

// TODO  not supported yet need to be tuned on the base of the schema this can be an advisor thing
func (c *Configurator) paramServerTableDefinitionCache(parameter Parameter) Parameter {

	return parameter
}

// TODO not supported yet need to be tuned on the base of the schema this can be an advisor thing
func (c *Configurator) paramServerTableOpenCache(parameter Parameter) Parameter {

	return parameter
}

// TODO not supported yet need to be tuned on the base of the schema this can be an advisor thing
func (c *Configurator) paramServerThreadStack(parameter Parameter) Parameter {

	return parameter
}

// default is 16, but we have seen that this value is crazy high and create memory overload and a lot of fragmentation Advisor to tune
func (c *Configurator) paramServerTableOpenCacheInstances(parameter Parameter) Parameter {
	parameter.Value = strconv.Itoa(4)
	return parameter
}

func (c *Configurator) getGaleraProvider(inParameter Parameter) Parameter {
	for key, param := range c.providerParams {

		switch key {
		case "gcache.size":
			param.Value = c.reference.gcache
		case "evs.stats_report_period":
			param.Value = 1
		default:
			if param.Value >= 0 && c.reference.loadFactor > 0 {
				param.Value = int64(float32(param.RMax) * c.reference.loadFactor)
			} else if param.Value >= 0 {
				param.Value = param.Defvalue
			}

		}
		c.providerParams[key] = param

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

func (c *Configurator) getGaleraSyncWait(parameter Parameter) Parameter {
	switch c.reference.loadID {
	case LoadTypeMostlyReads:
		parameter.Value = "0"
		return parameter
	case LoadTypeSomeWrites:
		parameter.Value = "3"
		return parameter
	case LoadTypeEqualReadsWrites:
		parameter.Value = "3"
		return parameter
	default:
		parameter.Value = "0"
		return parameter
	}

}

func (c *Configurator) getGaleraSlaveThreads(parameter Parameter) Parameter {

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
func (c *Configurator) getGaleraFragmentSize(parameter Parameter) Parameter {
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

func (c *Configurator) setResources(group GroupObj, cpus float64, memory float64) GroupObj {
	// we set the memory request as 95% of the available memory and set Limit as 100%
	parameter := group.Parameters["request_memory"]
	parameter.Value = strconv.FormatFloat(float64(memory)*0.95, 'f', 0, 64)
	group.Parameters["request_memory"] = parameter

	parameter = group.Parameters["limit_memory"]
	parameter.Value = strconv.FormatFloat(memory, 'f', 0, 64)
	group.Parameters["limit_memory"] = parameter

	parameter = group.Parameters["request_cpu"]
	parameter.Value = strconv.FormatFloat(float64(cpus)*0.95, 'f', 0, 64) + "m"
	group.Parameters["request_cpu"] = parameter

	parameter = group.Parameters["limit_cpu"]
	parameter.Value = strconv.FormatFloat(cpus, 'f', 0, 64) + "m"
	group.Parameters["limit_cpu"] = parameter

	return group
}

// EvaluateResources here we give a basic check about the resources and if is over we just set the message as overload and remove the families details
func (c *Configurator) EvaluateResources(responseMsg ResponseMessage) (ResponseMessage, bool) {
	totMeme := c.reference.memory
	reqConnections := c.reference.connections
	reqCpu := c.reference.cpus

	gcacheFootPrint := c.reference.gcacheFootprint
	temTableFootprint := c.reference.tmpTableFootprint
	connectionMem := c.reference.connBuffersMemTot
	memLeftOver := c.reference.memoryLeftover

	var b bytes.Buffer
	b.WriteString("\n\nTot Memory Bytes    = " + strconv.FormatFloat(totMeme, 'f', 0, 64) + "\n")
	b.WriteString("Tot CPU                 = " + strconv.Itoa(reqCpu) + "\n")
	b.WriteString("Tot Connections         = " + strconv.Itoa(reqConnections) + "\n")
	b.WriteString("\n")
	b.WriteString("memory assign to mysql Bytes   = " + strconv.FormatFloat(c.reference.memoryMySQL, 'f', 0, 64) + "\n")
	b.WriteString("memory assign to Proxy Bytes   = " + strconv.FormatFloat(c.reference.memoryProxy, 'f', 0, 64) + "\n")
	b.WriteString("memory assign to Monitor Bytes = " + strconv.FormatFloat(c.reference.memoryPmm, 'f', 0, 64) + "\n")
	b.WriteString("cpus assign to mysql  = " + strconv.FormatFloat(c.reference.cpusMySQL, 'f', 0, 64) + "\n")
	b.WriteString("cpus assign to Proxy  = " + strconv.FormatFloat(c.reference.cpusProxy, 'f', 0, 64) + "\n")
	b.WriteString("cpus assign to Monitor= " + strconv.FormatFloat(c.reference.cpusPmm, 'f', 0, 64) + "\n")
	b.WriteString("\n")
	if c.request.DBType == "pxc" {
		b.WriteString("Gcache mem on disk      = " + strconv.FormatInt(c.reference.gcache, 10) + "\n")
		b.WriteString("Gcache mem Footprint    = " + strconv.FormatInt(gcacheFootPrint, 10) + "\n")
		b.WriteString("\n")
	}

	if c.request.DBType == "group_replication" {
		b.WriteString("GCS cache mem limit      = " + strconv.FormatInt(c.reference.gcscache, 10) + "\n")
		b.WriteString("GCS cache mem possible Footprint    = " + strconv.FormatInt(c.reference.gcscacheFootprint, 10) + "\n")
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

	return fillResponseMessage(bpPct, responseMsg, b, c.request.DBType)

}

func (c *Configurator) getResourcesByFamily(family string) (float64, float64) {
	cpus := 0.0
	memory := 0.0

	switch family {
	case FamilyTypeMysql:
		cpus = c.reference.cpusMySQL
		memory = c.reference.memoryMySQL
	case FamilyTypeProxy:
		cpus = c.reference.cpusPmm
		memory = c.reference.memoryProxy
	case FamilyTypeMonitor:
		cpus = c.reference.cpusPmm
		memory = c.reference.memoryPmm
	}

	return cpus, memory
}

// We assign value for parallel read of clustered index equal to the number of virtual cpu available for MySQL
func (c *Configurator) paramInnoDBinnodb_parallel_read_threads(parameter Parameter) Parameter {
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
func (c *Configurator) getGCScache(parameter Parameter) Parameter {
	//c.reference.gcache = int64(float64(c.reference.innodbRedoLogDim) * c.reference.gcacheLoad)
	//c.reference.gcacheFootprint = int64(math.Ceil(float64(c.reference.gcache) * 0.3))
	//c.reference.memoryLeftover -= c.reference.gcacheFootprint
	mem := uint64(c.reference.memoryLeftover / 11)

	//We need to consider that the cache stucture takes 50MB so we need to remove them from the available
	mem = mem - GroupRepGCSCacheMemStructureCost

	switch c.reference.loadID {
	case LoadTypeMostlyReads:
		mem = uint64(float64(mem) * 0.40)
	case LoadTypeSomeWrites:
		mem = uint64(float64(mem) * 0.60)
	case LoadTypeEqualReadsWrites:
		mem = uint64(float64(mem) * 0.80)
	case LoadTypeHeavyWrites:
		mem = uint64(float64(mem) * 1)
	}

	def, err := strconv.ParseUint(parameter.Default, 10, 64)
	if err != nil {
		print(err.Error())
	}

	// If the default value is less than the tenth part of the memory footprint then we will use that as value,
	// otherwise we will calculate it as the tenth of memory leftover
	if def < mem {
		parameter.Value = strconv.FormatUint(def, 10)
	}

	if mem >= parameter.Min {
		parameter.Value = strconv.FormatUint(mem, 10)
	} else {
		parameter.Value = strconv.FormatUint(parameter.Min, 10)
	}

	c.reference.gcscacheFootprint, _ = strconv.ParseInt(parameter.Value, 10, 64)
	c.reference.gcscache = c.reference.gcscacheFootprint
	c.reference.gcscacheFootprint = c.reference.gcscacheFootprint * 6
	c.reference.memoryLeftover -= c.reference.gcscacheFootprint

	return parameter

}

// We calculate the expel timeout based on a Max value that is reasonable, not the maximum value defined in MySQL config
// the value is calculated don the level of the load
func (c *Configurator) paramGroupReplicationMemberExpelTimeout(parameter Parameter) Parameter {
	val := int(math.Ceil(float64(float32(parameter.Max) * c.reference.loadFactor)))
	def, _ := strconv.Atoi(parameter.Default)
	if val < def {
		val = def
	}
	parameter.Value = strconv.Itoa(val)

	return parameter
}

// We calculate the expel timeout based on a Max value that is reasonable, not the maximum value defined in MySQL config
// the value is calculated don the level of the load
func (c *Configurator) paramGroupReplicationAutorejoinTries(parameter Parameter) Parameter {
	val := int(math.Ceil(float64(float32(parameter.Max) * c.reference.loadFactor)))
	def, _ := strconv.Atoi(parameter.Default)
	if val < def {
		val = def
	}
	parameter.Value = strconv.Itoa(val)

	return parameter
}

// We use a small message default size when in lack of memory resource to force the fragmentation
func (c *Configurator) paramGroupReplicationMessageCacheSize(parameter Parameter) Parameter {
	val := int64(1048576) //1 mb

	switch c.request.Dimension.Id {
	case 1:
		parameter.Value = strconv.FormatInt(val, 10)
	case 2:
		val = val * 2
		parameter.Value = strconv.FormatInt(val, 10)
	case 3:
		val = val * 4
		parameter.Value = strconv.FormatInt(val, 10)
	case 4:
		val = val * 6
		parameter.Value = strconv.FormatInt(val, 10)
	default:
		parameter.Value = parameter.Default

	}

	return parameter
}

// We tune the timeout based on the load factor, higher load longer timeout
func (c *Configurator) paramGroupReplicationUnreachableMajorityTimeout(parameter Parameter) Parameter {
	val := int(math.Ceil(float64(float32(parameter.Max) * c.reference.loadFactor)))
	min := int(parameter.Min)
	if val < min {
		val = min
	}
	parameter.Value = strconv.Itoa(val)

	return parameter
}

// we tune the parameter based on the utilization given intense OLTP will require more message handling as such less wait
// conversely read intensive load may benefit from longer wait to reduce context switching
func (c *Configurator) paramGroupReplicationPollSpinLoops(parameter Parameter) Parameter {
	val, _ := strconv.Atoi(parameter.Value)
	switch c.request.LoadType.Id {
	case LoadTypeMostlyReads:
		val = int(parameter.Max)
	case LoadTypeSomeWrites:
		val = int(parameter.Max) / 2
	case LoadTypeEqualReadsWrites:
		val = int(parameter.Min)

	}

	parameter.Value = strconv.Itoa(val)

	return parameter

}

// [EXPERIMENTAL] The tuning for this variable is wip
// given some memory issues we had in the GCS I want to see if using compression can benefit the memory consumption in the GCS cache
func (c *Configurator) paramGroupReplicationCompressionThreshold(parameter Parameter) Parameter {

	val := int64(parameter.Min) //126 KB

	switch c.request.Dimension.Id {
	case 1:
		parameter.Value = strconv.FormatInt(val, 10)
	case 2:
		val = val * 2
		parameter.Value = strconv.FormatInt(val, 10)
	case 3:
		val = val * 4
		parameter.Value = strconv.FormatInt(val, 10)
	case 4:
		val = val * 6
		parameter.Value = strconv.FormatInt(val, 10)
	default:
		parameter.Value = parameter.Default

	}

	return parameter
}

// we use default MySQL formula here
func (c *Configurator) paramServerThreadCacheSize(parameter Parameter) Parameter {
	maxConn := c.request.Connections
	val := (maxConn / 50) + 8
	if val > int(parameter.Max) {
		val = int(parameter.Max)
	}

	parameter.Value = strconv.Itoa(val)
	return parameter
}

func fillResponseMessage(pct float64, msg ResponseMessage, b bytes.Buffer, DBType string) (ResponseMessage, bool) {
	overUtilizing := false
	minlimit := float64(0.45)
	if DBType == "group_replication" {
		minlimit = float64(0.34)
	}

	if pct < minlimit {
		msg.MType = OverutilizingI
		msg.MText = "Request cancelled not enough resources details: " + b.String()
		msg.MName = msg.GetMessageText(msg.MType)
		overUtilizing = true
	} else if pct > minlimit && pct <= 0.65 {
		msg.MType = ClosetolimitI
		msg.MText = "Request processed however not optimal details: " + b.String()
		msg.MName = msg.GetMessageText(msg.MType)
	} else if pct > 0.65 {
		msg.MType = OkI
		msg.MText = "Request ok, resources details: " + b.String()
		msg.MName = msg.GetMessageText(msg.MType)
	}

	return msg, overUtilizing
}
