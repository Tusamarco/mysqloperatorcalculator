package mysqloperatorcalculator

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"code.cloudfoundry.org/bytefmt"
)

// ***********************************
// Constants
// ***********************************
const (
	VERSION = "v1.11.0"

	OkI                    = 1001
	ClosetolimitI          = 2001
	OverutilizingI         = 3001
	ErrorexecI             = 5001
	ConnectionRecalculated = 6001
	ResourcesRecalculated  = 7001

	OkT                       = "Execution was successful and resources match the possible requests"
	ClosetolimitT             = "Execution was successful however resources are close to saturation based on the load requested"
	OverutilizingT            = "Resources not enough to cover the requested load "
	ErrorexecT                = "There is an error while processing. See details: %s"
	ConnectionRecalculatedTxt = "The number of connection has been recalculated to match the available resources"
	ResourcesRecalculatedTxt  = "All resources have been recalculated to match the requested connections"

	LoadTypeMostlyReads      = 1
	LoadTypeSomeWrites       = 2
	LoadTypeEqualReadsWrites = 3
	LoadTypeHeavyWrites      = 4

	DimensionOpen       = 999
	ConnectionDimension = 998

	FamilyTypeMysql   = "mysql"
	FamilyTypeProxy   = "proxy"
	FamilyTypeMonitor = "monitor"

	GroupNameMySQLd    = "mysqld"
	GroupNameProbes    = "probes"
	GroupNameResources = "resources"
	GroupNameHAProxy   = "haproxyConfig"

	DbTypePXC              = "pxc"
	DbTypeGroupReplication = "group_replication"

	ResultOutputFormatJson  = "json"
	ResultOutputFormatHuman = "human"

	InnoDBPctValuePXC = 0.80
	InnoDBPctValueGR  = 0.68

	GroupRepGCSCacheMemStructureCost = 52428800

	MinLimitPXC            = 0.45
	MinLimitGR             = 0.40
	MemoryFreeMinimumLimit = 0.06

	GcacheFootPrintFactorRead       = 0.5
	GcacheFootPrintFactorLightWrite = 0.6
	GcacheFootPrintFactorReadWrite  = 0.8

	GCSWeightRead           = 0.20
	GCSWeightReadLightWrite = 0.50
	GCSWeightReadWrite      = 0.60
	GCSWeightReadHeavyWrite = 1

	CPUIncrement    = 500
	MemoryIncrement = 500

	CpuConncetionMillFactorRead           = 1.2
	CpuConncetionMillFactorReadWriteLight = 2.2
	CpuConncetionMillFactorReadWriteEqual = 3.6
	CpuConncetionMillFactorReadWriteHeavy = 4

	ConnectionWeighPctLimit = 0.50
	MinConnectionNumber     = 20
)

//*********************************
// Structure definitions
//********************************

type Version struct {
	Major int `json:"major"`
	Minor int `json:"minor"`
	Patch int `json:"patch"`
}

type MySQLVersions struct {
	Min Version `json:"min"`
	Max Version `json:"max"`
}

type ResponseMessage struct {
	MType int    `json:"type"`
	MName string `json:"name"`
	MText string `json:"text"`
}

type Configuration struct {
	DBType          []string      `json:"dbtype"`
	Dimension       []Dimension   `json:"dimension"`
	LoadType        []LoadType    `json:"loadtype"`
	Connections     []int         `json:"connections"`
	Output          []string      `json:"output"`
	Mysqlversions   MySQLVersions `json:"mysqlversions"`
	ProviderCostPct float64       `json:"providercostpct"`
}

type ConfigurationRequest struct {
	DBType          string    `json:"dbtype"`
	Dimension       Dimension `json:"dimension"`
	LoadType        LoadType  `json:"loadtype"`
	Connections     int       `json:"connections"`
	Output          string    `json:"output"`
	Mysqlversion    Version   `json:"mysqlversion"`
	ProviderCostPct float64   `json:"providercostpct"`
}

type Dimension struct {
	Id          int     `json:"id"`
	Name        string  `json:"name"`
	Cpu         int     `json:"cpu"`
	Memory      string  `json:"memory"`
	MemoryBytes float64 `json:"-"`
	MysqlCpu    int     `json:"mysqlCpu"`
	ProxyCpu    int     `json:"proxyCpu"`
	PmmCpu      int     `json:"pmmCpu"`
	MysqlMemory float64 `json:"mysqlMemory"`
	ProxyMemory float64 `json:"proxyMemory"`
	PmmMemory   float64 `json:"pmmMemory"`
}

type LoadType struct {
	Id      int    `json:"id"`
	Name    string `json:"name"`
	Example string `json:"example"`
}

