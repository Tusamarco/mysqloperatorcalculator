package Objects

//*********************************
// Structure definitions
//********************************

// used to pass available configurations
type Configuration struct {
	Dimension   []Dimension `json:"dimension"`
	LoadType    []LoadType  `json:"loadtype"`
	Connections []int       `json:"connections"`
}

// used to store the incoming request
type ConfigurationRequest struct {
	Dimension   Dimension `json:"dimension"`
	LoadType    LoadType  `json:"loadtype"`
	Connections int       `json:"connections"`
	Human       bool      `json:"human"`
}

// used to represent the POD dimension
type Dimension struct {
	Id     int    `json:"id"`
	Name   string `json:"name"`
	Cpu    int    `json:"cpu"`
	Memory int64  `json:"memory"`
}

// The different kind of load type
type LoadType struct {
	Id      int    `json:"id"`
	Name    string `json:"name"`
	Example string `json:"example"`
}

// generic structure to store Parameters values
type Parameter struct {
	Name    string `yaml:"name" json:"name"`
	Section string `yaml:"section" json:"section"`
	Group   string `yaml:"group" json:"group"`
	Value   string `yaml:"value" json:"value"`
	Default string `yaml:"default" json:"default"`
	Min     int    `yaml:"min" json:"min"`
	Max     int    `yaml:"max" json:"max"`
}

// Parameters are groupped by typology
type GroupObj struct {
	Name       string               `yaml:"name" json:"name"`
	Parameters map[string]Parameter `yaml:"parameters" json:"parameters"`
}

// Groups are organized by Families
type Family struct {
	Name   string              `yaml:"name" json:"name"`
	Groups map[string]GroupObj `yaml:"groups" json:"groups"`
}

// returns the Dimension using ID attribute
func (conf *Configuration) GetDimensionByID(id int) Dimension {
	for i := 0; i < len(conf.Dimension); i++ {
		if conf.Dimension[i].Id == id {
			return conf.Dimension[i]
		}

	}
	return Dimension{0, "", 0, 0}
}

// returns the Load Type using ID attribute
func (conf *Configuration) GetLoadByID(id int) LoadType {
	for i := 0; i < len(conf.LoadType); i++ {
		if conf.LoadType[i].Id == id {
			return conf.LoadType[i]
		}

	}
	return LoadType{0, "", ""}

}

// here is where we define the different options
// it will be possible to increment the supported solutions adding here the items
func (conf *Configuration) Init() {

	conf.Dimension = []Dimension{
		{1, "XSmall", 1000, 2},
		{2, "Small", 2500, 4},
		{3, "Medium", 4500, 8},
		{4, "Large", 6500, 16},
		{5, "XLarge", 8500, 32},
	}

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
}

