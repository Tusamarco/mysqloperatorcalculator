package mysqloperatorcalculator

import (
	"bytes"
	"strconv"
	"testing"
)

const (
	testMB = 1024 * 1024
	testGB = 1024 * testMB
)

// newTestConfigurator builds a Configurator with controlled state for unit testing.
// It does not call Init() so families/providerParams are intentionally empty.
func newTestConfigurator(loadID int, dbtype string, connections int, mysqlCPU float64, mysqlMemBytes float64) *Configurator {
	return &Configurator{
		request: ConfigurationRequest{
			DBType:      dbtype,
			Connections: connections,
			LoadType:    LoadType{Id: loadID},
		},
		reference: &references{
			loadID:      loadID,
			connections: connections,
			cpusMySQL:   mysqlCPU,
			memoryMySQL: mysqlMemBytes,
		},
	}
}

// ---------------------------------------------------------------------------
// CalculateReturnBytes
// ---------------------------------------------------------------------------

func TestCalculateReturnBytes_BelowMinThreshold(t *testing.T) {
	c := &Configurator{reference: &references{}}
	input := int64(100 * testMB)
	got := c.CalculateReturnBytes(input)
	want := int64(float64(input) * 0.20)
	if got != want {
		t.Errorf("CalculateReturnBytes(100MB) = %d, want %d", got, want)
	}
}

func TestCalculateReturnBytes_AboveMaxThreshold(t *testing.T) {
	c := &Configurator{reference: &references{}}
	input := int64(2048 * testMB)
	got := c.CalculateReturnBytes(input)
	want := int64(float64(input) * 0.85)
	if got != want {
		t.Errorf("CalculateReturnBytes(2048MB) = %d, want %d", got, want)
	}
}

func TestCalculateReturnBytes_Interpolated(t *testing.T) {
	c := &Configurator{reference: &references{}}
	input := int64(1024 * testMB) // 1GB: midpoint between 300MB and 2GB thresholds
	got := c.CalculateReturnBytes(input)
	// Should be between 20% and 85%
	low := int64(float64(input) * 0.20)
	high := int64(float64(input) * 0.85)
	if got < low || got > high {
		t.Errorf("CalculateReturnBytes(1GB) = %d, want value in [%d, %d]", got, low, high)
	}
}

// ---------------------------------------------------------------------------
// loadValues / loadFloat helpers
// ---------------------------------------------------------------------------

func TestLoadValues_AllLoadTypes(t *testing.T) {
	cases := []struct {
		loadID int
		want   string
	}{
		{LoadTypeMostlyReads, "A"},
		{LoadTypeSomeWrites, "B"},
		{LoadTypeEqualReadsWrites, "C"},
		{LoadTypeHeavyWrites, "D"},
		{99, "B"}, // unknown ID falls back to index 1 (SomeWrites)
	}
	for _, tc := range cases {
		c := newTestConfigurator(tc.loadID, DbTypePXC, 50, 1200, 4*testGB)
		got := c.loadValues([4]string{"A", "B", "C", "D"})
		if got != tc.want {
			t.Errorf("loadID=%d: loadValues() = %q, want %q", tc.loadID, got, tc.want)
		}
	}
}

