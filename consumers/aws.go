package consumers

import "github.bus.zalan.do/teapot/mate/pkg"

// Implementations provide access to AWS Route53 API's
// required calls.
type AwsClient interface {
	ListRecordSets() ([]*pkg.Endpoint, error)
	ChangeRecordSets(upsert, del []*pkg.Endpoint) error
}

type aws struct {
	client AwsClient
}

// NewAWS reates a Consumer instance to sync and process DNS
// entries in AWS Route53.
func NewAWS(c AwsClient) Consumer {
	return &aws{c}
}

func (a *aws) Sync(endpoints []*pkg.Endpoint) error {
	current, err := a.client.ListRecordSets()
	if err != nil {
		return err
	}

	var upsert, del []*pkg.Endpoint

	for _, ep := range endpoints {
		if needsUpsert(ep, current) {
			upsert = append(upsert, ep)
		}
	}

	for _, ep := range current {
		if needsDelete(ep, endpoints) {
			del = append(del, ep)
		}
	}

	if len(upsert) > 0 || len(del) > 0 {
		return a.client.ChangeRecordSets(upsert, del)
	}

	return nil
}

func (a *aws) Process(*pkg.Endpoint) error { return nil }

func needsUpsert(ep *pkg.Endpoint, currentEndpoints []*pkg.Endpoint) bool {
	for _, cep := range currentEndpoints {
		if cep.DNSName == ep.DNSName {
			return cep.IP != ep.IP
		}
	}

	return true
}

func needsDelete(ep *pkg.Endpoint, nextEndpoints []*pkg.Endpoint) bool {
	for _, nep := range nextEndpoints {
		if nep.DNSName == ep.DNSName {
			return false
		}
	}

	return true
}
