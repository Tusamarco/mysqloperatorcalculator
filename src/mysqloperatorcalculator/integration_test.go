package mysqloperatorcalculator

import (
	"strconv"
	"testing"
)

// makeRequest builds a minimal valid ConfigurationRequest for integration tests.
func makeRequest(dbtype string, dimID, loadID, connections int) ConfigurationRequest {
	return ConfigurationRequest{
		DBType:       dbtype,
		Connections:  connections,
		Dimension:    Dimension{Id: dimID},
		LoadType:     LoadType{Id: loadID},
		Mysqlversion: Version{Major: 8, Minor: 0, Patch: 46},
	}
}

// runCalculate is a shorthand: Init + GetCalculate.
func runCalculate(req ConfigurationRequest) (error, ResponseMessage, map[string]Family) {
	var conf Configuration
	conf.Init()
	var moc MysqlOperatorCalculator
	moc.Init(req, conf)
	return moc.GetCalculate()
}

// redoLogCapacityBytes extracts innodb_redo_log_capacity from the returned families.
func redoLogCapacityBytes(t *testing.T, families map[string]Family) int64 {
	t.Helper()
	mysql, ok := families[FamilyTypeMysql]
	if !ok {
		t.Fatal("mysql family missing from response")
	}
	innodb, ok := mysql.Groups["configuration_innodb"]
	if !ok {
		t.Fatal("configuration_innodb group missing")
	}
	param, ok := innodb.Parameters["innodb_redo_log_capacity"]
	if !ok {
		t.Skip("innodb_redo_log_capacity not present (version filter applied)")
	}
	val, err := strconv.ParseInt(param.Value, 10, 64)
	if err != nil {
		t.Fatalf("cannot parse innodb_redo_log_capacity %q: %v", param.Value, err)
	}
	return val
}

// bufferPoolBytes extracts innodb_buffer_pool_size from the returned families.
func bufferPoolBytes(t *testing.T, families map[string]Family) int64 {
	t.Helper()
	mysql, ok := families[FamilyTypeMysql]
	if !ok {
		t.Fatal("mysql family missing from response")
	}
	innodb, ok := mysql.Groups["configuration_innodb"]
	if !ok {
		t.Fatal("configuration_innodb group missing")
	}
	bp, ok := innodb.Parameters["innodb_buffer_pool_size"]
	if !ok {
		t.Fatal("innodb_buffer_pool_size missing")
	}
	val, err := strconv.ParseInt(bp.Value, 10, 64)
	if err != nil {
		t.Fatalf("cannot parse innodb_buffer_pool_size %q: %v", bp.Value, err)
	}
	return val
}

// ---------------------------------------------------------------------------
// Happy-path PXC
// ---------------------------------------------------------------------------

func TestIntegration_PXC_Small_MostlyReads_OK(t *testing.T) {
	err, msg, families := runCalculate(makeRequest(DbTypePXC, 2, LoadTypeMostlyReads, 50))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.MType == OverutilizingI {
		t.Errorf("expected Ok or CloseToLimit, got OverutilizingI: %s", msg.MText)
	}
	bp := bufferPoolBytes(t, families)
	if bp <= 0 {
		t.Errorf("innodb_buffer_pool_size must be > 0, got %d", bp)
	}
}

func TestIntegration_PXC_Medium_HeavyOLTP_OK(t *testing.T) {
	err, msg, families := runCalculate(makeRequest(DbTypePXC, 3, LoadTypeEqualReadsWrites, 200))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.MType == OverutilizingI {
		t.Errorf("unexpected OverutilizingI: %s", msg.MText)
	}
	if bufferPoolBytes(t, families) <= 0 {
		t.Error("innodb_buffer_pool_size must be > 0")
	}
}

// ---------------------------------------------------------------------------
// Happy-path Group Replication
// ---------------------------------------------------------------------------

func TestIntegration_GR_Small_SomeWrites_OK(t *testing.T) {
	err, msg, families := runCalculate(makeRequest(DbTypeGroupReplication, 2, LoadTypeSomeWrites, 100))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.MType == OverutilizingI {
		t.Errorf("unexpected OverutilizingI: %s", msg.MText)
	}

	// GR must have the group_replication group with the GCS cache parameter
	mysql := families[FamilyTypeMysql]
	grGroup, ok := mysql.Groups["configuration_groupReplication"]
	if !ok {
		t.Fatal("configuration_groupReplication group missing for GR request")
	}
	gcs, ok := grGroup.Parameters["loose_group_replication_message_cache_size"]
	if !ok {
		t.Fatal("loose_group_replication_message_cache_size missing")
	}
	val, _ := strconv.ParseInt(gcs.Value, 10, 64)
	if val <= 0 {
		t.Errorf("loose_group_replication_message_cache_size must be > 0, got %d", val)
	}
}