type Parameter struct {
	Name          string        `yaml:"name" json:"name"`
	Section       string        `yaml:"section" json:"section"`
	Group         string        `yaml:"group" json:"group"`
	Value         string        `yaml:"value" json:"value"`
	Default       string        `yaml:"default" json:"default"`
	Min           uint64        `yaml:"min" json:"min"`
	Max           uint64        `yaml:"max" json:"max"`
	Mysqlversions MySQLVersions `json:"mysqlversions"`
}

func (p Parameter) MarshalJSON() ([]byte, error) {
	// Create an inline anonymous struct with only the fields you want

	return json.Marshal(&struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}{
		Name:  p.Name,
		Value: p.Value,
	})
}

type GroupObj struct {
	Name       string               `yaml:"name" json:"name"`
	Parameters map[string]Parameter `yaml:"parameters" json:"parameters"`
}

type Family struct {
	Name   string              `yaml:"name" json:"name"`
	Groups map[string]GroupObj `yaml:"groups" json:"groups"`
}

func (conf *Configuration) GetDimensionByID(id int) Dimension {
	for _, dim := range conf.Dimension {
		if dim.Id == id {
			return dim
		}
	}
	return Dimension{}
}

func (conf *Configuration) GetLoadByID(id int) LoadType {
	for _, load := range conf.LoadType {
		if load.Id == id {
			return load
		}
	}
	return LoadType{}
}

func (respM *ResponseMessage) GetMessageText(id int) string {
	switch id {
	case OkI:
		return OkT
	case ClosetolimitI:
		return ClosetolimitT
	case OverutilizingI:
		return OverutilizingT
	case ErrorexecI:
		return ErrorexecT
	case ConnectionRecalculated:
		return ConnectionRecalculatedTxt
	case ResourcesRecalculated:
		return ResourcesRecalculatedTxt
	default:
		return "Unhandled message ID"
	}
}

func (conf *Configuration) Init() {
	conf.DBType = []string{DbTypeGroupReplication, DbTypePXC}
	conf.Output = []string{ResultOutputFormatHuman, ResultOutputFormatJson}
	conf.Dimension = []Dimension{
		{1, "XSmall", 1000, "2GB", 2147483648, 600, 200, 100, 1825361100, 214748364, 107374182},
		{2, "Small", 2500, "4GB", 4294967296, 2000, 350, 150, 3758096384, 429496729, 107374182},
		{3, "Medium", 4500, "8GB", 8589934592, 3800, 500, 200, 7516192768, 751619276, 322122547},
		{4, "Large", 6500, "16GB", 17179869184, 5500, 700, 300, 15032385536, 1610612736, 536870912},
		{5, "2XLarge", 8500, "32GB", 34359738368, 7400, 800, 300, 32212254720, 1610612736, 536870912},
		{6, "4XLarge", 16000, "64GB", 68719476736, 14000, 1500, 500, 66571993088, 1610612736, 536870912},
		{7, "8XLarge", 32000, "128GB", 137438953472, 29000, 2000, 1000, 135291469824, 1610612736, 536870912},
		{8, "12XLarge", 48000, "192GB", 206158430208, 45000, 2000, 1000, 204010946560, 1610612736, 536870912},
		{9, "16XLarge", 64000, "256GB", 274877906944, 60000, 3000, 1000, 271656681472, 2147483648, 1073741824},
		{10, "24XLarge", 96000, "384GB", 412316860416, 90000, 4000, 2000, 408021893120, 2684354560, 1610612736},
		{DimensionOpen, "Open request by resources", 0, "0GB", 0, 0, 0, 0, 0, 0, 0},
		{ConnectionDimension, "Open request by Connection", 0, "0GB", 0, 0, 0, 0, 0, 0, 0},
	}

	conf.LoadType = []LoadType{
		{1, "Mainly Reads", "Blogs ~10% Writes 90% Reads"},
		{2, "Light OLTP", "Shops online up to ~40% Writes "},
		{3, "Heavy OLTP", "Intense analytics, telephony, gaming. 30/70% Reads and Writes"},
	}

	conf.Connections = []int{50, 100, 200, 500, 1000, 2000}
	conf.getMySQLVersion()
}

