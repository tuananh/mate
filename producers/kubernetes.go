package producers

import (
	"errors"
	"fmt"
	"net/url"
	"sync"

	log "github.com/Sirupsen/logrus"
	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/zalando-incubator/mate/pkg"
)

const (
	annotationKey = "zalando.org/dnsname"
)

var kubernetesParams struct {
	project string
	zone    string

	kubeServer     *url.URL
	format         string
	trackNodePorts bool
}

type kubernetesProducer struct {
	ingress   Producer
	service   Producer
	nodePorts Producer
}

func init() {
	kingpin.Flag("kubernetes-server", "The address of the Kubernetes API server.").URLVar(&kubernetesParams.kubeServer)
	kingpin.Flag("kubernetes-format", "Format of DNS entries, e.g. {{.Name}}-{{.Namespace}}.example.com").StringVar(&kubernetesParams.format)
	kingpin.Flag("kubernetes-track-node-ports", "When true, generates DNS entries for type=NodePort services").BoolVar(&kubernetesParams.trackNodePorts)
}

func NewKubernetes() (*kubernetesProducer, error) {
	if kubernetesParams.format == "" {
		return nil, errors.New("Please provide --kubernetes-format")
	}

	if kubernetesParams.trackNodePorts {
		log.Infof("Please note, creating DNS entries for NodePort services doesn't currently work in combination with the AWS consumer.")
	}

	var err error

	producer := &kubernetesProducer{}

	producer.ingress, err = NewKubernetesIngress()
	if err != nil {
		return nil, fmt.Errorf("[Kubernetes] Error creating producer: %v", err)
	}

	producer.service, err = NewKubernetesService()
	if err != nil {
		return nil, fmt.Errorf("[Kubernetes] Error creating producer: %v", err)
	}

	if kubernetesParams.trackNodePorts {
		producer.nodePorts, err = NewKubernetesNodePorts()
	} else {
		producer.nodePorts, err = NewNull()
	}
	if err != nil {
		return nil, fmt.Errorf("[Kubernetes] Error creating producer: %v", err)
	}

	return producer, nil
}

func (a *kubernetesProducer) Endpoints() ([]*pkg.Endpoint, error) {
	ingressEndpoints, err := a.ingress.Endpoints()
	if err != nil {
		return nil, fmt.Errorf("[Kubernetes] Error getting endpoints from producer: %v", err)
	}

	serviceEndpoints, err := a.service.Endpoints()
	if err != nil {
		return nil, fmt.Errorf("[Kubernetes] Error getting endpoints from producer: %v", err)
	}

	nodePortsEndpoints, err := a.nodePorts.Endpoints()
	if err != nil {
		return nil, fmt.Errorf("[Kubernetes] Error getting endpoints from producer: %v", err)
	}

	ingressEndpoints = append(ingressEndpoints, serviceEndpoints...)
	return append(ingressEndpoints, nodePortsEndpoints...), nil
}

func (a *kubernetesProducer) Monitor(results chan *pkg.Endpoint, errChan chan error, done chan struct{}, wg *sync.WaitGroup) {
	wg.Add(1)
	defer wg.Done()

	go a.ingress.Monitor(results, errChan, done, wg)
	go a.service.Monitor(results, errChan, done, wg)
	go a.nodePorts.Monitor(results, errChan, done, wg)

	<-done
	log.Info("[Kubernetes] Exited monitoring loop.")
}
