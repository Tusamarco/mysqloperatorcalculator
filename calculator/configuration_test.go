package calculator

import "testing"

// ---------------------------------------------------------------------------
// Configuration.GetDimensionByID
// ---------------------------------------------------------------------------

func TestGetDimensionByID_Valid(t *testing.T) {
	var conf Configuration
	conf.Init()

	cases := []struct {
		id      int
		wantCPU int
	}{
		{1, 1000},  // XSmall
		{2, 2500},  // Small
		{3, 4500},  // Medium
		{10, 96000}, // 24XLarge
	}
	for _, tc := range cases {
		dim := conf.GetDimensionByID(tc.id)
		if dim.ID != tc.id {
			t.Errorf("GetDimensionByID(%d).Id = %d, want %d", tc.id, dim.ID, tc.id)
		}
		if dim.Cpu != tc.wantCPU {
			t.Errorf("GetDimensionByID(%d).Cpu = %d, want %d", tc.id, dim.Cpu, tc.wantCPU)
		}
		if dim.MySQLCPU == 0 {
			t.Errorf("GetDimensionByID(%d).MysqlCpu not set", tc.id)
		}
		if dim.MySQLMemory == 0 {
			t.Errorf("GetDimensionByID(%d).MysqlMemory not set", tc.id)
		}
	}
}

func TestGetDimensionByID_Invalid(t *testing.T) {
	var conf Configuration
	conf.Init()
	dim := conf.GetDimensionByID(99999)
	if dim.ID != 0 {
		t.Errorf("expected zero Dimension for unknown ID, got Id=%d", dim.ID)
	}
}

func TestGetDimensionByID_ResourceSplit(t *testing.T) {
	var conf Configuration
	conf.Init()
	// Each component must be positive and the sum must not exceed the total CPU.
	// Some dimensions intentionally reserve a portion of CPU as node overhead.
	for _, dim := range conf.Dimension {
		if dim.ID == DimensionOpen || dim.ID == ConnectionDimension {
			continue
		}
		total := dim.MySQLCPU + dim.ProxyCPU + dim.PMMCPU
		if total > dim.Cpu {
			t.Errorf("dim %d (%s): CPU split %d+%d+%d=%d exceeds total %d",
				dim.ID, dim.Name, dim.MySQLCPU, dim.ProxyCPU, dim.PMMCPU, total, dim.Cpu)
		}
		if dim.MySQLCPU <= 0 || dim.ProxyCPU <= 0 || dim.PMMCPU <= 0 {
			t.Errorf("dim %d (%s): all CPU components must be > 0, got MySQL=%d Proxy=%d PMM=%d",
				dim.ID, dim.Name, dim.MySQLCPU, dim.ProxyCPU, dim.PMMCPU)
		}
	}
}

func TestGetDimensionByID_MemoryBytes(t *testing.T) {
	var conf Configuration
	conf.Init()
	for _, dim := range conf.Dimension {
		if dim.ID == DimensionOpen || dim.ID == ConnectionDimension {
			continue
		}
		if dim.MemoryBytes == 0 {
			t.Errorf("dim %d (%s): MemoryBytes is 0", dim.ID, dim.Name)
		}
		if dim.MySQLMemory == 0 {
			t.Errorf("dim %d (%s): MysqlMemory is 0", dim.ID, dim.Name)
		}
	}
}

// ---------------------------------------------------------------------------
// Configuration.GetLoadByID
// ---------------------------------------------------------------------------

func TestGetLoadByID_Valid(t *testing.T) {
	var conf Configuration
	conf.Init()
	for _, id := range []int{LoadTypeMostlyReads, LoadTypeSomeWrites, LoadTypeEqualReadsWrites, LoadTypeHeavyWrites} {
		load := conf.GetLoadByID(id)
		if load.ID != id {
			t.Errorf("GetLoadByID(%d) returned Id=%d", id, load.ID)
		}
		if load.Name == "" {
			t.Errorf("GetLoadByID(%d) has empty Name", id)
		}
	}
}

func TestGetLoadByID_Invalid(t *testing.T) {
	var conf Configuration
	conf.Init()
	load := conf.GetLoadByID(9999)
	if load.ID != 0 {
		t.Errorf("expected zero LoadType for unknown ID, got Id=%d", load.ID)
	}
}

// ---------------------------------------------------------------------------
// Dimension.ConvertMemoryToBytes
// ---------------------------------------------------------------------------

func TestConvertMemoryToBytes(t *testing.T) {
	cases := []struct {
		input string
		want  float64
	}{
		{"2GB", 2 * 1024 * 1024 * 1024},
		{"4GB", 4 * 1024 * 1024 * 1024},
		{"512MB", 512 * 1024 * 1024},
	}
	for _, tc := range cases {
		var d Dimension
		got, err := d.ConvertMemoryToBytes(tc.input)
		if err != nil {
			t.Errorf("ConvertMemoryToBytes(%q) unexpected error: %v", tc.input, err)
		}
		if got != tc.want {
			t.Errorf("ConvertMemoryToBytes(%q) = %f, want %f", tc.input, got, tc.want)
		}
	}
}

func TestConvertMemoryToBytes_Invalid(t *testing.T) {
	var d Dimension
	_, err := d.ConvertMemoryToBytes("not-a-size")
	if err == nil {
		t.Error("expected error for invalid memory string, got nil")
	}
}