func (family *Family) Init(DBTypeRequest string) map[string]Family {
	// Group declarations shortened for brevity, functionally identical
	replicaGroup := map[string]Parameter{
		"replica_compressed_protocol":   {"replica_compressed_protocol", "configuration", "replication", "1", "1", 0, 1, MySQLVersions{Version{8, 0, 30}, Version{9, 7, 0}}},
		"replica_exec_mode":             {"replica_exec_mode", "configuration", "replication", "STRICT", "STRICT", 0, 0, MySQLVersions{Version{8, 0, 30}, Version{9, 7, 0}}},
		"replica_parallel_type":         {"replica_parallel_type", "configuration", "replication", "LOGICAL_CLOCK", "LOGICAL_CLOCK", 0, 0, MySQLVersions{Version{8, 0, 30}, Version{9, 7, 0}}},
		"replica_parallel_workers":      {"replica_parallel_workers", "configuration", "replication", "4", "4", 0, 1024, MySQLVersions{Version{8, 0, 30}, Version{9, 7, 0}}},
		"replica_preserve_commit_order": {"replica_preserve_commit_order", "configuration", "replication", "ON", "ON", 0, 1, MySQLVersions{Version{8, 0, 30}, Version{9, 7, 0}}},
	}
	connectionGroup := map[string]Parameter{
		"binlog_cache_size":      {"binlog_cache_size", "configuration", "connection", "32768", "32768", 32768, 0, MySQLVersions{Version{8, 0, 30}, Version{10, 1, 0}}},
		"binlog_stmt_cache_size": {"binlog_stmt_cache_size", "configuration", "connection", "32768", "32768", 32768, 0, MySQLVersions{Version{8, 0, 30}, Version{10, 1, 0}}},
		"join_buffer_size":       {"join_buffer_size", "configuration", "connection", "262144", "262144", 262144, 0, MySQLVersions{Version{8, 0, 30}, Version{10, 1, 0}}},
		"read_rnd_buffer_size":   {"read_rnd_buffer_size", "configuration", "connection", "262144", "262144", 262144, 0, MySQLVersions{Version{8, 0, 30}, Version{10, 1, 0}}},
		"sort_buffer_size":       {"sort_buffer_size", "configuration", "connection", "524288", "524288", 524288, 0, MySQLVersions{Version{8, 0, 30}, Version{10, 1, 0}}},
		"max_heap_table_size":    {"max_heap_table_size", "configuration", "connection", "16777216", "16777216", 16777216, 0, MySQLVersions{Version{8, 0, 30}, Version{10, 1, 0}}},
		"tmp_table_size":         {"tmp_table_size", "configuration", "connection", "16777216", "16777216", 16777216, 0, MySQLVersions{Version{8, 0, 30}, Version{10, 1, 0}}},
	}
	serverGroup := map[string]Parameter{
		"max_connections":                   {"max_connections", "configuration", "server", "50", "2", 2, 65536, MySQLVersions{Version{8, 0, 30}, Version{10, 1, 0}}},
		"table_definition_cache":            {"table_definition_cache", "configuration", "server", "4096", "4096", 400, 524288, MySQLVersions{Version{8, 0, 30}, Version{10, 1, 0}}},
		"table_open_cache":                  {"table_open_cache", "configuration", "server", "4096", "4096", 400, 524288, MySQLVersions{Version{8, 0, 30}, Version{10, 1, 0}}},
		"thread_stack":                      {"thread_stack", "configuration", "server", "1048576", "1048576", 131072, 393216, MySQLVersions{Version{8, 0, 30}, Version{10, 1, 0}}},
		"table_open_cache_instances":        {"table_open_cache_instances", "configuration", "server", "4", "16", 1, 64, MySQLVersions{Version{8, 0, 30}, Version{10, 1, 0}}},
		"tablespace_definition_cache":       {"tablespace_definition_cache", "configuration", "server", "512", "256", 256, 524288, MySQLVersions{Version{8, 0, 30}, Version{10, 1, 0}}},
		"sync_binlog":                       {"sync_binlog", "configuration", "server", "1", "1", 0, 1, MySQLVersions{Version{8, 0, 30}, Version{10, 1, 0}}},
		"sql_mode":                          {"sql_mode", "configuration", "server", "'ONLY_FULL_GROUP_BY,STRICT_TRANS_TABLES,NO_ZERO_IN_DATE,NO_ZERO_DATE,ERROR_FOR_DIVISION_BY_ZERO,NO_ENGINE_SUBSTITUTION,TRADITIONAL,STRICT_ALL_TABLES'", "0", 0, 1, MySQLVersions{Version{8, 0, 30}, Version{10, 1, 0}}},
		"binlog_expire_logs_seconds":        {"binlog_expire_logs_seconds", "configuration", "server", "604800", "0", 0, 0, MySQLVersions{Version{8, 0, 30}, Version{10, 1, 0}}},
		"binlog_format":                     {"binlog_format", "configuration", "server", "ROW", "0", 0, 0, MySQLVersions{Version{8, 0, 30}, Version{10, 1, 0}}},
		"thread_cache_size":                 {"thread_cache_size", "configuration", "server", "8", "8", 4, 16384, MySQLVersions{Version{8, 0, 30}, Version{10, 1, 0}}},
		"global-connection-memory-limit":    {"global_connection_memory_limit", "configuration", "server", "18446744073709551615", "16777216", 4, 18446744073709551615, MySQLVersions{Version{8, 0, 30}, Version{10, 1, 0}}},
		"global-connection-memory-tracking": {"global_connection_memory_tracking", "configuration", "server", "false", "false", 0, 1, MySQLVersions{Version{8, 0, 30}, Version{10, 1, 0}}},
	}
	innodbGroup := map[string]Parameter{
		"innodb_adaptive_hash_index":     {"innodb_adaptive_hash_index", "configuration", "innodb", "0", "0", 0, 1, MySQLVersions{Version{8, 0, 30}, Version{10, 1, 0}}},
		"innodb_buffer_pool_size":        {"innodb_buffer_pool_size", "configuration", "innodb", "1073741824", "134217728", 5242880, 0, MySQLVersions{Version{8, 0, 30}, Version{10, 1, 0}}},
		"innodb_ddl_threads":             {"innodb_ddl_threads", "configuration", "innodb", "2", "4", 1, 64, MySQLVersions{Version{8, 0, 30}, Version{10, 1, 0}}},
		"innodb_buffer_pool_instances":   {"innodb_buffer_pool_instances", "configuration", "innodb", "1", "8", 1, 64, MySQLVersions{Version{8, 0, 30}, Version{10, 1, 0}}},
		"innodb_flush_method":            {"innodb_flush_method", "configuration", "innodb", "O_DIRECT", "O_DIRECT", 0, 0, MySQLVersions{Version{8, 0, 30}, Version{10, 1, 0}}},
		"innodb_flush_log_at_trx_commit": {"innodb_flush_log_at_trx_commit", "configuration", "innodb", "2", "1", 0, 2, MySQLVersions{Version{8, 0, 30}, Version{10, 2, 0}}},
		"innodb_log_file_size":           {"innodb_log_file_size", "configuration", "innodb", "119537664", "50331648", 4194304, 0, MySQLVersions{Version{8, 0, 27}, Version{8, 0, 30}}},
		"innodb_log_files_in_group":      {"innodb_log_files_in_group", "configuration", "innodb", "2", "2", 2, 100, MySQLVersions{Version{8, 0, 27}, Version{8, 0, 30}}},
		"innodb_redo_log_capacity":       {"innodb_redo_log_capacity", "configuration", "innodb", "119537664", "104857600", 8388608, 137438953472, MySQLVersions{Version{8, 0, 31}, Version{10, 1, 0}}},
		"innodb_page_cleaners":           {"innodb_page_cleaners", "configuration", "innodb", "1", "4", 1, 64, MySQLVersions{Version{8, 0, 30}, Version{10, 1, 0}}},
		"innodb_purge_threads":           {"innodb_purge_threads", "configuration", "innodb", "1", "4", 1, 32, MySQLVersions{Version{8, 0, 30}, Version{10, 1, 0}}},
		"innodb_io_capacity_max":         {"innodb_io_capacity_max", "configuration", "innodb", "20000", "20000", 100, 0, MySQLVersions{Version{8, 0, 30}, Version{8, 8, 0}}},
		"innodb_numa_interleave":         {"innodb_numa_interleave", "configuration", "innodb", "0", "1", 0, 0, MySQLVersions{Version{8, 0, 30}, Version{8, 8, 0}}},
		"innodb_buffer_pool_chunk_size":  {"innodb_buffer_pool_chunk_size", "configuration", "innodb", "2097152", "134217728", 1048576, 0, MySQLVersions{Version{8, 0, 30}, Version{10, 1, 0}}},
		"innodb_parallel_read_threads":   {"innodb_parallel_read_threads", "configuration", "innodb", "1", "4", 1, 256, MySQLVersions{Version{8, 0, 30}, Version{10, 1, 0}}},
		"innodb_monitor_enable":          {"innodb_monitor_enable", "configuration", "innodb", "ALL", "ALL", 0, 0, MySQLVersions{Version{8, 0, 30}, Version{10, 1, 0}}},
	}
	wsrepGroup := map[string]Parameter{
		"wsrep_sync_wait":         {"wsrep_sync_wait", "configuration", "galera", "0", "0", 0, 8, MySQLVersions{Version{8, 0, 30}, Version{10, 1, 0}}},
		"wsrep_slave_threads":     {"wsrep_slave_threads", "configuration", "galera", "2", "1", 1, 0, MySQLVersions{Version{8, 0, 30}, Version{10, 1, 0}}},
		"wsrep_trx_fragment_size": {"wsrep_trx_fragment_size", "configuration", "galera", "1048576", "0", 0, 0, MySQLVersions{Version{8, 0, 30}, Version{10, 1, 0}}},
		"wsrep_trx_fragment_unit": {"wsrep_trx_fragment_unit", "configuration", "galera", "bytes", "bytes", 0, 0, MySQLVersions{Version{8, 0, 30}, Version{10, 1, 0}}},
		"wsrep-provider-options":  {"wsrep-provider-options", "configuration", "galera", "<placeholder>", "", 0, 0, MySQLVersions{Version{8, 0, 30}, Version{10, 1, 0}}},
	}
	groupReplicationGroup := map[string]Parameter{
		"loose_group_replication_autorejoin_tries":               {"loose_group_replication_autorejoin_tries", "configuration", "groupReplication", "2", "3", 0, 8, MySQLVersions{Version{8, 0, 30}, Version{10, 1, 0}}},
		"loose_group_replication_flow_control_period":            {"loose_group_replication_flow_control_period", "configuration", "groupReplication", "1", "1", 1, 5, MySQLVersions{Version{8, 0, 30}, Version{10, 1, 0}}},
		"loose_group_replication_message_cache_size":             {"loose_group_replication_message_cache_size", "configuration", "groupReplication", "134217728", "1073741824", 134217728, 18446744073709551615, MySQLVersions{Version{8, 0, 30}, Version{10, 1, 0}}},
		"loose_group_replication_communication_max_message_size": {"loose_group_replication_communication_max_message_size", "configuration", "groupReplication", "5097152", "10485760", 0, 1073741824, MySQLVersions{Version{8, 0, 30}, Version{10, 1, 0}}},
		"loose_group_replication_member_expel_timeout":           {"loose_group_replication_member_expel_timeout", "configuration", "groupReplication", "15", "5", 0, 3600, MySQLVersions{Version{8, 0, 30}, Version{10, 1, 0}}},
		"loose_group_replication_poll_spin_loops":                {"loose_group_replication_poll_spin_loops", "configuration", "groupReplication", "0", "0", 10000, 40000, MySQLVersions{Version{8, 0, 30}, Version{10, 1, 0}}},
		"loose_group_replication_paxos_single_leader":            {"loose_group_replication_paxos_single_leader", "configuration", "groupReplication", "ON", "OFF", 0, 1, MySQLVersions{Version{8, 0, 30}, Version{10, 1, 0}}},
		"loose_binlog_transaction_dependency_tracking":           {"loose_binlog_transaction_dependency_tracking", "configuration", "groupReplication", "WRITESET", "COMMIT_ORDER", 0, 0, MySQLVersions{Version{8, 0, 30}, Version{8, 3, 0}}},
	}

	mysqlGroups := map[string]GroupObj{
		"readinessProbe": {"readinessProbe", map[string]Parameter{"timeoutSeconds": {"timeoutSeconds", "", "readinessProbe", "15", "15", 15, 600, MySQLVersions{}}}},
		"livenessProbe":  {"livenessProbe", map[string]Parameter{"timeoutSeconds": {"timeoutSeconds", "", "readinessProbe", "5", "5", 5, 600, MySQLVersions{}}}},
		"resources": {"resources", map[string]Parameter{
			"request_memory": {"memory", "request", "resources", "2", "2", 2, 32, MySQLVersions{}},
			"request_cpu":    {"cpu", "request", "resources", "1000", "1000", 1000, 8500, MySQLVersions{}},
			"limit_memory":   {"memory", "limit", "resources", "2", "2", 2, 32, MySQLVersions{}},
			"limit_cpu":      {"cpu", "limit", "resources", "1000", "1000", 1000, 8500, MySQLVersions{}},
		}},
	}

	haproxyGroups := map[string]GroupObj{
		"readinessProbe": {"readinessProbe", map[string]Parameter{"timeoutSeconds": {"timeoutSeconds", "", "readinessProbe", "5", "5", 5, 30, MySQLVersions{}}}},
		"livenessProbe":  {"livenessProbe", map[string]Parameter{"timeoutSeconds": {"timeoutSeconds", "", "readinessProbe", "5", "5", 5, 60, MySQLVersions{}}}},
		"haproxyConfig": {"haproxy", map[string]Parameter{
			"ha_connection_timeout": {"ha_connection_timeout", "", "haproxyConfig", "5", "1000", 1000, 5000, MySQLVersions{}},
			"maxconn":               {"maxconn", "", "haproxyConfig", "4048", "2024", 1000, 5000, MySQLVersions{}},
			"timeout_client":        {"timeout_client", "", "haproxyConfig", "28800", "14400", 1000, 50000, MySQLVersions{}},
			"timeout_connect":       {"timeout_connect", "", "haproxyConfig", "100500", "100500", 1000, 500000, MySQLVersions{}},
			"timeout_server":        {"timeout_server", "", "haproxyConfig", "28800", "14400", 1000, 50000, MySQLVersions{}},
		}},
		"resources": {"resources", map[string]Parameter{
			"request_memory": {"memory", "request", "resources", "1", "1", 1, 2, MySQLVersions{}},
			"request_cpu":    {"cpu", "request", "resources", "1000", "1000", 1000, 2000, MySQLVersions{}},
			"limit_memory":   {"memory", "limit", "resources", "1", "!", 1, 2, MySQLVersions{}},
			"limit_cpu":      {"cpu", "limit", "resources", "1000", "1000", 1000, 2000, MySQLVersions{}},
		}},
	}

	pmmGroups := map[string]GroupObj{
		"readinessProbe": {"readinessProbe", map[string]Parameter{"timeoutSeconds": {"timeoutSeconds", "", "readinessProbe", "5", "5", 5, 30, MySQLVersions{}}}},
		"livenessProbe":  {"livenessProbe", map[string]Parameter{"timeoutSeconds": {"timeoutSeconds", "", "readinessProbe", "5", "5", 5, 60, MySQLVersions{}}}},
		"resources": {"resources", map[string]Parameter{
			"request_memory": {"memory", "request", "resources", "1", "1", 1, 2, MySQLVersions{}},
			"request_cpu":    {"cpu", "request", "resources", "1000", "1000", 100, 2000, MySQLVersions{}},
			"limit_memory":   {"memory", "limit", "resources", "1", "!", 1, 2, MySQLVersions{}},
			"limit_cpu":      {"cpu", "limit", "resources", "1000", "1000", 100, 2000, MySQLVersions{}},
		}},
	}

	mysqlGroups["configuration_connection"] = GroupObj{"connections", connectionGroup}
	mysqlGroups["configuration_server"] = GroupObj{"server", serverGroup}
	mysqlGroups["configuration_innodb"] = GroupObj{"innodb", innodbGroup}
	mysqlGroups["configuration_replica"] = GroupObj{"replica", replicaGroup}

	if DBTypeRequest == DbTypePXC {
		mysqlGroups["configuration_galera"] = GroupObj{"galera", wsrepGroup}
	}

	if DBTypeRequest == DbTypeGroupReplication {
		mysqlGroups["configuration_groupReplication"] = GroupObj{"groupReplication", groupReplicationGroup}
	}

	return map[string]Family{
		FamilyTypeMysql:   {"mysql", mysqlGroups},
		FamilyTypeProxy:   {"haproxy", haproxyGroups},
		FamilyTypeMonitor: {"pmm", pmmGroups},
	}
}

