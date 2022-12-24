package Objects

type Configuration struct {
	Dimension   []Dimension `json:"dimension"`
	LoadType    []LoadType  `json:"loadtype"`
	Connections []int       `json:"connections"`
}

type Dimension struct {
	Id     int    `json:"id"`
	Name   string `json:"name"`
	Cpu    int    `json:"cpu"`
	memory int    `json:"memory"`
}

type LoadType struct {
	Id      int    `json:"id"`
	Name    string `json:"name"`
	Example string `json:"example"`
}

func (conf *Configuration) Init() {

	conf.Dimension = []Dimension{
		{1, "XSmall", 1000, 2},
		{2, "Small", 1500, 4},
		{3, "Medium", 2500, 8},
		{4, "Large", 4500, 16},
		{5, "XLarge", 8500, 32},
	}

	conf.LoadType = []LoadType{}

	loadT := make(map[string]int)
	loadT["Mainly Reads"] = 1
	loadT["Light OLTP"] = 2
	loadT["Intense OLTP (50/50 R/W)"] = 3

	conf.LoadType = []LoadType{
		{1, "Mainly Reads", "Blogs ~1-2% Writes 95% Reads"},
		{2, "Light OLTP", "Shops online ~< 20% Writes "},
		{3, "Heavy OLTP", "Intense analitics, telphony, gaming. 50/50% Reads and Writes"},
	}

	conf.Connections = []int{50, 100, 200, 500, 1000, 2000}
}