func TestLoadFloat_AllLoadTypes(t *testing.T) {
	cases := []struct {
		loadID int
		want   float64
	}{
		{LoadTypeMostlyReads, 1.0},
		{LoadTypeSomeWrites, 2.0},
		{LoadTypeEqualReadsWrites, 3.0},
		{LoadTypeHeavyWrites, 4.0},
		{99, 1.0}, // unknown ID falls back to index 0 (MostlyReads)
	}
	for _, tc := range cases {
		c := newTestConfigurator(tc.loadID, DbTypePXC, 50, 1200, 4*testGB)
		got := c.loadFloat([4]float64{1.0, 2.0, 3.0, 4.0})
		if got != tc.want {
			t.Errorf("loadID=%d: loadFloat() = %f, want %f", tc.loadID, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// getGcacheLoad
// ---------------------------------------------------------------------------

func TestGetGcacheLoad(t *testing.T) {
	cases := []struct {
		loadID int
		want   float64
	}{
		{LoadTypeMostlyReads, 1.0},
		{LoadTypeSomeWrites, 1.15},
		{LoadTypeEqualReadsWrites, 1.2},
		{LoadTypeHeavyWrites, 1.0},
	}
	for _, tc := range cases {
		c := newTestConfigurator(tc.loadID, DbTypePXC, 50, 1200, 4*testGB)
		got := c.getGcacheLoad()
		if got != tc.want {
			t.Errorf("loadID=%d: getGcacheLoad() = %f, want %f", tc.loadID, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// calculateTmpTableFootprint
// ---------------------------------------------------------------------------

func TestCalculateTmpTableFootprint(t *testing.T) {
	base := int64(testMB) // 1 MB tmp_table_size
	cases := []struct {
		loadID   int
		wantMult float64
	}{
		{LoadTypeMostlyReads, 0.2},
		{LoadTypeSomeWrites, 0.1},
		{LoadTypeEqualReadsWrites, 0.3},
		{LoadTypeHeavyWrites, 0.05},
	}
	for _, tc := range cases {
		c := newTestConfigurator(tc.loadID, DbTypePXC, 50, 1200, 4*testGB)
		p := Parameter{Value: strconv.FormatInt(base, 10)}
		c.calculateTmpTableFootprint(p)
		want := int64(float64(base) * tc.wantMult)
		if c.reference.tmpTableFootprint != want {
			t.Errorf("loadID=%d: tmpTableFootprint = %d, want %d", tc.loadID, c.reference.tmpTableFootprint, want)
		}
	}
}

// ---------------------------------------------------------------------------
// Connection buffer parameter values
// ---------------------------------------------------------------------------

func TestParamConnectionBuffers(t *testing.T) {
	cases := []struct {
		loadID      int
		wantJoin    string
		wantSort    string
		wantReadRnd string
		wantBinlog  string
	}{
		{LoadTypeMostlyReads, "262144", "262144", "262144", "32768"},
		{LoadTypeSomeWrites, "524288", "524288", "393216", "131072"},
		{LoadTypeEqualReadsWrites, "1048576", "1572864", "707788", "262144"},
		{LoadTypeHeavyWrites, "1048576", "2097152", "707788", "358400"},
	}
	for _, tc := range cases {
		c := newTestConfigurator(tc.loadID, DbTypePXC, 50, 1200, 4*testGB)
		p := Parameter{}

		if got := c.paramJoinBuffer(p).Value; got != tc.wantJoin {
			t.Errorf("loadID=%d paramJoinBuffer: got %s, want %s", tc.loadID, got, tc.wantJoin)
		}
		if got := c.paramSortBuffer(p).Value; got != tc.wantSort {
			t.Errorf("loadID=%d paramSortBuffer: got %s, want %s", tc.loadID, got, tc.wantSort)
		}
		if got := c.paramReadRndBuffer(p).Value; got != tc.wantReadRnd {
			t.Errorf("loadID=%d paramReadRndBuffer: got %s, want %s", tc.loadID, got, tc.wantReadRnd)
		}
		if got := c.paramBinlogCacheSize(p).Value; got != tc.wantBinlog {
			t.Errorf("loadID=%d paramBinlogCacheSize: got %s, want %s", tc.loadID, got, tc.wantBinlog)
		}
	}
}

func TestParamInnoDBIOCapacityMax(t *testing.T) {
	cases := []struct {
		loadID int
		want   string
	}{
		{LoadTypeMostlyReads, "28000"},
		{LoadTypeSomeWrites, "24000"},
		{LoadTypeEqualReadsWrites, "20000"},
		{LoadTypeHeavyWrites, "20000"},
	}
	for _, tc := range cases {
		c := newTestConfigurator(tc.loadID, DbTypePXC, 50, 1200, 4*testGB)
		got := c.paramInnoDBIOCapacityMax(Parameter{}).Value
		if got != tc.want {
			t.Errorf("loadID=%d paramInnoDBIOCapacityMax: got %s, want %s", tc.loadID, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// calculateLoadConnectionFactor
// ---------------------------------------------------------------------------

func TestCalculateLoadConnectionFactor(t *testing.T) {
	cases := []struct {
		name        string
		loadID      int
		mysqlCPU    int
		connections int
		wantOver    bool
	}{
		{"reads well within capacity", LoadTypeMostlyReads, 4000, 50, false},
		{"reads at comfortable limit", LoadTypeMostlyReads, 4000, 3000, false},
		{"heavy writes over limit", LoadTypeHeavyWrites, 1000, 5000, true},
		{"light writes over limit", LoadTypeSomeWrites, 600, 1000, true},
		{"equal reads-writes within limit", LoadTypeEqualReadsWrites, 8000, 200, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dim := Dimension{MysqlCpu: tc.mysqlCPU}
			c := &Configurator{
				reference: &references{
					loadID:      tc.loadID,
					connections: tc.connections,
				},
			}
			_, _, over := c.calculateLoadConnectionFactor(dim, ResponseMessage{})
			if over != tc.wantOver {
				t.Errorf("over=%v, want %v", over, tc.wantOver)
			}
		})
	}
}

func TestCalculateLoadConnectionFactor_ReturnsPositiveFactor(t *testing.T) {
	dim := Dimension{MysqlCpu: 4000}
	c := &Configurator{
		reference: &references{
			loadID:      LoadTypeMostlyReads,
			connections: 100,
		},
	}
	factor, _, over := c.calculateLoadConnectionFactor(dim, ResponseMessage{})
	if over {
		t.Fatal("expected not over, got over")
	}
	if factor <= 0 || factor > 1 {
		t.Errorf("loadFactor = %f, want value in (0, 1]", factor)
	}
}

// ---------------------------------------------------------------------------
// paramInnoDBBufferPool — first pass
// ---------------------------------------------------------------------------

func TestParamInnoDBBufferPool_FirstPass_PXC(t *testing.T) {
	memLeftover := int64(8 * testGB)
	c := &Configurator{
		request:   ConfigurationRequest{DBType: DbTypePXC},
		reference: &references{memoryLeftover: memLeftover},
	}
	result := c.paramInnoDBBufferPool(Parameter{}, false)
	want := int64(float64(memLeftover) * InnoDBPctValuePXC)
	got, _ := strconv.ParseInt(result.Value, 10, 64)
	if got != want {
		t.Errorf("PXC first pass bufferPool = %d, want %d", got, want)
	}
	if c.reference.innoDBbpSize != want {
		t.Errorf("PXC first pass innoDBbpSize = %d, want %d", c.reference.innoDBbpSize, want)
	}
	// leftover should be reduced by the allocated amount
	if c.reference.memoryLeftover != memLeftover-want {
		t.Errorf("PXC first pass memoryLeftover = %d, want %d", c.reference.memoryLeftover, memLeftover-want)
	}
}

func TestParamInnoDBBufferPool_FirstPass_GR(t *testing.T) {
	memLeftover := int64(8 * testGB)
	c := &Configurator{
		request:   ConfigurationRequest{DBType: DbTypeGroupReplication},
		reference: &references{memoryLeftover: memLeftover},
	}
	result := c.paramInnoDBBufferPool(Parameter{}, false)
	want := int64(float64(memLeftover) * InnoDBPctValueGR)
	got, _ := strconv.ParseInt(result.Value, 10, 64)
	if got != want {
		t.Errorf("GR first pass bufferPool = %d, want %d", got, want)
	}
}

// ---------------------------------------------------------------------------
// paramInnoDBBufferPool — second pass, positive leftover
// ---------------------------------------------------------------------------

func TestParamInnoDBBufferPool_SecondPass_PositiveLeftover(t *testing.T) {
	bpSize := int64(5 * testGB)
	leftover := int64(500 * testMB)
	c := &Configurator{
		request: ConfigurationRequest{DBType: DbTypePXC},
		reference: &references{
			memoryMySQL:    float64(8 * testGB),
			innoDBbpSize:   bpSize,
			memoryLeftover: leftover,
		},
	}
	result := c.paramInnoDBBufferPool(Parameter{}, true)
	got, _ := strconv.ParseInt(result.Value, 10, 64)
	// Buffer pool must be larger than the initial size (leftover was reassigned)
	if got <= bpSize {
		t.Errorf("second pass positive leftover: bufferPool %d should be > initial %d", got, bpSize)
	}
}

// ---------------------------------------------------------------------------
// paramInnoDBBufferPool — second pass, negative leftover with floor guard
// ---------------------------------------------------------------------------

func TestParamInnoDBBufferPool_SecondPass_NegativeLeftover_FloorEnforced_PXC(t *testing.T) {
	totalMySQLMem := float64(8 * testGB)
	// Set a buffer pool so small that subtracting the overrun would go below MinLimitPXC
	c := &Configurator{
		request: ConfigurationRequest{DBType: DbTypePXC},
		reference: &references{
			memoryMySQL:    totalMySQLMem,
			innoDBbpSize:   int64(2 * testGB),   // very small initial BP
			memoryLeftover: int64(-3 * testGB),  // large overrun
		},
	}
	result := c.paramInnoDBBufferPool(Parameter{}, true)
	got, _ := strconv.ParseInt(result.Value, 10, 64)
	minAllowed := int64(totalMySQLMem * MinLimitPXC)
	if got < minAllowed {
		t.Errorf("PXC bufferPool %d is below minimum floor %d", got, minAllowed)
	}
}

func TestParamInnoDBBufferPool_SecondPass_NegativeLeftover_FloorEnforced_GR(t *testing.T) {
	totalMySQLMem := float64(8 * testGB)
	c := &Configurator{
		request: ConfigurationRequest{DBType: DbTypeGroupReplication},
		reference: &references{
			memoryMySQL:    totalMySQLMem,
			innoDBbpSize:   int64(2 * testGB),
			memoryLeftover: int64(-3 * testGB),
		},
	}
	result := c.paramInnoDBBufferPool(Parameter{}, true)
	got, _ := strconv.ParseInt(result.Value, 10, 64)
	minAllowed := int64(totalMySQLMem * MinLimitGR)
	if got < minAllowed {
		t.Errorf("GR bufferPool %d is below minimum floor %d", got, minAllowed)
	}
}

// ---------------------------------------------------------------------------
// FillResponseMessage — saturation classification thresholds
// ---------------------------------------------------------------------------

func TestFillResponseMessage(t *testing.T) {
	cases := []struct {
		name     string
		bpPct    float64
		dbtype   string
		wantType int
	}{
		// PXC: MinLimitPXC=0.50, close-to-limit band [0.50, 0.60]
		{"PXC below min", 0.35, DbTypePXC, OverutilizingI},
		{"PXC at min", 0.50, DbTypePXC, ClosetolimitI},
		{"PXC close to limit", 0.55, DbTypePXC, ClosetolimitI},
		{"PXC ok", 0.70, DbTypePXC, OkI},

		// GR: MinLimitGR=0.40, close-to-limit band [0.40, 0.50]
		{"GR below min", 0.30, DbTypeGroupReplication, OverutilizingI},
		{"GR at min", 0.40, DbTypeGroupReplication, ClosetolimitI},
		{"GR close to limit", 0.45, DbTypeGroupReplication, ClosetolimitI},
		{"GR ok", 0.60, DbTypeGroupReplication, OkI},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := &Configurator{reference: &references{}}
			msg, _ := c.FillResponseMessage(tc.bpPct, ResponseMessage{}, bytes.Buffer{}, tc.dbtype)
			if msg.MType != tc.wantType {
				t.Errorf("bpPct=%.2f dbtype=%s: MType=%d, want %d", tc.bpPct, tc.dbtype, msg.MType, tc.wantType)
			}
		})
	}
}

func TestFillResponseMessage_OverutilizingReturnsTrue(t *testing.T) {
	c := &Configurator{reference: &references{}}
	_, over := c.FillResponseMessage(0.20, ResponseMessage{}, bytes.Buffer{}, DbTypePXC)
	if !over {
		t.Error("expected overUtilizing=true for bpPct=0.20 on PXC")
	}
}

func TestFillResponseMessage_OkReturnsFalse(t *testing.T) {
	c := &Configurator{reference: &references{}}
	_, over := c.FillResponseMessage(0.75, ResponseMessage{}, bytes.Buffer{}, DbTypePXC)
	if over {
		t.Error("expected overUtilizing=false for bpPct=0.75 on PXC")
	}
}