type ProviderParam struct {
	Name     string
	Literal  string
	Value    int64
	Defvalue int64
	RMin     int64
	RMax     int64
}

func (pP *ProviderParam) Init() map[string]ProviderParam {
	return map[string]ProviderParam{
		"pc.recovery":               {"pc.recovery", "true", -1, 0, 0, 0},
		"gcache.size":               {"gcache.size", "%s", 0, 0, 0, 0},
		"gcache.recover":            {"gcache.recover", "yes", -1, 0, 0, 0},
		"evs.delayed_keep_period":   {"evs.delayed_keep_period", "PT5%sS", 0, 30, 30, 60},
		"evs.delay_margin":          {"evs.delay_margin", "PT%sS", 0, 1, 1, 30},
		"evs.send_window":           {"evs.send_window", "%s", 0, 4, 4, 1024},
		"evs.user_send_window":      {"evs.user_send_window", "%s", 0, 2, 2, 1024},
		"evs.inactive_check_period": {"evs.inactive_check_period", "PT%sS", 0, 1, 1, 5},
		"evs.inactive_timeout":      {"evs.inactive_timeout", "PT%sS", 0, 15, 15, 120},
		"evs.join_retrans_period":   {"evs.join_retrans_period", "PT%sS", 0, 1, 1, 5},
		"evs.suspect_timeout":       {"evs.suspect_timeout", "PT%sS", 0, 5, 5, 60},
		"evs.stats_report_period":   {"evs.stats_report_period", "PT%sM", 0, 1, 1, 1},
		"gcs.fc_limit":              {"gcs.fc_limit", "%s", 0, 16, 16, 128},
		"gcs.max_packet_size":       {"gcs.max_packet_size", "%s", 0, 32616, 32616, 131072},
		"gmcast.peer_timeout":       {"gmcast.peer_timeout", "PT%sS", 0, 3, 3, 15},
		"gmcast.time_wait":          {"gmcast.time_wait", "PT%sS", 0, 5, 5, 18},
		"evs.max_install_timeouts":  {"evs.max_install_timeouts", "%s", 0, 1, 1, 5},
		"pc.announce_timeout":       {"pc.announce_timeout", "PT%sS", 0, 3, 3, 60},
		"pc.linger":                 {"pc.linger", "PT%sS", 0, 2, 2, 60},
	}
}