// ---------------------------------------------------------------------------
// Saturation — XSmall + heavy load + many connections
// ---------------------------------------------------------------------------

func TestIntegration_PXC_XSmall_Saturated(t *testing.T) {
	// XSmall (1 CPU, 2 GB) cannot handle 5000 heavy-write connections
	err, msg, _ := runCalculate(makeRequest(DbTypePXC, 1, LoadTypeEqualReadsWrites, 5000))
	if err != nil {
		t.Fatalf("unexpected hard error: %v", err)
	}
	// Expect either OverutilizingI or ConnectionRecalculated (auto back-off)
	if msg.MType != OverutilizingI && msg.MType != ConnectionRecalculated {
		t.Errorf("expected saturation response, got MType=%d text=%s", msg.MType, msg.MText)
	}
}

// ---------------------------------------------------------------------------
// Invalid requests
// ---------------------------------------------------------------------------

func TestIntegration_MissingDimensionID(t *testing.T) {
	req := makeRequest(DbTypePXC, 0, LoadTypeMostlyReads, 50) // dim ID 0 is invalid
	err, _, _ := runCalculate(req)
	if err == nil {
		t.Error("expected error for missing dimension ID, got nil")
	}
}

func TestIntegration_MissingLoadTypeID(t *testing.T) {
	req := makeRequest(DbTypePXC, 2, 0, 50) // load ID 0 is invalid
	err, _, _ := runCalculate(req)
	if err == nil {
		t.Error("expected error for missing load type ID, got nil")
	}
}

func TestIntegration_InvalidDBType(t *testing.T) {
	req := makeRequest("oracle", 2, LoadTypeMostlyReads, 50)
	err, _, _ := runCalculate(req)
	if err == nil {
		t.Error("expected error for invalid DBType, got nil")
	}
}

// ---------------------------------------------------------------------------
// Auto-scale by connections (dimension ID 998)
// ---------------------------------------------------------------------------

