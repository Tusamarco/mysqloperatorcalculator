package mysqloperatorcalculator

import (
	"bytes"
	"fmt"
	"math"
	"strconv"

	"github.com/hashicorp/go-version"
	log "github.com/sirupsen/logrus"
)

type Configurator struct {
	request            ConfigurationRequest
	families           map[string]Family
	providerParams     map[string]ProviderParam
	reference          *references
	connectionResearch bool
}

// references structure keeps information needed while calculating parameters
type references struct {
	memory             float64 // total memory available
	cpus               int     // total cpus
	gcache             int64   // assigned gcache dimension
	gcacheFootprint    int64   // expected file footprint in memory
	gcacheLoad         float64 // gcache load adj factor base on type of load
	memoryLeftover     int64   // memory free after all calculation
	innodbRedoLogDim   int64   // total redolog dimension
	innoDBbpSize       int64   // Calculated BP to apply
	loadAdjustment     float32 // load adjustment indicator based on CPU weight against connections
	loadAdjustmentMax  float64 // Upper limit given optimal condition between CPU resources and connections using as minimal connections=MinConnectionNumber
	loadFactor         float32 // Load factor for calculation based on loadAdjustment
	loadID             int     // loadID coming from request
	dimension          int     // Dimension Id coming from request
	connections        int     // raw number of connections
	tmpTableFootprint  int64   // tempTable expected footprint in memory
	connBuffersMemTot  int64   // Total mem use for all connection buffers + temp table
	idealBufferPoolDIm int64   // Theoretical ideal BP dimension (rule of the thumb)
	innoDBBPInstances  int     // assigned number of BP
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

// GetAllGaleraProviderOptionsAsString returns all provider options considered as a single string
func (c *Configurator) GetAllGaleraProviderOptionsAsString() bytes.Buffer {
	var b bytes.Buffer

	for key, param := range c.providerParams {
		b.WriteString(key)
		b.WriteString(`=`)
		if param.Value >= 0 {
			// Used Fprintf directly to the buffer to avoid intermediary string allocations
			fmt.Fprintf(&b, param.Literal, strconv.FormatInt(param.Value, 10))
		} else {
			b.WriteString(param.Literal)
		}
		b.WriteString(";")
	}
	return b
}

func (c *Configurator) Init(r ConfigurationRequest, fam map[string]Family, conf Configuration, message ResponseMessage) (ResponseMessage, bool) {
	var dim Dimension
	if r.Dimension.Id != DimensionOpen && r.Dimension.Name != "scaled" {
		dim = conf.GetDimensionByID(r.Dimension.Id)
	} else {
		dim = r.Dimension
	}

	load := conf.GetLoadByID(r.LoadType.Id)
	if load.Id == 0 || dim.Id == 0 {
		log.Warningf("Invalid load %d or Dimension %d detected", load.Id, dim.Id)
	}

	connections := r.Connections
	if connections < MinConnectionNumber || connections == 0 {
		connections = MinConnectionNumber
	}

	c.reference = &references{
		memory:       dim.MemoryBytes,
		cpus:         dim.Cpu,
		gcacheLoad:   1,
		loadID:       load.Id,
		dimension:    dim.Id,
		connections:  connections,
		cpusPmm:      float64(dim.PmmCpu),
		cpusProxy:    float64(dim.ProxyCpu),
		cpusMySQL:    float64(dim.MysqlCpu),
		memoryMySQL:  dim.MysqlMemory,
		memoryProxy:  dim.ProxyMemory,
		memoryPmm:    dim.PmmMemory,
		gcscacheLoad: 1,
	}

	loadConnectionFactor, responseMessage, done := c.calculateLoadConnectionFactor(dim, message)
	if done {
		return responseMessage, true
	}

	c.reference.loadFactor = loadConnectionFactor
	if c.request.DBType == DbTypePXC {
		c.reference.idealBufferPoolDIm = int64(c.reference.memoryMySQL * InnoDBPctValuePXC)
	} else {
		c.reference.idealBufferPoolDIm = int64(c.reference.memoryMySQL * InnoDBPctValueGR)
	}
	c.reference.gcacheLoad = c.getGcacheLoad()

	var p ProviderParam
	c.families = fam
	c.request = r
	c.providerParams = p.Init()

	return message, false
}

func (c *Configurator) calculateLoadConnectionFactor(dim Dimension, message ResponseMessage) (float32, ResponseMessage, bool) {
	CpuConncetionMillFactor := 0.0

	switch c.reference.loadID {
	case LoadTypeMostlyReads:
		CpuConncetionMillFactor = CpuConncetionMillFactorRead
	case LoadTypeSomeWrites:
		CpuConncetionMillFactor = CpuConncetionMillFactorReadWriteLight
	case LoadTypeEqualReadsWrites:
		CpuConncetionMillFactor = CpuConncetionMillFactorReadWriteEqual
	case LoadTypeHeavyWrites:
		CpuConncetionMillFactor = CpuConncetionMillFactorReadWriteHeavy
	default:
		CpuConncetionMillFactor = CpuConncetionMillFactorReadWriteLight
	}

	c.reference.loadAdjustmentMax = float64(dim.MysqlCpu) / CpuConncetionMillFactor
	loadConnectionFactor := float32(c.reference.connections) / float32(c.reference.loadAdjustmentMax)

	if loadConnectionFactor > 1 {
		message.MType = OverutilizingI
		return 0, message, true
	}
	return loadConnectionFactor, ResponseMessage{}, false
}

func (c *Configurator) ProcessRequest() map[string]Family {
	c.getConnectionBuffers()

	conWeight := float64(c.reference.connBuffersMemTot) / c.reference.memoryMySQL
	if conWeight < ConnectionWeighPctLimit {
		c.getInnodbRedolog()
		c.getInnodbBufferPool(false)

		if c.request.DBType == "pxc" {
			c.getGcache()
		}

		c.getInnodbParameters()
		c.getServerParameters()
		c.getReplicationParameters()

		if c.request.DBType == "pxc" {
			c.getGaleraParameters()
		}

		if c.request.DBType == "group_replication" {
			c.getGroupReplicationParameters()

			group := c.families["mysql"].Groups["configuration_groupReplication"]
			group.Parameters["loose_group_replication_message_cache_size"] = c.getGCScache(group.Parameters["loose_group_replication_message_cache_size"])
			c.families["mysql"].Groups["configuration_groupReplication"] = group
		}

		// Before returning the values we want tore recover any left over from the memory and assign back to the buffer pool by a %
		c.getInnodbBufferPool(true)

		c.getProbesAndResources(FamilyTypeMysql)
		c.getProbesAndResources(FamilyTypeProxy)
		c.getProbesAndResources(FamilyTypeMonitor)
	}

	return c.filterByMySQLVersion()
}

func (c *Configurator) filterByMySQLVersion() map[string]Family {
	incomingVStr := fmt.Sprintf("%d.%d.%d", c.request.Mysqlversion.Major, c.request.Mysqlversion.Minor, c.request.Mysqlversion.Patch)
	incomingV, _ := version.NewVersion(incomingVStr)

	for _, l1Val := range c.families {
		for _, l2Val := range l1Val.Groups {
			for l3Key, l3Val := range l2Val.Parameters {
				if l3Val.Mysqlversions.Min.Major > 0 {
					minStr := fmt.Sprintf("%d.%d.%d", l3Val.Mysqlversions.Min.Major, l3Val.Mysqlversions.Min.Minor, l3Val.Mysqlversions.Min.Patch)
					maxStr := fmt.Sprintf("%d.%d.%d", l3Val.Mysqlversions.Max.Major, l3Val.Mysqlversions.Max.Minor, l3Val.Mysqlversions.Max.Patch)

					paramVmin, _ := version.NewVersion(minStr)
					paramVmax, _ := version.NewVersion(maxStr)

					// Simplified logic to remove out-of-bounds versions
					if incomingV.LessThan(paramVmin) || incomingV.GreaterThan(paramVmax) {
						delete(l2Val.Parameters, l3Key)
					}
				}
			}
		}
	}
	return c.families
}

// loadValues returns one of four values indexed by current load type [MostlyReads, SomeWrites, EqualReadsWrites, HeavyWrites].
func (c *Configurator) loadValues(byLoad [4]string) string {
	switch c.reference.loadID {
	case LoadTypeMostlyReads:
		return byLoad[0]
	case LoadTypeSomeWrites:
		return byLoad[1]
	case LoadTypeEqualReadsWrites:
		return byLoad[2]
	case LoadTypeHeavyWrites:
		return byLoad[3]
	default:
		return byLoad[1]
	}
}

// loadFloat returns one of four floats indexed by current load type [MostlyReads, SomeWrites, EqualReadsWrites, HeavyWrites].
func (c *Configurator) loadFloat(byLoad [4]float64) float64 {
	switch c.reference.loadID {
	case LoadTypeMostlyReads:
		return byLoad[0]
	case LoadTypeSomeWrites:
		return byLoad[1]
	case LoadTypeEqualReadsWrites:
		return byLoad[2]
	case LoadTypeHeavyWrites:
		return byLoad[3]
	default:
		return byLoad[0]
	}
}

func (c *Configurator) getGcache() {
	c.reference.gcache = int64(float64(c.reference.innodbRedoLogDim) * c.reference.gcacheLoad)
	if c.reference.gcache > (c.reference.memoryLeftover / 3) {
		c.reference.gcache = c.reference.memoryLeftover / 3
	}

	gcacheFootPrintFactor := c.loadFloat([4]float64{
		GcacheFootPrintFactorRead,
		GcacheFootPrintFactorLightWrite,
		GcacheFootPrintFactorReadWrite,
		0.5,
	})

	c.reference.gcacheFootprint = int64(math.Ceil(float64(c.reference.gcache) * gcacheFootPrintFactor))
	c.reference.memoryLeftover -= c.reference.gcacheFootprint
}

//
//func (c *Configurator) getAdjFactor(loadConnectionFactor float32) float32 {
//	impedance := loadConnectionFactor / float32(c.reference.loadAdjustmentMax)
//
//	switch c.reference.loadID {
//	case LoadTypeMostlyReads, LoadTypeSomeWrites, LoadTypeEqualReadsWrites, LoadTypeHeavyWrites:
//		return impedance
//	default:
//		return float32(c.reference.loadAdjustmentMax)
//	}
//}

func (c *Configurator) getConnectionBuffers() {
	group := c.families["mysql"].Groups["configuration_connection"]
	//group.Parameters["binlog_cache_size"] = c.paramBinlogCacheSize(group.Parameters["binlog_cache_size"])
	//group.Parameters["binlog_stmt_cache_size"] = c.paramBinlogCacheSize(group.Parameters["binlog_stmt_cache_size"])
	group.Parameters["join_buffer_size"] = c.paramJoinBuffer(group.Parameters["join_buffer_size"])
	group.Parameters["read_rnd_buffer_size"] = c.paramReadRndBuffer(group.Parameters["read_rnd_buffer_size"])
	group.Parameters["sort_buffer_size"] = c.paramSortBuffer(group.Parameters["sort_buffer_size"])

	c.calculateTmpTableFootprint(group.Parameters["tmp_table_size"])
	c.sumConnectionBuffers(group.Parameters)

	c.families["mysql"].Groups["configuration_connection"] = group
}

func (c *Configurator) paramBinlogCacheSize(inParameter Parameter) Parameter {
	inParameter.Value = c.loadValues([4]string{"32768", "131072", "262144", "358400"})
	return inParameter
}

func (c *Configurator) paramJoinBuffer(inParameter Parameter) Parameter {
	inParameter.Value = c.loadValues([4]string{"262144", "524288", "1048576", "1048576"})
	return inParameter
}

func (c *Configurator) paramReadRndBuffer(inParameter Parameter) Parameter {
	inParameter.Value = c.loadValues([4]string{"262144", "393216", "707788", "707788"})
	return inParameter
}

func (c *Configurator) paramSortBuffer(inParameter Parameter) Parameter {
	inParameter.Value = c.loadValues([4]string{"262144", "524288", "1572864", "2097152"})
	return inParameter
}

// Fixed bug: this method previously returned a variable that was always 0
func (c *Configurator) calculateTmpTableFootprint(inParameter Parameter) {
	c.reference.tmpTableFootprint, _ = strconv.ParseInt(inParameter.Value, 10, 64)

	multiplier := c.loadFloat([4]float64{0.2, 0.1, 0.3, 0.05})

	c.reference.tmpTableFootprint = int64(float64(c.reference.tmpTableFootprint) * multiplier)
}

func (c *Configurator) sumConnectionBuffers(params map[string]Parameter) {
	var totMemory int64
	for key, param := range params {
		if key != "tmp_table_size" && key != "max_heap_table_size" {
			v, _ := strconv.ParseInt(param.Value, 10, 64)
			totMemory += v
		}
	}

	possibleConnectionTmp := float64(c.reference.connections) * float64(c.reference.loadFactor)
	possibleTmpMemPressure := int64(math.Floor(possibleConnectionTmp)) * c.reference.tmpTableFootprint

	c.reference.connBuffersMemTot = (totMemory * int64(possibleConnectionTmp)) + possibleTmpMemPressure
	c.reference.memoryLeftover = int64(c.reference.memoryMySQL) - c.reference.connBuffersMemTot
}

func (c *Configurator) getInnodbRedolog() {
	group := c.families["mysql"].Groups["configuration_innodb"]
	group.Parameters["innodb_log_file_size"] = c.getRedologDimensionTot(group.Parameters["innodb_log_file_size"])
	c.families["mysql"].Groups["configuration_innodb"] = group
}

func (c *Configurator) getRedologDimensionTot(inParameter Parameter) Parameter {
	var redologTotDimension int64
	baseDim := float32(c.reference.idealBufferPoolDIm)

	switch c.reference.loadID {
	case LoadTypeSomeWrites:
		redologTotDimension = int64(baseDim * (0.2 + (0.2 * c.reference.loadFactor)))
	case LoadTypeEqualReadsWrites:
		redologTotDimension = int64(baseDim * (0.3 + (0.3 * c.reference.loadFactor)))
	case LoadTypeHeavyWrites:
		// TODO this is mainly in case of injest and it may need more calculation, probably if connected to PMM for now is good like this
		redologTotDimension = int64(baseDim * (0.4 + (0.4 * c.reference.loadFactor)))
	default:
		redologTotDimension = int64(baseDim * (0.15 + (0.15 * c.reference.loadFactor)))
	}

	c.reference.innodbRedoLogDim = redologTotDimension

	// Access map once, modify directly
	group := c.families["mysql"].Groups["configuration_innodb"]

	parameterIbC := group.Parameters["innodb_redo_log_capacity"]
	parameterIbC.Value = strconv.FormatInt(c.reference.innodbRedoLogDim, 10)
	group.Parameters["innodb_redo_log_capacity"] = parameterIbC

	parameter := group.Parameters["innodb_log_files_in_group"]
	parameter = c.getRedologfilesNumber(redologTotDimension, parameter)
	group.Parameters["innodb_log_files_in_group"] = parameter

	a, _ := strconv.ParseInt(parameter.Value, 10, 64)
	if a > 0 {
		inParameter.Value = strconv.FormatInt(redologTotDimension/a, 10)
	}

	// Save back to map
	c.families["mysql"].Groups["configuration_innodb"] = group
	return inParameter
}

func (c *Configurator) getRedologfilesNumber(dimension int64, parameter Parameter) Parameter {
	dimMB := float64(dimension) / (1024 * 1024)

	switch {
	case dimMB < 500:
		parameter.Value = "2"
	case dimMB >= 500 && dimMB <= 1000:
		parameter.Value = map[bool]string{true: "2", false: "3"}[c.reference.loadID == 1]
	case dimMB > 1000 && dimMB <= 2000:
		parameter.Value = map[bool]string{true: "3", false: "5"}[c.reference.loadID == 1]
	case dimMB > 2000 && dimMB <= 4000:
		parameter.Value = map[bool]string{true: "5", false: "8"}[c.reference.loadID == 1]
	default:
		parameter.Value = strconv.FormatFloat(math.Floor(dimMB/400), 'f', 0, 64)
	}

	return parameter
}

func (c *Configurator) getGcacheLoad() float64 {
	return c.loadFloat([4]float64{1.0, 1.15, 1.2, 1.0})
}

func (c *Configurator) getInnodbBufferPool(final bool) {
	group := c.families["mysql"].Groups["configuration_innodb"]

	if !final {
		group.Parameters["innodb_buffer_pool_size"] = c.paramInnoDBBufferPool(group.Parameters["innodb_buffer_pool_size"], final)
		group.Parameters["innodb_buffer_pool_instances"] = c.paramInnoDBBufferPoolInstances(group.Parameters["innodb_buffer_pool_instances"])
	} else {
		group.Parameters["innodb_buffer_pool_size"] = c.paramInnoDBBufferPool(group.Parameters["innodb_buffer_pool_size"], final)
	}
	c.families["mysql"].Groups["configuration_innodb"] = group
}

func (c *Configurator) getInnodbParameters() {
	group := c.families["mysql"].Groups["configuration_innodb"]
	group.Parameters["innodb_adaptive_hash_index"] = c.paramInnoDBAdaptiveHashIndex(group.Parameters["innodb_adaptive_hash_index"])
	//group.Parameters["innodb_page_cleaners"] = c.paramInnoDBBufferPoolCleaners(group.Parameters["innodb_buffer_pool_instances"])
	group.Parameters["innodb_purge_threads"] = c.paramInnoDPurgeThreads(group.Parameters["innodb_purge_threads"])
	group.Parameters["innodb_io_capacity_max"] = c.paramInnoDBIOCapacityMax(group.Parameters["innodb_io_capacity_max"])
	group.Parameters["innodb_parallel_read_threads"] = c.paramInnoDBinnodb_parallel_read_threads(group.Parameters["innodb_parallel_read_threads"])
	c.families["mysql"].Groups["configuration_innodb"] = group
}

func (c *Configurator) getGroupReplicationParameters() {
	group := c.families["mysql"].Groups["configuration_groupReplication"]
	group.Parameters["loose_group_replication_member_expel_timeout"] = c.paramGroupReplicationMemberExpelTimeout(group.Parameters["loose_group_replication_member_expel_timeout"])
	group.Parameters["loose_group_replication_autorejoin_tries"] = c.paramGroupReplicationAutorejoinTries(group.Parameters["loose_group_replication_autorejoin_tries"])
	group.Parameters["loose_group_replication_communication_max_message_size"] = c.paramGroupReplicationMessageMaxSize(group.Parameters["loose_group_replication_communication_max_message_size"])
	//group.Parameters["loose_group_replication_poll_spin_loops"] = c.paramGroupReplicationPollSpinLoops(group.Parameters["loose_group_replication_poll_spin_loops"])
	group.Parameters["loose_group_replication_flow_control_period"] = c.paramGroupReplicationFlowControlPeriod(group.Parameters["loose_group_replication_flow_control_period"])
	c.families["mysql"].Groups["configuration_groupReplication"] = group
}

func (c *Configurator) paramInnoDBAdaptiveHashIndex(parameter Parameter) Parameter {
	if c.reference.loadID == LoadTypeMostlyReads {
		parameter.Value = "True"
	} else {
		parameter.Value = "False"
	}
	return parameter
}

func (c *Configurator) paramInnoDBBufferPool(parameter Parameter, final bool) Parameter {
	bufferPollPct := InnoDBPctValueGR
	if c.request.DBType == "pxc" {
		bufferPollPct = InnoDBPctValuePXC
	}

	if !final {
		bufferPool := int64(math.Floor(float64(c.reference.memoryLeftover) * bufferPollPct))
		bufferPoolSubstract := int64(math.Floor(float64(c.reference.memoryLeftover) * bufferPollPct))

		parameter.Value = strconv.FormatInt(bufferPool, 10)
		c.reference.innoDBbpSize = bufferPool
		c.reference.memoryLeftover -= bufferPoolSubstract
	} else {
		bufferPool := int64(0)

		// We identify if the allocations have brought the memory to be negative in that case we can only take it back
		// from the bufferpool
		if c.reference.memoryLeftover > 0 {
			reassignFreeMemory := c.CalculateReturnBytes(c.reference.memoryLeftover)
			bufferPool = c.reference.innoDBbpSize + reassignFreeMemory
			c.reference.memoryLeftover -= reassignFreeMemory
		} else {
			// I know is cumbersome and I can simply use the "+" operand, but that will confuse reading
			// the memory left over at this point is expected to be negative
			bufferPool = c.reference.innoDBbpSize - int64(math.Abs(float64(c.reference.memoryLeftover)))
			c.reference.memoryLeftover = 0

			// Enforce minimum buffer pool floor to prevent going dangerously low
			minPct := MinLimitGR
			if c.request.DBType == "pxc" {
				minPct = MinLimitPXC
			}
			minBufferPool := int64(c.reference.memoryMySQL * minPct)
			if bufferPool < minBufferPool {
				bufferPool = minBufferPool
			}
		}

		parameter.Value = strconv.FormatInt(bufferPool, 10)
		c.reference.innoDBbpSize = bufferPool

	}
	return parameter
}

// CalculateReturnBytes determines the number of bytes to return based on input size.
func (c *Configurator) CalculateReturnBytes(incomingBytes int64) int64 {
	Megabyte := 1024 * 1024
	Gigabyte := 1024 * Megabyte

	MinThreshold := int64(300 * Megabyte)
	MaxThreshold := int64(2 * Gigabyte)

	MinPercent := 0.20 // 20% returned at or below MinThreshold
	MaxPercent := 0.85 // 85% returned at or above MaxThreshold

	// 1. Handle the minimum threshold (300 MB or below → return 20%)
	if incomingBytes <= MinThreshold {
		return int64(float64(incomingBytes) * MinPercent)
	}

	// 2. Handle the maximum threshold (2 GB or above → return 85%)
	if incomingBytes >= MaxThreshold {
		return int64(float64(incomingBytes) * MaxPercent)
	}

	// 3. Linearly interpolate the percentage for anything in between
	// Calculate how far along the input is between the min and max thresholds (0.0 to 1.0)
	progress := float64(incomingBytes-MinThreshold) / float64(MaxThreshold-MinThreshold)

	// Apply that progress to our percentage range (20% to 85%)
	percentage := MinPercent + (progress * (MaxPercent - MinPercent))

	// Calculate and return the final byte count
	return int64(float64(incomingBytes) * percentage)
}

func (c *Configurator) paramInnoDBBufferPoolInstances(parameter Parameter) Parameter {
	instances := 1
	if c.reference.cpus > 2000 {
		bpSizeGB := float64(c.reference.innoDBbpSize) / (1024 * 1024 * 1024)
		maxCpus := c.reference.cpusMySQL / 1000.0

		factor := bpSizeGB / maxCpus
		if factor > 1 {
			instances = int(maxCpus * 2)
		} else if factor > 0.4 {
			instances = int(maxCpus)
		} else {
			instances = int(math.Ceil(maxCpus / 2))
		}
		parameter.Value = strconv.Itoa(instances)
	} else {
		parameter.Value = "1"
	}

	if instances > 64 {
		instances = 64
		parameter.Value = strconv.Itoa(instances)
	}
	c.reference.innoDBBPInstances = instances
	return parameter
}

func (c *Configurator) paramInnoDBBufferPoolCleaners(parameter Parameter) Parameter {
	parameter.Value = strconv.Itoa(c.reference.innoDBBPInstances)
	return parameter
}

func (c *Configurator) paramInnoDPurgeThreads(parameter Parameter) Parameter {
	threads := 4
	if (c.reference.cpus / 1000) > 4 {
		adjValue := c.reference.gcscacheLoad
		if c.request.DBType == "pxc" {
			adjValue = c.reference.gcacheLoad
		}
		valore := (c.reference.cpusMySQL / 1000.0) * adjValue
		threads = int(math.Ceil(valore))
	}

	if threads > 32 {
		threads = 32
	}

	parameter.Value = strconv.Itoa(threads)
	return parameter
}

func (c *Configurator) paramInnoDBIOCapacityMax(parameter Parameter) Parameter {
	parameter.Value = c.loadValues([4]string{"28000", "24000", "20000", "20000"})
	return parameter
}

func (c *Configurator) getServerParameters() {
	group := c.families["mysql"].Groups["configuration_server"]
	group.Parameters["max_connections"] = c.paramServerMaxConnections(group.Parameters["max_connections"])
	// TODO re-enable once we have better TP handling in PS
	//group.Parameters["thread_pool_size"] = c.paramServerThreadPool(group.Parameters["thread_pool_size"])
	//group.Parameters["table_definition_cache"] = c.paramServerTableDefinitionCache(group.Parameters["table_definition_cache"])
	//group.Parameters["table_open_cache"] = c.paramServerTableOpenCache(group.Parameters["table_open_cache"])
	//group.Parameters["thread_stack"] = c.paramServerThreadStack(group.Parameters["thread_stack"])
	//group.Parameters["table_open_cache_instances"] = c.paramServerTableOpenCacheInstances(group.Parameters["table_open_cache_instances"])
	group.Parameters["thread_cache_size"] = c.paramServerThreadCacheSize(group.Parameters["thread_cache_size"])
	c.families["mysql"].Groups["configuration_server"] = group
}

func (c *Configurator) paramServerMaxConnections(parameter Parameter) Parameter {
	parameter.Value = strconv.Itoa(c.reference.connections + 2)
	return parameter
}

func (c *Configurator) paramServerThreadPool(parameter Parameter) Parameter {
	threads := 4
	cpus := int(c.reference.cpusMySQL / 1000)

	if cpus > 2 && cpus <= 256 {
		threads = cpus * 2
	}

	parameter.Value = strconv.Itoa(threads)
	return parameter
}

func (c *Configurator) paramServerTableDefinitionCache(parameter Parameter) Parameter {
	return parameter
}

func (c *Configurator) paramServerTableOpenCache(parameter Parameter) Parameter {
	return parameter
}

func (c *Configurator) paramServerThreadStack(parameter Parameter) Parameter {
	return parameter
}

func (c *Configurator) paramServerTableOpenCacheInstances(parameter Parameter) Parameter {
	parameter.Value = "4"
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
	if c.reference.loadID == LoadTypeSomeWrites || c.reference.loadID == LoadTypeEqualReadsWrites {
		parameter.Value = "3"
	} else {
		parameter.Value = "0"
	}
	return parameter
}

func (c *Configurator) getGaleraSlaveThreads(parameter Parameter) Parameter {
	cpus := int(c.reference.cpusMySQL / 1000.0)
	if cpus <= 1 {
		cpus = 1
	} else {
		cpus = cpus / 2
	}
	parameter.Value = strconv.Itoa(cpus)
	return parameter
}

func (c *Configurator) getGaleraFragmentSize(parameter Parameter) Parameter {
	return parameter
}

func (c *Configurator) getProbesAndResources(family string) {
	group := c.families[family].Groups["resources"]
	cpus, memory := c.getResourcesByFamily(family)
	group = c.setResources(group, cpus, memory)
	c.families[family].Groups["resources"] = group

	group = c.families[family].Groups["readinessProbe"]
	parameter := group.Parameters["timeoutSeconds"]
	val := int(math.Ceil(float64(parameter.Max) * float64(c.reference.loadFactor)))
	if val < 1 {
		val = int(parameter.Min)
	}
	parameter.Value = strconv.Itoa(val)
	group.Parameters["timeoutSeconds"] = parameter
	c.families[family].Groups["readinessProbe"] = group

	group = c.families[family].Groups["livenessProbe"]
	val = int(math.Ceil(float64(parameter.Max) * float64(c.reference.loadFactor)))
	if val < 1 {
		val = int(parameter.Min)
	}
	parameter.Value = strconv.Itoa(val)
	group.Parameters["timeoutSeconds"] = parameter
	c.families[family].Groups["livenessProbe"] = group
}

func (c *Configurator) setResources(group GroupObj, cpus float64, memory float64) GroupObj {
	parameter := group.Parameters["request_memory"]
	parameter.Value = strconv.FormatFloat(memory*0.95, 'f', 0, 64)
	group.Parameters["request_memory"] = parameter

	parameter = group.Parameters["limit_memory"]
	parameter.Value = strconv.FormatFloat(memory, 'f', 0, 64)
	group.Parameters["limit_memory"] = parameter

	parameter = group.Parameters["request_cpu"]
	parameter.Value = strconv.FormatFloat(cpus*0.95, 'f', 0, 64) + "m"
	group.Parameters["request_cpu"] = parameter

	parameter = group.Parameters["limit_cpu"]
	parameter.Value = strconv.FormatFloat(cpus, 'f', 0, 64) + "m"
	group.Parameters["limit_cpu"] = parameter

	return group
}

func (c *Configurator) EvaluateResources(responseMsg ResponseMessage) (ResponseMessage, bool) {
	var b bytes.Buffer

	// Fprintf is heavily optimized for constructing complex text blocks and eliminates numerous temporary strings
	fmt.Fprintf(&b, "\n\nTot Memory Bytes    = %.0f\n", c.reference.memory)
	fmt.Fprintf(&b, "Tot CPU                 = %d\n", c.reference.cpus)
	fmt.Fprintf(&b, "Tot Connections         = %d\n\n", c.reference.connections)

	fmt.Fprintf(&b, "memory assign to mysql Bytes   = %.0f\n", c.reference.memoryMySQL)
	fmt.Fprintf(&b, "memory assign to Proxy Bytes   = %.0f\n", c.reference.memoryProxy)
	fmt.Fprintf(&b, "memory assign to Monitor Bytes = %.0f\n", c.reference.memoryPmm)
	fmt.Fprintf(&b, "cpus assign to mysql  = %.0f\n", c.reference.cpusMySQL)
	fmt.Fprintf(&b, "cpus assign to Proxy  = %.0f\n", c.reference.cpusProxy)
	fmt.Fprintf(&b, "cpus assign to Monitor= %.0f\n\n", c.reference.cpusPmm)

	if c.request.DBType == "pxc" {
		fmt.Fprintf(&b, "Gcache mem on disk      = %d\n", c.reference.gcache)
		fmt.Fprintf(&b, "Gcache mem Footprint    = %d\n\n", c.reference.gcacheFootprint)
	}

	if c.request.DBType == "group_replication" {
		fmt.Fprintf(&b, "GCS cache mem limit      = %d\n", c.reference.gcscache)
		fmt.Fprintf(&b, "GCS cache mem possible Footprint    = %d\n\n", c.reference.gcscacheFootprint)
	}

	fmt.Fprintf(&b, "Tmp Table mem Footprint = %d\n", c.reference.tmpTableFootprint)
	fmt.Fprintf(&b, "By connection mem tot   = %d\n\n", c.reference.connBuffersMemTot)
	fmt.Fprintf(&b, "Innodb Bufferpool       = %d\n", c.reference.innoDBbpSize)

	bpPct := float64(c.reference.innoDBbpSize) / c.reference.memory
	fmt.Fprintf(&b, "%% BP over av memory     = %.2f\n\n", bpPct)
	fmt.Fprintf(&b, "memory leftover         = %d\n\n", c.reference.memoryLeftover)
	fmt.Fprintf(&b, "Load factor cpu        = %.2f\n", c.reference.loadFactor)
	fmt.Fprintf(&b, "Load mem factor= %.2f\n\n", bpPct)

	return c.FillResponseMessage(bpPct, responseMsg, b, c.request.DBType)
}

func (c *Configurator) getResourcesByFamily(family string) (float64, float64) {
	switch family {
	case FamilyTypeMysql:
		return c.reference.cpusMySQL, c.reference.memoryMySQL
	case FamilyTypeProxy:
		return c.reference.cpusProxy, c.reference.memoryProxy
	case FamilyTypeMonitor:
		return c.reference.cpusPmm, c.reference.memoryPmm
	default:
		return 0.0, 0.0
	}
}

func (c *Configurator) paramInnoDBinnodb_parallel_read_threads(parameter Parameter) Parameter {
	threads := 1
	cpus := int(c.reference.cpusMySQL / 1000)

	if cpus > 2 && cpus < 256 {
		threads = cpus
	} else if cpus > 256 {
		threads = 256
	}
	parameter.Value = strconv.Itoa(threads)
	return parameter
}

func (c *Configurator) getGCScache(parameter Parameter) Parameter {
	/*
		This calculation should be done against the running connections not against the load type
		considering the cost of connection x how much of the group_replication_message_cache_size
		we can use considering the maximum the default?
	*/
	// TODO: gcsFactor is currently unbounded. A high connection count relative to CPU
	// (e.g. 200 connections on a 1-core node: gcsFactor = 200/100 = 2.0) can produce a
	// GCS cache larger than available MySQL memory, pushing memoryLeftover deeply negative
	// and collapsing the InnoDB buffer pool below MinLimitGR.
	// Fix: cap gcsFactor at MaxGCSCacheFactor (e.g. 4.0) AND cap the result at
	// memoryLeftover/3 to mirror the GCache guard in getGcache().
	mem, _ := strconv.ParseFloat(parameter.Default, 64)

	gcsFactor := float64(c.reference.connections) / float64(c.reference.cpus/GCSConnWeight)
	mem *= gcsFactor

	val := parameter.Min

	if uint64(mem) >= val {
		parameter.Value = strconv.FormatUint(uint64(mem), 10)
	} else {
		// If the value calculated is less than the minimum, we adjust the value to minimum value.
		parameter.Value = strconv.FormatUint(val, 10)
	}

	c.reference.gcscacheFootprint, _ = strconv.ParseInt(parameter.Value, 10, 64)
	c.reference.gcscache = c.reference.gcscacheFootprint
	c.reference.memoryLeftover -= c.reference.gcscacheFootprint

	return parameter
}

func (c *Configurator) paramGroupReplicationMemberExpelTimeout(parameter Parameter) Parameter {
	valS, err := strconv.ParseFloat(parameter.Value, 64)
	if err != nil {
		log.Warnf("ParseFloat Error: %v", err)
	}

	val := int(math.Ceil(valS * float64(c.reference.loadFactor)))
	def, _ := strconv.Atoi(parameter.Default)

	if val < def {
		val = def
	}
	parameter.Value = strconv.Itoa(val)
	return parameter
}

func (c *Configurator) paramGroupReplicationFlowControlPeriod(parameter Parameter) Parameter {
	val := int(math.Ceil(float64(parameter.Max) * float64(1.0-c.reference.loadFactor)))
	mind := int(parameter.Min)
	if val < mind {
		val = mind
	}
	parameter.Value = strconv.Itoa(val)
	return parameter
}

func (c *Configurator) paramGroupReplicationAutorejoinTries(parameter Parameter) Parameter {
	val := int(math.Ceil(float64(parameter.Max) * float64(c.reference.loadFactor)))
	def, _ := strconv.Atoi(parameter.Default)
	if val < def {
		val = def
	}
	parameter.Value = strconv.Itoa(val)
	return parameter
}

func (c *Configurator) paramGroupReplicationMessageMaxSize(parameter Parameter) Parameter {
	pval, err := strconv.Atoi(parameter.Value)
	if err != nil {
		log.Warnf("Atoi Error: %v", err)
	}

	val := float64(pval)
	switch c.request.LoadType.Id {
	case 1:
		val *= 1.0
	case 2:
		val *= 1.5
	case 3:
		val *= 2.0
	case 4:
		val *= 2.2
	default:
		parameter.Value = parameter.Value
	}

	defVl, _ := strconv.ParseUint(parameter.Default, 10, 64)
	if uint64(val) > defVl {
		parameter.Value = strconv.FormatUint(defVl, 10)
		return parameter
	} else if uint64(val) < parameter.Min {
		return parameter
	}

	parameter.Value = strconv.FormatFloat(val, 'f', 0, 64)
	return parameter
}

func (c *Configurator) paramGroupReplicationUnreachableMajorityTimeout(parameter Parameter) Parameter {
	val := int(math.Ceil(float64(parameter.Max) * float64(c.reference.loadFactor)))
	min := int(parameter.Min)
	if val < min {
		val = min
	}
	parameter.Value = strconv.Itoa(val)
	return parameter
}

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

// TODO disabled for now
//func (c *Configurator) paramGroupReplicationCompressionThreshold(parameter Parameter) Parameter {
//	val := int64(parameter.Min) // 126 KB
//
//	switch c.request.Dimension.Id {
//	case 1:
//		// val remains identical
//	case 2:
//		val *= 2
//	case 3:
//		val *= 4
//	case 4:
//		val *= 6
//	default:
//		parameter.Value = parameter.Default
//		return parameter
//	}
//
//	parameter.Value = strconv.FormatInt(val, 10)
//	return parameter
//}

func (c *Configurator) paramServerThreadCacheSize(parameter Parameter) Parameter {
	val := c.request.Connections
	if val > int(parameter.Max) {
		val = int(parameter.Max)
	}

	parameter.Value = strconv.Itoa(val)
	return parameter
}

func (c *Configurator) FillResponseMessage(pct float64, msg ResponseMessage, b bytes.Buffer, DBType string) (ResponseMessage, bool) {
	overUtilizing := false
	minlimit := float64(MinLimitPXC)
	if DBType == "group_replication" {
		minlimit = float64(MinLimitGR)
	}

	// Not used anymore the memory leftover is managed dealing with the Bufferpool size
	//if c.reference.memoryLeftover < 0 {
	//	overUtilizing = true
	//	pct = 0.0
	//} else {
	//	minMemoryAccepted := int64(c.reference.memory * MemoryFreeMinimumLimit)
	//	if c.reference.memoryLeftover < minMemoryAccepted {
	//		overUtilizing = true
	//		pct = 0.0
	//	}
	//}

	if pct < minlimit {
		msg.MType = OverutilizingI
		msg.MText = "Request cancelled not enough resources details: " + b.String()
		msg.MName = msg.GetMessageText(msg.MType)
		overUtilizing = true
	} else if pct <= minlimit+0.10 {
		msg.MType = ClosetolimitI
		msg.MText = "Request processed however not optimal details: " + b.String()
		msg.MName = msg.GetMessageText(msg.MType)
	} else {
		msg.MType = OkI
		msg.MText = "Request ok, resources details: " + b.String()
		msg.MName = msg.GetMessageText(msg.MType)
	}

	return msg, overUtilizing
}

func (c *Configurator) getReplicationParameters() {
	group := c.families["mysql"].Groups["configuration_replica"]
	group.Parameters["replica_parallel_workers"] = c.paramReplicaParallelWorkers(group.Parameters["replica_parallel_workers"])

	c.families["mysql"].Groups["configuration_replica"] = group
}

// paramReplicaParallelWorkers adjusts the "replica_parallel_workers" parameter value based on available CPU resources.
// Following the formula of 2.5 workers for available 1000 cpu
func (c *Configurator) paramReplicaParallelWorkers(parameter Parameter) Parameter {
	cpu := c.reference.cpusMySQL
	value, _ := strconv.Atoi(parameter.Default)

	if cpu > 1000 {
		proposedWorkers := int((cpu / 1000) * 2.5)
		if proposedWorkers > value {
			value = proposedWorkers
		}
	}
	parameter.Value = strconv.Itoa(value)
	return parameter

}
