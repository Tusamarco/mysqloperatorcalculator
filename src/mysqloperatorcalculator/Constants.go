package mysqloperatorcalculator

const (
	// VERSION is embedded in the --version CLI flag and in the /supported API response.
	VERSION = "v1.20.0"

	// ---------------------------------------------------------------------------
	// Response message type codes — returned as message.type in every response.
	// Callers should branch on these rather than parsing message.text.
	// ---------------------------------------------------------------------------

	OkI                    = 1001 // resources fully satisfy the request
	ClosetolimitI          = 2001 // within safety margin of saturation; usable but watch carefully
	OverutilizingI         = 3001 // resources insufficient; no configuration is returned
	ErrorexecI             = 5001 // request rejected (malformed input, missing field, unsupported version)
	ConnectionRecalculated = 6001 // connection count was reduced to fit available CPU/memory
	ResourcesRecalculated  = 7001 // dimension was scaled up automatically (dimension.id = 998)

	// ---------------------------------------------------------------------------
	// Response message text templates — matched by type code above.
	// ErrorexecT is a fmt format string; the remaining constants are plain strings.
	// ---------------------------------------------------------------------------

	OkT                       = "Execution was successful and resources match the possible requests"
	ClosetolimitT             = "Execution was successful however resources are close to saturation based on the load requested"
	OverutilizingT            = "Resources not enough to cover the requested load "
	ErrorexecT                = "There is an error while processing. See details: %s"
	ConnectionRecalculatedTxt = "The number of connection has been recalculated to match the available resources"
	ResourcesRecalculatedTxt  = "Minimum resource estimates have been updated to correspond with the specified connection count."

	// ---------------------------------------------------------------------------
	// Load type IDs — describe the read/write ratio of the workload.
	// The ID is passed in loadtype.id and drives per-parameter tuning multipliers.
	// ---------------------------------------------------------------------------

	LoadTypeMostlyReads      = 1 // ~95% reads, ~5% writes (blogs, reporting, read replicas)
	LoadTypeSomeWrites       = 2 // ~80% reads, ~20% writes (e-commerce, light OLTP)
	LoadTypeEqualReadsWrites = 3 // ~50/50 (mixed analytics, heavy OLTP)
	LoadTypeHeavyWrites      = 4 // write-dominated (log ingestion, event streams, high-frequency inserts)

	// ---------------------------------------------------------------------------
	// Special dimension IDs — sentinel values outside the normal dimension table.
	// ---------------------------------------------------------------------------

	// DimensionOpen (999): user provides explicit CPU millicores and memory in the
	// request; the calculator distributes them across MySQL, proxy, and monitoring.
	DimensionOpen = 999

	// ConnectionDimension (998): connection-driven auto-sizing. The calculator picks
	// the smallest pre-defined dimension that can sustain the requested connection count.
	ConnectionDimension = 998

	// ---------------------------------------------------------------------------
	// Family and group name keys — used to address sections of the response map.
	// ---------------------------------------------------------------------------

	FamilyTypeMysql   = "mysql"   // MySQL container configuration family
	FamilyTypeProxy   = "proxy"   // HAProxy sidecar configuration family
	FamilyTypeMonitor = "monitor" // PMM monitoring sidecar configuration family

	GroupNameMySQLd    = "mysqld"        // MySQL daemon parameters (my.cnf variables)
	GroupNameProbes    = "probes"        // Kubernetes liveness / readiness probe timings
	GroupNameResources = "resources"     // Kubernetes CPU and memory requests/limits
	GroupNameHAProxy   = "haproxyConfig" // HAProxy-specific settings (maxconn, timeouts)

	// ---------------------------------------------------------------------------
	// Database type strings — passed in the dbtype field of the request.
	// ---------------------------------------------------------------------------

	DbTypePXC              = "pxc"               // Percona XtraDB Cluster (Galera-based synchronous replication)
	DbTypeGroupReplication = "group_replication" // MySQL Group Replication (InnoDB Cluster)

	// ---------------------------------------------------------------------------
	// Output format strings — passed in the output field of the request.
	// ---------------------------------------------------------------------------

	ResultOutputFormatJson  = "json"  // structured JSON — suitable for automation and Operators
	ResultOutputFormatHuman = "human" // INI-style flat text — suitable for my.cnf or manual review

	// ---------------------------------------------------------------------------
	// InnoDB buffer pool sizing fractions
	// ---------------------------------------------------------------------------

	// InnoDBPctValuePXC is the maximum fraction of MySQL-allocated memory that may be
	// given to the InnoDB buffer pool in a PXC deployment. Galera's GCache footprint
	// is relatively small and predictable, so a higher ceiling is safe.
	InnoDBPctValuePXC = 0.80

	// InnoDBPctValueGR is the same ceiling for Group Replication. GR's certification
	// cache can spike during long write transactions, so a lower ceiling reserves
	// headroom to avoid OOM kills.
	InnoDBPctValueGR = 0.70

	// GroupRepGCSCacheMemStructureCost is a fixed 50 MiB reserved for the Group
	// Replication message-cache data structure itself. It is deducted from MySQL
	// memory before the buffer pool is sized, on top of the per-connection GCS cost.
	GroupRepGCSCacheMemStructureCost = 52428800 // 50 MiB

	// ---------------------------------------------------------------------------
	// Minimum buffer pool floors
	// Prevent the buffer pool from being squeezed below a usable size when many
	// connections with large per-connection buffers are requested.
	// ---------------------------------------------------------------------------

	// MinLimitPXC: InnoDB buffer pool will never drop below 50% of MySQL memory in
	// a PXC deployment, even under extreme connection pressure.
	MinLimitPXC = 0.50

	// MinLimitGR: same floor for Group Replication, set lower because GR needs more
	// memory headroom for certification and message caches.
	MinLimitGR = 0.40

	// MemoryFreeMinimumLimit reserves 2% of total MySQL memory, never allocated to
	// any structure, as a safety margin for allocator overhead and OS paging.
	MemoryFreeMinimumLimit = 0.02

	// ---------------------------------------------------------------------------
	// GCache footprint factors (PXC only)
	// Scale the configured GCache file size to its estimated resident-memory footprint.
	// A lower write ratio means fewer in-flight transactions and a smaller hot portion
	// of the cache actually resident in RAM.
	// ---------------------------------------------------------------------------

	GcacheFootPrintFactorRead       = 0.5 // mostly reads: GCache is largely idle
	GcacheFootPrintFactorLightWrite = 0.6 // light OLTP: moderate certification backlog
	GcacheFootPrintFactorReadWrite  = 0.8 // equal reads/writes: heavier certification backlog
	// Heavy-write workloads reuse the Read factor (0.5): many small transactions
	// that certify quickly keep the resident portion of GCache small despite high
	// write throughput.

	// ---------------------------------------------------------------------------
	// GCS cache connection weight (Group Replication only)
	// ---------------------------------------------------------------------------

	// GCSConnWeight is the assumed per-connection cost in bytes for the GR message
	// cache. Multiplied by max_connections to estimate total GCS memory demand.
	GCSConnWeight = 10

	// ---------------------------------------------------------------------------
	// Auto-scale stepping increments (used when dimension.id = 998)
	// These control how coarsely the dimension search steps through resource levels.
	// ---------------------------------------------------------------------------

	CPUIncrement    = 200 // millicores added per step when searching for a fitting dimension
	MemoryIncrement = 500 // megabytes added per step

	// ---------------------------------------------------------------------------
	// CPU-to-connection scaling factors
	// Express the average CPU millicores consumed per active connection.
	// The formula is: connections × factor ≤ MySQL CPU millicores.
	// Exceeding this ratio returns OverutilizingI; approaching it returns ClosetolimitI.
	// ---------------------------------------------------------------------------

	CpuConncetionMillFactorRead           = 0.8 // mostly reads: low write overhead
	CpuConncetionMillFactorReadWriteLight = 1.2 // light OLTP: binlog + replication add up
	CpuConncetionMillFactorReadWriteEqual = 1.6 // equal reads/writes: lock contention overhead
	CpuConncetionMillFactorReadWriteHeavy = 2   // heavy writes: maximum CPU demand per connection

	// ---------------------------------------------------------------------------
	// Connection thresholds
	// ---------------------------------------------------------------------------

	// ConnectionWeighPctLimit: when the memory cost of all connection buffers exceeds
	// this fraction of available MySQL memory, the response is flagged ClosetolimitI
	// and InnoDB/redo-log sizing is skipped.
	ConnectionWeighPctLimit = 0.50

	// MinConnectionNumber is the hard floor applied to every request. Values below
	// this (including 0) are silently raised before the calculation begins.
	MinConnectionNumber = 20

	// MaxAutoConnections caps the auto-connection search loop (connections = 0) to
	// prevent an unbounded loop on very large instances.
	MaxAutoConnections = 500000
)