func TestIntegration_AutoScaleByConnections(t *testing.T) {
	req := makeRequest(DbTypePXC, ConnectionDimension, LoadTypeMostlyReads, 200)
	err, msg, families := runCalculate(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.MType != ResourcesRecalculated {
		t.Errorf("expected ResourcesRecalculated, got MType=%d", msg.MType)
	}
	if bufferPoolBytes(t, families) <= 0 {
		t.Error("innodb_buffer_pool_size must be > 0")
	}
}

// ---------------------------------------------------------------------------
// Open dimension (ID 999)
// ---------------------------------------------------------------------------

func TestIntegration_OpenDimension_PXC(t *testing.T) {
	req := ConfigurationRequest{
		DBType:      DbTypePXC,
		Connections: 100,
		Dimension: Dimension{
			Id:          DimensionOpen,
			Cpu:         4000,
			Memory:      "8GB",
			MemoryBytes: 8 * 1024 * 1024 * 1024,
		},
		LoadType:     LoadType{Id: LoadTypeMostlyReads},
		Mysqlversion: Version{Major: 8, Minor: 0, Patch: 46},
	}
	err, msg, families := runCalculate(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.MType == OverutilizingI {
		t.Errorf("unexpected OverutilizingI for open dimension: %s", msg.MText)
	}
	if bufferPoolBytes(t, families) <= 0 {
		t.Error("innodb_buffer_pool_size must be > 0")
	}
}

// ---------------------------------------------------------------------------
// Resources group — probes and limits always present
// ---------------------------------------------------------------------------

func TestIntegration_ProbesAndResourcesPresent(t *testing.T) {
	_, _, families := runCalculate(makeRequest(DbTypePXC, 2, LoadTypeMostlyReads, 50))

	for _, familyName := range []string{FamilyTypeMysql, FamilyTypeProxy, FamilyTypeMonitor} {
		fam, ok := families[familyName]
		if !ok {
			t.Errorf("family %q missing", familyName)
			continue
		}
		for _, group := range []string{"resources", "readinessProbe", "livenessProbe"} {
			g, ok := fam.Groups[group]
			if !ok {
				t.Errorf("family %q: group %q missing", familyName, group)
				continue
			}
			if len(g.Parameters) == 0 {
				t.Errorf("family %q group %q has no parameters", familyName, group)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// HeavyWrites (load type 4) — end-to-end
// ---------------------------------------------------------------------------

// Happy path: PXC + HeavyWrites produces a valid configuration.
func TestIntegration_PXC_HeavyWrites_OK(t *testing.T) {
	err, msg, families := runCalculate(makeRequest(DbTypePXC, 3, LoadTypeHeavyWrites, 100))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.MType == OverutilizingI {
		t.Errorf("unexpected OverutilizingI: %s", msg.MText)
	}
	if bufferPoolBytes(t, families) <= 0 {
		t.Error("innodb_buffer_pool_size must be > 0")
	}
	if redoLogCapacityBytes(t, families) <= 0 {
		t.Error("innodb_redo_log_capacity must be > 0")
	}
}

// Happy path: GR + HeavyWrites produces a valid configuration with GCS cache set.
func TestIntegration_GR_HeavyWrites_OK(t *testing.T) {
	err, msg, families := runCalculate(makeRequest(DbTypeGroupReplication, 3, LoadTypeHeavyWrites, 100))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.MType == OverutilizingI {
		t.Errorf("unexpected OverutilizingI: %s", msg.MText)
	}
	if bufferPoolBytes(t, families) <= 0 {
		t.Error("innodb_buffer_pool_size must be > 0")
	}

	mysql := families[FamilyTypeMysql]
	grGroup, ok := mysql.Groups["configuration_groupReplication"]
	if !ok {
		t.Fatal("configuration_groupReplication group missing for GR+HeavyWrites request")
	}
	gcs, ok := grGroup.Parameters["loose_group_replication_message_cache_size"]
	if !ok {
		t.Fatal("loose_group_replication_message_cache_size missing")
	}
	val, _ := strconv.ParseInt(gcs.Value, 10, 64)
	if val <= 0 {
		t.Errorf("loose_group_replication_message_cache_size must be > 0, got %d", val)
	}
}

// HeavyWrites saturates a dimension at fewer connections than EqualReadsWrites
// because its CPU-per-connection cost factor (2.0) is higher than EqualReadsWrites (1.6).
func TestIntegration_HeavyWrites_SaturatesEarlierThanEqualReadsWrites(t *testing.T) {
	// Find the connection count that overloads HeavyWrites but not EqualReadsWrites
	// on XSmall (MysqlCPU=600m): HeavyWrites max = 600/2=300, EqualReadsWrites max = 600/1.6=375
	_, msgHW, _ := runCalculate(makeRequest(DbTypePXC, 1, LoadTypeHeavyWrites, 350))
	_, msgEq, _ := runCalculate(makeRequest(DbTypePXC, 1, LoadTypeEqualReadsWrites, 350))

	if msgHW.MType != OverutilizingI && msgHW.MType != ConnectionRecalculated {
		t.Errorf("expected HeavyWrites to saturate at 350 connections on XSmall, got MType=%d", msgHW.MType)
	}
	if msgEq.MType == OverutilizingI {
		t.Errorf("EqualReadsWrites should not saturate at 350 connections on XSmall, got MType=%d", msgEq.MType)
	}
}

// Redo log must increase monotonically with write intensity for identical resources and connections.
// Uses Large dimension (id=4) with low connections to keep loadFactor small and avoid the 1.0 cap.
func TestIntegration_HeavyWrites_RedoLog_MonotonicByLoadType(t *testing.T) {
	const dim = 4  // Large — MysqlCPU=5500m, high enough that 50 connections yields a low loadFactor
	const conn = 50

	_, _, famMR := runCalculate(makeRequest(DbTypePXC, dim, LoadTypeMostlyReads, conn))
	_, _, famSW := runCalculate(makeRequest(DbTypePXC, dim, LoadTypeSomeWrites, conn))
	_, _, famEq := runCalculate(makeRequest(DbTypePXC, dim, LoadTypeEqualReadsWrites, conn))
	_, _, famHW := runCalculate(makeRequest(DbTypePXC, dim, LoadTypeHeavyWrites, conn))

	redoMR := redoLogCapacityBytes(t, famMR)
	redoSW := redoLogCapacityBytes(t, famSW)
	redoEq := redoLogCapacityBytes(t, famEq)
	redoHW := redoLogCapacityBytes(t, famHW)

	if redoSW <= redoMR {
		t.Errorf("SomeWrites redo log %d should be > MostlyReads %d", redoSW, redoMR)
	}
	if redoEq <= redoSW {
		t.Errorf("EqualReadsWrites redo log %d should be > SomeWrites %d", redoEq, redoSW)
	}
	if redoHW <= redoEq {
		t.Errorf("HeavyWrites redo log %d should be > EqualReadsWrites %d", redoHW, redoEq)
	}
}

// ---------------------------------------------------------------------------
// MySQL version filtering
// ---------------------------------------------------------------------------

func TestIntegration_VersionFilter_OldVersion(t *testing.T) {
	// 8.0.44 is below MySQLMinSupported (8.0.46) — most parameters get filtered out
	req := makeRequest(DbTypePXC, 2, LoadTypeMostlyReads, 50)
	req.Mysqlversion = Version{Major: 8, Minor: 0, Patch: 44}
	err, _, families := runCalculate(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Result must still be a valid, non-empty families map
	if len(families) == 0 {
		t.Error("families map should not be empty even for old MySQL version")
	}
}
