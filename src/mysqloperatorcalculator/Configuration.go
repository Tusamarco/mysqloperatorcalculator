package mysqloperatorcalculator

import (
	"bytes"
	"code.cloudfoundry.org/bytefmt"
	"errors"
)

// ***********************************
// Constants
// ***********************************
const VERSION = "v1.5.2"

const OkI = 1001
const ClosetolimitI = 2001
const OverutilizingI = 3001
const ErrorexecI = 5001

const OkT = "Execution was successful and resources match the possible requests"
const ClosetolimitT = "Execution was successful however resources are close to saturation based on the load requested"
const OverutilizingT = "Resources not enough to cover the requested load "
const ErrorexecT = "There is an error while processing. See details: %s"

const LoadTypeMostlyReads = 1
const LoadTypeSomeWrites = 2
const LoadTypeEqualReadsWrites = 3
const LoadTypeHeavyWrites = 4

const DimensionOpen = 999

const FamilyTypeMysql = "mysql"
const FamilyTypeProxy = "proxy"
const FamilyTypeMonitor = "monitor"

const GroupNameMySQLd = "mysqld"
const GroupNameProbes = "probes"
const GroupNameResources = "resources"
const GroupNameHAProxy = "haproxyConfig"

const DbTypePXC = "pxc"
const DbTypeGroupReplication = "group_replication"

const ResultOutputFormatJson = "json"
const ResultOutputFormatHuman = "human"

//*********************************
// Structure definitions
//********************************

// MySQL version definition
type Version struct {
	Major int `json:"major"`
	Minor int `json:"minor"`
	Patch int `json:"patch"`
}

type MySQLVersions struct {
	Min Version `json:"min"`
	Max Version `json:"max"`
}

// ResponseMessage Message is the retruned message
type ResponseMessage struct {
	MType int    `json:"type"`
	MName string `json:"name"`
	MText string `json:"text"`
}

// Configuration used to pass available configurations
type Configuration struct {
	DBType        []string      `json:"dbtype"`
	Dimension     []Dimension   `json:"dimension"`
	LoadType      []LoadType    `json:"loadtype"`
	Connections   []int         `json:"connections"`
	Output        []string      `json:"output"`
	Mysqlversions MySQLVersions `json:"mysqlversions"`
}

// ConfigurationRequest used to store the incoming request
type ConfigurationRequest struct {
	DBType       string    `json:"dbtype"`
	Dimension    Dimension `json:"dimension"`
	LoadType     LoadType  `json:"loadtype"`
	Connections  int       `json:"connections"`
	Output       string    `json:"output"`
	Mysqlversion Version   `json:"mysqlversion"`
}

// Dimension used to represent the POD dimension
type Dimension struct {
	Id          int    `json:"id"`
	Name        string `json:"name"`
	Cpu         int    `json:"cpu"`
	Memory      string `json:"memory"`
	MemoryBytes float64
	MysqlCpu    int     `json:"mysqlCpu"`
	ProxyCpu    int     `json:"proxyCpu"`
	PmmCpu      int     `json:"pmmCpu"`
	MysqlMemory float64 `json:"mysqlMemory"`
	ProxyMemory float64 `json:"proxyMemory"`
	PmmMemory   float64 `json:"pmmMemory"`
}

// LoadType The different kind of load type
type LoadType struct {
	Id      int    `json:"id"`
	Name    string `json:"name"`
	Example string `json:"example"`
}

// Parameter generic structure to store Parameters values
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

// GroupObj Parameters are groupped by typology
type GroupObj struct {
	Name       string               `yaml:"name" json:"name"`
	Parameters map[string]Parameter `yaml:"parameters" json:"parameters"`
}

// Family Groups are organized by Families
type Family struct {
	Name   string              `yaml:"name" json:"name"`
	Groups map[string]GroupObj `yaml:"groups" json:"groups"`
}

// GetDimensionByID returns the Dimension using ID attribute
func (conf *Configuration) GetDimensionByID(id int) Dimension {
	for i := 0; i < len(conf.Dimension); i++ {
		if conf.Dimension[i].Id == id {
			return conf.Dimension[i]
		}

	}
	return Dimension{0, "", 0, "", 0, 0, 0, 0, 0, 0, 0}
}

