package mysqloperatorcalculator

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
	ResourcesRecalculatedTxt  = "Minimum resource estimates have been updated to correspond with the specified connection count."

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
	InnoDBPctValueGR  = 0.70

	GroupRepGCSCacheMemStructureCost = 52428800

	MinLimitPXC            = 0.50
	MinLimitGR             = 0.40
	MemoryFreeMinimumLimit = 0.02

	/* Minlimit is the % of memory assigned to Innodb buffer pool compared to the total memory assigned to mysql
	We have different min limit per type of replication (galera and Group replication) because the different impact of the internal cache.
	In GR the certification cache is suffering of an issue. In short the certification cache is clean/flushed on commit, but if the operation is a long one like insert into A from select * from b ;
	and we have large dataset, this may cause the cache buffer to be very large and causing issues like swap or OOM kill.
	While we cannot prevent this to happen, we need to consider the possible impact of it.
	Also other caches used by GR and more frequently cleanup are causing higher memory consumption than galera.
	as such we need to allocate less memory to innodb and more to buffers.
	By consequence the % of innodb memory for galera is higher than GR
	*/
	// MemoryFreeMinimumLimit This is the amount of memory in % that we must keep free no matter what to give some space to the server

	// Weight to use when using PXC for Gcache in mem footprint
	GcacheFootPrintFactorRead       = 0.5
	GcacheFootPrintFactorLightWrite = 0.6
	GcacheFootPrintFactorReadWrite  = 0.8

	// Weights to use to tune the GCS calculation against connections
	GCSConnWeight = 10 //each connection costs 10 millicycles
	// TODO Deprecated
	//GCSWeightRead           = 0.40
	//GCSWeightReadLightWrite = 0.60
	//GCSWeightReadWrite      = 1.20
	//GCSWeightReadHeavyWrite = 1

	CPUIncrement    = 200
	MemoryIncrement = 500

	CpuConncetionMillFactorRead           = 0.8
	CpuConncetionMillFactorReadWriteLight = 1.2
	CpuConncetionMillFactorReadWriteEqual = 1.6
	CpuConncetionMillFactorReadWriteHeavy = 2

	ConnectionWeighPctLimit = 0.50
	MinConnectionNumber     = 20
	MaxAutoConnections      = 500000

	/*Connection / CPU adjustment factor this is the factor by which we divide the available CPU mill reporting th emaximum number of connections available
	if we assign 2000 CPU and ask for 100 connection the formula will be CPUmill/CpuConncetionMillFactor < Connection asked
	if the number of CPUmill/CpuConncetionMillFactor > Connection asked we are overloading the platform
	We hace 3 different load factors:
	- mainly read
	- read/write 80/20
	- read/write 50/50
	*/
	// Global

)