func (family *Family) Init() map[string]Family {

	// supported parameters are defined here with defaults value and ranges.
	// to add new we can add here the new one then create a proper method to handle the calculation

	connectionGroup := map[string]Parameter{
		"binlog_cache_size":      {"binlog_cache_size", "configuration", "connection", "32768", "32768", 32768, 0},
		"binlog_stmt_cache_size": {"binlog_stmt_cache_size", "configuration", "connection", "32768", "32768", 32768, 0},
		"join_buffer_size":       {"join_buffer_size", "configuration", "connection", "262144", "262144", 262144, 0},
		"read_rnd_buffer_size":   {"read_rnd_buffer_size", "configuration", "connection", "262144", "262144", 262144, 0},
		"sort_buffer_size":       {"sort_buffer_size", "configuration", "connection", "524288", "524288", 524288, 0},
		"max_heap_table_size":    {"max_heap_table_size", "configuration", "connection", "16777216", "16777216", 16777216, 0},
		"tmp_table_size":         {"tmp_table_size", "configuration", "connection", "16777216", "16777216", 16777216, 0},
	}

	serverGroup := map[string]Parameter{
		"max_connections":             {"max_connections", "configuration", "server", "50", "2", 2, 65536},
		"thread_pool_size":            {"thread_pool_size", "configuration", "server", "2", "2", 2, 64},
		"table_definition_cache":      {"table_definition_cache", "configuration", "server", "4096", "4096", 400, 524288},
		"table_open_cache":            {"table_open_cache", "configuration", "server", "4096", "4096", 400, 524288},
		"thread_stack":                {"thread_stack", "configuration", "server", "1024", "1024", 125, 1048576},
		"table_open_cache_instances":  {"table_open_cache_instances", "configuration", "server", "4", "16", 1, 64},
		"tablespace_definition_cache": {"tablespace_definition_cache", "configuration", "server", "512", "256", 256, 524288},
	}

	innodbGroup := map[string]Parameter{
		"innodb_adaptive_hash_index":     {"innodb_adaptive_hash_index", "configuration", "innodb", "1", "1", 0, 1},
		"innodb_buffer_pool_size":        {"innodb_buffer_pool_size", "configuration", "innodb", "1073741824", "134217728", 5242880, 0},
		"innodb_buffer_pool_instances":   {"innodb_buffer_pool_instances", "configuration", "innodb", "1", "8", 1, 64},
		"innodb_flush_method":            {"innodb_flush_method", "configuration", "innodb", "O_DIRECT", "O_DIRECT", 0, 0},
		"innodb_flush_log_at_trx_commit": {"innodb_flush_log_at_trx_commit", "configuration", "innodb", "2", "1", 0, 2},
		"innodb_log_file_size":           {"innodb_log_file_size", "configuration", "innodb", "119537664", "50331648", 4194304, 0},
		"innodb_log_files_in_group":      {"innodb_log_files_in_group", "configuration", "innodb", "2", "2", 2, 100},
		"innodb_page_cleaners":           {"innodb_page_cleaners", "configuration", "innodb", "1", "4", 1, 64},
		"innodb_purge_threads":           {"innodb_purge_threads", "configuration", "innodb", "1", "4", 1, 32},
		"innodb_io_capacity_max":         {"innodb_io_capacity_max", "configuration", "innodb", "1000", "1400", 100, 0},
	}

	wsrepGroup := map[string]Parameter{
		"wsrep_sync_wait":         {"wsrep_sync_wait", "configuration", "galera", "0", "0", 0, 8},
		"wsrep_slave_threads":     {"wsrep_slave_threads", "configuration", "galera", "2", "1", 1, 0},
		"wsrep_trx_fragment_size": {"wsrep_trx_fragment_size", "configuration", "galera", "1048576", "0", 0, 0},
		"wsrep_trx_fragment_unit": {"wsrep_trx_fragment_unit", "configuration", "galera", "bytes", "bytes", -1, -1},
		"wsrep-provider-options":  {"wsrep-provider-options", "configuration", "galera", "<placeholder>", "", 0, 0},
	}

	pxcGroups := map[string]GroupObj{
		"readinessProbe": {"redinessProbe", map[string]Parameter{"timeoutSeconds": Parameter{"timeoutSeconds", "", "readinessProbe", "15", "15", 15, 600}}},
		"livenessProbe":  {"livenessProbe", map[string]Parameter{"timeoutSeconds": Parameter{"timeoutSeconds", "", "readinessProbe", "5", "5", 5, 600}}},
		"resources": {"resources", map[string]Parameter{
			"request_memory": {"memory", "request", "resources", "2", "2", 2, 32},
			"request_cpu":    {"cpu", "request", "resources", "1000", "1000", 1000, 8500},
			"limit_memory":   {"memory", "limit", "resources", "2", "2", 2, 32},
			"limit_cpu":      {"cpu", "limit", "resources", "1000", "1000", 1000, 8500},
		}},
	}

	haproxyGroups := map[string]GroupObj{
		"readinessProbe":        {"redinessProbe", map[string]Parameter{"timeoutSeconds": Parameter{"timeoutSeconds", "", "readinessProbe", "5", "5", 5, 30}}},
		"livenessProbe":         {"livenessProbe", map[string]Parameter{"timeoutSeconds": Parameter{"timeoutSeconds", "", "readinessProbe", "5", "5", 5, 60}}},
		"ha_connection_timeout": {"ha_connection_timeout", map[string]Parameter{"timeoutSeconds": Parameter{"timeoutSeconds", "", "ha_connection_timeout", "5", "1000", 1000, 5000}}},
		"resources": {"resources", map[string]Parameter{
			"request_memory": {"memory", "request", "resources", "1", "1", 1, 2},
			"request_cpu":    {"cpu", "request", "resources", "1000", "1000", 1000, 2000},
			"limit_memory":   {"memory", "limit", "resources", "1", "!", 1, 2},
			"limit_cpu":      {"cpu", "limit", "resources", "1000", "1000", 1000, 2000},
		}},
	}

	pxcGroups["configuration_connection"] = GroupObj{"connections", connectionGroup}
	pxcGroups["configuration_server"] = GroupObj{"server", serverGroup}
	pxcGroups["configuration_innodb"] = GroupObj{"innodb", innodbGroup}
	pxcGroups["configuration_galera"] = GroupObj{"galera", wsrepGroup}

	families := map[string]Family{"pxc": {"pxc", pxcGroups}, "haproxy": {"haproxy", haproxyGroups}}

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
		"evs.send_window":           {"evs.send_window", "PT%sS", 0, 4, 4, 1024},
		"evs.user_send_window":      {"evs.user_send_window", "PT%sS", 0, 2, 2, 1024},
		"evs.inactive_check_period": {"evs.inactive_check_period", "PT%sS", 0, 1, 1, 5},
		"evs.inactive_timeout":      {"evs.inactive_timeout", "PT%sS", 0, 15, 15, 120},
		"evs.join_retrans_period":   {"evs.join_retrans_period", "PT%sS", 0, 1, 1, 5},
		"evs.suspect_timeout":       {"evs.suspect_timeout", "PT%sS", 0, 5, 5, 60},
		"evs.stats_report_period":   {"evs.stats_report_period", "PT%sM", 0, 1, 1, 1},
		"gcs.fc_limit":              {"gcs.fc_limit", "PT%sS", 0, 16, 16, 128},
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