// GetLoadByID returns the Load Type using ID attribute
func (conf *Configuration) GetLoadByID(id int) LoadType {
	for i := 0; i < len(conf.LoadType); i++ {
		if conf.LoadType[i].Id == id {
			return conf.LoadType[i]
		}

	}
	return LoadType{0, "", ""}

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
	}
	return "Unhandled message ID"
}

// Init here is where we define the different options
// it will be possible to increment the supported solutions adding here the items
func (conf *Configuration) Init() {
	conf.DBType = []string{DbTypeGroupReplication, DbTypePXC}
	conf.Output = []string{ResultOutputFormatHuman, ResultOutputFormatJson}
	conf.Dimension = []Dimension{
		{1, "XSmall", 1000, "2GB", 2147483648, 600, 200, 100, 1825361100, 214748364, 107374182},
		{2, "Small", 2500, "4GB", 4294967296, 2000, 350, 150, 3758096384, 429496729, 107374182},
		{3, "Medium", 4500, "8GB", 8589934592, 3800, 500, 200, 7516192768, 751619276, 322122547},
		{4, "Large", 6500, "16GB", 17179869184, 5500, 700, 300, 15032385536, 1610612736, 536870912},
		{5, "2XLarge", 8500, "32GB", 34359738368, 7400, 800, 300, 32212254720, 1610612736, 536870912},
		{6, "4XLarge", 16000, "64GB", 34359738368, 14000, 1500, 500, 66571993088, 1610612736, 536870912},
		{7, "8XLarge", 32000, "128GB", 137438953472, 29000, 2000, 1000, 135291469824, 1610612736, 536870912},
		{8, "12XLarge", 48000, "192GB", 206158430208, 45000, 2000, 1000, 204010946560, 1610612736, 536870912},
		{9, "16XLarge", 64000, "256GB", 274877906944, 60000, 3000, 1000, 271656681472, 2147483648, 1073741824},
		{10, "24XLarge", 96000, "384GB", 412316860416, 90000, 4000, 2000, 408021893120, 2684354560, 1610612736},
		{DimensionOpen, "Open request", 0, "0GB", 0, 0, 0, 0, 0, 0, 0},
	}
	//		{999, "Open request", 0, 0, 0.875, 0.09375, 0.00025, 0.96875, 0.0234375, 0.0078125},
	conf.LoadType = []LoadType{}

	loadT := make(map[string]int)
	loadT["Mainly Reads"] = 1
	loadT["Light OLTP"] = 2
	loadT["Intense OLTP (50/50 R/W)"] = 3

	conf.LoadType = []LoadType{
		{1, "Mainly Reads", "Blogs ~2% Writes 95% Reads"},
		{2, "Light OLTP", "Shops online  up to 20% Writes "},
		{3, "Heavy OLTP", "Intense analitics, telephony, gaming. 50/50% Reads and Writes"},
	}

	conf.Connections = []int{50, 100, 200, 500, 1000, 2000}

	//MySQL SUpported version (testing is with hardcoded then we need to query check.percona.com
	conf.getMySQLVersion()
}

