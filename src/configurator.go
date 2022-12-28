package main

import (
	"bytes"
	"fmt"
	o "pxccalculator/src/Objects"
	"strconv"
)

type Configurator struct {
	request        o.ConfigurationRequest
	families       map[string]o.Family
	providerParams map[string]providerParam
}

type references struct {
	memory            int64
	cpus              int
	gcache            int64
	gcacheFootprint   int64
	gcacheLoad        int
	memoryAvailable   int64
	memoryLeftover    int64
	innodbRedoLogDim  int64
	loadAdjustment    int
	loadAdjustmentMax int
	loadFactor        float32
}

type providerParam struct {
	name     string
	literal  string
	value    int64
	defvalue int64
	rMin     int64
	rMax     int64
}

func (pP *providerParam) init() map[string]providerParam {

	pMap := map[string]providerParam{
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

func (c *Configurator) GetAllOptionsAsString() bytes.Buffer {
	var b bytes.Buffer
	b.WriteString(`"`)

	for key, param := range c.providerParams {
		b.WriteString(key)
		b.WriteString(`=`)
		if param.value >= 0 {
			b.WriteString(fmt.Sprintf(param.literal, strconv.FormatInt(param.value, 10)))
		} else {
			b.WriteString(param.literal)
		}
		b.WriteString(";")
	}
	b.WriteString(`"`)
	return b
}

func (c *Configurator) init(r o.ConfigurationRequest, fam map[string]o.Family) {
	var p providerParam
	c.families = fam
	c.request = r
	c.providerParams = p.init()
}

func (c *Configurator) checkValidity() bool {

	return false
}

func (c *Configurator) ProcessRequest() map[string]o.Family {

	b := c.GetAllOptionsAsString()

	print(b.String())

	return c.families

}
