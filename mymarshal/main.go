package main

import (
	"encoding/json"
	"fmt"
)

type HealthCheckConfig struct {
	HttpPath            string `json:"httpPath"`
	Timeout             int    `json:"timeout"`
	HealthCheckInterval int    `json:"healthCheckInterval"`
	HealthySuccesses    int    `json:"healthySuccesses"`
	UnhealthyFailures   int    `json:"unhealthyFailures"`
}

func (hc *HealthCheckConfig) UnmarshalJSON(data []byte) error {
	type Temp HealthCheckConfig
	var tmp Temp
	if err := json.Unmarshal(data, &tmp); err == nil {
		*hc = HealthCheckConfig(tmp)
		return nil
	}
	return nil
}

type HS struct {
	Name string            `json:"name"`
	Hcs  HealthCheckConfig `json:"hcs"`
}

func main() {
	//hc0 := HS{
	//	Name: "Jack",
	//	Hcs: HealthCheckConfig{
	//		HttpPath: "/a/b/c",
	//		Timeout:  10,
	//	},
	//}
	//bs, _ := json.Marshal(hc0)
	//fmt.Println(string(bs))
	bs := []byte("{\"name\":\"Jack\",\"hcs\":{\"httpPath\":\"/a/b/c\",\"timeout\":\"10\",\"healthCheckInterval\":0,\"healthySuccesses\":0,\"unhealthyFailures\":0}}")

	var hs HS
	err := json.Unmarshal(bs, &hs)

	fmt.Println(err, hs)
}