func (family *Family) Init(DBTypeRequest string) map[string]Family {

	// supported parameters are defined here with defaults value and ranges.
	// to add new we can add here the new one then create a proper method to handle the calculation

	replicaGroup := map[string]Parameter{
		"replica_compressed_protocol": {"replica_compressed_protocol", "configuration", "replication", "1", "1", 0, 1, MySQLVersions{Version{8, 0, 30}, Version{8, 1, 0}}},
		"replica_exec_mode":           {"replica_exec_mode", "configuration", "replication", "STRICT", "STRICT", 0, 0, MySQLVersions{Version{8, 0, 30}, Version{8, 1, 0}}},
		"replica_parallel_type":       {"replica_parallel_type", "configuration", "replication", "LOGICAL_CLOCK", "LOGICAL_CLOCK", 0, 0, MySQLVersions{Version{8, 0, 30}, Version{8, 1, 0}}},
		//TODO add calculation
		"replica_parallel_workers":      {"replica_parallel_workers", "configuration", "replication", "4", "4", 0, 1024, MySQLVersions{Version{8, 0, 30}, Version{8, 1, 0}}},
		"replica_preserve_commit_order": {"replica_preserve_commit_order", "configuration", "replication", "ON", "ON", 0, 1, MySQLVersions{Version{8, 0, 30}, Version{8, 1, 0}}},
	}

	connectionGroup := map[string]Parameter{
		"binlog_cache_size":      {"binlog_cache_size", "configuration", "connection", "32768", "32768", 32768, 0, MySQLVersions{Version{8, 0, 30}, Version{8, 1, 0}}},
		"binlog_stmt_cache_size": {"binlog_stmt_cache_size", "configuration", "connection", "32768", "32768", 32768, 0, MySQLVersions{Version{8, 0, 30}, Version{8, 1, 0}}},
		"join_buffer_size":       {"join_buffer_size", "configuration", "connection", "262144", "262144", 262144, 0, MySQLVersions{Version{8, 0, 30}, Version{8, 1, 0}}},
		"read_rnd_buffer_size":   {"read_rnd_buffer_size", "configuration", "connection", "262144", "262144", 262144, 0, MySQLVersions{Version{8, 0, 30}, Version{8, 1, 0}}},
		"sort_buffer_size":       {"sort_buffer_size", "configuration", "connection", "524288", "524288", 524288, 0, MySQLVersions{Version{8, 0, 30}, Version{8, 1, 0}}},
		"max_heap_table_size":    {"max_heap_table_size", "configuration", "connection", "16777216", "16777216", 16777216, 0, MySQLVersions{Version{8, 0, 30}, Version{8, 1, 0}}},
		"tmp_table_size":         {"tmp_table_size", "configuration", "connection", "16777216", "16777216", 16777216, 0, MySQLVersions{Version{8, 0, 30}, Version{8, 1, 0}}},
	}
	//TODO !!! Review thread_pool_size & thread_cache_size !!!
	serverGroup := map[string]Parameter{
		"max_connections":             {"max_connections", "configuration", "server", "50", "2", 2, 65536, MySQLVersions{Version{8, 0, 30}, Version{8, 1, 0}}},
		"thread_pool_size":            {"thread_pool_size", "configuration", "server", "2", "2", 2, 64, MySQLVersions{Version{8, 0, 30}, Version{8, 1, 0}}},
		"table_definition_cache":      {"table_definition_cache", "configuration", "server", "4096", "4096", 400, 524288, MySQLVersions{Version{8, 0, 30}, Version{8, 1, 0}}},
		"table_open_cache":            {"table_open_cache", "configuration", "server", "4096", "4096", 400, 524288, MySQLVersions{Version{8, 0, 30}, Version{8, 1, 0}}},
		"thread_stack":                {"thread_stack", "configuration", "server", "1048576", "1048576", 131072, 393216, MySQLVersions{Version{8, 0, 30}, Version{8, 1, 0}}},
		"table_open_cache_instances":  {"table_open_cache_instances", "configuration", "server", "4", "16", 1, 64, MySQLVersions{Version{8, 0, 30}, Version{8, 1, 0}}},
		"tablespace_definition_cache": {"tablespace_definition_cache", "configuration", "server", "512", "256", 256, 524288, MySQLVersions{Version{8, 0, 30}, Version{8, 1, 0}}},
		//Adding values to match common advisors checks
		"sync_binlog":                {"sync_binlog", "configuration", "server", "1", "1", 0, 1, MySQLVersions{Version{8, 0, 30}, Version{8, 1, 0}}},
		"sql_mode":                   {"sql_mode", "configuration", "server", "'ONLY_FULL_GROUP_BY,STRICT_TRANS_TABLES,NO_ZERO_IN_DATE,NO_ZERO_DATE,ERROR_FOR_DIVISION_BY_ZERO,NO_ENGINE_SUBSTITUTION,TRADITIONAL,STRICT_ALL_TABLES'", "0", 0, 1, MySQLVersions{Version{8, 0, 30}, Version{8, 1, 0}}},
		"binlog_expire_logs_seconds": {"binlog_expire_logs_seconds", "configuration", "server", "604800", "0", 0, 0, MySQLVersions{Version{8, 0, 30}, Version{8, 1, 0}}},
		"binlog_format":              {"binlog_format", "configuration", "server", "ROW", "0", 0, 0, MySQLVersions{Version{8, 0, 30}, Version{8, 1, 0}}},
		"thread_cache_size":          {"thread_cache_size", "configuration", "server", "8", "8", 4, 16384, MySQLVersions{Version{8, 0, 30}, Version{8, 1, 0}}},
	}

	innodbGroup := map[string]Parameter{
		"innodb_adaptive_hash_index": {"innodb_adaptive_hash_index", "configuration", "innodb", "1", "1", 0, 1, MySQLVersions{Version{8, 0, 30}, Version{8, 1, 0}}},
		"innodb_buffer_pool_size":    {"innodb_buffer_pool_size", "configuration", "innodb", "1073741824", "134217728", 5242880, 0, MySQLVersions{Version{8, 0, 30}, Version{8, 1, 0}}},
		//TODO implement method
		"innodb_ddl_threads":             {"innodb_ddl_threads", "configuration", "innodb", "2", "4", 1, 64, MySQLVersions{Version{8, 0, 30}, Version{8, 1, 0}}},
		"innodb_buffer_pool_instances":   {"innodb_buffer_pool_instances", "configuration", "innodb", "1", "8", 1, 64, MySQLVersions{Version{8, 0, 30}, Version{8, 1, 0}}},
		"innodb_flush_method":            {"innodb_flush_method", "configuration", "innodb", "O_DIRECT", "O_DIRECT", 0, 0, MySQLVersions{Version{8, 0, 30}, Version{8, 1, 0}}},
		"innodb_flush_log_at_trx_commit": {"innodb_flush_log_at_trx_commit", "configuration", "innodb", "2", "1", 0, 2, MySQLVersions{Version{8, 0, 30}, Version{8, 1, 0}}},
		"innodb_log_file_size":           {"innodb_log_file_size", "configuration", "innodb", "119537664", "50331648", 4194304, 0, MySQLVersions{Version{8, 0, 30}, Version{8, 1, 0}}},
		"innodb_log_files_in_group":      {"innodb_log_files_in_group", "configuration", "innodb", "2", "2", 2, 100, MySQLVersions{Version{8, 0, 30}, Version{8, 1, 0}}},
		"innodb_page_cleaners":           {"innodb_page_cleaners", "configuration", "innodb", "1", "4", 1, 64, MySQLVersions{Version{8, 0, 30}, Version{8, 1, 0}}},
		"innodb_purge_threads":           {"innodb_purge_threads", "configuration", "innodb", "1", "4", 1, 32, MySQLVersions{Version{8, 0, 30}, Version{8, 1, 0}}},
		"innodb_io_capacity_max":         {"innodb_io_capacity_max", "configuration", "innodb", "1000", "1400", 100, 0, MySQLVersions{Version{8, 0, 30}, Version{8, 1, 0}}},
		"innodb_buffer_pool_chunk_size":  {"innodb_buffer_pool_chunk_size", "configuration", "innodb", "2097152", "134217728", 1048576, 0, MySQLVersions{Version{8, 0, 30}, Version{8, 1, 0}}},
		"innodb_parallel_read_threads":   {"innodb_parallel_read_threads", "configuration", "innodb", "1", "4", 1, 256, MySQLVersions{Version{8, 0, 30}, Version{8, 1, 0}}},
		"innodb_monitor_enable":          {"innodb_monitor_enable", "configuration", "innodb", "ALL", "ALL", 0, 0, MySQLVersions{Version{8, 0, 30}, Version{8, 1, 0}}},
	}

	wsrepGroup := map[string]Parameter{
		"wsrep_sync_wait":         {"wsrep_sync_wait", "configuration", "galera", "0", "0", 0, 8, MySQLVersions{Version{8, 0, 30}, Version{8, 1, 0}}},
		"wsrep_slave_threads":     {"wsrep_slave_threads", "configuration", "galera", "2", "1", 1, 0, MySQLVersions{Version{8, 0, 30}, Version{8, 1, 0}}},
		"wsrep_trx_fragment_size": {"wsrep_trx_fragment_size", "configuration", "galera", "1048576", "0", 0, 0, MySQLVersions{Version{8, 0, 30}, Version{8, 1, 0}}},
		"wsrep_trx_fragment_unit": {"wsrep_trx_fragment_unit", "configuration", "galera", "bytes", "bytes", 0, 0, MySQLVersions{Version{8, 0, 30}, Version{8, 1, 0}}},
		"wsrep-provider-options":  {"wsrep-provider-options", "configuration", "galera", "<placeholder>", "", 0, 0, MySQLVersions{Version{8, 0, 30}, Version{8, 1, 0}}},
	}

	groupReplicationGroup := map[string]Parameter{

		"loose_group_replication_member_expel_timeout":           {"loose_group_replication_member_expel_timeout", "configuration", "groupReplication", "5", "5", 0, 20, MySQLVersions{Version{8, 0, 30}, Version{8, 1, 0}}},
		"loose_group_replication_autorejoin_tries":               {"loose_group_replication_autorejoin_tries", "configuration", "groupReplication", "2", "3", 0, 8, MySQLVersions{Version{8, 0, 30}, Version{8, 1, 0}}},
		"loose_group_replication_message_cache_size":             {"loose_group_replication_message_cache_size", "configuration", "groupReplication", "268435456", "1073741824", 134217728, 18446744073709551615, MySQLVersions{Version{8, 0, 30}, Version{8, 1, 0}}},
		"loose_group_replication_communication_max_message_size": {"loose_group_replication_poll_spin_loops", "configuration", "groupReplication", "2097152", "10485760", 0, 1073741824, MySQLVersions{Version{8, 0, 30}, Version{8, 1, 0}}},
		"loose_group_replication_unreachable_majority_timeout":   {"loose_group_replication_unreachable_majority_timeout", "configuration", "groupReplication", "3600", "0", 300, 3600, MySQLVersions{Version{8, 0, 30}, Version{8, 1, 0}}},
		"loose_group_replication_poll_spin_loops":                {"loose_group_replication_poll_spin_loops", "configuration", "groupReplication", "0", "0", 10000, 40000, MySQLVersions{Version{8, 0, 30}, Version{8, 1, 0}}},
		//"loose_group_replication_compression_threshold":          {"loose_group_replication_compression_threshold", "configuration", "groupReplication", "1000000", "1000000", 129024, 1000000},
		"loose_group_replication_paxos_single_leader":  {"loose_group_replication_paxos_single_leader", "configuration", "groupReplication", "ON", "OFF", 0, 1, MySQLVersions{Version{8, 0, 30}, Version{8, 1, 0}}},
		"loose_binlog_transaction_dependency_tracking": {"loose_binlog_transaction_dependency_tracking", "configuration", "groupReplication", "WRITESET", "COMMIT_ORDER", 0, 0, MySQLVersions{Version{8, 0, 30}, Version{8, 1, 0}}},
		//"loose_group_replication_view_change_uuid":               {"loose_group_replication_view_change_uuid", "configuration", "groupReplication", "AUTOMATIC", "AUTOMATIC", 0, 0},
		//"loose_group_replication_exit_state_action":              {"loose_group_replication_exit_state_action", "configuration", "groupReplication", "READ_ONLY", "READ_ONLY", 0, 0},

		//"loose_group_replication_compression_threshold":          {"loose_group_replication_compression_threshold", "configuration", "groupReplication", "1000000", "1000000", 0, 4294967295},
		//"loose_group_replication_autorejoin_tries":             {"loose_group_replication_autorejoin_tries", "configuration", "groupReplication", "2", "3", 0, 2016},
		//"loose_group_replication_unreachable_majority_timeout": {"loose_group_replication_unreachable_majority_timeout", "configuration", "groupReplication", "0", "0", 0, 31536000},
		//"loose_group_replication_poll_spin_loops":              {"loose_group_replication_poll_spin_loops", "configuration", "groupReplication", "20000", "0", 0, 18446744073709551615},
		//"loose_group_replication_member_expel_timeout":         {"loose_group_replication_member_expel_timeout", "configuration", "groupReplication", "5", "5", 0, 3600},
	}

	mysqlGroups := map[string]GroupObj{
		"readinessProbe": {"redinessProbe", map[string]Parameter{"timeoutSeconds": Parameter{"timeoutSeconds", "", "readinessProbe", "15", "15", 15, 600, MySQLVersions{}}}},
		"livenessProbe":  {"livenessProbe", map[string]Parameter{"timeoutSeconds": Parameter{"timeoutSeconds", "", "readinessProbe", "5", "5", 5, 600, MySQLVersions{}}}},
		"resources": {"resources", map[string]Parameter{
			"request_memory": {"memory", "request", "resources", "2", "2", 2, 32, MySQLVersions{}},
			"request_cpu":    {"cpu", "request", "resources", "1000", "1000", 1000, 8500, MySQLVersions{}},
			"limit_memory":   {"memory", "limit", "resources", "2", "2", 2, 32, MySQLVersions{}},
			"limit_cpu":      {"cpu", "limit", "resources", "1000", "1000", 1000, 8500, MySQLVersions{}},
		}},
	}

	haproxyGroups := map[string]GroupObj{
		"readinessProbe": {"redinessProbe", map[string]Parameter{"timeoutSeconds": Parameter{"timeoutSeconds", "", "readinessProbe", "5", "5", 5, 30, MySQLVersions{}}}},
		"livenessProbe":  {"livenessProbe", map[string]Parameter{"timeoutSeconds": Parameter{"timeoutSeconds", "", "readinessProbe", "5", "5", 5, 60, MySQLVersions{}}}},
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
		"readinessProbe": {"redinessProbe", map[string]Parameter{"timeoutSeconds": Parameter{"timeoutSeconds", "", "readinessProbe", "5", "5", 5, 30, MySQLVersions{}}}},
		"livenessProbe":  {"livenessProbe", map[string]Parameter{"timeoutSeconds": Parameter{"timeoutSeconds", "", "readinessProbe", "5", "5", 5, 60, MySQLVersions{}}}},
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

	families := map[string]Family{FamilyTypeMysql: {"mysql", mysqlGroups}, FamilyTypeProxy: {"haproxy", haproxyGroups}, FamilyTypeMonitor: {"pmm", pmmGroups}}

	return families

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

	pMap := map[string]ProviderParam{
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
	//		"gcs.fc_factor":             {"gcs.fc_factor", "%s", 0, 0.5, 0.5, 0.9},

	return pMap
}

// the function return all the groups in the family in one shot as byte buffer
func (f Family) ParseGroupsHuman() bytes.Buffer {
	var b bytes.Buffer

	b.WriteString("[" + f.Name + "]" + "\n")
	for key, group := range f.Groups {
		b.WriteString("  [" + key + "]" + "\n")
		pb := f.parseParamsHuman(group, "    ")
		b.Write(pb.Bytes())
	}

	return b

}

// the function return the group by name as byte buffer
func (f Family) ParseFamilyGroup(groupName string, padding string) (bytes.Buffer, error) {
	var b bytes.Buffer
	var err1 error
	switch groupName {
	case GroupNameMySQLd:
		return f.parseMySQLDHuman(padding), err1
	case GroupNameHAProxy:
		return f.parseHumanProxy(padding), err1
	case GroupNameProbes:
		return f.parseProbesHuman(padding), err1
	case GroupNameResources:
		return f.parseResourcesHuman(padding), err1
	default:
		err1 = errors.New("ERROR: Invalid Group name " + groupName)
		return b, err1
	}

}

func (f Family) parseHumanProxy(padding string) bytes.Buffer {
	var b bytes.Buffer
	for key, group := range f.Groups {
		if key != "readinessProbe" && key != "livenessProbe" && key != "resources" {
			pb := f.parseParamsHuman(group, padding)
			b.Write(pb.Bytes())
		}
	}
	return b
}

func (f Family) parseMySQLDHuman(padding string) bytes.Buffer {
	var b bytes.Buffer

	b.WriteString("[mysqld]" + "\n")
	for key, group := range f.Groups {
		if key != "readinessProbe" && key != "livenessProbe" && key != "resources" {
			pb := f.parseParamsHuman(group, padding)
			b.Write(pb.Bytes())
		}
	}

	return b

}

func (f Family) parseProbesHuman(padding string) bytes.Buffer {
	var b bytes.Buffer

	for key, group := range f.Groups {
		if key == "readinessProbe" || key == "livenessProbe" {
			b.WriteString("[" + key + "]" + "\n")
			pb := f.parseParamsHuman(group, padding)
			b.Write(pb.Bytes())
		}
	}

	return b

}

func (f Family) parseResourcesHuman(padding string) bytes.Buffer {
	var b bytes.Buffer

	for key, group := range f.Groups {
		if key == "resources" {
			b.WriteString("[" + key + "]" + "\n")
			pb := f.parseParamsHuman(group, padding)
			b.Write(pb.Bytes())
		}
	}

	return b

}

func (f Family) parseParamsHuman(group GroupObj, padding string) bytes.Buffer {
	var b bytes.Buffer
	for key, param := range group.Parameters {
		b.WriteString(padding + key + " = " + param.Value + "\n")
	}

	return b
}

func (conf *Configuration) CalculateOpenDimension(dimension Dimension) Dimension {
	if dimension.Cpu > 0 && dimension.MemoryBytes > 0 {
		//		{999, "Open request", 0, 0, 0.875, 0.09375, 0.00025, 0.96875, 0.0234375, 0.0078125},
		// first identify the range request fits in
		calcDimension := conf.getDimensionForFreeCalculation(dimension)

		dimension.MysqlCpu = int(float64(dimension.Cpu) * float64(calcDimension.MysqlCpu) / float64(calcDimension.Cpu))
		dimension.ProxyCpu = int(float64(dimension.Cpu) * float64(calcDimension.ProxyCpu) / float64(calcDimension.Cpu))
		dimension.PmmCpu = int(float64(dimension.Cpu) * float64(calcDimension.PmmCpu) / float64(calcDimension.Cpu))
		dimension.MysqlMemory = float64(dimension.MemoryBytes) * calcDimension.MysqlMemory / calcDimension.MemoryBytes
		dimension.ProxyMemory = float64(dimension.MemoryBytes) * calcDimension.ProxyMemory / calcDimension.MemoryBytes
		dimension.PmmMemory = float64(dimension.MemoryBytes) * calcDimension.PmmMemory / calcDimension.MemoryBytes

	}

	return dimension
}

// We return the dimension that is closer to the request
func (conf *Configuration) getDimensionForFreeCalculation(dimension Dimension) Dimension {
	var calcDimension Dimension
	for i := 0; i < len(conf.Dimension)-1; i++ {
		if i == 0 && (dimension.Cpu <= conf.Dimension[i].Cpu || dimension.MemoryBytes <= conf.Dimension[i].MemoryBytes) {
			calcDimension = conf.Dimension[i]
			break
		} else if i > 0 && (InBetween(dimension.Cpu, conf.Dimension[i-1].Cpu, conf.Dimension[i].Cpu) ||
			InBetween(int(dimension.MemoryBytes), int(conf.Dimension[i-1].MemoryBytes), int(conf.Dimension[i].MemoryBytes))) {
			// we always pich the smaller set for more conservative approach
			calcDimension = conf.Dimension[i-1]
			break
		} else if i == len(conf.Dimension)-1 && (dimension.Cpu >= conf.Dimension[i].Cpu || dimension.Memory >= conf.Dimension[i].Memory) {
			calcDimension = conf.Dimension[i]
			break
		}
	}

	return calcDimension
}
func InBetween(i, min, max int) bool {
	if (i >= min) && (i <= max) {
		return true
	} else {
		return false
	}
}

// TODO	***MySQL Supported version (testing is with hardcoded then we need to query check.percona.com ***
func (conf *Configuration) getMySQLVersion() {
	conf.Mysqlversions.Max = Version{8, 1, 0}
	conf.Mysqlversions.Min = Version{8, 0, 32}
}

// =====================================================
// Dimension section
// =====================================================
func (d *Dimension) ConvertMemoryToBytes(memoryHuman string) (float64, error) {
	var memoryBytes float64
	b, err1 := bytefmt.ToBytes(memoryHuman)
	if err1 != nil {
		return 0, err1
	}
	memoryBytes = float64(b)
	//memoryBytes, err1 = strconv.ParseFloat(b.String(), 64)
	return memoryBytes, err1
}
