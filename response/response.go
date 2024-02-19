package response

type ServerResponse struct {
	SubscriberUID string `json:"subscriber_uid"`
	Location      string `json:"location"`
	MacAddress    string `json:"mac_address"`
	Role          string `json:"role"`
	UpstreamQoS   string `json:"upstream_qos"`
	DownstreamQoS string `json:"downstream_qos"`
	Hostname      string `json:"hostname"`
}