// ParseGroupsHuman returns all the groups in the family in one shot as a byte buffer
func (f Family) ParseGroupsHuman() bytes.Buffer {
	var b bytes.Buffer
	skipCommon := false
	padding := "    "
	fmt.Fprintf(&b, "[%s]\n", f.Name)

	// 1. Extract and sort keys
	keys := make([]string, 0, len(f.Groups))
	for k := range f.Groups {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// 2. Iterate in alphabetical order
	for _, key := range keys {
		if skipCommon || (key == "readinessProbe" || key == "livenessProbe" || key == "resources") {
			continue
		}

		group := f.Groups[key]
		f.parseParamsHuman(&b, group, padding)
	}
	for _, key := range keys {
		if key == "readinessProbe" || key == "livenessProbe" || key == "resources" {
			group := f.Groups[key]
			fmt.Fprintf(&b, "    [%s]\n", group.Name)
			f.parseParamsHuman(&b, group, "        ")
		}

	}

	return b
}

// ParseFamilyGroup returns the group by name as a byte buffer
func (f Family) ParseFamilyGroup(groupName string, padding string) (bytes.Buffer, error) {
	switch groupName {
	case GroupNameMySQLd:
		return f.ParseGroupsHuman(), nil // f.parseGroupHuman("[mysqld]", padding, true), nil
	case GroupNameHAProxy:
		return f.ParseGroupsHuman(), nil //f.parseGroupHuman("", padding, true), nil
	case GroupNameProbes:
		return f.parseProbesHuman(padding), nil
	case GroupNameResources:
		return f.parseResourcesHuman(padding), nil
	default:
		return bytes.Buffer{}, fmt.Errorf("ERROR: Invalid Group name %s", groupName)
	}
}

func (f Family) parseProbesHuman(padding string) bytes.Buffer {
	var b bytes.Buffer
	for key, group := range f.Groups {
		if key == "readinessProbe" || key == "livenessProbe" {
			fmt.Fprintf(&b, "[%s]\n", key)
			f.parseParamsHuman(&b, group, padding)
		}
	}
	return b
}

func (f Family) parseResourcesHuman(padding string) bytes.Buffer {
	var b bytes.Buffer
	if group, ok := f.Groups["resources"]; ok {
		fmt.Fprintf(&b, "[resources]\n")
		f.parseParamsHuman(&b, group, padding)
	}
	return b
}

func (f Family) parseParamsHuman(b *bytes.Buffer, group GroupObj, padding string) {

	keys := make([]string, 0, len(group.Parameters))
	for k := range group.Parameters {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// 2. Iterate in alphabetical order
	for _, key := range keys {
		fmt.Fprintf(b, "%s%s = %s\n", padding, key, group.Parameters[key].Value)
	}

}

func (conf *Configuration) CalculateOpenDimension(dimension Dimension) Dimension {
	if dimension.Cpu > 0 && dimension.MemoryBytes > 0 {
		calcDimension := conf.getDimensionForFreeCalculation(dimension)

		// Calculate ratios
		ratioMysqlCpu := float64(calcDimension.MysqlCpu) / float64(calcDimension.Cpu)
		ratioProxyCpu := float64(calcDimension.ProxyCpu) / float64(calcDimension.Cpu)
		ratioPmmCpu := float64(calcDimension.PmmCpu) / float64(calcDimension.Cpu)

		ratioMysqlMem := calcDimension.MysqlMemory / calcDimension.MemoryBytes
		ratioProxyMem := calcDimension.ProxyMemory / calcDimension.MemoryBytes
		ratioPmmMem := calcDimension.PmmMemory / calcDimension.MemoryBytes

		dimension.MysqlCpu = int(float64(dimension.Cpu) * ratioMysqlCpu)
		dimension.ProxyCpu = int(float64(dimension.Cpu) * ratioProxyCpu)
		dimension.PmmCpu = int(float64(dimension.Cpu) * ratioPmmCpu)

		dimension.MysqlMemory = dimension.MemoryBytes * ratioMysqlMem
		dimension.ProxyMemory = dimension.MemoryBytes * ratioProxyMem
		dimension.PmmMemory = dimension.MemoryBytes * ratioPmmMem
	}

	return dimension
}

// getDimensionForFreeCalculation returns the dimension that is closer to the request
func (conf *Configuration) getDimensionForFreeCalculation(dimension Dimension) Dimension {
	for i := 0; i < len(conf.Dimension); i++ {
		curr := conf.Dimension[i]
		if curr.Cpu == 0 {
			// Skip DimensionOpen and ConnectionDimension (usually the last 2 items)
			continue
		}

		if i == 0 && (dimension.Cpu <= curr.Cpu || dimension.MemoryBytes <= curr.MemoryBytes) {
			return curr
		}

		if i > 0 {
			prev := conf.Dimension[i-1]
			if InBetweenFloat(float64(dimension.Cpu), float64(prev.Cpu), float64(curr.Cpu)) ||
				InBetweenFloat(dimension.MemoryBytes, prev.MemoryBytes, curr.MemoryBytes) {
				return prev
			}
		}

		// If this is the last valid standard size, and the dimension is larger
		if i+1 < len(conf.Dimension) && conf.Dimension[i+1].Cpu == 0 {
			if dimension.Cpu >= curr.Cpu || dimension.MemoryBytes >= curr.MemoryBytes {
				return curr
			}
		}
	}
	// Fallback
	return conf.Dimension[0]
}

func InBetweenFloat(val, min, max float64) bool {
	return val >= min && val <= max
}

func (conf *Configuration) getMySQLVersion() {
	conf.Mysqlversions.Max = Version{10, 10, 0}
	conf.Mysqlversions.Min = Version{8, 0, 32}
}

// =====================================================
// Dimension section
// =====================================================
func (d *Dimension) ConvertMemoryToBytes(memoryHuman string) (float64, error) {
	b, err := bytefmt.ToBytes(memoryHuman)
	if err != nil {
		return 0, err
	}
	return float64(b), nil
}

// formatMemoryMB converts bytes to a string in megabytes (MB).
func (d *Dimension) formatMemoryMB(bytes float64) string {
	megabytes := bytes / 1000 / 1000
	return fmt.Sprintf("%.0fMB", megabytes)
}

// ScaleDimension increases the resources of the starting dimension by
// 100 CPU and 500 MB, allocating the extra resources to sub‑components
func (conf *Configuration) ScaleDimension(start Dimension) (Dimension, error) {
	// Find the higher predefined dimension securely
	var higher Dimension
	var id = start.Id
	found := false

	for i, dim := range conf.Dimension {
		if dim.Id == start.Id {
			if i+1 < len(conf.Dimension) && conf.Dimension[i+1].Cpu > 0 {
				higher = conf.Dimension[i+1]
				id = higher.Id // Update target id to higher dimension id
				found = true
			}
			break
		}
	}

	if !found {
		return Dimension{}, errors.New("higher dimension not found or target is invalid")
	}
	if higher.Cpu == 0 || higher.MemoryBytes == 0 {
		return Dimension{}, errors.New("higher dimension has zero total resources")
	}

	// Calculate resource percentages from the higher dimension.
	pctMysqlCPU := float64(higher.MysqlCpu) / float64(higher.Cpu)
	pctProxyCPU := float64(higher.ProxyCpu) / float64(higher.Cpu)
	pctPmmCPU := float64(higher.PmmCpu) / float64(higher.Cpu)

	pctMysqlMem := higher.MysqlMemory / higher.MemoryBytes
	pctProxyMem := higher.ProxyMemory / higher.MemoryBytes
	pctPmmMem := higher.PmmMemory / higher.MemoryBytes

	// Increment totals.
	const cpuIncrement = CPUIncrement
	const memIncrementBytes = MemoryIncrement * 1_000_000 // 500 MB in bytes (1 MB = 10^6 bytes)

	newCPU := start.Cpu + cpuIncrement
	newMemoryBytes := start.MemoryBytes + float64(memIncrementBytes)

	// Compute component increments based on percentages.
	incMysqlCPU := int(float64(cpuIncrement) * pctMysqlCPU)
	incProxyCPU := int(float64(cpuIncrement) * pctProxyCPU)
	incPmmCPU := int(float64(cpuIncrement) * pctPmmCPU)

	incMysqlMem := float64(memIncrementBytes) * pctMysqlMem
	incProxyMem := float64(memIncrementBytes) * pctProxyMem
	incPmmMem := float64(memIncrementBytes) * pctPmmMem

	// Adjust CPU increments to ensure the sum exactly equals cpuIncrement (handles rounding errors).
	totalIncCPU := incMysqlCPU + incProxyCPU + incPmmCPU
	if diff := cpuIncrement - totalIncCPU; diff != 0 {
		switch {
		case incMysqlCPU >= incProxyCPU && incMysqlCPU >= incPmmCPU:
			incMysqlCPU += diff
		case incProxyCPU >= incMysqlCPU && incProxyCPU >= incPmmCPU:
			incProxyCPU += diff
		default:
			incPmmCPU += diff
		}
	}

	// If the new memory set is equal or higher the next Dimension we shift the id
	if newMemoryBytes < higher.MemoryBytes {
		id = start.Id // Keep original ID if threshold is not crossed
	}

	// Build the new dimension.
	return Dimension{
		Id:          id,
		Name:        "scaled",
		Cpu:         newCPU,
		MemoryBytes: newMemoryBytes,
		Memory:      start.formatMemoryMB(newMemoryBytes), // Call using start as receiver
		MysqlCpu:    start.MysqlCpu + incMysqlCPU,
		ProxyCpu:    start.ProxyCpu + incProxyCPU,
		PmmCpu:      start.PmmCpu + incPmmCPU,
		MysqlMemory: start.MysqlMemory + incMysqlMem,
		ProxyMemory: start.ProxyMemory + incProxyMem,
		PmmMemory:   start.PmmMemory + incPmmMem,
	}, nil
}
