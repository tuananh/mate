package consumers

import (
	"testing"

	"github.bus.zalan.do/teapot/mate/awsclient/awsclienttest"
	"github.bus.zalan.do/teapot/mate/pkg"
	"github.com/aws/aws-sdk-go/service/route53"
)

type awsTestItem struct {
	msg          string
	init         map[string]string
	initAlias    map[string]string
	sync         []*pkg.Endpoint
	process      *pkg.Endpoint
	fail         bool
	expectUpsert []*pkg.Endpoint
	expectDelete []*pkg.Endpoint
	expectFail   bool
}

func checkTestError(t *testing.T, err error, expect bool) bool {
	if err == nil && expect {
		t.Error("failed to fail")
		return false
	}

	if err != nil && !expect {
		t.Error("unexpected error", err)
		return false
	}

	return true
}

func checkEndpointSlices(got []*route53.ResourceRecordSet, expect []*pkg.Endpoint) bool {
	for _, ep := range got {
		if *ep.Type != "A" {
			continue
		}
		var found bool
		for _, eep := range expect {
			if *ep.Name == pkg.SanitizeDNSName(eep.DNSName) && *ep.AliasTarget.DNSName == eep.Hostname {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

func testAWSConsumer(t *testing.T, ti awsTestItem) {
	client := &awsclienttest.Client{Records: ti.init, AliasRecords: ti.initAlias, Options: awsclienttest.Options{
		HostedZone:   "test",
		RecordSetTTL: 10,
		GroupID:      "test",
	}}
	if ti.fail {
		client.FailNext()
	}

	if client.Records == nil {
		client.Records = make(map[string]string)
	}

	consumer := withClient(client)

	if ti.process == nil {
		err := consumer.Sync(ti.sync)
		if !checkTestError(t, err, ti.expectFail) {
			return
		}
	} else {
		err := consumer.Process(ti.process)
		if !checkTestError(t, err, ti.expectFail) {
			return
		}
	}

	if !checkEndpointSlices(client.LastUpsert, ti.expectUpsert) {
		t.Error("failed to post the right upsert items", client.LastUpsert, ti.expectUpsert)
	}

	if !checkEndpointSlices(client.LastDelete, ti.expectDelete) {
		t.Error("failed to post the right delete items", client.LastDelete, ti.expectDelete)
	}
}

func TestAWSConsumer(t *testing.T) { //exclude IP endpoints for now only Alias works
	for _, ti := range []awsTestItem{{
		msg: "no initial, no change",
	}, {
		msg: "no initial, sync new ones",
		sync: []*pkg.Endpoint{{
			"bar.org", "", "abc.def.ghi",
		}},
		expectUpsert: []*pkg.Endpoint{{
			"bar.org", "", "abc.def.ghi",
		}},
	}, {
		msg: "sync delete all",
		init: map[string]string{
			"foo.org": "1.2.3.4",
		},
		initAlias: map[string]string{
			"bar.org": "abc.def.ghi",
		},
		expectDelete: []*pkg.Endpoint{{
			"foo.org", "1.2.3.4", "",
		}, {
			"bar.org", "", "abc.def.ghi",
		}},
	}, {
		msg: "insert, update, delete, leave",
		initAlias: map[string]string{
			"foo.org": "foo.elb",
			"baz.org": "baz.elb",
			"bar.org": "abc.def.ghi",
		},
		sync: []*pkg.Endpoint{{
			"qux.org", "", "qux.elb",
		}, {
			"foo.org", "", "foo.elb2",
		}, {
			"baz.org", "", "baz.elb2",
		}},
		expectUpsert: []*pkg.Endpoint{{
			"qux.org", "", "qux.elb",
		}, {
			"foo.org", "", "foo.elb2",
		}, {
			"baz.org", "", "baz.elb2",
		}},
		expectDelete: []*pkg.Endpoint{{
			"bar.org", "", "abc.def.ghi",
		}},
	}, {
		msg: "fail on list",
		init: map[string]string{
			"foo.org": "1.2.3.4",
			"bar.org": "5.6.7.8",
		},
		sync: []*pkg.Endpoint{{
			"baz.org", "9.0.1.2", "",
		}},
		fail:       true,
		expectFail: true,
	}, {
		msg: "fail on change",
		init: map[string]string{
			"foo.org": "1.2.3.4",
			"bar.org": "5.6.7.8",
		},
		sync: []*pkg.Endpoint{{
			"baz.org", "9.0.1.2", "",
		}},
		fail:       true,
		expectFail: true,
	}, {
		msg: "process existing",
		init: map[string]string{
			"foo.org": "1.2.3.4",
			"bar.org": "5.6.7.8",
		},
		process:      &pkg.Endpoint{DNSName: "foo.org", IP: "2.3.4.5"},
		expectUpsert: []*pkg.Endpoint{{DNSName: "foo.org", IP: "2.3.4.5"}},
	}, {
		msg: "process new",
		init: map[string]string{
			"foo.org": "1.2.3.4",
			"bar.org": "5.6.7.8",
		},
		process:      &pkg.Endpoint{DNSName: "baz.org", IP: "9.0.1.2"},
		expectUpsert: []*pkg.Endpoint{{DNSName: "baz.org", IP: "9.0.1.2"}},
	}, {
		msg: "fail on process",
		init: map[string]string{
			"foo.org": "1.2.3.4",
			"bar.org": "5.6.7.8",
		},
		process:    &pkg.Endpoint{DNSName: "foo.org", IP: "2.3.4.5"},
		fail:       true,
		expectFail: true,
	},
	} {
		t.Run(ti.msg, func(t *testing.T) {
			testAWSConsumer(t, ti)
		})
	}
}
